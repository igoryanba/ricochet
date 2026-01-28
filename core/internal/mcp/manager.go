package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager handles reading and writing to the MCP settings file.
type Manager struct {
	configPath string
	mu         sync.RWMutex
}

// NewManager creates a new MCP Manager.
func NewManager(configDir string) *Manager {
	return &Manager{
		configPath: filepath.Join(configDir, "mcp_settings.json"),
	}
}

// LoadSettings reads the existing MCP settings.
func (m *Manager) LoadSettings() (*McpSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty settings if file doesn't exist
			return &McpSettings{
				McpServers: make(map[string]McpServerConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read mcp_settings.json: %w", err)
	}

	var settings McpSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse mcp_settings.json: %w", err)
	}

	if settings.McpServers == nil {
		settings.McpServers = make(map[string]McpServerConfig)
	}

	return &settings, nil
}

// SaveSettings writes the MCP settings to disk.
func (m *Manager) SaveSettings(settings *McpSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mcp settings: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write mcp_settings.json: %w", err)
	}

	return nil
}

// AddServer adds or updates an MCP server configuration.
func (m *Manager) AddServer(name string, config McpServerConfig) error {
	settings, err := m.LoadSettings()
	if err != nil {
		return err
	}

	if _, exists := settings.McpServers[name]; exists {
		// For now, we allow overwriting, but maybe warn?
		// User can just uninstall and reinstall to update.
		// Or we can treat this as "Update".
	}

	settings.McpServers[name] = config

	return m.SaveSettings(settings)
}

// RemoveServer removes an MCP server configuration.
func (m *Manager) RemoveServer(name string) error {
	settings, err := m.LoadSettings()
	if err != nil {
		return err
	}

	if _, exists := settings.McpServers[name]; !exists {
		return fmt.Errorf("server '%s' not found", name)
	}

	delete(settings.McpServers, name)

	return m.SaveSettings(settings)
}

// ListServers returns a map of all configured servers.
func (m *Manager) ListServers() (map[string]McpServerConfig, error) {
	settings, err := m.LoadSettings()
	if err != nil {
		return nil, err
	}
	return settings.McpServers, nil
}
