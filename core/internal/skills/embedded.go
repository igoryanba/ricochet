package skills

// Simple struct for embedded skills
type EmbeddedSkill struct {
	Name        string
	Description string
	Content     string
	Enforcement string
	Triggers    TriggerConfig
}

// PluginDevSkills returns the set of embedded skills for plugin development
func PluginDevSkills() []EmbeddedSkill {
	return []EmbeddedSkill{
		{
			Name:        "plugin-structure",
			Description: "Understand the Ricochet Plugin structure.",
			Content: `
# Ricochet Plugin Structure

A Ricochet plugin is a directory containing a manifest and components.

## Directory Layout
- ` + "`" + `plugin.json` + "`" + `: Manifest file (Required)
- ` + "`" + `commands/` + "`" + `: Slash command definitions (*.md)
- ` + "`" + `agents/` + "`" + `: Specialized agent personas (*.md)
- ` + "`" + `hooks/` + "`" + `: Event hooks (hooks.json)
- ` + "`" + `skills/` + "`" + `: Knowledge modules (SKILL.md)

## Manifest (plugin.json)
` + "```json" + `
{
  "name": "my-plugin",
  "version": "0.1.0",
  "description": "Does amazing things",
  "author": { "name": "Me" }
}
` + "```" + `
`,
			Enforcement: "Load when user asks about creating plugins or plugin structure.",
			Triggers: TriggerConfig{
				Keywords: []string{"plugin structure", "create plugin", "plugin.json"},
			},
		},
		{
			Name:        "agent-development",
			Description: "Guide for creating specialized agent personas.",
			Content: `
# Agent Development Guide

To create a new agent persona in Ricochet:

1.  **Create File**: ` + "`" + `agents/my-agent.md` + "`" + `
2.  **Frontmatter**:
    ` + "```yaml" + `
    ---
    name: my-agent
    description: Use this agent when [trigger condition]...
    color: cyan
    model: inherit
    ---
    ` + "```" + `
3.  **System Prompt**:
    -   Define the **Role** ("You are an expert...")
    -   Define **Responsibilities**
    -   Define **Process** (Step 1, Step 2...)
    -   Use <example> blocks in description to help Ricochet know when to pick this agent.

## Best Practices
-   Be specific about when to trigger.
-   Give the agent a distinct personality/expertise.
-   Limit tools if necessary.
`,
			Enforcement: "Load when user wants to create a new agent.",
			Triggers: TriggerConfig{
				Keywords: []string{"create agent", "new agent", "agent persona"},
			},
		},
		{
			Name:        "mcp-integration",
			Description: "Guide for integrating MCP servers into plugins.",
			Content: `
# MCP Integration Guide

Plugins can bundle MCP servers to extend Ricochet's capabilities.

## Configuration (.mcp.json)
Create ` + "`" + `.mcp.json` + "`" + ` in your plugin root:

` + "```json" + `
{
  "mcpServers": {
    "sqlite": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "mcp/sqlite"]
    }
  }
}
` + "```" + `

## Usage
-   Ricochet will automatically start these servers when the plugin is loaded.
-   Tools exposed by the server will be available to the agent.
`,
			Enforcement: "Load when user wants to add an MCP server or external integration.",
			Triggers: TriggerConfig{
				Keywords: []string{"mcp server", "mcp integration", ".mcp.json"},
			},
		},
	}
}
