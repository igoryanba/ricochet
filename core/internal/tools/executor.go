package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/browser"
	"github.com/igoryan-dao/ricochet/internal/codegraph"
	contextPkg "github.com/igoryan-dao/ricochet/internal/context"
	"github.com/igoryan-dao/ricochet/internal/context/parser"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/index"
	mcpHubPkg "github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/safeguard"
	"github.com/igoryan-dao/ricochet/internal/workflow"
)

// Tool execution interface
type Executor interface {
	Execute(ctx context.Context, name string, args json.RawMessage) (string, error)
	GetDefinitions() []ToolDefinition
}

// ToolDefinition for LLM
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// LiveModeProvider is an interface for checking live mode status and asking remote user
type LiveModeProvider interface {
	IsEnabled() bool
	AskUserRemote(ctx context.Context, question string) (string, error)
}

// NativeExecutor implements Executor using a Host for OS operations and ModeManager for permissions
type NativeExecutor struct {
	host           host.Host
	modes          *modes.Manager
	safeguard      *safeguard.Manager
	browser        *browser.BrowserManager
	mcpHub         *mcpHubPkg.Hub
	indexer        *index.Indexer
	codegraph      *codegraph.Service
	workflows      *workflow.Manager
	livemode       LiveModeProvider
	shadowVerifier *safeguard.ShadowVerifier
}

func NewNativeExecutor(h host.Host, m *modes.Manager, sg *safeguard.Manager, mcpHub *mcpHubPkg.Hub, idx *index.Indexer, cg *codegraph.Service, wm *workflow.Manager) *NativeExecutor {
	return &NativeExecutor{
		host:           h,
		modes:          m,
		safeguard:      sg,
		browser:        browser.NewBrowserManager(os.Getenv("RICOCHET_BROWSER_URL")),
		mcpHub:         mcpHub,
		indexer:        idx,
		codegraph:      cg,
		workflows:      wm,
		shadowVerifier: safeguard.NewShadowVerifier(),
	}
}

// SetLiveMode sets the live mode provider for remote approval routing
func (e *NativeExecutor) SetLiveMode(lm LiveModeProvider) {
	e.livemode = lm
}

