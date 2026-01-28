package agent

import (
	"fmt"
	"sort"
)

// GetRunnableTasks returns tasks that are pending and have all dependencies satisfied
// Sorted by Priority DESC (critical first)
func (pm *PlanManager) GetRunnableTasks() []TaskItem {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var runnable []TaskItem
	// Build a map of completed tasks for O(1) lookup
	completed := make(map[string]bool)
	for _, t := range pm.Tasks {
		if t.Status == "done" || t.Status == "completed" {
			completed[t.ID] = true
		}
	}

	for _, t := range pm.Tasks {
		if t.Status != "pending" {
			continue
		}

		allDepsMet := true
		for _, depID := range t.Dependencies {
			if !completed[depID] {
				allDepsMet = false
				break
			}
		}

		if allDepsMet {
			runnable = append(runnable, t)
		}
	}

	// Sort by Priority (descending: 2=critical, 1=high, 0=normal)
	sort.Slice(runnable, func(i, j int) bool {
		return runnable[i].Priority > runnable[j].Priority
	})

	return runnable
}

// MarkTaskComplete updates status to done
func (pm *PlanManager) MarkTaskComplete(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Status = "done" // Normalized status
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// MarkTaskFailed updates status to failed
func (pm *PlanManager) MarkTaskFailed(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Status = "failed"
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// MarkTaskActive updates status to active
func (pm *PlanManager) MarkTaskActive(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Status = "active"
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// UpdateTaskDependencies updates the dependencies of a specific task
func (pm *PlanManager) UpdateTaskDependencies(id string, deps []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Dependencies = deps
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// SetTaskOutput stores the result summary of a completed task
func (pm *PlanManager) SetTaskOutput(id, output string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for i, task := range pm.Tasks {
		if task.ID == id {
			pm.Tasks[i].Output = output
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task ID '%s' not found", id)
	}

	return pm.saveInternal()
}

// ValidatePlan checks for cycles in the dependency graph using DFS
func (pm *PlanManager) ValidatePlan() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Build adjacency list
	graph := make(map[string][]string)
	taskIDs := make(map[string]bool)
	for _, t := range pm.Tasks {
		taskIDs[t.ID] = true
		graph[t.ID] = t.Dependencies
	}

	// Validate all dependencies exist
	for _, t := range pm.Tasks {
		for _, dep := range t.Dependencies {
			if !taskIDs[dep] {
				return fmt.Errorf("task %s depends on non-existent task %s", t.ID, dep)
			}
		}
	}

	// DFS to detect cycles
	const (
		white = 0 // unvisited
		gray  = 1 // visiting
		black = 2 // visited
	)
	colors := make(map[string]int)

	var dfs func(id string) error
	dfs = func(id string) error {
		colors[id] = gray
		for _, dep := range graph[id] {
			if colors[dep] == gray {
				return fmt.Errorf("cycle detected: %s -> %s", id, dep)
			}
			if colors[dep] == white {
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		colors[id] = black
		return nil
	}

	for id := range taskIDs {
		if colors[id] == white {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}

	return nil
}
