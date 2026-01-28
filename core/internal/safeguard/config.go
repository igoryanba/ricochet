package safeguard

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PermissionConfig holds the rules definition
type PermissionConfig struct {
	Files    FileRules    `yaml:"files"`
	Tools    ToolRules    `yaml:"tools"`
	Commands CommandRules `yaml:"commands"`
}

// FileRules defines file access patterns
type FileRules struct {
	Allow []string `yaml:"allow"` // Glob patterns to allow
	Deny  []string `yaml:"deny"`  // Glob patterns to deny (precedence over allow)
}

// ToolRules defines tool usage permissions
type ToolRules struct {
	Allow []string `yaml:"allow"` // Tool names to allow
	Deny  []string `yaml:"deny"`  // Tool names to deny
}

// CommandRules defines shell command permissions
type CommandRules struct {
	Allow []string `yaml:"allow"` // Command prefixes or exact matches to allow
	Deny  []string `yaml:"deny"`  // Command prefixes or exact matches to deny
}

// LoadConfig loads permissions from the project root
func LoadConfig(cwd string) (*PermissionConfig, error) {
	configPath := filepath.Join(cwd, ".ricochet", "permissions.yaml")

	// Default config (safe defaults)
	// By default, if no config exists, we might want to be permissive for dev velocity
	// OR restrictive for safety.
	// Current behavior is permissive. Let's keep it permissive if no file.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &PermissionConfig{
			Files: FileRules{
				Allow: []string{"**"},                              // Allow everything by default if no config
				Deny:  []string{".git/**", ".env*", "**/*.secret"}, // Basic sanity denials
			},
			Tools: ToolRules{
				Allow: []string{"*"},
			},
			Commands: CommandRules{
				Allow: []string{"*"},
				Deny:  []string{"rm -rf /", ":(){ :|:& };:"}, // Basic sanity
			},
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read permissions config: %w", err)
	}

	var cfg PermissionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse permissions config: %w", err)
	}

	return &cfg, nil
}
