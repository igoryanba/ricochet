---
description: Interactive wizard to create a new Ricochet plugin
---

# Plugin Creation Wizard

This workflow guides you through creating a new plugin from scratch.

## Step 1: Discovery
[User Input] What is the goal of your new plugin? (e.g., "A plugin to manage AWS EC2 instances" or "A plugin to refactor CSS")

## Step 2: Architecture
> **Architect Agent** is analyzing your request...

Action: ` + "`" + `!architect "Design a plugin for: {{input}}. List the Commands, Agents, and MCP servers needed."` + "`" + `

## Step 3: Scaffolding
> **Code Agent** is setting up the project...

Action: ` + "`" + `!cmd "mkdir -p new-plugin/{commands,agents,hooks} && echo '{\"name\":\"new-plugin\",\"version\":\"0.1.0\"}' > new-plugin/plugin.json"` + "`" + `

## Step 4: Component Implementation
> **Agent Creator** is interpreting the design...

Action: ` + "`" + `!agent-creator "Based on the architect's design, generate the system prompts for the required agents in new-plugin/agents/"` + "`" + `

## Completion
✅ Plugin structure created in ` + "`" + `new-plugin/` + "`" + `.
You can now install it via ` + "`" + `/plugin install ./new-plugin` + "`" + `.
