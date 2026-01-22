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

// TaskBoundaryTool updates the current task state following the Antigravity pattern
var TaskBoundaryTool = ToolDefinition{
	Name:        "task_boundary",
	Description: "Update the current task state (Mode, Name, Status, Summary). This tool controls the UI progress card and synchronizes the agent's internal state with the user's view.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"TaskName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the current major task (e.g., 'Implementing Auth')",
			},
			"Mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"PLANNING", "EXECUTION", "VERIFICATION"},
				"description": "The current phase of work",
			},
			"TaskSummary": map[string]interface{}{
				"type":        "string",
				"description": "Concise summary of what has been accomplished so far",
			},
			"TaskStatus": map[string]interface{}{
				"type":        "string",
				"description": "What you are about to do next (displayed as active status)",
			},
			"PredictedTaskSize": map[string]interface{}{
				"type":        "integer",
				"description": "Estimated remaining tool calls for this task",
			},
		},
		"required": []string{"TaskName", "Mode", "TaskSummary", "TaskStatus", "PredictedTaskSize"},
	},
}

// UpdatePlanTool updates the persistent task list (PlanManager)
var UpdatePlanTool = ToolDefinition{
	Name:        "update_plan",
	Description: "Update the status of a task in the Master Plan. Use this to mark tasks as 'active' (when you start working on them) or 'done' (when completed).",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the task to update (e.g. '1', '2')",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"pending", "active", "done", "failed"},
				"description": "The new status of the task",
			},
		},
		"required": []string{"task_id", "status"},
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
