package tui

import (
	"strings"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

func (m *Model) updateTaskTree(msg protocol.TaskProgress) {
	// Simple implementation: Find node by TaskName (assuming unique for now) or append
	// Real implementation should probably have IDs in protocol.TaskProgress

	// Check if root task Exists
	var root *TaskNode

	// 1. Try to find by ID/Name
	for _, n := range m.TaskTree {
		if n.Name == msg.TaskName {
			root = n
			break
		}
	}

	// 2. If not found, check if the LAST node is still running/pending.
	// If so, assume this is a rename/update of the current task context.
	if root == nil && len(m.TaskTree) > 0 {
		last := m.TaskTree[len(m.TaskTree)-1]
		if last.Status == "running" || last.Status == "pending" {
			root = last
			// Update the name to match the new more specific one
			root.Name = msg.TaskName
			root.ID = msg.TaskName // Sync ID if we use it
		}
	}

	if root == nil {
		root = &TaskNode{
			ID:       msg.TaskName,
			Name:     msg.TaskName,
			Status:   "running",
			Expanded: true,
			Depth:    0,
		}
		m.TaskTree = append(m.TaskTree, root)
	}

	// Update Status
	if msg.Status != "" {
		// e.g. "Active", "Completed"
		root.Status = msg.Status
	}

	// Steps are children
	// We need to sync steps.
	// This is a bit naive, identifying by content string.
	for _, step := range msg.Steps {
		found := false
		for _, child := range root.Children {
			if child.Name == step {
				found = true
				break
			}
		}
		if !found {
			root.Children = append(root.Children, &TaskNode{
				ID:     step,
				Name:   step,
				Status: "done", // Past steps usually done in this list
				Depth:  1,
			})
		}
	}

	// Smart Meta Update (High Fidelity)
	// Smart Update (High Fidelity)
	if len(root.Children) > 0 {
		lastChild := root.Children[len(root.Children)-1]

		// 1. Meta from Summary
		if msg.Summary != "" {
			// Heuristics for "Smart Summaries"
			lowerSum := msg.Summary
			isMeta := false
			if strings.HasPrefix(lowerSum, "Read") && strings.Contains(lowerSum, "line") {
				isMeta = true
			} else if strings.HasPrefix(lowerSum, "Found") && strings.Contains(lowerSum, "match") {
				isMeta = true
			}

			if isMeta {
				lastChild.Meta = msg.Summary
				lastChild.Expanded = false
			}
		}

		// 2. Result for Terminal Output
		if msg.Result != "" {
			lastChild.Result = msg.Result
			// Keep it collapsed by default so it shows the "5 lines" preview
			lastChild.Expanded = false
		}
	}
}
