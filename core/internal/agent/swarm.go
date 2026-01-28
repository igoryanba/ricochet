package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// SwarmConfig holds configuration for the orchestrator
type SwarmConfig struct {
	MaxWorkers int `json:"max_workers"`
}

// SwarmOrchestrator manages parallel execution of tasks
type SwarmOrchestrator struct {
	controller *Controller
	plan       *PlanManager
	config     SwarmConfig
	stopChan   chan struct{}
	mu         sync.Mutex
	active     bool
	paused     bool
}

// NewSwarmOrchestrator creates a new orchestrator
func NewSwarmOrchestrator(c *Controller, pm *PlanManager, cfg SwarmConfig) *SwarmOrchestrator {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5 // Default
	}
	return &SwarmOrchestrator{
		controller: c,
		plan:       pm,
		config:     cfg,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the orchestration loop
func (so *SwarmOrchestrator) Start(ctx context.Context) {
	so.mu.Lock()
	if so.active {
		so.mu.Unlock()
		return
	}
	so.active = true
	so.mu.Unlock()

	log.Printf("🐝 [Swarm] Starting with %d workers", so.config.MaxWorkers)

	// Hard Auto-Seeding: Ensure plan is never empty at start
	if len(so.plan.GetTasks()) == 0 {
		log.Println("🐝 Plan is empty. Auto-seeding default analysis tasks...")
		so.plan.AddTask("Project Reconnaissance", "Scan root directory and identify project type (Go/JS/Python)")
		so.plan.AddTask("Architecture Scan", "Analyze internal/ and cmd/ directories for core logic")
		so.plan.AddTask("Configuration Check", "Read go.mod, config.yaml, or .env files")

		so.controller.ReportTaskProgress(ctx, protocol.TaskProgress{
			TaskName:        "Swarm Boot",
			Status:          "done",
			AgentIdentifier: "System",
			AgentColor:      "#FFFFFF",
			Summary:         "Auto-generated 3 initial tasks",
		})
	}

	go so.loop(ctx)
}

// Stop halts the orchestration loop
func (so *SwarmOrchestrator) Stop() {
	so.mu.Lock()
	defer so.mu.Unlock()
	if !so.active {
		return
	}
	so.active = false
	close(so.stopChan)
}

// Pause temporarily halts task spawning
func (so *SwarmOrchestrator) Pause() {
	so.mu.Lock()
	defer so.mu.Unlock()
	so.paused = true
	log.Println("⏸️ [Swarm] Paused")
}

// Resume continues task spawning
func (so *SwarmOrchestrator) Resume() {
	so.mu.Lock()
	defer so.mu.Unlock()
	so.paused = false
	log.Println("▶️ [Swarm] Resumed")
}

// IsPaused returns current pause state
func (so *SwarmOrchestrator) IsPaused() bool {
	so.mu.Lock()
	defer so.mu.Unlock()
	return so.paused
}

func (so *SwarmOrchestrator) loop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Dynamic semaphore based on current config (simplification: fixed at start for now)
	sem := make(chan struct{}, so.config.MaxWorkers)

	for {
		select {
		case <-so.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Skip if paused
			if so.IsPaused() {
				continue
			}

			// Check for runnable tasks
			runnable := so.plan.GetRunnableTasks()
			total := len(so.plan.GetTasks())

			if len(runnable) == 0 {
				if time.Now().Unix()%20 == 0 {
					log.Printf("🐝 Swarm Heartbeat: Active, %d total tasks, 0 runnable", total)
				}
				continue
			}

			log.Printf("🐝 Swarm Loop: Found %d runnable tasks (Total: %d)", len(runnable), total)

			for _, task := range runnable {
				// Acquire semaphore (blocks if limit reached)
				sem <- struct{}{}

				// Mark as active to prevent duplicate re-queueing
				so.plan.MarkTaskActive(task.ID)

				// Spawn agent
				go func(t TaskItem) {
					defer func() { <-sem }() // Release semaphore

					// Use defaults if not set
					maxRetries := t.MaxRetries
					if maxRetries == 0 {
						maxRetries = 3 // System default
					}

					attemptStr := ""
					if t.RetryCount > 0 {
						attemptStr = fmt.Sprintf(" (Attempt %d/%d)", t.RetryCount+1, maxRetries)
					}

					log.Printf("🐝 Spawning Agent for Task %s: %s%s", t.ID, t.Title, attemptStr)

					// Update TUI with Swarm Event
					so.controller.ReportTaskProgress(ctx, protocol.TaskProgress{
						TaskName:        t.Title,
						Status:          "In Progress" + attemptStr,
						Mode:            "execution",
						IsActive:        true,
						AgentIdentifier: fmt.Sprintf("Swarm-%s", t.ID),
						AgentColor:      "#00FF99", // Neon Green
					})

					// TIMEOUT: Wrap context if TimeoutSeconds is set
					taskCtx := ctx
					var cancel context.CancelFunc
					if t.TimeoutSeconds > 0 {
						taskCtx, cancel = context.WithTimeout(ctx, time.Duration(t.TimeoutSeconds)*time.Second)
						defer cancel()
					}

					output, err := so.controller.RunSubtask(taskCtx, "SWARM_ROOT", t.Title, t.Context, "swarm-worker")

					if err != nil {
						log.Printf("❌ Task %s failed: %v", t.ID, err)

						// RETRY LOGIC
						retryCount, _ := so.plan.IncrementRetryCount(t.ID)

						if retryCount < maxRetries {
							log.Printf("⚠️ Task %s failed, scheduling retry %d/%d", t.ID, retryCount, maxRetries)
							so.plan.UpdateTaskStatus(t.ID, "pending") // Re-queue

							so.controller.ReportTaskProgress(ctx, protocol.TaskProgress{
								TaskName:        t.Title,
								Status:          fmt.Sprintf("Retrying (%d/%d)...", retryCount, maxRetries),
								IsActive:        false,
								AgentIdentifier: fmt.Sprintf("Swarm-%s", t.ID),
								AgentColor:      "#FFA500", // Orange for retry
							})
						} else {
							so.plan.MarkTaskFailed(t.ID)
							so.controller.ReportTaskProgress(ctx, protocol.TaskProgress{
								TaskName:        t.Title,
								Status:          "Failed (Max Retries Exceeded)",
								IsActive:        false,
								AgentIdentifier: fmt.Sprintf("Swarm-%s", t.ID),
								AgentColor:      "#FF0000",
							})
						}
					} else {
						log.Printf("✅ Task %s completed", t.ID)
						so.plan.MarkTaskComplete(t.ID)
						// Store output
						so.plan.SetTaskOutput(t.ID, output)
						so.controller.ReportTaskProgress(ctx, protocol.TaskProgress{
							TaskName:        t.Title,
							Status:          "Completed",
							IsActive:        false,
							AgentIdentifier: fmt.Sprintf("Swarm-%s", t.ID),
							AgentColor:      "#00FF99",
						})
					}

				}(task)
			}
		}
	}
}
