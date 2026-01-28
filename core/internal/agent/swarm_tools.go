package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/tools"
)

// StartSwarmToolImpl implements the logic for starting the swarm
type StartSwarmToolImpl struct {
	Orchestrator *SwarmOrchestrator
}

func (t *StartSwarmToolImpl) Definition() protocol.Tool {
	def := tools.StartSwarmTool
	return protocol.Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: def.InputSchema,
	}
}

func (t *StartSwarmToolImpl) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.Orchestrator == nil {
		return "", fmt.Errorf("swarm orchestrator not initialized")
	}
	// Auto-Seed moved to SwarmOrchestrator.Start (internal/agent/swarm.go) to ensure it runs in background context

	// Detach context: Use a background context for the Swarm so it survives request cancellation (UI disconnects)
	swarmCtx := context.Background()
	t.Orchestrator.Start(swarmCtx)
	return `✅ SWARM STARTED SUCCESSFULLY.
---------------------------------------------------
⛔ STOP PLANNING. STOP WRITING FILES.
👀 ACTION REQUIRED: WAIT.
The Swarm Workers have taken over. Do NOT execute more tools.
Watch the TUI for progress updates (Green Badges).
---------------------------------------------------`, nil
}

// UpdatePlanToolImpl implements the logic for updating the plan
type UpdatePlanToolImpl struct {
	Plan *PlanManager
}

func (t *UpdatePlanToolImpl) Definition() protocol.Tool {
	def := tools.UpdatePlanTool
	return protocol.Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: def.InputSchema,
	}
}

func (t *UpdatePlanToolImpl) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		TaskID       string   `json:"task_id"`
		Status       string   `json:"status"`
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if t.Plan == nil {
		return "", fmt.Errorf("plan manager not initialized")
	}

	// Update status
	if err := t.Plan.UpdateTask(input.TaskID, input.Status); err != nil {
		return "", err
	}

	// Update dependencies if provided
	if len(input.Dependencies) > 0 {
		if err := t.Plan.SetDependencies(input.TaskID, input.Dependencies); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("✅ Task %s updated to %s", input.TaskID, input.Status), nil
}
