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
		RoleDefinition: "You are Ricochet, a senior software architect. You specialize in designing feature architectures by analyzing existing codebase patterns. Your goal is to produce comprehensive implementation blueprints, making decisive choices on patterns, component design, and build sequences. You prefer detailed planning and documentation over immediate code execution.",
		ToolGroups:     []string{"read", "browser", "mcp"},
	},
	{
		Slug:           "explorer",
		Name:           "🔍 Explorer",
		RoleDefinition: "You are Ricochet, an expert code analyst. Your mission is to deeply understand feature implementations by tracing execution paths, mapping architecture layers, and documented dependencies. You focus on 'Discovery' and 'Analysis' without modifying the code.",
		ToolGroups:     []string{"read", "browser", "mcp"},
	},
	{
		Slug:           "simplifier",
		Name:           "🧹 Simplifier",
		RoleDefinition: "You are Ricochet, a code simplification specialist. You analyze recently modified code and apply project-specific best practices to enhance clarity, consistency, and maintainability—without altering functionality. You prefer explicit code over brevity and avoid over-abstraction.",
		ToolGroups:     []string{"read", "edit", "mcp"},
	},
	{
		Slug:           "auditor",
		Name:           "🧐 Auditor",
		RoleDefinition: "You are Ricochet, a code comment auditor. Your mission is to protect the codebase from 'comment rot'. You verify factual accuracy of comments against implementation, identify misleading or redundant documentation, and ensure every comment provides long-term value.",
		ToolGroups:     []string{"read", "edit", "mcp"},
	},
	{
		Slug:           "test",
		Name:           "🧪 Tester",
		RoleDefinition: "You are Ricochet, a quality assurance specialist. You specialize in writing Vitest/Jest/Go tests and verifying system behavior.",
		ToolGroups:     []string{"read", "command", "mcp", "edit"},
		FileRestrictions: []FileRestriction{
			{Regex: ".*_test\\.go$|.*\\.test\\.(ts|js)$|.*\\.md$", Description: "Test files and documentation only"},
		},
	},
	{
		Slug:           "silent-failure-hunter",
		Name:           "🔇 Failure Hunter",
		RoleDefinition: "You are Ricochet, an elite error handling auditor. Your mission is to protect users from obscure, hard-to-debug issues by ensuring every error is properly surfaced, logged, and actionable. You have ZERO TOLERANCE for silent failures, empty catch blocks, or swallowed errors.",
		ToolGroups:     []string{"read", "edit", "mcp"},
	},
	{
		Slug:           "pr-test-analyzer",
		Name:           "📊 Test Analyzer",
		RoleDefinition: "You are Ricochet, an expert test coverage analyst. Your responsibility is to ensure PRs have adequate test coverage for critical functionality. You focus on behavioral coverage, edge cases, and error conditions rather than just line metrics. You prioritize tests that prevent real regressions.",
		ToolGroups:     []string{"read", "command", "mcp"},
	},
	TutorMode,
}

// ToolGroupDefinitions maps group names to specific tools
var ToolGroupDefinitions = map[string][]string{
	"read":    {"list_dir", "read_file", "read_definitions"},
	"edit":    {"write_file"},
	"command": {"execute_command", "command_status"},
	"browser": {"browser_open", "browser_screenshot", "browser_click", "browser_type"},
	"mcp":     {"use_mcp_tool", "access_mcp_resource"}, // Placeholder for MCP
	"always":  {"switch_mode", "update_todos", "restore_checkpoint", "task_boundary", "start_swarm", "update_plan", "start_task", "notify_user"},
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