func (e *NativeExecutor) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	// 1. Enforce Trust Zones
	if e.safeguard != nil {
		if err := e.safeguard.CheckPermission(name); err != nil {
			return "", fmt.Errorf("safeguard violation: %w", err)
		}
	}

	switch name {
	case "list_dir":
		return e.ListDir(args)
	case "read_file":
		return e.ReadFile(args)
	case "write_file":
		return e.WriteFile(ctx, args)
	case "execute_command":
		return e.ExecuteCommand(ctx, args)
	case "codebase_search":
		return e.CodebaseSearch(ctx, args)
	case "command_status":
		return e.GetCommandStatus(args)
	case "restore_checkpoint":
		return e.RestoreCheckpoint(args)
	case "read_definitions":
		return e.ReadDefinitions(args)
	case "browser_open":
		return e.BrowserOpen(ctx, args)
	case "browser_screenshot":
		return e.BrowserScreenshot(ctx, args)
	case "browser_click":
		return e.BrowserClick(ctx, args)
	case "browser_type":
		return e.BrowserType(ctx, args)
	case "get_diagnostics":
		return e.GetDiagnostics(ctx, args)
	case "get_definitions":
		return e.GetDefinitionsLSP(ctx, args)
	case "switch_mode":
		return e.SwitchMode(args)
	case "update_todos":
		return "Interpreted by controller", nil
	case "get_workflows":
		return e.GetWorkflows(ctx, args)
	case "start_task":
		return e.handleStartTask(ctx, args)

	case "replace_file_content":
		// This method is defined in fs_tools.go but called on NativeExecutor
		return e.ReplaceFileContent(ctx, args)

	case "execute_python":
		return e.ExecutePythonTool(ctx, args)
	default:
		// Check MCP tools
		if e.mcpHub != nil {
			var argsMap map[string]interface{}
			if err := json.Unmarshal(args, &argsMap); err != nil {
				return "", fmt.Errorf("invalid arguments for MCP tool: %w", err)
			}

			result, err := e.mcpHub.CallTool(ctx, name, argsMap)
			if err != nil {
				return "", fmt.Errorf("mcp tool error: %w", err)
			}

			// Format result
			var sb strings.Builder

			// Marshal/Unmarshal result content to inspect it generically
			contentBytes, _ := json.Marshal(result.Content)
			var contentList []map[string]interface{}
			_ = json.Unmarshal(contentBytes, &contentList)

			for _, content := range contentList {
				switch content["type"].(string) {
				case "text":
					if text, ok := content["text"].(string); ok {
						sb.WriteString(text)
						sb.WriteString("\n")
					}
				case "image":
					sb.WriteString("[Image returned]\n")
				case "resource":
					sb.WriteString("[Resource returned]\n")
				}
			}

			if result.IsError {
				return "", fmt.Errorf("tool execution failed: %s", sb.String())
			}
			return sb.String(), nil
		}

		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (e *NativeExecutor) GetDefinitions() []ToolDefinition {
	defs := []ToolDefinition{
		{
			Name:        "list_dir",
			Description: "List files in a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory path (absolute or relative to cwd)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read file content",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path (absolute or relative to cwd)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Create a NEW file or completely overwrite an existing one. Use 'replace_file_content' for editing.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "replace_file_content",
			Description: "Edit an existing file by replacing specific content. PREFERRED over write_file for edits. PRESERVES history.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path to edit",
					},
					"TargetContent": map[string]interface{}{
						"type":        "string",
						"description": "The exact text chunk to verify and replace (must match exactly)",
					},
					"ReplacementContent": map[string]interface{}{
						"type":        "string",
						"description": "The new content to write",
					},
				},
				"required": []string{"path", "TargetContent", "ReplacementContent"},
			},
		},
		{
			Name:        "execute_command",
			Description: "Execute a shell command. Supports background execution.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute",
					},
					"background": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to run the command in the background",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "command_status",
			Description: "Check the status of a background command",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Command ID returned by execute_command",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "switch_mode",
			Description: "Switch the agent's operating mode (persona).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "Mode slug to switch to (e.g., 'architect', 'code', 'test')",
					},
					"handoff": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, triggers 'Intelligent Handoff' (summarizes context to SPEC.md before switching).",
					},
				},
				"required": []string{"mode"},
			},
		},
		{
			Name:        "update_todos",
			Description: "Update the list of todos/tasks for the current session. Use this to track progress and keep the user informed of your plan.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"todos": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text":   map[string]interface{}{"type": "string", "description": "Description of the task"},
								"status": map[string]interface{}{"type": "string", "enum": []string{"pending", "current", "completed"}},
							},
							"required": []string{"text", "status"},
						},
					},
				},
				"required": []string{"todos"},
			},
		},
		{
			Name:        "codebase_search",
			Description: "Perform semantic search over the codebase using embeddings. Returns relevant code snippets.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Natural language search query",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results to return (default: 5)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "execute_python",
			Description: "Execute a Python script. Use this to analyze files, perform math, or automate tasks instead of making multiple tool calls. Stdout/stderr are captured.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"script": map[string]interface{}{
						"type":        "string",
						"description": "The valid Python script to execute.",
					},
				},
				"required": []string{"script"},
			},
		},
	}

	if e.safeguard != nil {
		defs = append(defs, ToolDefinition{
			Name:        "restore_checkpoint",
			Description: "Restore the workspace to a previous checkpoint (Undo changes)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hash": map[string]interface{}{
						"type":        "string",
						"description": "Commit hash to restore to. Use 'HEAD' for latest.",
					},
				},
				"required": []string{"hash"},
			},
		})
	}

	// Add read_definitions tool
	defs = append(defs, ToolDefinition{
		Name:        "read_definitions",
		Description: "Read code definitions (functions, structs) from a file. Currently supports .go files.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to parse",
				},
			},
			"required": []string{"path"},
		},
	})

	// Add LSP tools
	defs = append(defs, ToolDefinition{
		Name:        "get_diagnostics",
		Description: "Get diagnostics (errors/warnings) for a file from the IDE.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to check",
				},
			},
			"required": []string{"path"},
		},
	}, ToolDefinition{
		Name:        "get_definitions",
		Description: "Get symbol definitions via LSP (Go to Definition). Preferred over read_definitions for precise lookups.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (1-indexed)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-indexed)",
				},
			},
			"required": []string{"path", "line", "character"},
		},
	}, ToolDefinition{
		Name:        "get_workflows",
		Description: "Get list of available workflow commands defined in .agent/workflows. Used for autocomplete.",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	})

	// Add Task Workspace tools
	defs = append(defs, StartTaskTool)

	// Add browser tools
	defs = append(defs, ToolDefinition{
		Name:        "browser_open",
		Description: "Open a URL in the browser",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{"type": "string"},
			},
			"required": []string{"url"},
		},
	}, ToolDefinition{
		Name:        "browser_screenshot",
		Description: "Capture a screenshot of a URL",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{"type": "string"},
			},
			"required": []string{"url"},
		},
	}, ToolDefinition{
		Name:        "browser_click",
		Description: "Click an element by selector",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":      map[string]interface{}{"type": "string"},
				"selector": map[string]interface{}{"type": "string"},
			},
			"required": []string{"url", "selector"},
		},
	}, ToolDefinition{
		Name:        "browser_type",
		Description: "Type text into an element",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":      map[string]interface{}{"type": "string"},
				"selector": map[string]interface{}{"type": "string"},
				"text":     map[string]interface{}{"type": "string"},
			},
			"required": []string{"url", "selector", "text"},
		},
	})

	// Add MCP tools
	if e.mcpHub != nil {
		mcpTools := e.mcpHub.GetTools()
		for _, t := range mcpTools {
			// Convert InputSchema using JSON marshaling for safety
			var schema map[string]interface{}
			schemaBytes, _ := json.Marshal(t.InputSchema)
			_ = json.Unmarshal(schemaBytes, &schema)

			defs = append(defs, ToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: schema,
			})
		}
	}

	return defs
}

