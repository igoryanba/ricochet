package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/igoryan-dao/ricochet/internal/checkpoints"
	"github.com/igoryan-dao/ricochet/internal/codegraph"
	"github.com/igoryan-dao/ricochet/internal/config"
	context_manager "github.com/igoryan-dao/ricochet/internal/context"
	"github.com/igoryan-dao/ricochet/internal/context/handoff"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/index"
	mcpHubPkg "github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/paths"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/qc"
	"github.com/igoryan-dao/ricochet/internal/rules"
	"github.com/igoryan-dao/ricochet/internal/safeguard"
	"github.com/igoryan-dao/ricochet/internal/skills"
	"github.com/igoryan-dao/ricochet/internal/tools"
	"github.com/igoryan-dao/ricochet/internal/workflow"
)

// Controller manages chat sessions and AI interactions
type Controller struct {
	mu                sync.RWMutex
	provider          Provider
	sessionManager    *SessionManager
	config            *Config
	executor          tools.Executor
	envTracker        *context_manager.EnvironmentTracker
	safeguard         *safeguard.Manager
	modes             *modes.Manager
	rules             *rules.Manager
	host              host.Host
	checkpointService *checkpoints.CheckpointService
	providersManager  *config.ProvidersManager
	indexer           *index.Indexer

	codegraph      *codegraph.Service
	handoffService *handoff.Service
	workflows      *workflow.Manager
	skills         *skills.Manager
	qcManager      *qc.Manager

	// Abort support
	abortMu     sync.Mutex
	abortCancel context.CancelFunc
}

// Config holds agent configuration
type Config struct {
	Provider          ProviderConfig               `json:"provider"`
	EmbeddingProvider *ProviderConfig              `json:"embedding_provider,omitempty"`
	SystemPrompt      string                       `json:"system_prompt"`
	MaxTokens         int                          `json:"max_tokens"`     // Max tokens for response generation
	ContextWindow     int                          `json:"context_window"` // Context window limit for pruning
	EnableCodeIndex   bool                         `json:"enable_code_index"`
	AutoApproval      *config.AutoApprovalSettings `json:"auto_approval"`
}

// Session represents a chat session
type Session struct {
	ID           string                       `json:"id"`
	StateHandler *MessageStateHandler         `json:"-"` // Internal state handler
	FileTracker  *context_manager.FileTracker `json:"-"` // Tracks accessed files
	Todos        []protocol.Todo              `json:"todos"`
	TotalCost    float64                      `json:"total_cost"`
	CreatedAt    time.Time                    `json:"created_at"`
}

// ControllerOptions allows overriding default components
type ControllerOptions struct {
	Host             host.Host
	Modes            *modes.Manager
	Rules            *rules.Manager
	McpHub           *mcpHubPkg.Hub
	ProvidersManager *config.ProvidersManager
	Codegraph        *codegraph.Service
	WorkflowManager  *workflow.Manager
}

