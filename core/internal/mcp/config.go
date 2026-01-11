package mcp

// McpSettings represents the root of mcp_settings.json
type McpSettings struct {
	McpServers map[string]McpServerConfig `json:"mcpServers"`
}

// McpServerConfig represents the configuration for a single MCP server
type McpServerConfig struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	AutoApprove []string          `json:"autoApprove,omitempty"`
}
