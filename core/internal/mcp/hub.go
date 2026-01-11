package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Hub manages connections to multiple MCP servers
type Hub struct {
	connections map[string]*McpConnection
	mu          sync.RWMutex
	configDir   string
	lastModTime time.Time
}

// McpConnection represents an active connection to an MCP server
type McpConnection struct {
	Name   string
	Client *client.Client
	Cmd    *exec.Cmd
	Tools  []mcp.Tool
}

// NewHub creates a new MCP Hub
func NewHub(configDir string) *Hub {
	h := &Hub{
		connections: make(map[string]*McpConnection),
		configDir:   configDir,
	}
	h.StartWatcher()
	return h
}

func (h *Hub) StartWatcher() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		settingsPath := filepath.Join(h.configDir, "mcp_settings.json")

		// Initial wait for file to exist or just load if exists
		// We'll trust the loop to pick it up or load initially if exists
		if _, err := os.Stat(settingsPath); err == nil {
			h.LoadFromSettings(settingsPath)
		}

		for range ticker.C {
			info, err := os.Stat(settingsPath)
			if err != nil {
				continue
			}

			if info.ModTime().After(h.lastModTime) {
				// File changed, reload
				h.LoadFromSettings(settingsPath)
			}
		}
	}()
}

func (h *Hub) LoadFromSettings(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Warning: Failed to read %s: %v\n", path, err)
		return
	}

	var settings McpSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Printf("Error parsing mcp_settings.json: %v\n", err)
		return
	}

	// Update lastModTime immediately to avoid double loading
	info, _ := os.Stat(path)
	if info != nil {
		h.lastModTime = info.ModTime()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Identify removed servers
	for name, conn := range h.connections {
		if _, exists := settings.McpServers[name]; !exists {
			fmt.Printf("Removing MCP server: %s\n", name)
			conn.Client.Close()
			delete(h.connections, name)
		}
	}

	// 2. Add/Update servers
	for name, config := range settings.McpServers {
		if config.Disabled {
			if conn, exists := h.connections[name]; exists {
				fmt.Printf("Disabling MCP server: %s\n", name)
				conn.Client.Close()
				delete(h.connections, name)
			}
			continue
		}

		if _, exists := h.connections[name]; !exists {
			// New connection
			// We launch a goroutine to connect to avoid blocking the hub lock for too long,
			// BUT we are holding the lock right now.
			// Ideally we should gather new configs and connect outside lock.
			// For simplicity in MVP, we connect synchronously or strictly assume fast startup.
			// Startups are NOT fast (process spawn).
			// So we should do it outside lock.

			// Strategy: Unlock, Connect, Lock, Assign.
			// But that's complicated with loop.
			// Simplest: Just use a separate goroutine for each connection attempt.

			go h.connectAsync(name, config)
		} else {
			// TODO: Check if config changed and reconnect?
			// Ignoring update for existing connections for now.
		}
	}
}

func (h *Hub) connectAsync(name string, config McpServerConfig) {
	fmt.Printf("Connecting to MCP server: %s\n", name)
	if err := h.connectInternal(context.Background(), name, config); err != nil {
		fmt.Printf("Failed to connect %s: %v\n", name, err)
	} else {
		fmt.Printf("Connected to MCP server: %s\n", name)
	}
}

// Connect establishes a connection to an MCP server via Stdio (Public API)
func (h *Hub) Connect(ctx context.Context, name string, config McpServerConfig) error {
	return h.connectInternal(ctx, name, config)
}

func (h *Hub) connectInternal(ctx context.Context, name string, config McpServerConfig) error {
	// 1. Create Client (Stdio)
	// NewStdioMCPClient(command string, args []string) based on my fix
	mcpClient, err := client.NewStdioMCPClient(config.Command, config.Args)
	if err != nil {
		return fmt.Errorf("failed to create MCP client for %s: %w", name, err)
	}

	// 2. Start (Launch process)
	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP client for %s: %w", name, err)
	}

	// 3. Initialize
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = "2024-11-05"
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "ricochet",
		Version: "1.0.0",
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client for %s: %w", name, err)
	}

	// 4. Fetch Tools (with timeout)
	ctxTools, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	listToolsResult, err := mcpClient.ListTools(ctxTools, mcp.ListToolsRequest{})

	tools := []mcp.Tool{}
	if listToolsResult != nil {
		tools = listToolsResult.Tools
	}

	conn := &McpConnection{
		Name:   name,
		Client: mcpClient,
		Tools:  tools,
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[name] = conn
	return nil
}

// GetTools returns a flat list of all tools from all servers
func (h *Hub) GetTools() []mcp.Tool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var allTools []mcp.Tool
	for _, conn := range h.connections {
		for _, tool := range conn.Tools {
			allTools = append(allTools, tool)
		}
	}
	return allTools
}

// CallTool executes a tool on the appropriate server
func (h *Hub) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Find server with this tool
	var targetConn *McpConnection
	for _, conn := range h.connections {
		for _, tool := range conn.Tools {
			if tool.Name == name {
				targetConn = conn
				break
			}
		}
		if targetConn != nil {
			break
		}
	}

	if targetConn == nil {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	// Calculate timeout (default 60s)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return targetConn.Client.CallTool(ctxWithTimeout, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
}

// Close closes all connections
func (h *Hub) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, conn := range h.connections {
		conn.Client.Close()
	}
	return nil
}