// NewController creates a new agent controller
func NewController(cfg *Config, opts ...ControllerOptions) (*Controller, error) {
	provider, err := NewProvider(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	cwd, _ := os.Getwd()

	var h host.Host
	var mm *modes.Manager
	var rm *rules.Manager
	var mcpHub *mcpHubPkg.Hub
	var pm *config.ProvidersManager
	var cg *codegraph.Service
	var wm *workflow.Manager

	if len(opts) > 0 {
		h = opts[0].Host
		mm = opts[0].Modes
		rm = opts[0].Rules
		mcpHub = opts[0].McpHub
		pm = opts[0].ProvidersManager
		cg = opts[0].Codegraph
		wm = opts[0].WorkflowManager
	}

	if h == nil {
		h = host.NewNativeHost(cwd)
	}
	if mm == nil {
		mm = modes.NewManager(cwd)
	}
	if rm == nil {
		rm = rules.NewManager(cwd)
	}
	// mcpHub can be nil
	// cg (codegraph) can be nil (feature disabled or not provided)
	if wm == nil {
		wm = workflow.NewManager(cwd)
	}

	// Initialize safeguard manager
	safeguardMgr, err := safeguard.NewManager(cwd)
	if err != nil {
		log.Printf("Warning: Failed to initialize safeguard manager: %v", err)
	} else if cfg.AutoApproval != nil {
		safeguardMgr.SetAutoApproval(cfg.AutoApproval)
	}

	// Initialize Embedder
	// If EmbeddingProvider is configured, use it. Otherwise try to use main provider.
	var embedder index.Embedder
	embedder = provider // Default: use main provider (if it implements Embedder)

	if cfg.EmbeddingProvider != nil && cfg.EmbeddingProvider.Provider != "" {
		embProv, err := NewProvider(*cfg.EmbeddingProvider)
		if err != nil {
			log.Printf("Warning: Failed to create embedding provider: %v. Falling back to main provider.", err)
		} else {
			embedder = embProv
			log.Printf("Using separate embedding provider: %s", cfg.EmbeddingProvider.Provider)
		}
	} else if provider.Name() == "anthropic" {
		// Specific warning for Anthropic which doesn't support embeddings
		log.Printf("Warning: Main provider is Anthropic (no embeddings) and no separate embedding provider configured. Codebase search will not work.")
	}

	// Initialize indexer
	indexPath := filepath.Join(os.Getenv("HOME"), ".ricochet", "index.vdb")
	store, _ := index.NewLocalStore(indexPath)
	indexer := index.NewIndexer(store, embedder, cwd)

	// Initialize Skill Manager
	skillMgr := skills.NewManager(cwd)
	if err := skillMgr.LoadSkills(); err != nil {
		log.Printf("Warning: Failed to load skills: %v", err)
	}

	// Initialize QC Manager
	qcMgr := qc.NewManager(cwd)

	executor := tools.NewNativeExecutor(h, mm, safeguardMgr, mcpHub, indexer, cg, wm)

	// Trigger indexing in background
	if cfg.EnableCodeIndex {
		go func() {
			ctx := context.Background()
			if err := indexer.IndexAll(ctx); err != nil {
				log.Printf("Background indexing failed: %v", err)
			}
		}()

		// Also trigger CodeGraph rebuild if available
		if cg != nil {
			go func() {
				start := time.Now()
				log.Printf("Building code graph...")
				if err := cg.Rebuild(cwd); err != nil {
					log.Printf("Code graph rebuild failed: %v", err)
				} else {
					log.Printf("Code graph built in %v (files: %d)", time.Since(start), len(cg.GetAllFiles()))

					// Compute PageRank (takes a few iterations)
					log.Printf("Computing PageRank...")
					prStart := time.Now()
					cg.CalculatePageRank()
					log.Printf("PageRank computed in %v", time.Since(prStart))
				}
			}()
		}
	}

	// Initialize session manager
	storageDir := paths.GetSessionDir(cwd)
	sm := NewSessionManager(storageDir)

	return &Controller{
		provider:         provider,
		sessionManager:   sm,
		config:           cfg,
		executor:         executor,
		envTracker:       context_manager.NewEnvironmentTracker(cwd),
		safeguard:        safeguardMgr,
		modes:            mm,
		rules:            rm,
		host:             h,
		providersManager: pm,
		indexer:          indexer,
		skills:           skillMgr,
		qcManager:        qcMgr,
		handoffService: handoff.NewService(func(ctx context.Context, prompt string) (string, error) {
			req := &ChatRequest{
				Model:     cfg.Provider.Model,
				Messages:  []protocol.Message{{Role: "user", Content: prompt}},
				MaxTokens: 4000,
			}
			resp, err := provider.Chat(ctx, req)
			if err != nil {
				return "", err
			}
			return resp.Content, nil
		}),
	}, nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Safe UTF-8 truncation
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "... (truncated)"
}

// SetLiveMode sets the live mode provider for the executor
func (c *Controller) SetLiveMode(lm tools.LiveModeProvider) {
	if ne, ok := c.executor.(*tools.NativeExecutor); ok {
		ne.SetLiveMode(lm)
	}
}

// AbortCurrentSession cancels any running chat session
func (c *Controller) AbortCurrentSession() {
	c.abortMu.Lock()
	defer c.abortMu.Unlock()
	if c.abortCancel != nil {
		log.Printf("[Controller] Aborting current session...")
		c.abortCancel()
		c.abortCancel = nil
	}
}

// CreateSession creates a new session
func (c *Controller) CreateSession() *Session {
	s := c.sessionManager.CreateSession()
	if c.workflows != nil {
		c.workflows.Hooks.Trigger("on_session_created")
	}
	return s
}

// GetSession returns a session by ID, creating if not exists
func (c *Controller) GetSession(id string) *Session {
	return c.sessionManager.GetSession(id)
}

// ListSessions returns all sessions
func (c *Controller) ListSessions() []*Session {
	return c.sessionManager.ListSessions()
}

// DeleteSession deletes a session
func (c *Controller) DeleteSession(id string) error {
	return c.sessionManager.DeleteSession(id)
}

// ClearSession clears a session's messages
func (c *Controller) ClearSession(id string) {
	c.sessionManager.DeleteSession(id)
	c.sessionManager.CreateSession() // Recreate
}

// ChatRequest represents a request to chat
type ChatRequestInput struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	Via       string `json:"via,omitempty"` // Message source: telegram, discord, ide
}

// ChatUpdate represents a chat update event
type ChatUpdate struct {
	SessionID     string                  `json:"session_id"`
	Message       ChatMessage             `json:"message,omitempty"`
	ContextStatus *protocol.ContextStatus `json:"context_status,omitempty"`
}

