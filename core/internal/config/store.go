package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ContextSettings controls context window management
type ContextSettings struct {
	AutoCondense         bool `json:"auto_condense"`          // Enable automatic context condensation
	CondenseThreshold    int  `json:"condense_threshold"`     // % of context at which to trigger condensation (default: 70)
	SlidingWindowSize    int  `json:"sliding_window_size"`    // Fallback: how many messages to keep (default: 20)
	ShowContextIndicator bool `json:"show_context_indicator"` // Show context % in UI
	EnableCheckpoints    bool `json:"enable_checkpoints"`     // Enable workspace checkpointing
	CheckpointOnWrites   bool `json:"checkpoint_on_writes"`   // Auto-checkpoint after write operations
	EnableCodeIndex      bool `json:"enable_code_index"`      // Enable codebase indexing for semantic search
}

// AutoApprovalSettings controls which actions can run without user confirmation
type AutoApprovalSettings struct {
	Enabled             bool `json:"enabled"`               // Master switch for auto-approval
	ReadFiles           bool `json:"read_files"`            // Read files in workspace
	ReadFilesExternal   bool `json:"read_files_external"`   // Read files outside workspace
	EditFiles           bool `json:"edit_files"`            // Edit files in workspace
	EditFilesExternal   bool `json:"edit_files_external"`   // Edit files outside workspace
	ExecuteSafeCommands bool `json:"execute_safe_commands"` // Run safe commands (ls, cat, etc.)
	ExecuteAllCommands  bool `json:"execute_all_commands"`  // Run any command (dangerous!)
	DeleteFiles         bool `json:"delete_files"`          // Delete files in workspace
	DeleteFilesExternal bool `json:"delete_files_external"` // Delete files outside workspace
	UseBrowser          bool `json:"use_browser"`           // Browser automation
	UseMCP              bool `json:"use_mcp"`               // MCP server tools
	EnableNotifications bool `json:"enable_notifications"`  // Enable system notifications
}

type Settings struct {
	Provider     ProviderSettings     `json:"provider"`
	LiveMode     LiveModeSettings     `json:"live_mode"`
	Context      ContextSettings      `json:"context"`
	AutoApproval AutoApprovalSettings `json:"auto_approval"`
	Theme        string               `json:"theme"`
}

type ProviderSettings struct {
	Provider          string            `json:"provider"` // "anthropic", "openai", "openrouter"
	Model             string            `json:"model"`
	APIKey            string            `json:"api_key"`                      // Legacy single key (backwards compat)
	APIKeys           map[string]string `json:"api_keys,omitempty"`           // Per-provider keys
	EmbeddingProvider string            `json:"embedding_provider,omitempty"` // Separate provider for embeddings (e.g. openai)
	EmbeddingModel    string            `json:"embedding_model,omitempty"`    // Model for embeddings
}

type LiveModeSettings struct {
	Enabled        bool    `json:"enabled"`
	TelegramToken  string  `json:"telegram_token"`
	TelegramChatID int64   `json:"telegram_chat_id"`
	AllowedUserIDs []int64 `json:"allowed_user_ids"`
}

type Store struct {
	mu       sync.RWMutex
	path     string
	settings *Settings
}

func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ricochet")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	// Default provider - user must enter their own API key in Settings
	// NO hardcoded keys - security first!
	defaultProvider := "deepseek"
	defaultModel := "deepseek-chat"
	defaultAPIKey := "" // User enters in Settings UI

	// Optional: Check for dev environment variables (local dev only)
	if envKey := os.Getenv("RICOCHET_DEEPSEEK_KEY"); envKey != "" {
		defaultAPIKey = envKey
	} else if envKey := os.Getenv("RICOCHET_GEMINI_KEY"); envKey != "" {
		defaultProvider = "gemini"
		defaultModel = "gemini-3-flash"
		defaultAPIKey = envKey
	} else if envKey := os.Getenv("RICOCHET_MOONSHOT_KEY"); envKey != "" {
		defaultProvider = "moonshot"
		defaultModel = "moonshot-v1-8k"
		defaultAPIKey = envKey
	} else if envKey := os.Getenv("RICOCHET_ZHIPU_KEY"); envKey != "" {
		defaultProvider = "zhipu"
		defaultModel = "glm-4-flash"
		defaultAPIKey = envKey
	}

	store := &Store{
		path: filepath.Join(configDir, "settings.json"),
		settings: &Settings{
			Provider: ProviderSettings{
				Provider: defaultProvider,
				Model:    defaultModel,
				APIKey:   defaultAPIKey,
			},
			LiveMode: LiveModeSettings{},
			Context: ContextSettings{
				AutoCondense:         true,
				CondenseThreshold:    70,
				SlidingWindowSize:    20,
				ShowContextIndicator: true,
				EnableCheckpoints:    true,
				CheckpointOnWrites:   true,
				EnableCodeIndex:      true,
			},
			AutoApproval: AutoApprovalSettings{
				Enabled:             true,
				ReadFiles:           true,  // Safe: reading workspace files
				ReadFilesExternal:   false, // Unsafe: external files need approval
				EditFiles:           false, // Unsafe: edits need approval
				EditFilesExternal:   false, // Unsafe: external edits need approval
				ExecuteSafeCommands: true,  // Safe: ls, cat, etc.
				ExecuteAllCommands:  false, // Unsafe: any command needs approval
				UseBrowser:          false, // Disabled by default
				UseMCP:              true,  // MCP tools are generally safe
			},
			Theme: "dark",
		},
	}

	if err := store.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load settings: %w", err)
		}
		// If file doesn't exist, save default
		if err := store.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default settings: %w", err)
		}
	}

	return store, nil
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to parse settings.json: %w", err)
	}

	s.settings = &settings
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.settings
}

func (s *Store) Update(fn func(*Settings)) error {
	s.mu.Lock()
	fn(s.settings)
	s.mu.Unlock()
	return s.Save()
}
