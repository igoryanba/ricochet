package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProvidersConfig holds server-side providers configuration
type ProvidersConfig struct {
	Providers       map[string]ProviderConfig `yaml:"providers"`
	DefaultProvider string                    `yaml:"default_provider"`
	DefaultModel    string                    `yaml:"default_model"`
	BYOK            BYOKConfig                `yaml:"byok"`
}

// ProviderConfig defines a single provider's server configuration
type ProviderConfig struct {
	Enabled bool          `yaml:"enabled"`
	Key     string        `yaml:"key"`      // Can be ${ENV_VAR} reference
	BaseURL string        `yaml:"base_url"` // Optional custom endpoint
	Models  []ModelConfig `yaml:"models"`
}

// ModelConfig defines a model available from the provider
type ModelConfig struct {
	ID            string  `yaml:"id"`
	Name          string  `yaml:"name"`
	ContextWindow int     `yaml:"context_window"`
	InputPrice    float64 `yaml:"input_price"`
	OutputPrice   float64 `yaml:"output_price"`
	IsFree        bool    `yaml:"free"`
	SupportsTools bool    `yaml:"supports_tools"`
}

// BYOKConfig defines bring-your-own-key settings
type BYOKConfig struct {
	Enabled             bool `yaml:"enabled"`
	ShowServerProviders bool `yaml:"show_server_providers"`
}

// AvailableProvider is returned to frontend
type AvailableProvider struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	HasKey    bool             `json:"hasKey"`    // Server has key configured
	Available bool             `json:"available"` // User can use (server key OR BYOK)
	Models    []AvailableModel `json:"models"`
}

// AvailableModel is returned to frontend
type AvailableModel struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ContextWindow int     `json:"contextWindow"`
	InputPrice    float64 `json:"inputPrice"`
	OutputPrice   float64 `json:"outputPrice"`
	IsFree        bool    `json:"isFree"`
	SupportsTools bool    `json:"supportsTools"`
}

// ProvidersManager handles loading and querying providers config
type ProvidersManager struct {
	config   *ProvidersConfig
	userKeys map[string]string // User-provided keys from Settings
}

// NewProvidersManager creates a new providers manager
func NewProvidersManager(configPath string) (*ProvidersManager, error) {
	pm := &ProvidersManager{
		userKeys: make(map[string]string),
	}

	// Load local dev env file first (if exists)
	pm.loadEnvLocal()

	// Try to load config file
	if configPath != "" {
		if err := pm.loadConfig(configPath); err != nil {
			// Config file optional - use defaults
			pm.config = pm.defaultConfig()
		}
	} else {
		pm.config = pm.defaultConfig()
	}

	// Resolve environment variables in keys
	pm.resolveEnvVars()

	return pm, nil
}

// loadConfig loads providers config from yaml file
func (pm *ProvidersManager) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	pm.config = &ProvidersConfig{}
	return yaml.Unmarshal(data, pm.config)
}

// loadEnvLocal loads .env.local file for local development
// This file is gitignored and contains developer API keys
func (pm *ProvidersManager) loadEnvLocal() {
	// Get executable directory for relative paths
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)

	// Get home directory
	home, _ := os.UserHomeDir()

	// Look for .env.local in various locations
	paths := []string{
		// Absolute dev paths (Ricochet project)
		"/Users/igoryan_dao/GRIKAI/Ricochet/core/config/.env.local",
		// Relative to executable (bin/darwin-arm64 -> ../../core/config)
		filepath.Join(execDir, "..", "..", "core", "config", ".env.local"),
		// Current directory variations
		"config/.env.local",
		"core/config/.env.local",
		".env.local",
		// Home directory
		filepath.Join(home, ".ricochet", ".env.local"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "[Providers] Found .env.local at: %s\n", path)

		// Parse KEY=VALUE lines
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				os.Setenv(key, value)
				fmt.Fprintf(os.Stderr, "[Providers] Set ENV: %s=***\n", key)
			}
		}
		return // Only load first found file
	}
	fmt.Fprintf(os.Stderr, "[Providers] No .env.local found in search paths\n")
}

// resolveEnvVars replaces ${ENV_VAR} with actual values from environment
func (pm *ProvidersManager) resolveEnvVars() {
	for id, p := range pm.config.Providers {
		if strings.HasPrefix(p.Key, "${") && strings.HasSuffix(p.Key, "}") {
			envVar := p.Key[2 : len(p.Key)-1]
			val := os.Getenv(envVar)
			fmt.Fprintf(os.Stderr, "[Providers] Resolving %s -> (len=%d)\n", p.Key, len(val))
			p.Key = val
			pm.config.Providers[id] = p
		}
	}
}

// SetUserKey sets a user-provided API key for a provider
func (pm *ProvidersManager) SetUserKey(providerID, key string) {
	pm.userKeys[providerID] = key
}

