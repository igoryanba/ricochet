package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// StartTaskTool creates a dedicated workspace for a complex task
// following the "Hardcore" workflow pattern (Plan, Context, Checklist)
var StartTaskTool = ToolDefinition{
	Name:        "start_task",
	Description: "Create a structured workspace for a complex task. Generates plan, context, and checklist files in .agent/tasks/.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_name": map[string]interface{}{
				"type":        "string",
				"description": "Short, descriptive name (e.g., 'refactor_auth', 'implement_payment')",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Detailed description of the task goal",
			},
		},
		"required": []string{"task_name", "description"},
	},
}

func sanitizeTaskName(name string) string {
	name = strings.ToLower(name)
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return '_'
	}, name)
}

func (e *NativeExecutor) handleStartTask(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		TaskName    string `json:"task_name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	cwd, _ := os.Getwd()
	safeName := sanitizeTaskName(input.TaskName)
	taskDir := filepath.Join(cwd, ".agent", "tasks", safeName)

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create task directory: %w", err)
	}

	// 1. Create PLAN.md
	planContent := fmt.Sprintf("# Task Plan: %s\n\n## Goal\n%s\n\n## Implementation Strategy\n- [ ] Research...\n- [ ] Implement...\n\n## Risks\n- ...", input.TaskName, input.Description)
	if err := os.WriteFile(filepath.Join(taskDir, "PLAN.md"), []byte(planContent), 0644); err != nil {
		return "", err
	}

	// 2. Create CONTEXT.md
	contextContent := fmt.Sprintf("# Task Context: %s\n\n## Key Files\n- ...\n\n## Decisions\n- ...", input.TaskName)
	if err := os.WriteFile(filepath.Join(taskDir, "CONTEXT.md"), []byte(contextContent), 0644); err != nil {
		return "", err
	}

	// 3. Create CHECKLIST.md
	checklistContent := fmt.Sprintf("# Task Checklist: %s\n\n- [ ] Initialize task workspace\n- [ ] ...", input.TaskName)
	if err := os.WriteFile(filepath.Join(taskDir, "CHECKLIST.md"), []byte(checklistContent), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Task Workspace created at `.agent/tasks/%s/`.\nGenerated:\n- PLAN.md\n- CONTEXT.md\n- CHECKLIST.md\n\nPlease switch to 'PLANNING' mode and fill out the PLAN.md.", safeName), nil
}
