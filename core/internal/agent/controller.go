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

	"github.com/google/uuid"
	"github.com/igoryan-dao/ricochet/internal/agent/hooks"
	"github.com/igoryan-dao/ricochet/internal/codegraph"
	"github.com/igoryan-dao/ricochet/internal/config"
	context_manager "github.com/igoryan-dao/ricochet/internal/context"
	"github.com/igoryan-dao/ricochet/internal/context/handoff"
	"github.com/igoryan-dao/ricochet/internal/git"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/index"
	mcpHubPkg "github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/memory"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/prompts"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/qc"
	"github.com/igoryan-dao/ricochet/internal/rules"
	"github.com/igoryan-dao/ricochet/internal/safeguard"
	"github.com/igoryan-dao/ricochet/internal/skills"
	"github.com/igoryan-dao/ricochet/internal/terminal"
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
	checkpointManager *CheckpointManager
	providersManager  *config.ProvidersManager
	indexer           *index.Indexer

	codegraph          *codegraph.Service
	handoffService     *handoff.Service
	workflows          *workflow.Manager
	workflowEngine     *workflow.Engine
	skills             *skills.Manager
	qcManager          *qc.Manager
	dynamicHooks       *hooks.DynamicHookManager
	memoryManager      *memory.Manager
	injectionProcessor *InjectionProcessor
	mcpManager         *mcpHubPkg.Manager
	gitManager         *git.Manager       // Git integration
	contextManager     *ContextManager    // Context compaction
	loopDetector       *LoopDetector      // Detects repetitive content patterns
	planManager        *PlanManager       // Manages long-term plan
	swarm              *SwarmOrchestrator // Swarm Orchestrator
	helpAgent          *HelpAgent         // Handles help queries
	defaultModel       string             // Default model for internal tasks

	// Abort support
	abortMu     sync.Mutex
	abortCancel context.CancelFunc

	// UI Callbacks
	onTaskProgress func(protocol.TaskProgress)
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
	Tools             config.ToolsSettings         `json:"tools"`
	Swarm             SwarmConfig                  `json:"swarm"`
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
	} else {
		if cfg.AutoApproval != nil {
			safeguardMgr.SetAutoApproval(cfg.AutoApproval)
		}
		// Set Tools Settings (DisableLLMCorrection)
		safeguardMgr.SetToolsSettings(&cfg.Tools)
	}

	// Initialize Session Manager
	// Store sessions in .ricochet/sessions
	configDir := filepath.Join(os.Getenv("HOME"), ".ricochet")
	sessionDir := filepath.Join(configDir, "sessions")
	sessionManager := NewSessionManager(sessionDir)

	// Initialize MCP Manager
	mcpManager := mcpHubPkg.NewManager(configDir)

	// Initialize Git Manager
	gitMgr := git.NewManager(cwd)

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

	// Initialize Dynamic Hook Manager (Hookify)
	hooksMgr := hooks.NewDynamicHookManager(cwd)

	// Initialize Memory Manager (Phase 15)
	memoryMgr, _ := memory.NewManager(cwd)

	// Initialize Injection Processor (Phase 17)
	injectionProc := NewInjectionProcessor(cwd)

	// Initialize Plan Manager (Autonomous Agent)
	pmMgr := NewPlanManager(cwd)
	if err := pmMgr.Load(); err != nil {
		log.Printf("Warning: Failed to load plan: %v", err)
	}

	executor := tools.NewNativeExecutor(h, mm, safeguardMgr, mcpHub, indexer, cg, wm)

	// Register Subtask Tool (circular dependency handled via interface or setter later)
	// For now, we'll inject it into the executor if supported, or handle via special tool dispatch.
	// Ideally, NativeExecutor should accept custom tools.
	// Let's add it to the NativeExecutor manually or via a wrapper.
	// Since NativeExecutor is in `internal/tools`, we might need to extend it.
	// For simplicity in this phase, let's assume `executor` can register dynamic tools or we handle it in `Chat` loop.
	// BUT, the cleanest way is for NativeExecutor to know about it.
	// Actually, `RunSubtask` is on Controller. `SubtaskTool` calls `Executor.RunSubtask`.
	// So Controller *is* the Executor.

	// Better approach: Controller creates the SubtaskTool and passes it to NativeExecutor's registry.
	subtaskTool := &tools.SubtaskTool{} // Executor set later to avoid circular init
	executor.RegisterTool(subtaskTool)

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
	// storageDir := paths.GetSessionDir(cwd) // Using sessionDir from above
	// sm := NewSessionManager(storageDir)

	// Initialize Checkpoint Manager (Phase 18)
	checkpointMgr := NewCheckpointManager(cwd)

	c := &Controller{
		provider:           provider,
		sessionManager:     sessionManager,
		config:             cfg,
		executor:           executor,
		envTracker:         context_manager.NewEnvironmentTracker(cwd),
		safeguard:          safeguardMgr,
		modes:              mm,
		rules:              rm,
		host:               h,
		providersManager:   pm,
		indexer:            indexer,
		skills:             skillMgr,
		qcManager:          qcMgr,
		dynamicHooks:       hooksMgr,
		memoryManager:      memoryMgr,
		injectionProcessor: injectionProc,
		mcpManager:         mcpManager,
		gitManager:         gitMgr,
		contextManager:     NewContextManager(provider, cfg.ContextWindow, 4000),
		checkpointManager:  checkpointMgr,
		planManager:        pmMgr,
		helpAgent:          NewHelpAgent(),
		defaultModel:       cfg.Provider.Model,
		loopDetector:       NewLoopDetector(3), // Detect loops after 3 repetitions
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
		workflows: wm,
	}

	// Initialize Swarm Orchestrator
	// Initialize Swarm Orchestrator
	c.swarm = NewSwarmOrchestrator(c, pmMgr, c.config.Swarm)

	// Register Swarm Tools (Now that swarm is init)
	executor.RegisterTool(&StartSwarmToolImpl{Orchestrator: c.swarm})
	executor.RegisterTool(&UpdatePlanToolImpl{Plan: pmMgr})

	// Initialize Workflow Engine with Controller as executor
	// Initialize Workflow Engine with Controller as executor
	// We pass a simple adapter for command execution
	c.workflowEngine = workflow.NewEngine(c, &CommandExecutorAdapter{Host: h})

	// Close the loop: Set Controller as the SubtaskExecutor
	subtaskTool.Executor = c

	return c, nil
}

