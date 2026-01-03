package install

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigPath represents a known location for MCP settings
type ConfigPath struct {
	Name string
	Path string
}

// GetUserConfigPaths returns a list of candidate paths for MCP configurations
func GetUserConfigPaths() []ConfigPath {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	if runtime.GOOS != "darwin" {
		// Currently focusing on macOS as requested, but structure allows expansion
		return nil
	}

	return []ConfigPath{
		{
			Name: "Claude Desktop",
			Path: filepath.Join(home, "Library/Application Support/Claude/claude_desktop_config.json"),
		},
		{
			Name: "Cursor",
			Path: filepath.Join(home, ".cursor/mcp.json"),
		},
		{
			Name: "Claude Code CLI",
			Path: filepath.Join(home, ".claude.json"),
		},
		{
			Name: "Cline (VS Code)",
			Path: filepath.Join(home, "Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"),
		},
		{
			Name: "Kilo Code",
			Path: filepath.Join(home, ".kiro/settings/mcp.json"),
		},
		{
			Name: "VS Code (Generic MCP)",
			Path: filepath.Join(home, "Library/Application Support/Code/User/mcp.json"),
		},
	}
}

// FullConfig represents the standard MCP config structure
type FullConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig represents individual server settings
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Install updates existing config files with Ricochet settings
func Install(token string, binaryPath string) error {
	configs := GetUserConfigPaths()
	installedCount := 0

	for _, cfg := range configs {
		if _, err := os.Stat(cfg.Path); os.IsNotExist(err) {
			continue
		}

		log.Printf("Found config for %s at %s. Patching...", cfg.Name, cfg.Path)

		err := patchConfigFile(cfg.Path, token, binaryPath)
		if err != nil {
			log.Printf("Failed to patch %s: %v", cfg.Name, err)
			continue
		}

		installedCount++
		fmt.Printf("✅ Successfully configured %s\n", cfg.Name)
	}

	if installedCount == 0 {
		return fmt.Errorf("no supported MCP configurations found. Please make sure at least one of these is installed: Cursor, Claude Desktop, Claude Code, Cline, or Kilo Code")
	}

	return nil
}

func patchConfigFile(path, token, binaryPath string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config FullConfig
	// Note: Some files might be empty or have different root.
	// Standard MCP is "mcpServers" at root.
	if err := json.Unmarshal(data, &config); err != nil {
		// If fails, it might be a new file or different structure.
		// For safety, let's try to see if it's just an empty object.
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	// Update Ricochet entry
	config.MCPServers["ricochet"] = MCPServerConfig{
		Command: binaryPath,
		Env: map[string]string{
			"TELEGRAM_BOT_TOKEN": token,
		},
	}

	// Write back
	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, newData, 0644)
}
