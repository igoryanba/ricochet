package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TaskItem represents a single step in the agent's plan
type TaskItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"` // "pending", "active", "done", "failed"
	Context string `json:"context,omitempty"`
}

// PlanManager handles the agent's long-term plan
type PlanManager struct {
	mu       sync.RWMutex
	Tasks    []TaskItem `json:"tasks"`
	FilePath string     `json:"-"`
}

// NewPlanManager creates a new plan manager associated with a specific directory
func NewPlanManager(cwd string) *PlanManager {
	return &PlanManager{
		Tasks:    make([]TaskItem, 0),
		FilePath: filepath.Join(cwd, "task_plan.json"),
	}
}

// Load reads the plan from disk
func (pm *PlanManager) Load() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := os.ReadFile(pm.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No plan yet
		}
		return err
	}

	return json.Unmarshal(data, &pm.Tasks)
}

// Save writes the plan to disk
func (pm *PlanManager) Save() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.saveInternal()
}

// saveInternal writes to disk without locking (caller must hold lock)
func (pm *PlanManager) saveInternal() error {
	data, err := json.MarshalIndent(pm.Tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.FilePath, data, 0644)
}

// GenerateContext creates the pinned context string for the system prompt
func (pm *PlanManager) GenerateContext() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.Tasks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== 🧭 MASTER PLAN (AUTONOMOUS MODE) ===\n")

	for _, task := range pm.Tasks {
		icon := "[ ]"
		suffix := ""

		switch task.Status {
		case "done", "completed":
			icon = "[x]"
		case "active", "in_progress":
			icon = "[>]"
			suffix = " (CURRENT FOCUS)"
		case "failed":
			icon = "[!]"
		}

		sb.WriteString(fmt.Sprintf("%s %s. %s%s\n", icon, task.ID, task.Title, suffix))
	}
	sb.WriteString("========================================\n")

	return sb.String()
}

// UpdateTask updates the status of a specific task
func (pm *PlanManager) UpdateTask(id string, status string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Status = status
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// SetPlan replaces the entire plan
func (pm *PlanManager) SetPlan(tasks []TaskItem) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.Tasks = tasks
	return pm.saveInternal()
}
