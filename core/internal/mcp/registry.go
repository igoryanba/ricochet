package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// RegistryItem defines a mapping between a project type and an MCP server.
type RegistryItem struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	TriggerFiles   []string `json:"trigger_files"`   // e.g. ["package.json"]
	TriggerExts    []string `json:"trigger_exts"`    // e.g. [".ts", ".js"]
	InstallCommand string   `json:"install_command"` // e.g. "npx -y @modelcontextprotocol/server-filesystem"
	DefaultArgs    []string `json:"default_args"`    // e.g. ["./"]
}

const (
	DefaultRegistryURL = "https://raw.githubusercontent.com/igoryan-dao/ricochet/main/registry.json"
	RegistryFileName   = "registry.json"
)

// GetBuiltInRegistry returns the hardcoded fallback registry.
func GetBuiltInRegistry() []RegistryItem {
	return []RegistryItem{
		{
			Name:           "filesystem",
			Description:    "Access to local filesystem (Highly Recommended)",
			TriggerFiles:   []string{"*"},
			InstallCommand: "npx",
			DefaultArgs:    []string{"-y", "@modelcontextprotocol/server-filesystem", "./"},
		},
		{
			Name:           "time-server",
			Description:    "Time and timezone utilities",
			TriggerFiles:   []string{"README.md"},
			InstallCommand: "npx",
			DefaultArgs:    []string{"-y", "@modelcontextprotocol/server-time"},
		},
		{
			Name:        "github",
			Description: "GitHub repository interaction",
			TriggerFiles: []string{
				".git",
				".github",
			},
			InstallCommand: "npx",
			DefaultArgs:    []string{"-y", "@modelcontextprotocol/server-github"},
		},
		{
			Name:        "postgres",
			Description: "PostgreSQL Database Inspector",
			TriggerFiles: []string{
				"docker-compose.yml",
			},
			TriggerExts: []string{
				".sql",
			},
			InstallCommand: "npx",
			DefaultArgs:    []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://user:password@localhost/dbname"},
		},
	}
}

// LoadRegistry loads the registry from cache or fetches it from remote.
func LoadRegistry(configDir string) ([]RegistryItem, error) {
	cachePath := filepath.Join(configDir, RegistryFileName)

	// 1. Check Cache Age
	info, err := os.Stat(cachePath)
	if err == nil {
		if time.Since(info.ModTime()) < 24*time.Hour {
			// Cache is fresh, load it
			if items, err := loadFile(cachePath); err == nil {
				return items, nil
			}
		}
	}

	// 2. Fetch Remote
	// For now, we'll skip actual HTTP fetch to avoid external deps failure in this environment
	// without explicit networking approval or `http` usage.
	// But the plan says "Enable ... fetch".
	// Let's implement it but fallback gracefully.

	items, err := fetchRemote(DefaultRegistryURL)
	if err == nil {
		// Save to cache
		_ = saveFile(cachePath, items)
		return items, nil
	}

	// 3. Fallback to Cache (even if stale)
	if info != nil { // info exists from previous stat
		if items, err := loadFile(cachePath); err == nil {
			return items, nil
		}
	}

	// 4. Fallback to Built-in
	return GetBuiltInRegistry(), nil
}

func loadFile(path string) ([]RegistryItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []RegistryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func saveFile(path string, items []RegistryItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func fetchRemote(url string) ([]RegistryItem, error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var items []RegistryItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}