// GetAvailableProviders returns providers available to the user
func (pm *ProvidersManager) GetAvailableProviders() []AvailableProvider {
	result := make([]AvailableProvider, 0)

	providerNames := map[string]string{
		"gemini":    "Google Gemini",
		"deepseek":  "DeepSeek",
		"anthropic": "Anthropic (Claude)",
		"openai":    "OpenAI",
		"xai":       "xAI (Grok)",
		"minimax":   "MiniMax",
	}

	for id, p := range pm.config.Providers {
		if !p.Enabled {
			continue
		}

		hasServerKey := p.Key != ""
		hasUserKey := pm.userKeys[id] != ""
		available := hasServerKey || (pm.config.BYOK.Enabled && hasUserKey)

		models := make([]AvailableModel, 0, len(p.Models))
		for _, m := range p.Models {
			models = append(models, AvailableModel{
				ID:            m.ID,
				Name:          m.Name,
				ContextWindow: m.ContextWindow,
				InputPrice:    m.InputPrice,
				OutputPrice:   m.OutputPrice,
				IsFree:        m.IsFree,
				SupportsTools: m.SupportsTools,
			})
		}

		name := providerNames[id]
		if name == "" {
			name = id
		}

		result = append(result, AvailableProvider{
			ID:        id,
			Name:      name,
			HasKey:    hasServerKey,
			Available: available,
			Models:    models,
		})
	}

	return result
}

// GetAPIKey returns the API key to use for a provider (server key or user key)
func (pm *ProvidersManager) GetAPIKey(providerID string) string {
	// User key takes priority
	if key := pm.userKeys[providerID]; key != "" {
		return key
	}
	// Fallback to server key
	if p, ok := pm.config.Providers[providerID]; ok {
		return p.Key
	}
	return ""
}

// GetBaseURL returns custom base URL for a provider if configured
func (pm *ProvidersManager) GetBaseURL(providerID string) string {
	if p, ok := pm.config.Providers[providerID]; ok {
		return p.BaseURL
	}
	return ""
}

// GetDefaultProvider returns the default provider ID
func (pm *ProvidersManager) GetDefaultProvider() string {
	if pm.config.DefaultProvider != "" {
		return pm.config.DefaultProvider
	}
	return "deepseek"
}

// GetDefaultModel returns the default model ID
func (pm *ProvidersManager) GetDefaultModel() string {
	if pm.config.DefaultModel != "" {
		return pm.config.DefaultModel
	}
	return "deepseek-chat"
}

// defaultConfig returns default configuration when no yaml file
func (pm *ProvidersManager) defaultConfig() *ProvidersConfig {
	return &ProvidersConfig{
		Providers: map[string]ProviderConfig{
			"deepseek": {
				Enabled: true,
				Key:     os.Getenv("DEEPSEEK_API_KEY"),
				BaseURL: "https://api.deepseek.com/v1",
				Models: []ModelConfig{
					{ID: "deepseek-chat", Name: "DeepSeek V3.2", ContextWindow: 128000, InputPrice: 0.27, OutputPrice: 1.10, SupportsTools: true},
					{ID: "deepseek-reasoner", Name: "DeepSeek R1", ContextWindow: 64000, InputPrice: 0.55, OutputPrice: 2.19, SupportsTools: false},
				},
			},
			"gemini": {
				Enabled: true,
				Key:     os.Getenv("GEMINI_API_KEY"),
				Models: []ModelConfig{
					{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", ContextWindow: 1000000, IsFree: true, SupportsTools: true},
				},
			},
			"anthropic": {
				Enabled: true,
				Key:     os.Getenv("ANTHROPIC_API_KEY"),
				Models: []ModelConfig{
					{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextWindow: 200000, InputPrice: 3.0, OutputPrice: 15.0, SupportsTools: true},
				},
			},
			"openai": {
				Enabled: true,
				Key:     os.Getenv("OPENAI_API_KEY"),
				Models: []ModelConfig{
					{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128000, InputPrice: 2.5, OutputPrice: 10.0, SupportsTools: true},
				},
			},
		},
		DefaultProvider: "deepseek",
		DefaultModel:    "deepseek-chat",
		BYOK: BYOKConfig{
			Enabled:             true,
			ShowServerProviders: true,
		},
	}
}

// FindConfigFile looks for providers.yaml in standard locations
func FindConfigFile() string {
	// Look in local project during dev
	projectPaths := []string{
		"/Users/igoryan_dao/GRIKAI/Ricochet/core/config/providers.yaml",
		"config/providers.yaml",
		"core/config/providers.yaml",
	}

	for _, p := range projectPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Check home directory
	home, _ := os.UserHomeDir()
	if home != "" {
		path := filepath.Join(home, ".ricochet", "providers.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check /etc for server deployment
	if _, err := os.Stat("/etc/ricochet/providers.yaml"); err == nil {
		return "/etc/ricochet/providers.yaml"
	}

	return ""
}
