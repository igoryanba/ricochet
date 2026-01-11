package modes

// Mode represents a specialized agent persona
type Mode struct {
	Slug               string            `json:"slug" yaml:"slug"`
	Name               string            `json:"name" yaml:"name"`
	RoleDefinition     string            `json:"role_definition" yaml:"role_definition"`
	CustomInstructions string            `json:"custom_instructions" yaml:"custom_instructions"`
	ToolGroups         []string          `json:"tool_groups" yaml:"tool_groups"`
	FileRestrictions   []FileRestriction `json:"file_restrictions,omitempty" yaml:"file_restrictions,omitempty"`
	Source             string            `json:"source" yaml:"source"` // project, global, builtin
}

// FileRestriction limits which files the mode can interact with
type FileRestriction struct {
	Regex       string `json:"regex" yaml:"regex"`
	Description string `json:"description" yaml:"description"`
}

// Config is the root configuration for modes
type Config struct {
	CustomModes []Mode `json:"custom_modes" yaml:"custom_modes"`
}

// Default Modes
var BuiltinModes = []Mode{
	{
		Slug:           "code",
		Name:           "💻 Code",
		RoleDefinition: "You are Ricochet, a high-performance software engineer. You specialize in implementation, refactoring, and following project best practices.",
		ToolGroups:     []string{"read", "edit", "command", "browser", "mcp"},
	},
	{
		Slug:           "architect",
		Name:           "📐 Architect",
		RoleDefinition: "You are Ricochet, a senior software architect. You specialize in system design, research, and documentation. You prefer reading and analysis over frequent code changes.",
		ToolGroups:     []string{"read", "browser", "mcp"},
	},
	{
		Slug:           "test",
		Name:           "🧪 Tester",
		RoleDefinition: "You are Ricochet, a quality assurance specialist. You specialize in writing Vitest/Jest/Go tests and verifying system behavior.",
		ToolGroups:     []string{"read", "command", "mcp"},
		FileRestrictions: []FileRestriction{
			{Regex: ".*_test\\.go$|.*\\.test\\.(ts|js)$", Description: "Test files only"},
		},
	},
}

// ToolGroupDefinitions maps group names to specific tools
var ToolGroupDefinitions = map[string][]string{
	"read":    {"list_dir", "read_file", "read_definitions"},
	"edit":    {"write_file"},
	"command": {"execute_command", "command_status"},
	"browser": {"browser_open", "browser_screenshot", "browser_click", "browser_type"},
	"mcp":     {"use_mcp_tool", "access_mcp_resource"}, // Placeholder for MCP
	"always":  {"switch_mode", "update_todos", "restore_checkpoint"},
}

func IsToolAllowed(mode Mode, toolName string) bool {
	// check "always" group first
	for _, t := range ToolGroupDefinitions["always"] {
		if t == toolName {
			return true
		}
	}

	for _, group := range mode.ToolGroups {
		if tools, ok := ToolGroupDefinitions[group]; ok {
			for _, t := range tools {
				if t == toolName {
					return true
				}
			}
		}

		// Special handling for MCP?
		// If group is "mcp" allow any tool starting with mcp_?
		// For now simple mapping.
	}
	return false
}
