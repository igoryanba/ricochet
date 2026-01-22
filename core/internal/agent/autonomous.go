package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AutonomousController manages the Plan-and-Solve loop
type AutonomousController struct {
	baseController *Controller
	maxIterations  int
	planPath       string
}

func NewAutonomousController(base *Controller) *AutonomousController {
	return &AutonomousController{
		baseController: base,
		maxIterations:  50,
		planPath:       "PLAN.md",
	}
}

// Run executes the autonomous loop
func (ac *AutonomousController) Run(ctx context.Context, goal string) error {
	log.Printf("🚀 Starting Autonomous Mode for: %s", goal)

	// Create a dummy callback for now
	callback := func(update interface{}) {
		// No-op or log
	}

	// 1. Initial Plan Generation
	if err := ac.generatePlan(ctx, goal, callback); err != nil {
		return err
	}

	iteration := 0
	for iteration < ac.maxIterations {
		iteration++
		log.Printf("🔄 Iteration %d/%d", iteration, ac.maxIterations)

		// Check if done
		if ac.isPlanComplete(ctx) {
			log.Printf("✅ Plan Complete! Verifying...")
			return nil
		}

		// Execute next step logic
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("autonomous mode reached max iterations (%d) without completion", ac.maxIterations)
}

func (ac *AutonomousController) generatePlan(ctx context.Context, goal string, callback func(interface{})) error {
	prompt := fmt.Sprintf("Goal: %s\n\nCreate a %s file with a step-by-step checklist to achieve this goal.", goal, ac.planPath)

	req := ChatRequestInput{
		SessionID: uuid.New().String(),
		Content:   prompt,
		Via:       "autonomous",
	}

	// Delegate to base controller Chat
	return ac.baseController.Chat(ctx, req, callback)
}

func (ac *AutonomousController) isPlanComplete(ctx context.Context) bool {
	toolArgs := []byte(fmt.Sprintf(`{"path": "%s"}`, ac.planPath))
	content, err := ac.baseController.executor.Execute(ctx, "read_file", toolArgs)
	if err != nil {
		return false
	}

	if strings.Contains(content, "- [ ]") {
		return false
	}
	return true
}