// ChatMessage represents a message for the frontend
type ChatMessage struct {
	ID             string         `json:"id"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	Timestamp      int64          `json:"timestamp"`
	IsStreaming    bool           `json:"isStreaming,omitempty"`
	ToolCalls      []ToolCallInfo `json:"toolCalls,omitempty"`
	Activities     []ActivityItem `json:"activities,omitempty"` // Files analyzed, edited, searched
	Steps          []ProgressStep `json:"steps,omitempty"`      // Real-time progress updates
	Metadata       *TaskMetadata  `json:"metadata,omitempty"`
	Via            string         `json:"via,omitempty"`            // Message source: telegram, discord, ide
	Username       string         `json:"username,omitempty"`       // Remote username for Ether messages
	CheckpointHash string         `json:"checkpointHash,omitempty"` // Workspace snapshot hash for restore
}

// ActivityItem represents a file operation (analyze, edit, search)
type ActivityItem struct {
	Type      string `json:"type"`                // search, analyze, edit, command
	File      string `json:"file,omitempty"`      // File path
	LineRange string `json:"lineRange,omitempty"` // "L16-815"
	Results   int    `json:"results,omitempty"`   // for search
	Additions int    `json:"additions,omitempty"` // for edit
	Deletions int    `json:"deletions,omitempty"` // for edit
	Query     string `json:"query,omitempty"`     // for search
}

// TaskMetadata tracks usage statistics
type TaskMetadata struct {
	TokensIn     int     `json:"tokensIn"`
	TokensOut    int     `json:"tokensOut"`
	TotalCost    float64 `json:"totalCost"`
	ContextLimit int     `json:"contextLimit"`
}

// ProgressStep represents a granular action taken by the agent
type ProgressStep struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Status  string   `json:"status"`            // pending, running, completed, error
	Details []string `json:"details,omitempty"` // Sub-items for breakdown
}

// ToolCallInfo represents tool call info for frontend
type ToolCallInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
	Status    string `json:"status"` // pending, running, completed, error
}

// Chat sends a message and returns response via streaming
func (c *Controller) Chat(ctx context.Context, input ChatRequestInput, callback func(update interface{})) error {
	// Create cancellable context for abort support
	ctx, cancel := context.WithCancel(ctx)
	c.abortMu.Lock()
	c.abortCancel = cancel
	c.abortMu.Unlock()
	defer func() {
		c.abortMu.Lock()
		c.abortCancel = nil
		c.abortMu.Unlock()
	}()

	session := c.GetSession(input.SessionID)

	// Add user message if content provided
	if input.Content != "" {
		userMsg := protocol.Message{
			Role:    "user",
			Content: input.Content,
			Via:     input.Via,
		}
		session.StateHandler.AddMessage(userMsg)
	}

	// Track edited files for task progress
	taskFiles := make(map[string]bool)

	// Helper to emit task progress
	emitTaskProgress := func(status string, newFiles []string) {
		// Update file list
		for _, f := range newFiles {
			taskFiles[f] = true
		}

		// Convert map to slice
		var fileList []string
		for f := range taskFiles {
			fileList = append(fileList, f)
		}

		// Construct task payload
		progress := protocol.TaskProgress{
			TaskName: "Implementing Feature", // TODO: Determined dynamically
			Status:   status,
			Steps:    []string{status},
			Files:    fileList,
			IsActive: true,
			Mode:     "execution",
		}

		// Persist to task_progress_current.md (User Request Parity)
		// We write this to the workspace root to mimic Antigravity's behavior
		taskMdContent := fmt.Sprintf("# Task Progress: %s\n\n", progress.TaskName)
		taskMdContent += fmt.Sprintf("**Status**: %s\n", status)
		taskMdContent += fmt.Sprintf("**Mode**: %s\n\n", progress.Mode)

		taskMdContent += "## Files Edited\n"
		if len(fileList) == 0 {
			taskMdContent += "- (No files edited yet)\n"
		} else {
			for _, f := range fileList {
				taskMdContent += fmt.Sprintf("- `%s`\n", filepath.Base(f))
			}
		}

		taskMdContent += "\n## Progress Log\n"
		// In a real implementation we would accumulate these steps in state,
		// but for now we'll just show the latest status as the active step
		taskMdContent += fmt.Sprintf("- [x] %s\n", status)

		// Best effort write - ignore errors to not block flow
		// Derive workspace root from session or cwd
		if cwd, err := os.Getwd(); err == nil {
			_ = os.WriteFile(filepath.Join(cwd, "task_progress_current.md"), []byte(taskMdContent), 0644)
		}

		callback(progress)
	}

	// Helper to emit chat updates is already defined below...
	emitUpdate := func(msg ChatMessage) {
		callback(ChatUpdate{
			SessionID: input.SessionID,
			Message:   msg,
		})
	}

	// MAX TURNS to prevent infinite loops
	const maxTurns = 50
	currentTurn := 0

	// Usage tracking
	var totalTokensIn int
	var totalTokensOut int

	// Create assistant message placeholder ONCE for the whole chat session
	// Use the current message count as a stable index for the ID
	currentMessagesCount := len(session.StateHandler.GetMessages())
	assistantMsgID := fmt.Sprintf("msg-%d", currentMessagesCount)
	assistantMsg := ChatMessage{
		ID:          assistantMsgID,
		Role:        "assistant",
		Content:     "",
		Timestamp:   time.Now().UnixMilli(),
		IsStreaming: true,
		Metadata: &TaskMetadata{
			TokensIn:     totalTokensIn,
			TokensOut:    totalTokensOut,
			TotalCost:    0,
			ContextLimit: c.config.MaxTokens,
		},
	}
	emitUpdate(assistantMsg)

	for currentTurn < maxTurns {
		currentTurn++

		// Prepare Tools config for provider
		defs := c.executor.GetDefinitions()
		var providerTools []protocol.Tool
		activeMode := c.modes.GetActiveMode()

		for _, d := range defs {
			if modes.IsToolAllowed(activeMode, d.Name) {
				providerTools = append(providerTools, protocol.Tool{
					Name:        d.Name,
					Description: d.Description,
					InputSchema: d.InputSchema,
				})
			}
		}

		// Manage context window (condensation + sliding window)
		// Use ContextWindow for pruning, not MaxTokens (response limit)
		contextLimit := c.config.ContextWindow
		if contextLimit <= 0 {
			contextLimit = 128000 // Default to 128k if not set
		}

		// Diagnostics
		currentMessages := session.StateHandler.GetMessages()
		log.Printf("[Agent] Starting context management. Limit: %d, WindowSize: %d, Msgs: %d, Provider: %s",
			contextLimit, c.config.ContextWindow, len(currentMessages), c.config.Provider.Provider)

		// Initialize Condense Adapter
		condenseProvider := &condenseAdapter{
			p:     c.provider,
			model: c.config.Provider.Model,
		}

		// Configure Smart Context settings (Reflex Engine)
		ctxSettings := &context_manager.ContextSettings{
			AutoCondense:         true,
			CondenseThreshold:    70, // Start condensing at 70% usage
			SlidingWindowSize:    20,
			ShowContextIndicator: true,
		}

		wm := context_manager.NewWindowManagerWithSettings(contextLimit, ctxSettings, condenseProvider)

		contextResult, err := wm.ManageContext(ctx, currentMessages, c.config.SystemPrompt)
		if err != nil {
			return fmt.Errorf("context management failure (Reflex Engine): %w", err)
		}

		if contextResult.WasCondensed {
			log.Printf("🧠 Reflex Engine: Context condensed (Summary length: %d chars)", len(contextResult.Summary))
		}

		// Inject CodeGraph Repo Map if available (Repo Intelligence)
		// We insert it as a User message at the very beginning of HISTORY (or strictly after System Prompt).
		// Since we handle contextResult.Messages which are the messages sent to LLM,
		// we can prepend a User message if it's the first turn or if we want it persistent.
		// However, context manager might have pruned.
		// A better strategy is to append it to the System Prompt, or use a "Developer" role message if supported.
		// Let's modify System Prompt for this turn.

		finalSystemPrompt := contextResult.SystemPrompt
		if c.codegraph != nil {
			// Limit size: 5% of context window or max 100 files
			repoMap := c.codegraph.GenerateRepoMap(100)
			if repoMap != "" {
				finalSystemPrompt += "\n\n" + repoMap + "\n\n(This repository map is auto-generated based on Code Graph PageRank analysis)"
			}
		}

		// Build request
		// Build request with enhanced context including Active Mode and Project Rules
		// activeMode retrieved earlier
		modePrompt := fmt.Sprintf("\n\n### Current Mode: %s\n%s\n%s",
			activeMode.Name,
			activeMode.RoleDefinition,
			activeMode.CustomInstructions)

		rulesContext := c.rules.GetRules()

		// Skill Injection (Hardcore Workflow)
		var skillContext string
		if c.skills != nil && input.Content != "" {
			// Gather active files from trackers
			activeFiles := session.FileTracker.GetFiles()

			matchedSkills := c.skills.FindApplicableSkills(input.Content, activeFiles)
			if len(matchedSkills) > 0 {
				var sb strings.Builder
				sb.WriteString("\n\n### 🧠 Active Skills (Auto-Activated)\n")
				for _, skill := range matchedSkills {
					sb.WriteString(fmt.Sprintf("#### Skill: %s (%s)\n%s\n\n", skill.Name, skill.Enforcement, skill.Content))
				}
				skillContext = sb.String()
				log.Printf("🧠 Skills Activated: %d skills injected into context", len(matchedSkills))
			}
		}

		// Combine system prompt parts (Base + Mode + Rules + Skills + RepoMap if any)
		// Usually Controller logic appends to c.config.SystemPrompt,
		// but here finalSystemPrompt already has repoMap if any.
		// Let's ensure we merge correctly.
		// If Repomap was appended to finalSystemPrompt, we should use that base.

		// Re-construct system prompt to be safe and ordered
		enhancedSystemPrompt := finalSystemPrompt + modePrompt + rulesContext + skillContext + "\n\n" + c.envTracker.GetContext() + "\n" + session.FileTracker.GetContext()

		// Use contextResult.Messages as prunedMessages
		prunedMessages := contextResult.Messages

		// SAFETY: Sanitize messages to ensure Tool Call/Result integrity
		// This prevents API 400 errors if a previous session crashed/was pruned incorrectly
		prunedMessages = c.sanitizeMessages(prunedMessages)

		req := &ChatRequest{
			Model:        c.config.Provider.Model,
			Messages:     prunedMessages,
			SystemPrompt: enhancedSystemPrompt,
			MaxTokens:    c.config.MaxTokens,
			Tools:        providerTools,
		}

		// Calculate Input Tokens (Prompt) - Heuristic: len / 4
		promptTokens := len(enhancedSystemPrompt) / 4
		for _, m := range prunedMessages {
			promptTokens += len(m.Content) / 4
		}
		totalTokensIn += promptTokens

		// Update metadata
		calcCost := func(in, out int) float64 {
			inputPrice := 3.0 // Default to Sonnet 3.5 prices if model not found
			outputPrice := 15.0
			isFree := false

			if c.providersManager != nil {
				providers := c.providersManager.GetAvailableProviders()
				for _, p := range providers {
					if p.ID == c.config.Provider.Provider {
						for _, m := range p.Models {
							if m.ID == c.config.Provider.Model {
								inputPrice = m.InputPrice
								outputPrice = m.OutputPrice
								isFree = m.IsFree
								break
							}
						}
					}
				}
			}

			if isFree {
				return 0
			}

			// Prices are usually per 1M tokens in providers.yaml
			return (float64(in)/1_000_000)*inputPrice + (float64(out)/1_000_000)*outputPrice
		}

		turnCost := calcCost(totalTokensIn, totalTokensOut)
		session.TotalCost += turnCost

		// Log context status
		statusEmoji := ""
		if contextResult.WasCondensed {
			statusEmoji = " 📦"
		} else if contextResult.WasTruncated {
			statusEmoji = " ✂️"
		}
		log.Printf("Context: %.1f%% (%d/%d tokens, %d msgs)%s",
			contextResult.Percentage, contextResult.TokensUsed, contextResult.TokensMax, len(prunedMessages), statusEmoji)

		// Emit context status to frontend
		callback(ChatUpdate{
			SessionID: input.SessionID,
			ContextStatus: &protocol.ContextStatus{
				TokensUsed:     contextResult.TokensUsed,
				TokensMax:      contextResult.TokensMax,
				Percentage:     contextResult.Percentage,
				WasCondensed:   contextResult.WasCondensed,
				WasTruncated:   contextResult.WasTruncated,
				CumulativeCost: session.TotalCost,
			},
		})

		var currentTurnContent string
		var currentTurnReasoning string // Track reasoning separately for DeepSeek R1
		var currentTurnToolCalls []ToolCallInfo

		// Throttling for streaming updates to prevent webview crash
		// Reduced to 50ms for smoother experience while still preventing overflow
		var lastEmitTime time.Time
		const streamThrottleInterval = 50 * time.Millisecond
		var firstChunk = true // Track first chunk to always emit it

		// Stream response from AI using standard ChatStream
		// We use prunedMessages (from context management) instead of session messages
		err = c.provider.ChatStream(ctx, req, func(chunk *StreamChunk) error {
			switch chunk.Type {
			case "content_block_delta":
				currentTurnContent += chunk.Delta
				assistantMsg.Content += chunk.Delta
				assistantMsg.IsStreaming = true

				// Accumulate reasoning separately for DeepSeek R1 tool call support
				if chunk.ReasoningDelta != "" {
					currentTurnReasoning += chunk.ReasoningDelta
				}

				// Update Output Tokens - Heuristic
				deltaTokens := len(chunk.Delta) / 4
				if deltaTokens < 1 && len(chunk.Delta) > 0 {
					deltaTokens = 1
				}
				totalTokensOut += deltaTokens
				assistantMsg.Metadata.TokensOut = totalTokensOut
				turnCost := calcCost(totalTokensIn, totalTokensOut)
				assistantMsg.Metadata.TotalCost = turnCost

				// Throttle streaming updates to prevent webview overflow
				// BUT always emit: first chunk, thinking tags (reasoning), and after throttle interval
				now := time.Now()
				isReasoningTag := strings.Contains(chunk.Delta, "<thinking>") || strings.Contains(chunk.Delta, "</thinking>")
				if firstChunk || isReasoningTag || now.Sub(lastEmitTime) >= streamThrottleInterval {
					emitUpdate(assistantMsg)
					lastEmitTime = now
					firstChunk = false
				}

			case "tool_use":
				if chunk.ToolUse != nil {
					tc := ToolCallInfo{
						ID:        chunk.ToolUse.ID,
						Name:      chunk.ToolUse.Name,
						Arguments: string(chunk.ToolUse.Input),
						Status:    "pending",
					}
					currentTurnToolCalls = append(currentTurnToolCalls, tc)
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, tc)
					emitUpdate(assistantMsg)
				}

			case "message_stop", "message_delta":
				assistantMsg.IsStreaming = false
				emitUpdate(assistantMsg)
			}
			return nil
		})

		if err != nil {
			log.Printf("Streaming error: %v", err)
			assistantMsg.Content += "\n\n" + TranslateError(err)
			assistantMsg.IsStreaming = false
			emitUpdate(assistantMsg)
			return err
		}

		// Store assistant message for this turn in protocol history
		var storedToolUse []protocol.ToolUseBlock
		for _, tc := range currentTurnToolCalls {
			storedToolUse = append(storedToolUse, protocol.ToolUseBlock{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: json.RawMessage(tc.Arguments),
			})
		}

		session.StateHandler.AddMessage(protocol.Message{
			Role:             "assistant",
			Content:          currentTurnContent,
			ReasoningContent: currentTurnReasoning, // DeepSeek R1 requires this for tool calls
			ToolUse:          storedToolUse,
		})

		// If no tools used, we are done
		if len(currentTurnToolCalls) == 0 {
			break
		}

		// Initialize QC flag
		runQC := false

		// EXECUTE TOOLS
		log.Printf("Executing %d tools...", len(currentTurnToolCalls))
		var toolResults []protocol.ToolResultBlock
		for i, tc := range currentTurnToolCalls {
			// Update status to running in both turn list and message list
			currentTurnToolCalls[i].Status = "running"
			for j := range assistantMsg.ToolCalls {
				if assistantMsg.ToolCalls[j].ID == tc.ID {
					assistantMsg.ToolCalls[j].Status = "running"
				}
			}
			assistantMsg.IsStreaming = true
			emitUpdate(assistantMsg)

			// Execute
			log.Printf("Running tool %s: %s", tc.Name, tc.Arguments)

			var result string
			var err error

			switch tc.Name {
			case "update_todos":
				var payload struct {
					Todos []protocol.Todo `json:"todos"`
				}
				if err = json.Unmarshal([]byte(tc.Arguments), &payload); err == nil {
					c.UpdateTodos(input.SessionID, payload.Todos)
					result = "Task list updated successfully."
				} else {
					result = fmt.Sprintf("Error parsing todos: %v", err)
				}
			case "switch_mode":
				var payload struct {
					Mode    string `json:"mode"`
					Handoff bool   `json:"handoff"`
				}
				if err = json.Unmarshal([]byte(tc.Arguments), &payload); err == nil {
					// 1. Execute mode switch
					result, err = c.executor.Execute(ctx, tc.Name, json.RawMessage(tc.Arguments))
					if err == nil && payload.Handoff {
						// 2. Trigger Handoff
						log.Printf("🧠 Triggering Intelligent Handoff...")
						cwd, _ := os.Getwd()

						// Summarize history (exclude current tool call)
						msgs := session.StateHandler.GetMessages()
						spec, hErr := c.handoffService.GenerateSpec(ctx, msgs)
						if hErr != nil {
							log.Printf("Handoff generation failed: %v", hErr)
							result += fmt.Sprintf("\n(Warning: Handoff failed: %v)", hErr)
						} else {
							sErr := c.handoffService.SaveSpec(cwd, spec)
							if sErr != nil {
								log.Printf("Handoff save failed: %v", sErr)
							} else {
								// 3. Condense Context: Re-initialize session but keep ID
								// For now, we just log it. Real pruning happens in ContextManager anyway.
								// But to "Start Fresh", we could archive messages.
								result += "\n\n🧠 **Intelligent Handoff Complete**\nContext condensed into `SPEC.md`. Mode switched."
							}
						}
					}
				} else {
					result = fmt.Sprintf("Error parsing switch_mode args: %v", err)
				}
			default:
				result, err = c.executor.Execute(ctx, tc.Name, json.RawMessage(tc.Arguments))
			}
			isError := false
			if err != nil {
				log.Printf("Tool execution failed: %v", err)
				result = TranslateError(err)
				isError = true
				currentTurnToolCalls[i].Status = "error"
			} else {
				currentTurnToolCalls[i].Status = "completed"
			}

			displayResult := truncateString(result, 1000)

			currentTurnToolCalls[i].Result = displayResult
			for j := range assistantMsg.ToolCalls {
				if assistantMsg.ToolCalls[j].ID == tc.ID {
					assistantMsg.ToolCalls[j].Status = currentTurnToolCalls[i].Status
					assistantMsg.ToolCalls[j].Result = displayResult
				}
			}
			assistantMsg.IsStreaming = true
			emitUpdate(assistantMsg)

			toolResults = append(toolResults, protocol.ToolResultBlock{
				ToolUseID: tc.ID,
				Content:   result,
				IsError:   isError,
			})

			// Track activities for the UI
			if !isError {
				activity := c.deriveActivity(tc.Name, tc.Arguments, result)
				if activity != nil {
					assistantMsg.Activities = append(assistantMsg.Activities, *activity)
					emitUpdate(assistantMsg)
				}
			}

			// Track file access for context
			if !isError && (tc.Name == "read_file" || tc.Name == "write_file" || tc.Name == "view_file") {
				var argsMap map[string]interface{}
				if json.Unmarshal([]byte(tc.Arguments), &argsMap) == nil {
					if path, ok := argsMap["path"].(string); ok {
						session.FileTracker.AddFile(path)
					} else if path, ok := argsMap["TargetFile"].(string); ok {
						session.FileTracker.AddFile(path)
					} else if path, ok := argsMap["AbsolutePath"].(string); ok {
						session.FileTracker.AddFile(path)
					}
				}
			}

			// Track file edits for task progress
			if !isError && (tc.Name == "write_file" || tc.Name == "replace_file_content" || tc.Name == "write_to_file") {
				var argsMap map[string]interface{}
				if json.Unmarshal([]byte(tc.Arguments), &argsMap) == nil {
					var target string
					if t, ok := argsMap["TargetFile"].(string); ok {
						target = t
					} else if t, ok := argsMap["AbsolutePath"].(string); ok {
						target = t
					}

					if target != "" {
						emitTaskProgress(fmt.Sprintf("Edited %s", path.Base(target)), []string{target})
					}
				}
			}

			// Auto-checkpoint after write operations
			if !isError && c.checkpointService != nil && isWriteTool(tc.Name) {
				hash, cpErr := c.checkpointService.Save(fmt.Sprintf("After %s", tc.Name))
				if cpErr == nil && hash != "" {
					assistantMsg.CheckpointHash = hash
					log.Printf("📸 Checkpoint saved: %s (after %s)", hash[:8], tc.Name)
				}
			}

			// Flag for QC if it's a code modification tool
			if !isError && (isWriteTool(tc.Name) || tc.Name == "apply_diff") {
				runQC = true
			}
		}

		// Run Auto-QC if code was modified
		var qcMessage string
		if runQC && c.qcManager != nil {
			log.Printf("🤖 Running Phase 15 Auto-QC...")
			qcRes, err := c.qcManager.RunCheck(ctx)
			if err != nil {
				log.Printf("QC Error: %v", err)
			} else if !qcRes.Success {
				log.Printf("❌ Auto-QC FAILED: %s", qcRes.Command)
				// Create a structured error message to feedback into the loop
				qcMessage = fmt.Sprintf("\n\n⚠️ **Auto-QC Failed** (Command: `%s`)\n```\n%s\n```\nPlease fix these errors before proceeding.",
					qcRes.Command, truncateString(qcRes.Output, 2000))
			} else if qcRes.Output != "" {
				log.Printf("✅ Auto-QC PASSED: %s", qcRes.Command)
			}
		}

		// Append tool results to session as a User message (standard for Anthropic)
		session.StateHandler.AddMessage(protocol.Message{
			Role:        "user",
			ToolResults: toolResults,
			Content:     qcMessage, // Append QC failure message if any
		})

		// Loop continues to get AI's reaction to tool results
	}

	return nil
}

// sanitizeMessages ensures every tool call has a corresponding result
func (c *Controller) sanitizeMessages(msgs []protocol.Message) []protocol.Message {
	if len(msgs) == 0 {
		return msgs
	}

	var clean []protocol.Message
	// We need to look ahead, so we iterate manually

	// Map of ToolCallID -> hasResult
	// But actually we just need strict pairs: Assistant(ToolUse) -> User(ToolResults)
	// If Assistant has ToolUse, next msg MUST be User with matching ToolResults

	skipNext := false

	for i := 0; i < len(msgs); i++ {
		if skipNext {
			skipNext = false
			continue
		}

		msg := msgs[i]

		// If it's a Tool Use message
		if msg.Role == "assistant" && len(msg.ToolUse) > 0 {
			// Check next message
			if i+1 >= len(msgs) {
				// Dangling tool call at end of history -> Drop it
				log.Printf("⚠️ Sanitizer: Dropping dangling tool call at end of history (ID: %s)", msg.ToolUse[0].ID)
				continue
			}

			nextMsg := msgs[i+1]
			if nextMsg.Role != "user" || len(nextMsg.ToolResults) == 0 {
				// Next message is NOT a result -> Drop this tool call
				log.Printf("⚠️ Sanitizer: Dropping orphaned tool call (ID: %s) followed by %s", msg.ToolUse[0].ID, nextMsg.Role)
				continue
			}

			// Optional: Verify IDs match?
			// DeepSeek expects strict ID matching.
			// Let's assume if it follows, it's correct enough for now.
			// But if we want to be safe, we should check.

			// Keep both
			clean = append(clean, msg)
			clean = append(clean, nextMsg)
			skipNext = true // Consumed nextMsg
			continue
		}

		// Determine if this is an orphan Tool Result (User message with results but no preceding call)
		// Since we handle pairs above, if we see a User with Results here, it means we skipped the call!
		if msg.Role == "user" && len(msg.ToolResults) > 0 {
			log.Printf("⚠️ Sanitizer: Dropping orphan tool result (ID: %s)", msg.ToolResults[0].ToolUseID)
			continue
		}

		clean = append(clean, msg)
	}

	return clean
}

// GetState returns the current state for a session
func (c *Controller) GetState(sessionID string) map[string]interface{} {
	session := c.GetSession(sessionID)

	stateMsgs := session.StateHandler.GetMessages()
	messages := make([]ChatMessage, 0)

	for i := 0; i < len(stateMsgs); i++ {
		msg := stateMsgs[i]

		// Skip tool result messages as they are merged into the preceding assistant message
		if msg.Role == "user" && len(msg.ToolResults) > 0 {
			continue
		}

		// If this is an assistant message and the previous message in our consolidated list
		// is also an assistant message, we merge them.
		if msg.Role == "assistant" && len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
			last := &messages[len(messages)-1]
			if msg.Content != "" {
				if last.Content != "" {
					last.Content += "\n" + msg.Content
				} else {
					last.Content = msg.Content
				}
			}

			// Add tool calls from this message
			toolCalls, activities := c.processAssistantTurn(stateMsgs, i)
			last.ToolCalls = append(last.ToolCalls, toolCalls...)
			last.Activities = append(last.Activities, activities...)
			continue
		}

		// New message
		chatMsg := ChatMessage{
			ID:        fmt.Sprintf("msg-%d", i),
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: session.CreatedAt.Add(time.Duration(i) * time.Second).UnixMilli(),
		}

		if msg.Role == "assistant" {
			tc, activities := c.processAssistantTurn(stateMsgs, i)
			chatMsg.ToolCalls = tc
			chatMsg.Activities = activities
		}

		messages = append(messages, chatMsg)
	}

	return map[string]interface{}{
		"messages":        messages,
		"liveModeEnabled": false,
		"mode":            c.modes.GetActiveMode().Slug,
		"todos":           session.Todos,
	}
}

// processAssistantTurn finds tool calls and derives activities for the message at index i
func (c *Controller) processAssistantTurn(stateMsgs []protocol.Message, i int) ([]ToolCallInfo, []ActivityItem) {
	msg := stateMsgs[i]

	var results map[string]string
	var errors map[string]bool

	// Look ahead for results in the next message
	if i+1 < len(stateMsgs) {
		nextMsg := stateMsgs[i+1]
		if nextMsg.Role == "user" && len(nextMsg.ToolResults) > 0 {
			results = make(map[string]string)
			errors = make(map[string]bool)
			for _, res := range nextMsg.ToolResults {
				results[res.ToolUseID] = res.Content
				errors[res.ToolUseID] = res.IsError
			}
		}
	}

	var toolCalls []ToolCallInfo
	var activities []ActivityItem

	for _, tu := range msg.ToolUse {
		status := "pending"
		result := ""
		isError := false

		if res, ok := results[tu.ID]; ok {
			result = res
			if errors[tu.ID] {
				status = "error"
				isError = true
			} else {
				status = "completed"
			}
		}

		toolCalls = append(toolCalls, ToolCallInfo{
			ID:        tu.ID,
			Name:      tu.Name,
			Arguments: string(tu.Input),
			Status:    status,
			Result:    result,
		})

		if !isError && result != "" {
			activity := c.deriveActivity(tu.Name, string(tu.Input), result)
			if activity != nil {
				activities = append(activities, *activity)
			}
		}
	}
	return toolCalls, activities
}

func (c *Controller) deriveActivity(name string, arguments string, result string) *ActivityItem {
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &argsMap); err != nil {
		return nil
	}

	activity := &ActivityItem{}
	switch name {
	case "read_file", "view_file":
		if path, ok := argsMap["path"].(string); ok {
			activity.Type = "analyze"
			activity.File = path
		} else if path, ok := argsMap["AbsolutePath"].(string); ok {
			activity.Type = "analyze"
			activity.File = path
		}
	case "write_file", "edit_file", "replace_file_content", "multi_replace_file_content":
		if path, ok := argsMap["TargetFile"].(string); ok {
			activity.Type = "edit"
			activity.File = path
			activity.Additions = strings.Count(result, "+")
			activity.Deletions = strings.Count(result, "-")
		} else if path, ok := argsMap["path"].(string); ok {
			activity.Type = "edit"
			activity.File = path
		}
	case "search_files", "grep_search", "find_by_name":
		if query, ok := argsMap["query"].(string); ok {
			activity.Type = "search"
			activity.Query = query
			activity.Results = strings.Count(result, "\n")
		} else if query, ok := argsMap["Query"].(string); ok {
			activity.Type = "search"
			activity.Query = query
			activity.Results = strings.Count(result, "\n")
		} else if pattern, ok := argsMap["Pattern"].(string); ok {
			activity.Type = "search"
			activity.Query = pattern
			activity.Results = strings.Count(result, "\n")
		}
	case "execute_command", "run_command":
		activity.Type = "command"
	}

	if activity.Type == "" {
		return nil
	}
	return activity
}

// UpdateTodos updates the task list for a session and notifies the host
func (c *Controller) UpdateTodos(sessionID string, todos []protocol.Todo) {
	session := c.GetSession(sessionID)
	if session != nil {
		session.Todos = todos
		c.sessionManager.Save(sessionID)
	}

	// Notify host about state change
	if c.host != nil {
		c.host.SendMessage(protocol.RPCMessage{
			Type: "task_state_updated",
			Payload: protocol.EncodeRPC(map[string]interface{}{
				"session_id": sessionID,
				"todos":      todos,
			}),
		})
	}
}

// condenseAdapter wraps the main AI provider to satisfy the CondenseProvider interface
type condenseAdapter struct {
	p     Provider
	model string
}

// Summarize asks the AI to summarize the text
func (a *condenseAdapter) Summarize(ctx context.Context, prompt string) (string, error) {
	req := &ChatRequest{
		Model: a.model,
		Messages: []protocol.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4000, // Allow reasonable space for summary
	}

	resp, err := a.p.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// isWriteTool returns true if the tool modifies workspace files
func isWriteTool(name string) bool {
	writingTools := map[string]bool{
		"write_to_file":     true,
		"write_file":        true,
		"execute_command":   true,
		"apply_diff":        true,
		"delete_file":       true,
		"create_directory":  true,
		"run_command":       true,
		"replace_in_file":   true,
		"insert_code_block": true,
	}
	return writingTools[name]
}

// SetCheckpointService sets the checkpoint service for auto-saving
func (c *Controller) SetCheckpointService(svc *checkpoints.CheckpointService) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkpointService = svc
}