func (e *NativeExecutor) ExecutePythonTool(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Script string `json:"script"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	return ExecutePython(ctx, payload.Script)
}

func (e *NativeExecutor) SwitchMode(args json.RawMessage) (string, error) {
	var payload struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := e.modes.SetMode(payload.Mode); err != nil {
		return "", err
	}

	mode := e.modes.GetActiveMode()
	return fmt.Sprintf("Successfully switched to %s mode. Current role: %s", mode.Name, mode.RoleDefinition), nil
}

func (e *NativeExecutor) RestoreCheckpoint(args json.RawMessage) (string, error) {
	if e.safeguard == nil {
		return "", fmt.Errorf("safeguard not initialized")
	}

	var payload struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := e.safeguard.RestoreCheckpoint(payload.Hash); err != nil {
		return "", fmt.Errorf("restore failed: %w", err)
	}

	return fmt.Sprintf("Successfully restored to checkpoint %s", payload.Hash), nil
}

func (e *NativeExecutor) ReadDefinitions(args json.RawMessage) (string, error) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	targetPath, err := e.resolvePath(payload.Path)
	if err != nil {
		return "", err
	}

	// Read content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	ext := filepath.Ext(targetPath)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Definitions in %s:\n", filepath.Base(targetPath)))

	// Use native Go parser for .go files
	if ext == ".go" {
		defs, err := parser.ParseGo(content)
		if err != nil {
			return "", fmt.Errorf("parse error: %w", err)
		}
		if len(defs) == 0 {
			return "No definitions found.", nil
		}
		for _, d := range defs {
			sb.WriteString(fmt.Sprintf("- [%s] %s (Lines %d-%d)\n", d.Type, d.Name, d.LineStart, d.LineEnd))
		}
		return sb.String(), nil
	}

	// Use Tree-sitter for other languages
	supportedExts := map[string]bool{
		".js": true, ".jsx": true, ".mjs": true,
		".ts": true, ".tsx": true,
		".py": true,
		".rs": true,
		".go": true,
	}

	if !supportedExts[ext] {
		return "", fmt.Errorf("unsupported file type: %s (supported: .go, .js, .ts, .py, .rs)", ext)
	}

	ctx := context.Background()
	langParser := contextPkg.NewLanguageParser()
	defer langParser.Close()

	analysis, err := langParser.ParseDefinitions(ctx, targetPath, content)
	if err != nil {
		return "", fmt.Errorf("tree-sitter parse error: %w", err)
	}

	if len(analysis.Definitions) == 0 {
		return "No definitions found.", nil
	}

	for _, d := range analysis.Definitions {
		sb.WriteString(fmt.Sprintf("- [%s] %s (Lines %d-%d)\n", d.Type, d.Name, d.LineStart, d.LineEnd))
	}

	return sb.String(), nil
}

// IsSafeCommand returns true if the command is a read-only or safe operation
func IsSafeCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return true
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return true
	}
	base := parts[0]

	// Strip path if present in base (e.g. /bin/ls -> ls)
	if lastSlash := strings.LastIndex(base, "/"); lastSlash != -1 {
		base = base[lastSlash+1:]
	}

	safeCommands := map[string]bool{
		"pwd":    true,
		"ls":     true,
		"dir":    true,
		"echo":   true,
		"cat":    true,
		"head":   true,
		"tail":   true,
		"grep":   true,
		"find":   true,
		"date":   true,
		"whoami": true,
		"df":     true,
		"du":     true,
		"ps":     true,
		"uname":  true,
		"file":   true,
		"type":   true,
		"which":  true,
		"top":    true,
	}

	if safeCommands[base] {
		return true
	}

	// Special cases for git read-only commands
	if base == "git" && len(parts) > 1 {
		sub := parts[1]
		gitSafe := map[string]bool{
			"status":    true,
			"diff":      true,
			"log":       true,
			"branch":    true,
			"remote":    true,
			"show":      true,
			"rev-parse": true,
			"ls-files":  true,
			"config":    true, // Usually safe to read
			"tag":       true,
			"describe":  true,
		}
		if gitSafe[sub] {
			return true
		}
	}

	return false
}

// Helper methods from fs_tools.go need to be accessible. They are methods on NativeExecutor.
// I will assume ListDir, ReadFile etc are correctly defined in fs_tools.go.
// I see I added ListDir implementation at bottom of my previous code block...
// Wait, I see `d.ListDir(args)` in switch...
// Ah, `e.ListDir` is method in `fs_tools.go`.
// So I don't need to implement it here.
// BUT, `ReadDefinitions` IS in `executor.go`.
// So I must include `ReadDefinitions`, `SwitchMode`, `RestoreCheckpoint`.
// And I must NOT include `ListDir` etc if they are in `fs_tools.go`.
// I will double check Step 2869 content.
// Yes, ListDir is NOT in executor.go.
// So my content above includes `ReadDefinitions`, `SwitchMode`, `RestoreCheckpoint` and calls `e.ListDir`.
// Wait, I added `ListDir` stub at the bottom of the rewrite block in `task_boundary` thought? No.
// I added `ListDir` implementation in the code block above?
// NO, I added func `(e *NativeExecutor) ListDir...`.
// THIS IS WRONG if it is already in `fs_tools.go`.
// I should REMOVE `ListDir` from my overwrite content if it conflicts.
// I'll check `fs_tools.go` again.
