package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/igoryan-dao/ricochet/internal/checkpoints"
	"github.com/igoryan-dao/ricochet/internal/config"
	context_manager "github.com/igoryan-dao/ricochet/internal/context"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/index"
	mcpHubPkg "github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/paths"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/rules"
	"github.com/igoryan-dao/ricochet/internal/safeguard"
	"github.com/igoryan-dao/ricochet/internal/tools"
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
}

// Config holds agent configuration
type Config struct {
	Provider        ProviderConfig `json:"provider"`
	SystemPrompt    string         `json:"system_prompt"`
	MaxTokens       int            `json:"max_tokens"`     // Max tokens for response generation
	ContextWindow   int            `json:"context_window"` // Context window limit for pruning
	EnableCodeIndex bool           `json:"enable_code_index"`
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

	if len(opts) > 0 {
		h = opts[0].Host
		mm = opts[0].Modes
		rm = opts[0].Rules
		mcpHub = opts[0].McpHub
		pm = opts[0].ProvidersManager
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

	// Initialize safeguard manager
	safeguardMgr, err := safeguard.NewManager(cwd)
	if err != nil {
		log.Printf("Warning: Failed to initialize safeguard manager: %v", err)
	}

	// Initialize indexer
	indexPath := filepath.Join(os.Getenv("HOME"), ".ricochet", "index.vdb")
	store, _ := index.NewLocalStore(indexPath)
	indexer := index.NewIndexer(store, provider, cwd)

	executor := tools.NewNativeExecutor(h, mm, safeguardMgr, mcpHub, indexer)

	// Trigger indexing in background
	if cfg.EnableCodeIndex {
		go func() {
			ctx := context.Background()
			if err := indexer.IndexAll(ctx); err != nil {
				log.Printf("Background indexing failed: %v", err)
			}
		}()
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

// CreateSession creates a new session
func (c *Controller) CreateSession() *Session {
	return c.sessionManager.CreateSession()
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
func (c *Controller) Chat(ctx context.Context, input ChatRequestInput, callback func(update ChatUpdate)) error {
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

	// Helper to emit updates
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

		wm := context_manager.NewWindowManager(contextLimit)
		// Add custom settings from config if available (phase 12)
		// For now using defaults in NewWindowManager
		contextResult, err := wm.ManageContext(ctx, currentMessages, c.config.SystemPrompt)
		if err != nil {
			log.Printf("Context management warning: %v", err)
		}
		prunedMessages := contextResult.Messages

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

		// Build request
		// Build request with enhanced context including Active Mode and Project Rules
		// activeMode retrieved earlier
		modePrompt := fmt.Sprintf("\n\n### Current Mode: %s\n%s\n%s",
			activeMode.Name,
			activeMode.RoleDefinition,
			activeMode.CustomInstructions)

		rulesContext := c.rules.GetRules()

		enhancedSystemPrompt := c.config.SystemPrompt + modePrompt + rulesContext + "\n\n" + c.envTracker.GetContext() + "\n" + session.FileTracker.GetContext()

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

		assistantMsg.Metadata = &TaskMetadata{
			TokensIn:     totalTokensIn,
			TokensOut:    totalTokensOut,
			TotalCost:    turnCost,
			ContextLimit: c.config.MaxTokens,
		}

		var currentTurnContent string
		var currentTurnToolCalls []ToolCallInfo

		// Stream response from AI
		err = c.provider.ChatStream(ctx, req, func(chunk *StreamChunk) error {
			switch chunk.Type {
			case "content_block_delta":
				currentTurnContent += chunk.Delta
				assistantMsg.Content += chunk.Delta
				assistantMsg.IsStreaming = true

				// Update Output Tokens - Heuristic
				deltaTokens := len(chunk.Delta) / 4
				if deltaTokens < 1 && len(chunk.Delta) > 0 {
					deltaTokens = 1
				}
				totalTokensOut += deltaTokens
				assistantMsg.Metadata.TokensOut = totalTokensOut
				turnCost := calcCost(totalTokensIn, totalTokensOut)
				assistantMsg.Metadata.TotalCost = turnCost

				emitUpdate(assistantMsg)

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
			Role:    "assistant",
			Content: currentTurnContent,
			ToolUse: storedToolUse,
		})

		// If no tools used, we are done
		if len(currentTurnToolCalls) == 0 {
			break
		}

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

			if tc.Name == "update_todos" {
				var payload struct {
					Todos []protocol.Todo `json:"todos"`
				}
				if err = json.Unmarshal([]byte(tc.Arguments), &payload); err == nil {
					c.UpdateTodos(input.SessionID, payload.Todos)
					result = "Task list updated successfully."
				} else {
					result = fmt.Sprintf("Error parsing todos: %v", err)
				}
			} else {
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

			// Auto-checkpoint after write operations
			if !isError && c.checkpointService != nil && isWriteTool(tc.Name) {
				hash, cpErr := c.checkpointService.Save(fmt.Sprintf("After %s", tc.Name))
				if cpErr == nil && hash != "" {
					assistantMsg.CheckpointHash = hash
					log.Printf("📸 Checkpoint saved: %s (after %s)", hash[:8], tc.Name)
				}
			}
		}

		// Append tool results to session as a User message (standard for Anthropic)
		// Append tool results to session as a User message (standard for Anthropic)
		session.StateHandler.AddMessage(protocol.Message{
			Role:        "user",
			ToolResults: toolResults,
		})

		// Loop continues to get AI's reaction to tool results
	}

	return nil
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