// RunSubtask executes a goal in an isolated session
func (c *Controller) RunSubtask(ctx context.Context, parentSessionID string, goal string, contextInfo string, role string) (string, error) {
	log.Printf("[Controller] Starting SUBTASK: %s (Role: %s, Parent: %s)", goal, role, parentSessionID)

	// 1. Create Child Session
	childSession := c.CreateSession() // Start fresh

	// 1.5 Context Inheritance: Copy Active Files from Parent
	if parentSessionID != "" {
		parentSession := c.sessionManager.GetSession(parentSessionID)
		if parentSession != nil {
			activeFiles := parentSession.FileTracker.GetFiles()
			if len(activeFiles) > 0 {
				log.Printf("Inheriting %d active files from parent session %s", len(activeFiles), parentSessionID)
				for _, f := range activeFiles {
					childSession.FileTracker.AddFile(f)
				}
			}
		}
	}

	// 2. Prime the session with specialized role
	var sysPrompt string
	switch role {
	case "architect":
		sysPrompt = fmt.Sprintf("You are a specialized System Architect Agent.\nGOAL: %s\nCONTEXT: %s\n\nROLE: Focus on high-level design patterns, system scalability, and trade-offs. Do not get bogged down in implementation details unless necessary. Provide a concrete plan or design document.", goal, contextInfo)
	case "qa":
		sysPrompt = fmt.Sprintf("You are a specialized QA/Security Agent.\nGOAL: %s\nCONTEXT: %s\n\nROLE: Critically analyze the code/plan for bugs, security vulnerabilities, and edge cases. Be pedantic but constructive. Propose tests.", goal, contextInfo)
	case "researcher":
		sysPrompt = fmt.Sprintf("You are a specialized Research Agent.\nGOAL: %s\nCONTEXT: %s\n\nROLE: Gather information, summarize findings, and provide citations/file paths. Do not modify code unless asked.", goal, contextInfo)
	default: // "general"
		sysPrompt = fmt.Sprintf("You are a Sub-Agent focused on a specific task.\nGOAL: %s\nCONTEXT: %s\n\nPerform the task efficiently. When done, output a summary of your actions.", goal, contextInfo)
	}

	childSession.StateHandler.AddMessage(protocol.Message{Role: "system", Content: sysPrompt})

	// 3. Run Auto-Pilot Loop
	// We check for "TASK_COMPLETE" in the output to break the loop.
	// If the agent pauses (returns text without completion), we urge it to continue.
	var finalSummary string
	maxTurns := 15

	for i := 0; i < maxTurns; i++ {
		input := ChatRequestInput{
			SessionID: childSession.ID,
			Content:   "Please continue working on the goal. If you are finished, output 'TASK_COMPLETE:' followed by a summary.",
			Via:       "subtask",
		}

		// First turn specific prompt
		if i == 0 {
			input.Content = fmt.Sprintf("STARTING SUBTASK: %s\nContext: %s\nPlease proceed.", goal, contextInfo)
		}

		var lastResponse string

		// Run Chat (Blocking wait for this turn)
		// We use a done channel to wait for Chat to return (which it does after its internal loop)
		// Wait, Chat wraps everything in a goroutine?
		// No, Chat function signature in Controller (line 499) returns error.
		// It executes synchronously?
		// Checking Chat implementation...
		// It creates a `ctx, cancel`.
		// It does `go func()`? No.
		// It calls `callback` synchronously?
		// Let's verify `Chat` is synchronous or blocking.
		// `Chat` calls `c.provider.Chat` which is blocking.
		// It loops `for currentTurn < maxTurns`.
		// So `Chat` blocks until it finishes a "turn" (which might be multiple tool calls).
		// Yes, `Chat` is blocking.

		err := c.Chat(ctx, input, func(update interface{}) {
			// Forward events to parent UI if callback exists
			// Retrieve parent callback from context... wait, RunSubtask HAS the context.
			// But we need to EXTRACT it from ctx first.
			if parentCb, ok := ctx.Value("chat_callback").(func(interface{})); ok {
				// We need to re-wrap the update to target the parent session
				// and visually indicate it's a subtask.
				switch u := update.(type) {
				case ChatUpdate:
					// Rewrite Session ID to parent so it renders in main view
					u.SessionID = parentSessionID
					// Prefix content
					// Only prefix if it's content, not streaming chunks which might look weird if prefixed every time.
					// But we are not streaming deeply here yet? "IsStreaming" logic.
					// Let's just prefix the first chunk or all?
					// Simpler: Just forward it. The content will speak for itself.
					// Or append "[Subtask]" prefix.
					if u.Message.Content != "" {
						u.Message.Content = "nested > " + u.Message.Content
					}
					// Only forward assistant messages or system?
					// Forward everything for transparency.
					parentCb(u)

					// Capture for local logic
					if u.Message.Role == "assistant" && !u.Message.IsStreaming {
						lastResponse = u.Message.Content
					}
				case protocol.TaskProgress:
					// Forward task progress
					// Inject Identity for TUI Badges
					u.AgentIdentifier = strings.ToUpper(role)
					switch role {
					case "architect":
						u.AgentColor = "#9D65FF" // Purple
					case "qa":
						u.AgentColor = "#FF9D00" // Orange
					case "researcher":
						u.AgentColor = "#00AFFF" // Blue
					case "swarm-worker":
						u.AgentColor = "#00FF99" // Green
					default:
						u.AgentColor = "#767676" // Gray
					}
					parentCb(u)
				}
			} else {
				// Fallback local capture if no parent callback (shouldn't happen in real run)
				if u, ok := update.(ChatUpdate); ok {
					if u.Message.Role == "assistant" && !u.Message.IsStreaming {
						lastResponse = u.Message.Content
					}
				}
			}
		})

		if err != nil {
			return "", fmt.Errorf("subtask error on turn %d: %w", i+1, err)
		}

		finalSummary = lastResponse

		// Check for Completion Signal
		if strings.Contains(lastResponse, "TASK_COMPLETE") {
			finalSummary = strings.TrimPrefix(strings.Split(lastResponse, "TASK_COMPLETE")[1], ":") // Basic parsing
			break
		}

		// Check for Failure Signal (Phase 14)
		if strings.Contains(lastResponse, "TASK_FAILED") {
			failReason := strings.TrimPrefix(strings.Split(lastResponse, "TASK_FAILED")[1], ":")
			result := tools.SubtaskResult{
				Status:       "failed",
				Error:        strings.TrimSpace(failReason),
				RecoveryHint: "Check the error message and context. You may need to retry with different search terms or paths.",
			}
			resJSON, _ := json.Marshal(result)
			return string(resJSON), nil
		}

		// If no completion signal, loop continues with "Please continue..."
		// Unless the agent explicitly says "I cannot continue" or similar?
		// For now, we rely on the prompt instructing "TASK_COMPLETE".
	}

	// Default Success
	result := tools.SubtaskResult{
		Status:  "success",
		Summary: strings.TrimSpace(finalSummary),
	}
	if result.Summary == "" {
		// Fallback if loop finished without explicit signal (max turns reached)
		result.Status = "failed"
		result.Error = "Subtask timed out or did not report completion explicitly."
	}

	resJSON, _ := json.Marshal(result)
	return string(resJSON), nil
}

func (c *Controller) GetHost() host.Host {
	return c.host
}

func (c *Controller) GetMcpManager() *mcpHubPkg.Manager {
	return c.mcpManager
}

func (c *Controller) GetGitManager() *git.Manager {
	return c.gitManager
}

func (c *Controller) GetPlanManager() *PlanManager {
	return c.planManager
}

// GenerateCommitMessage asks the LLM to generate a commit message based on the diff
func (c *Controller) GenerateCommitMessage(ctx context.Context, diff string) (string, error) {
	if diff == "" {
		return "", fmt.Errorf("empty diff")
	}

	system := "You are a professional software engineer. Generate a concise, conventional commit message for the following git diff. Output ONLY the message, no extra text."
	user := fmt.Sprintf("Diff:\n%s", diff)

	messages := []protocol.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	req := &ChatRequest{
		Model:    c.defaultModel,
		Messages: messages,
	}

	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

// CommandExecutorAdapter adapts host.Host to workflow.CommandExecutor
type CommandExecutorAdapter struct {
	Host host.Host
}

func (a *CommandExecutorAdapter) Execute(command string) (string, error) {
	// We assume context background for now or TODO pass it
	res, err := a.Host.ExecuteCommand(context.Background(), command, false)
	if err != nil {
		return "", err
	}
	return res.Output, nil
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

// HydrateSession restores a session with messages from history
func (c *Controller) HydrateSession(sessionID string, messages []protocol.Message) {
	// CreateSessionWithID acts as GetOrCreate - returning existing if found, or creating new
	session := c.sessionManager.CreateSessionWithID(sessionID)
	session.StateHandler.SetMessages(messages)
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

// SetMainSessionID binds the controller and its components (PlanManager) to a specific active session.
// This ensures that planning artifacts are scoped to the current interaction and not global.
func (c *Controller) SetMainSessionID(sessionID string) {
	if c.planManager != nil {
		if err := c.planManager.SetSessionID(sessionID); err != nil {
			log.Printf("[Controller] Failed to set plan session ID: %v", err)
		}
	}
}

// ChatRequest represents a request to chat
type ChatRequestInput struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	Via       string `json:"via,omitempty"` // Message source: telegram, discord, ide
	PlanMode  bool   `json:"plan_mode,omitempty"`
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
	Reasoning      string         `json:"reasoning,omitempty"`
	Timestamp      int64          `json:"timestamp"`
	IsStreaming    bool           `json:"isStreaming,omitempty"`
	ToolCalls      []ToolCallInfo `json:"toolCalls,omitempty"`
	Activities     []ActivityItem `json:"activities,omitempty"` // Files analyzed, edited, searched
	Steps          []ProgressStep `json:"steps,omitempty"`      // Real-time progress updates
	Metadata       *TaskMetadata  `json:"metadata,omitempty"`
	Via            string         `json:"via,omitempty"`            // Message source: telegram, discord, ide
	SessionID      string         `json:"sessionId,omitempty"`      // Session context for this message
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
	Message   string `json:"message,omitempty"`   // for task_boundary/notifications
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
	// Update terminal title to show agent is working
	terminal.SetTerminalTitle(terminal.StateWorking)
	defer terminal.SetTerminalTitle(terminal.StateReady)

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

	// Inject Session ID into context for tools (e.g. SubtaskTool)
	ctx = context.WithValue(ctx, "session_id", input.SessionID)
	// Inject Callback for subtask event forwarding
	ctx = context.WithValue(ctx, "chat_callback", callback)

	session := c.GetSession(input.SessionID)
	if session == nil {
		return fmt.Errorf("session '%s' not found. Type /new to start.", input.SessionID)
	}

	// Add user message if content provided
	if input.Content != "" {
		if input.PlanMode {
			// Prepend a Plan Mode constraint
			session.StateHandler.AddMessage(protocol.Message{
				Role:    "system",
				Content: "PLAN MODE ENABLED: You are in read-only mode for exploration and planning. You can use search and read tools, but avoid making any file changes or executing destructive commands. If the user asks for changes, explain your plan first.",
			})
		}
		// SLASH COMMAND INTERCEPTION
		if strings.HasPrefix(input.Content, "/") {
			// 1. Model Switching: /model
			if strings.HasPrefix(input.Content, "/model") {
				args := strings.TrimSpace(strings.TrimPrefix(input.Content, "/model"))
				if args == "" {
					// List available models
					available := c.providersManager.GetAvailableProviders()
					var sb strings.Builder
					sb.WriteString("### 🤖 Available Models\n\n")
					for _, p := range available {
						icon := "🔹"
						if p.Available {
							icon = "✅"
						} else if p.HasKey {
							icon = "🔑"
						}
						sb.WriteString(fmt.Sprintf("%s **%s** (%s)\n", icon, p.Name, p.ID))
						for _, m := range p.Models {
							current := ""
							c.mu.RLock()
							if p.ID == c.config.Provider.Provider && m.ID == c.config.Provider.Model {
								current = " (current)"
							}
							c.mu.RUnlock()
							sb.WriteString(fmt.Sprintf("  - `%s:%s`%s\n", p.ID, m.ID, current))
						}
						sb.WriteString("\n")
					}
					sb.WriteString("**Usage**: `/model provider:model` (e.g. `/model anthropic:claude-3-5-sonnet`)")

					callback(ChatUpdate{
						SessionID: input.SessionID,
						Message: ChatMessage{
							ID:        uuid.New().String(),
							Role:      "assistant",
							Content:   sb.String(),
							Timestamp: time.Now().UnixMilli(),
						},
					})
					return nil
				}

				// Switch model
				parts := strings.Split(args, ":")
				if len(parts) != 2 {
					callback(ChatUpdate{
						SessionID: input.SessionID,
						Message: ChatMessage{
							ID:        uuid.New().String(),
							Role:      "assistant",
							Content:   "❌ Invalid format. Use: `/model provider:model`",
							Timestamp: time.Now().UnixMilli(),
						},
					})
					return nil
				}

				providerID := parts[0]
				modelID := parts[1]

				// Validate and get key
				apiKey := c.providersManager.GetAPIKey(providerID)
				if apiKey == "" {
					callback(ChatUpdate{
						SessionID: input.SessionID,
						Message: ChatMessage{
							ID:        uuid.New().String(),
							Role:      "assistant",
							Content:   fmt.Sprintf("❌ No API key found for provider '%s'. Please configure it in settings or .env.", providerID),
							Timestamp: time.Now().UnixMilli(),
						},
					})
					return nil
				}

				// Re-initialize provider
				newConfig := ProviderConfig{
					Provider: providerID,
					Model:    modelID,
					APIKey:   apiKey,
					BaseURL:  c.providersManager.GetBaseURL(providerID),
				}

				newProvider, err := NewProvider(newConfig)
				if err != nil {
					callback(ChatUpdate{
						SessionID: input.SessionID,
						Message: ChatMessage{
							ID:        uuid.New().String(),
							Role:      "assistant",
							Content:   fmt.Sprintf("❌ Failed to initialize provider: %v", err),
							Timestamp: time.Now().UnixMilli(),
						},
					})
					return nil
				}

				// Hot-swap
				c.mu.Lock()
				c.provider = newProvider
				c.config.Provider = newConfig
				c.defaultModel = modelID
				c.mu.Unlock()

				callback(ChatUpdate{
					SessionID: input.SessionID,
					Message: ChatMessage{
						ID:        uuid.New().String(),
						Role:      "assistant",
						Content:   fmt.Sprintf("✅ Switched to **%s** (`%s`)", newProvider.Name(), modelID),
						Timestamp: time.Now().UnixMilli(),
					},
				})
				return nil
			}

			cmdParts := strings.Split(input.Content, " ")
			cmdName := cmdParts[0]

			if c.workflows != nil {
				if wf, ok := c.workflows.GetWorkflow(cmdName); ok {
					go func() {
						// Notify workflow start
						callback(ChatUpdate{
							SessionID: input.SessionID,
							Message: ChatMessage{
								ID:        uuid.New().String(),
								Role:      "assistant", // System?
								Content:   fmt.Sprintf("🚀 Starting workflow: **%s**...", wf.Description),
								Timestamp: time.Now().UnixMilli(),
							},
						})

						// Execute Workflow
						def := workflow.WorkflowDefinition{
							Name:        wf.Command,
							Description: wf.Description,
							Steps:       wf.Steps,
						}
						res, err := c.workflowEngine.Execute(ctx, def, map[string]interface{}{
							"input": strings.TrimSpace(strings.TrimPrefix(input.Content, cmdName)),
						})

						if err != nil {
							callback(ChatUpdate{
								SessionID: input.SessionID,
								Message: ChatMessage{
									ID:        uuid.New().String(),
									Role:      "assistant",
									Content:   fmt.Sprintf("❌ Workflow failed: %v", err),
									Timestamp: time.Now().UnixMilli(),
								},
							})
							return
						}

						// Summarize results
						summary := "### Workflow Completed\n"
						for _, step := range res.History {
							icon := "✅"
							if step.Status == "failed" {
								icon = "❌"
							}
							summary += fmt.Sprintf("- %s **%s**: %s\n", icon, step.StepID, truncateString(step.Output, 100))
						}

						callback(ChatUpdate{
							SessionID: input.SessionID,
							Message: ChatMessage{
								ID:        uuid.New().String(),
								Role:      "assistant",
								Content:   summary,
								Timestamp: time.Now().UnixMilli(),
							},
						})
					}()
					return nil // Early return, handled by goroutine
				}
			}
		}

		// ─── SMART INJECTIONS (Phase 17) ───
		expandedContent, infoMsgs := c.injectionProcessor.Process(input.Content)
		for _, msg := range infoMsgs {
			callback(ChatUpdate{
				SessionID: input.SessionID,
				Message: ChatMessage{
					ID:        uuid.New().String(),
					Role:      "assistant", // informational
					Content:   msg,
					Timestamp: time.Now().UnixMilli(),
				},
			})
		}

		userMsg := protocol.Message{
			Role:    "user",
			Content: expandedContent,
			Via:     input.Via,
		}
		session.StateHandler.AddMessage(userMsg)
	}

	var totalToolCount int
	var totalTokenCount int

	// Track edited files for task progress
	taskFiles := make(map[string]bool)

	// Extract task name from user input (first 60 chars or until newline)
	dynamicTaskName := "Processing Request"
	if input.Content != "" {
		dynamicTaskName = input.Content
		if len(dynamicTaskName) > 60 {
			dynamicTaskName = dynamicTaskName[:60] + "..."
		}
		if idx := strings.Index(dynamicTaskName, "\n"); idx > 0 {
			dynamicTaskName = dynamicTaskName[:idx]
		}
	}

	// Accumulated progress steps for Antigravity-style numbered list
	var accumulatedSteps []string
	var taskSummary string
	stepCounter := 0

	// Helper to emit task progress with step accumulation
	emitTaskProgress := func(status string, newFiles []string, toolCount int, tokenCount int, result string) {
		totalToolCount += toolCount
		totalTokenCount += tokenCount
		// Update file list
		for _, f := range newFiles {
			taskFiles[f] = true
		}

		// Convert map to slice
		var fileList []string
		for f := range taskFiles {
			fileList = append(fileList, f)
		}

		// Accumulate steps with numbering (avoid duplicates)
		if status != "" && (len(accumulatedSteps) == 0 || accumulatedSteps[len(accumulatedSteps)-1] != status) {
			stepCounter++
			accumulatedSteps = append(accumulatedSteps, status)
		}

		// Generate summary from accumulated work
		if len(fileList) > 0 {
			taskSummary = fmt.Sprintf("Edited %d file(s)", len(fileList))
		} else {
			taskSummary = status
		}

		// Map agent mode slugs to protocol modes for UI badges
		protocolMode := "execution"
		switch c.modes.GetActiveMode().Slug {
		case "architect":
			protocolMode = "planning"
		case "code":
			protocolMode = "execution"
		case "test":
			protocolMode = "verification"
		}

		// Construct task payload
		progress := protocol.TaskProgress{
			TaskName:   dynamicTaskName,
			Status:     status,
			Summary:    taskSummary,
			Steps:      accumulatedSteps,
			Files:      fileList,
			IsActive:   true,
			Mode:       protocolMode,
			ToolCount:  totalToolCount,
			TokenCount: totalTokenCount,
			Result:     result,
		}

		// Persist to task_progress_current.md (User Request Parity)
		taskMdContent := fmt.Sprintf("# Task Progress: %s\n\n", progress.TaskName)
		taskMdContent += fmt.Sprintf("**Status**: %s\n", status)
		taskMdContent += fmt.Sprintf("**Summary**: %s\n", taskSummary)
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
		for i, step := range accumulatedSteps {
			taskMdContent += fmt.Sprintf("%d. %s\n", i+1, step)
		}

		// Best effort write - ignore errors to not block flow
		if cwd, err := os.Getwd(); err == nil {
			_ = os.WriteFile(filepath.Join(cwd, "task_progress_current.md"), []byte(taskMdContent), 0644)
		}

		callback(progress)
	}

	// Helper to emit chat updates matches the callback signature
	emitUpdate := func(msg ChatMessage) {
		msg.SessionID = input.SessionID // Ensure ID is on the message itself
		callback(ChatUpdate{
			SessionID: input.SessionID,
			Message:   msg,
		})
	}

	// REMOVED: Unconditional "Starting..." task emission.
	// This prevents simple chats ("Hi") from creating a task tree node.
	// Real tasks will trigger progress updates via tools or specific logic steps.

	// MAX TURNS to prevent infinite loops
	const maxTurns = 50
	currentTurn := 0
	stuckCounter := 0 // Counter for consecutive Loop Rule B errors (hard stop after 5)

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

		// LOOP DETECTION: Check if agent is stuck in repetitive pattern
		// LOOP PATTERN CHECK (Phase 1 - Tool & Error based)
		// We now check primarily for Tool/Error loops during execution.
		// Content-based check removed in favor of stricter tool signature checks.

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

		// PHASE 8: CONTEXT COMPACTION
		if c.contextManager != nil && c.contextManager.ShouldCompact(currentMessages) {
			compacted, err := c.contextManager.Compact(ctx, currentMessages, c.defaultModel)
			if err != nil {
				log.Printf("[Agent] Warning: Compaction failed: %v", err)
			} else {
				session.StateHandler.SetMessages(compacted)
				currentMessages = compacted // Update local reference

				// Notify frontend of compaction
				callback(ChatUpdate{
					SessionID: input.SessionID,
					Message: ChatMessage{
						ID:        uuid.New().String(),
						Role:      "system",
						Content:   "**Context Compacted**: History summarized to save tokens.",
						Timestamp: time.Now().UnixMilli(),
					},
				})
			}
		}

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

		// HELP AGENT INTERCEPTION
		// If query is about help, switch system prompt to Expert Help Agent
		currentSystemPrompt := c.config.SystemPrompt
		if c.helpAgent.IsHelpQuery(input.Content) {
			currentSystemPrompt = c.helpAgent.GetSystemPrompt()
			log.Printf("🤖 Help Agent Activated for query: %s", input.Content)
		}

		wm := context_manager.NewWindowManagerWithSettings(contextLimit, ctxSettings, condenseProvider)

		contextResult, err := wm.ManageContext(ctx, currentMessages, currentSystemPrompt)
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
		// Inject project-specific memory if available (Phase 15)
		memoryContext := c.memoryManager.GetSystemPromptPart()

		// Inject Plan Context (Autonomous Agent)
		planContext := c.planManager.GenerateContext()

		enhancedSystemPrompt := finalSystemPrompt + modePrompt + memoryContext + rulesContext + skillContext + planContext + "\n\n" + c.envTracker.GetContext() + "\n" + session.FileTracker.GetContext()

		// Use contextResult.Messages as prunedMessages
		prunedMessages := contextResult.Messages

		// SAFETY: Sanitize messages to ensure Tool Call/Result integrity
		// This prevents API 400 errors if a previous session crashed/was pruned incorrectly
		prunedMessages = c.sanitizeMessages(prunedMessages)

		// =============================
		// EPHEMERAL MESSAGE INJECTION
		// =============================
		// Normalize mode name for ephemeral messages
		normalizedMode := normalizeModeName(activeMode.Name)

		// Detect if we're in task mode (basic heuristic: has todos or artifacts)
		isInTaskMode := len(session.Todos) > 0

		// TODO: Detect artifacts by checking for task.md, implementation_plan.md, etc.
		// TODO: Track tool failures across turns
		// TODO: Detect if plan exists

		// Build dynamic context for ephemeral reminders
		ephemeralCtx := prompts.EphemeralContext{
			Mode:             normalizedMode,
			IsInTaskMode:     isInTaskMode,
			ToolCallCount:    0, // Will be tracked in future turns
			HasPlan:          false,
			LastToolFailed:   false,
			ArtifactsCreated: []string{},
		}

		ephemeralMsg := prompts.BuildEphemeralMessage(ephemeralCtx)
		if ephemeralMsg != "" {
			// Inject as final user message to maximize adherence
			prunedMessages = append(prunedMessages, protocol.Message{
				Role:    "user",
				Content: ephemeralMsg,
			})
			log.Printf("📨 Ephemeral message injected (mode=%s, inTask=%v)", normalizedMode, isInTaskMode)
		}

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

		// LOOP PREVENTION: Content deduplication and reasoning limits
		var lastEmittedContentLen int   // Track last emitted content length to detect actual changes
		var lastEmittedReasoningLen int // Track last emitted reasoning length
		var reasoningChunkCount int     // Count reasoning chunks to prevent infinite thinking
		const maxReasoningChunks = 500  // Hard limit on reasoning iterations
		var consecutiveEmptyDeltas int  // Track consecutive empty deltas
		const maxEmptyDeltas = 10       // Stop if too many empty deltas in a row

		// Stream response from AI using standard ChatStream
		// We use prunedMessages (from context management) instead of session messages
		err = c.provider.ChatStream(ctx, req, func(chunk *StreamChunk) error {
			switch chunk.Type {
			case "content_block_delta":
				// LOOP PREVENTION: Check for empty delta spam
				if len(chunk.Delta) == 0 {
					consecutiveEmptyDeltas++
					if consecutiveEmptyDeltas > maxEmptyDeltas {
						log.Printf("⚠️ Too many empty deltas (%d), possible loop detected", consecutiveEmptyDeltas)
						return nil // Skip but don't error
					}
				} else {
					consecutiveEmptyDeltas = 0 // Reset counter
				}

				currentTurnContent += chunk.Delta
				assistantMsg.Content += chunk.Delta
				assistantMsg.IsStreaming = true

				// Accumulate reasoning separately for DeepSeek R1 tool call support
				if chunk.ReasoningDelta != "" {
					currentTurnReasoning += chunk.ReasoningDelta
					assistantMsg.Reasoning += chunk.ReasoningDelta
					reasoningChunkCount++

					// LOOP PREVENTION: Max reasoning guard
					if reasoningChunkCount > maxReasoningChunks {
						log.Printf("⚠️ Max reasoning chunks exceeded (%d), forcing completion", reasoningChunkCount)
						return fmt.Errorf("reasoning limit exceeded - agent may be stuck in thought loop")
					}
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
				shouldEmit := firstChunk || isReasoningTag || now.Sub(lastEmitTime) >= streamThrottleInterval

				// LOOP PREVENTION: Only emit if content OR reasoning actually changed
				contentChanged := len(assistantMsg.Content) > lastEmittedContentLen
				reasoningChanged := len(assistantMsg.Reasoning) > lastEmittedReasoningLen

				if shouldEmit && (contentChanged || reasoningChanged) {
					emitUpdate(assistantMsg)
					lastEmitTime = now
					lastEmittedContentLen = len(assistantMsg.Content)
					lastEmittedReasoningLen = len(assistantMsg.Reasoning)
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

		// LOOP DETECTION: Track this turn's content
		if c.loopDetector != nil && currentTurnContent != "" {
			// LOOP DETECTION: Content check removed. Relying on Tool/Error loop detection.
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

		// ─── BATCH TOOL CONFIRMATION (Phase 19) ───
		if len(currentTurnToolCalls) > 0 {
			// ─── PLAN MODE GUARDRAIL: Hard block Write/Execute tools ───
			var blockedResults []protocol.ToolResultBlock
			for _, tc := range currentTurnToolCalls {
				if err := c.validateToolUse(tc.Name, input.PlanMode); err != nil {
					// Return error to LLM instead of executing - this teaches the agent
					blockedResults = append(blockedResults, protocol.ToolResultBlock{
						ToolUseID: tc.ID,
						Content:   err.Error(),
						IsError:   true,
					})
				}
			}
			// If any tools were blocked, send errors back to LLM and continue to next turn
			if len(blockedResults) > 0 {
				session.StateHandler.AddMessage(protocol.Message{
					Role:        "user",
					ToolResults: blockedResults,
				})
				continue // Go to next turn (AI will see the error and adjust)
			}

			needsApproval := false
			var summary strings.Builder
			summary.WriteString("The agent wants to execute the following tools:\n\n")

			for _, tc := range currentTurnToolCalls {
				if !c.isToolAutoApproved(tc, input.PlanMode) {
					needsApproval = true
				}
				summary.WriteString(fmt.Sprintf("• **%s**\n  %s\n", tc.Name, c.formatToolCall(tc)))
			}

			if needsApproval {
				// Set terminal title to show action required
				terminal.SetTerminalTitle(terminal.StateActionRequired)
				// Pause thinking status if we have one
				emitTaskProgress("Waiting for approval...", nil, 0, 0, "")

				choices := []string{
					"Yes",
					"Yes, and don't ask again for this tool",
					"No",
				}

				choiceIdx, err := c.host.AskUserChoice(summary.String(), choices)
				if err != nil {
					return fmt.Errorf("approval failed: %w", err)
				}

				// 0 = Yes
				// 1 = Yes + Whitelist (runtime only for now)
				// 2 = No

				if choiceIdx == 2 {
					// User denied. Send rejection messages for all tools.
					var toolResults []protocol.ToolResultBlock
					for _, tc := range currentTurnToolCalls {
						toolResults = append(toolResults, protocol.ToolResultBlock{
							ToolUseID: tc.ID,
							Content:   "User denied execution of this tool.",
							IsError:   true,
						})
					}
					session.StateHandler.AddMessage(protocol.Message{
						Role:        "user",
						ToolResults: toolResults,
					})
					continue // Go to next turn (AI will react to rejection)
				}

				if choiceIdx == 1 {
					// Whitelist the tools in this batch
					for _, tc := range currentTurnToolCalls {
						if c.config.AutoApproval != nil {
							// Naive runtime whitelist: just toggle the broad category if applicable?
							// Better: we can't easily persist fine-grained rules yet without config overhaul.
							// For now, let's just enable the category for this session.
							switch tc.Name {
							case "read_file", "list_dir":
								c.config.AutoApproval.ReadFiles = true
							case "execute_command":
								c.config.AutoApproval.ExecuteSafeCommands = true
							}
						}
					}
				}
			}
		}

		// EXECUTE TOOLS
		log.Printf("Executing %d tools...", len(currentTurnToolCalls))
		var toolResults []protocol.ToolResultBlock
		for i, tc := range currentTurnToolCalls {
			// Prettify tool name for progress
			// Prettify tool name for progress
			friendlyTool := c.formatToolCall(tc)
			// Use friendlyTool as the status so it shows up nicely in the tree
			emitTaskProgress(friendlyTool, nil, 0, 0, "")

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

			// LOOP DETECTOR: Rule A (Stupidity Check)
			if c.loopDetector != nil {
				if loopErr := c.loopDetector.CheckTool(tc.Name, tc.Arguments); loopErr != nil {
					log.Printf("🛑 Loop Rule A: %v", loopErr)
					err = loopErr
					// We act as if execution failed immediately
				}
			}

			// HOOKIFY: Dynamic Safety Checks
			if c.dynamicHooks != nil {
				var hookArgs map[string]interface{}
				if json.Unmarshal([]byte(tc.Arguments), &hookArgs) == nil {
					warnMsg, blockErr := c.dynamicHooks.CheckPreToolUse(tc.Name, hookArgs)
					if blockErr != nil {
						err = blockErr
						log.Printf("🚫 Hook Action Blocked: %v", err)
					} else if warnMsg != "" {
						log.Printf("⚠️ Hook Warning: %s", warnMsg)
						// Inject warning into the result effectively
						result = fmt.Sprintf("[HOOK WARNING] %s\n\n", warnMsg)
					}
				}
			}

			if err == nil {
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
				case "task_boundary":
					var payload struct {
						TaskName          string `json:"TaskName"`
						Mode              string `json:"Mode"`
						TaskSummary       string `json:"TaskSummary"`
						TaskStatus        string `json:"TaskStatus"`
						PredictedTaskSize int    `json:"PredictedTaskSize"`
					}
					if err = json.Unmarshal([]byte(tc.Arguments), &payload); err == nil {
						// 1. Update dynamic task name
						if payload.TaskName != "" && payload.TaskName != "%SAME%" {
							dynamicTaskName = payload.TaskName
						}
						// 2. Update mode if changed
						if payload.Mode != "" && payload.Mode != "%SAME%" {
							newMode := strings.ToLower(payload.Mode)
							// Map protocol modes back to agent slugs if needed
							switch newMode {
							case "planning":
								newMode = "architect"
							case "execution":
								newMode = "code"
							case "verification":
								newMode = "test"
							}
							c.modes.SetMode(newMode)
							log.Printf("🔄 Mode switched via task_boundary: %s (agent mode: %s)", payload.Mode, newMode)
						}
						// 3. Update summary if changed
						if payload.TaskSummary != "" && payload.TaskSummary != "%SAME%" {
							taskSummary = payload.TaskSummary
						}
						// 4. Emit progress update with the new status
						status := payload.TaskStatus
						if status == "%SAME%" {
							status = "" // Don't add a new step if status is same
						}
						emitTaskProgress(status, nil, 0, 0, "")
						result = "Task boundary updated."
					} else {
						result = fmt.Sprintf("Error parsing task_boundary args: %v", err)
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
				case "start_swarm":
					log.Printf("🐝 activating SWARM MODE")
					c.swarm.Start(ctx)
					result = "🐝 Swarm Mode Activated! Sub-agents are now processing runnable tasks in parallel."

				case "update_plan":
					var payload struct {
						TaskID       string   `json:"task_id"`
						Status       string   `json:"status"`
						Dependencies []string `json:"dependencies"`
					}
					if err = json.Unmarshal([]byte(tc.Arguments), &payload); err == nil {
						// Update Status
						if upErr := c.planManager.UpdateTask(payload.TaskID, payload.Status); upErr != nil {
							result = fmt.Sprintf("Failed to update plan status: %v", upErr)
						} else {
							result = fmt.Sprintf("Updated task %s -> %s.", payload.TaskID, payload.Status)
						}

						// Update Dependencies if provided
						if len(payload.Dependencies) > 0 {
							if depErr := c.planManager.UpdateTaskDependencies(payload.TaskID, payload.Dependencies); depErr != nil {
								result += fmt.Sprintf(" (Failed to set deps: %v)", depErr)
							} else {
								result += fmt.Sprintf(" Set dependencies: %v.", payload.Dependencies)
							}
						}
					} else {
						result = fmt.Sprintf("Error parsing update_plan args: %v", err)
					}
				default:
					result, err = c.executor.Execute(ctx, tc.Name, json.RawMessage(tc.Arguments))
				}
			}
			isError := false
			if err != nil {
				log.Printf("Tool execution failed: %v", err)
				result = TranslateError(err)
				isError = true
				currentTurnToolCalls[i].Status = "error"

				// LOOP DETECTOR: Rule B (Insanity Check)
				if c.loopDetector != nil {
					if loopErr := c.loopDetector.CheckError(result); loopErr != nil {
						log.Printf("🛑 Loop Rule B: %v", loopErr)
						stuckCounter++
						result += fmt.Sprintf("\n\nCRITICAL: %v", loopErr)

						// HARD STOP: Force-break after 5 consecutive stuck errors
						if stuckCounter >= 5 {
							log.Printf("🛑 HARD STOP: Agent stuck in infinite loop (%d consecutive errors). Aborting.", stuckCounter)
							assistantMsg.Content = "🛑 Agent was stuck in an infinite loop and has been stopped. Please try a different approach or restart the conversation."
							assistantMsg.IsStreaming = false
							emitUpdate(assistantMsg)
							return fmt.Errorf("agent stuck: loop detected %d times consecutively", stuckCounter)
						}
						err = loopErr
					}
				}
			} else {
				currentTurnToolCalls[i].Status = "completed"
				stuckCounter = 0 // Reset stuck counter on successful tool execution
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

			// Emitting result for TUI high-fidelity output
			// Re-emit with the same friendly name so it updates the same node (or appends, tree logic handles it)
			emitTaskProgress(friendlyTool, nil, 1, 0, result)

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
						emitTaskProgress(fmt.Sprintf("Edited %s", filepath.Base(target)), []string{target}, 0, 0, "")
					}
				}
			}

			// Auto-checkpoint after write operations (Phase 18)
			if !isError && c.checkpointManager != nil && isWriteTool(tc.Name) {
				// Detect target file for specific snapshot
				var targetFiles []string
				var argsMap map[string]interface{}
				if json.Unmarshal([]byte(tc.Arguments), &argsMap) == nil {
					if t, ok := argsMap["TargetFile"].(string); ok {
						targetFiles = append(targetFiles, t)
					} else if t, ok := argsMap["path"].(string); ok {
						targetFiles = append(targetFiles, t)
					} else if t, ok := argsMap["AbsolutePath"].(string); ok {
						targetFiles = append(targetFiles, t)
					}
				}

				cpID, cpErr := c.checkpointManager.Save(fmt.Sprintf("Auto: After %s", tc.Name), targetFiles)
				if cpErr == nil && cpID != "" {
					assistantMsg.CheckpointHash = cpID
					log.Printf("📸 Auto-Checkpoint saved: %s (after %s)", cpID[:8], tc.Name)
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
				// Dangling tool call at end of history
				// Instead of dropping, append a placeholder result to finalize the turn
				log.Printf("⚠️ Sanitizer: Fixing dangling tool call at end (ID: %s)", msg.ToolUse[0].ID)
				clean = append(clean, msg)
				clean = append(clean, protocol.Message{
					Role: "user",
					ToolResults: []protocol.ToolResultBlock{{
						ToolUseID: msg.ToolUse[0].ID,
						Content:   "Tool execution interrupted or result lost.",
						IsError:   true,
					}},
				})
				continue
			}

			nextMsg := msgs[i+1]
			if nextMsg.Role != "user" || len(nextMsg.ToolResults) == 0 {
				// Next message is NOT a result (e.g., User text or another Assistant msg)
				// We MUST provide a result for the tool call to be valid.
				log.Printf("⚠️ Sanitizer: Injecting missing result for tool call (ID: %s) followed by %s", msg.ToolUse[0].ID, nextMsg.Role)

				// Keep the tool call
				clean = append(clean, msg)

				// Create synthetic result
				syntheticResult := protocol.ToolResultBlock{
					ToolUseID: msg.ToolUse[0].ID,
					Content:   "Tool execution result missing (interrupted or lost).",
					IsError:   true,
				}

				// If next is User message, merge the synthetic result into it
				if nextMsg.Role == "user" {
					// Merge
					nextMsg.ToolResults = append([]protocol.ToolResultBlock{syntheticResult}, nextMsg.ToolResults...)
					clean = append(clean, nextMsg)
					skipNext = true
				} else {
					// Insert separate User message with result
					clean = append(clean, protocol.Message{
						Role:        "user",
						ToolResults: []protocol.ToolResultBlock{syntheticResult},
					})
					// Do NOT skip next (process it as normal next message)
				}
				continue
			}

			// Valid Pair: Assistant(Tool) -> User(Result)
			clean = append(clean, msg)
			clean = append(clean, nextMsg)
			skipNext = true
			continue
		}

		// Drop orphan tool results (User msg with results but no preceding call)
		// This happens if we dropped the call, or if history is corrupted.
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
	if session == nil {
		return map[string]interface{}{
			"messages":        []interface{}{},
			"liveModeEnabled": false,
			"mode":            "code",
			"todos":           []protocol.Todo{},
		}
	}

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
	case "task_boundary":
		activity.Type = "task_boundary"
		if tn, ok := argsMap["TaskName"].(string); ok {
			activity.File = tn
		}
		if ts, ok := argsMap["TaskStatus"].(string); ok {
			activity.Message = ts
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

// SetCheckpointManager sets the checkpoint manager for auto-saving
func (c *Controller) SetCheckpointManager(mgr *CheckpointManager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkpointManager = mgr
}

// normalizeModeName converts mode names to match ephemeral message expectations
// Maps various mode names to: "planning", "execution", "verification", or "code"
func normalizeModeName(modeName string) string {
	lower := strings.ToLower(modeName)

	// Map common variations
	switch {
	case strings.Contains(lower, "plan"):
		return "planning"
	case strings.Contains(lower, "exec") || strings.Contains(lower, "implement") || strings.Contains(lower, "build"):
		return "execution"
	case strings.Contains(lower, "verify") || strings.Contains(lower, "test") || strings.Contains(lower, "check"):
		return "verification"
	default:
		// Default: code mode
		return "code"
	}
}

// Execute implements workflow.AgentExecutor interface
// It allows the workflow engine to trigger agent actions
func (c *Controller) formatToolCall(tc ToolCallInfo) string {
	var args map[string]interface{}
	// If unmarshal fails, return raw string to avoid hiding info
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return tc.Arguments
	}

	// Helper to get string from map with multiple possible keys
	getStr := func(keys ...string) (string, bool) {
		for _, k := range keys {
			if v, ok := args[k].(string); ok {
				return v, true
			}
		}
		return "", false
	}

	switch tc.Name {
	case "read_file", "view_file", "view_file_outline":
		if path, ok := getStr("path", "AbsolutePath", "file", "target"); ok {
			return fmt.Sprintf("Read file **%s**", path)
		}
	case "list_dir":
		if path, ok := getStr("DirectoryPath", "path", "dir"); ok {
			return fmt.Sprintf("List directory **%s**", path)
		}
	case "write_file", "write_to_file":
		if path, ok := getStr("path", "TargetFile", "file"); ok {
			return fmt.Sprintf("Write to file **%s**", path)
		}
	case "replace_file_content", "multi_replace_file_content":
		if path, ok := getStr("TargetFile", "path", "file"); ok {
			return fmt.Sprintf("Edit file **%s**", path)
		}
	case "execute_command", "run_command":
		if cmd, ok := getStr("command", "CommandLine", "cmd"); ok {
			return fmt.Sprintf("Run command: `%s`", cmd)
		}
	case "grep_search":
		if q, ok := getStr("Query", "query", "pattern"); ok {
			return fmt.Sprintf("Search for \"%s\"", q)
		}
	case "codebase_search":
		if q, ok := getStr("Query", "query"); ok {
			return fmt.Sprintf("Semantic search: \"%s\"", q)
		}
	case "task_boundary":
		// Return empty string to HIDE this from the visual tree
		// The TUI listens to the actual event, so we don't need a tree node for the tool call itself
		return ""
	case "update_todos", "get_context_stats":
		return ""
	}

	// Fallback: try to find a meaningful "path" or "command" generic key
	if p, ok := getStr("path", "file"); ok {
		return fmt.Sprintf("%s **%s**", tc.Name, p)
	}

	return tc.Arguments
}

func (c *Controller) Execute(ctx context.Context, prompt string) (string, error) {
	// Create a temporary session for this step execution
	session := c.CreateSession()

	req := ChatRequestInput{
		SessionID: session.ID,
		Content:   prompt,
		Via:       "workflow_engine",
	}

	var responseBuilder strings.Builder
	var mu sync.Mutex

	// Helper to collect response
	err := c.Chat(ctx, req, func(update interface{}) {
		if cu, ok := update.(ChatUpdate); ok {
			if cu.Message.Role == "assistant" && cu.Message.Content != "" {
				mu.Lock()
				responseBuilder.WriteString(cu.Message.Content)
				mu.Unlock()
			}
		}
	})

	if err != nil {
		return "", err
	}

	return responseBuilder.String(), nil
}

// GetMemory returns the current persistent memory
func (c *Controller) GetMemory() (string, error) {
	return c.memoryManager.GetSystemPromptPart(), nil
}

// AddMemory appends a new entry to persistent memory
func (c *Controller) AddMemory(content string) error {
	return c.memoryManager.AddLegacy(content)
}

// ClearMemory wipes the persistent memory
func (c *Controller) ClearMemory() error {
	return c.memoryManager.Clear()
}

// GetActiveHooks returns a list of active dynamic hooks
func (c *Controller) GetActiveHooks() []string {
	if c.dynamicHooks == nil {
		return nil
	}
	hooks := c.dynamicHooks.ListHooks()
	result := make([]string, len(hooks))
	for i, h := range hooks {
		result[i] = fmt.Sprintf("%s (%s)", h.Name, h.Event)
	}
	return result
}

// InitProject performs automated discovery and populates memory
func (c *Controller) InitProject(ctx context.Context) (string, error) {
	scanner := NewProjectScanner(c.envTracker.GetCwd(), c)
	summary, err := scanner.ScanProject(ctx)
	if err != nil {
		return "", err
	}

	// Save to memory
	err = c.memoryManager.SetRaw("project_summary", summary)
	if err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}

	return summary, nil
}

// GetProvidersManager returns the providers manager
func (c *Controller) GetProvidersManager() *config.ProvidersManager {
	return c.providersManager
}

// --- Checkpoint Management (Phase 18) ---

// SaveCheckpoint creates a manual snapshot of current workspace files
func (c *Controller) SaveCheckpoint(name string, files []string) (string, error) {
	return c.checkpointManager.Save(name, files)
}

// ListCheckpoints returns all project snapshots
func (c *Controller) ListCheckpoints() ([]Checkpoint, error) {
	return c.checkpointManager.List()
}

// RestoreCheckpoint reverts project to a specific state
func (c *Controller) RestoreCheckpoint(idOrName string) error {
	return c.checkpointManager.Restore(idOrName)
}

// isToolAutoApproved checks if a tool call can proceed without manual confirmation.
// Uses Category-Based Permission System instead of hardcoded tool name lists.
func (c *Controller) isToolAutoApproved(tc ToolCallInfo, planMode bool) bool {
	category := tools.GetToolCategory(tc.Name)

	// ─── META TOOLS: ALWAYS ALLOW (Silent) ───
	// These tools have no side effects on the project files or system.
	if category == tools.CategoryMeta {
		return true
	}

	// ─── READ TOOLS: ALWAYS ALLOW (Silent) ───
	// Read-only operations should NEVER interrupt the user's flow.
	// This is unconditional - reading files is always safe.
	if category == tools.CategoryRead {
		return true
	}

	// ─── WRITE TOOLS: Plan Mode = BLOCKED, Act Mode = AUTO-APPROVE ───
	if category == tools.CategoryWrite {
		if planMode {
			// In Plan Mode, write tools are blocked (handled by validateToolUse)
			return false
		}
		// ACT MODE: Auto-approve write operations
		return true
	}

	// ─── EXECUTE TOOLS: Plan Mode = BLOCKED, Act Mode = AUTO-APPROVE ───
	if category == tools.CategoryExecute {
		if planMode {
			// In Plan Mode, execute tools are blocked
			return false
		}
		// ACT MODE: Auto-approve command execution
		return true
	}

	// ─── BROWSER TOOLS: Plan Mode = ASK, Act Mode = AUTO-APPROVE ───
	if category == tools.CategoryBrowser {
		if planMode {
			// In Plan Mode, browser tools require explicit approval
			if c.config.AutoApproval != nil && c.config.AutoApproval.UseBrowser {
				return true
			}
			return false
		}
		// ACT MODE: Auto-approve browser operations
		return true
	}

	// ─── MCP / UNKNOWN TOOLS: Default to requiring approval ───
	// Safety first for external/unknown tools
	return false
}

// validateToolUse implements the Plan Mode Guardrail.
// Returns error if Write/Execute tools are attempted in Plan Mode.
func (c *Controller) validateToolUse(toolName string, planMode bool) error {
	category := tools.GetToolCategory(toolName)

	// STRICT RULE: No side effects in Plan Mode
	if planMode {
		if category == tools.CategoryWrite {
			return fmt.Errorf("⚠️ Action denied: Tool '%s' (category: %s) is forbidden in PLAN MODE. Please switch to Act Mode using 'switch_mode' or complete your planning phase.", toolName, category)
		}
		if category == tools.CategoryExecute {
			return fmt.Errorf("⚠️ Action denied: Tool '%s' (category: %s) is forbidden in PLAN MODE. Shell commands require Act Mode.", toolName, category)
		}
	}
	return nil
}

// GetSafeguard returns the safeguard manager
func (c *Controller) GetSafeguard() *safeguard.Manager {
	return c.safeguard
}

// SetOnTaskProgress sets the callback for task progress updates
func (c *Controller) SetOnTaskProgress(callback func(protocol.TaskProgress)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onTaskProgress = callback
}

// ReportTaskProgress sends a progress update to the UI
func (c *Controller) ReportTaskProgress(ctx context.Context, progress protocol.TaskProgress) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.onTaskProgress != nil {
		c.onTaskProgress(progress)
	}
}
