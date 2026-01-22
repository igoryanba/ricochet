package tui

import (
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// getLastBlock returns the last block in history, or nil if empty
func (m *Model) getLastBlock() *HistoryBlock {
	if len(m.Blocks) == 0 {
		return nil
	}
	return m.Blocks[len(m.Blocks)-1]
}

// ensureActiveTreeBlock ensures an active tree block exists.
// If the last block is an inactive tree, returns it (reactivating it).
// If the last block is text, creates a new tree block.
func (m *Model) ensureActiveTreeBlock() *HistoryBlock {
	last := m.getLastBlock()

	// If already have active tree, return it
	if last != nil && last.Type == BlockAgentTree && last.IsActive {
		return last
	}

	// If last is an inactive tree, reactivate it (don't create duplicate)
	if last != nil && last.Type == BlockAgentTree && !last.IsActive {
		last.IsActive = true
		return last
	}

	// Need to create new tree block
	newBlock := &HistoryBlock{
		Type:     BlockAgentTree,
		TaskTree: nil,
		IsActive: true,
	}
	m.Blocks = append(m.Blocks, newBlock)
	return newBlock
}

// getOrCreateTextBlock ensures the last block is an Agent Text block.
// If last block is already text, returns it (for streaming continuation).
// If last block is tree, freezes it and creates new text block.
func (m *Model) getOrCreateTextBlock() *HistoryBlock {
	last := m.getLastBlock()

	// If already text block, just return it
	if last != nil && last.Type == BlockAgentText {
		return last
	}

	// Freeze any active tree before creating text
	if last != nil && last.Type == BlockAgentTree && last.IsActive {
		last.IsActive = false
	}

	// Create new text block
	newBlock := &HistoryBlock{
		Type:    BlockAgentText,
		Content: "",
	}
	m.Blocks = append(m.Blocks, newBlock)
	return newBlock
}

// appendUserBlock adds a user query block and prepares for agent response
func (m *Model) appendUserBlock(content string) {
	// Freeze any active blocks
	m.finishActiveBlocks()

	// Add user message
	m.Blocks = append(m.Blocks, &HistoryBlock{
		Type:    BlockUserQuery,
		Content: content,
	})

	// Pre-create active tree for tool calls
	m.Blocks = append(m.Blocks, &HistoryBlock{
		Type:     BlockAgentTree,
		TaskTree: nil,
		IsActive: true,
	})
}

// updateBlockTaskTree updates the task tree in the current active tree block.
// This is the ONLY place where TaskTree nodes are modified.
func (m *Model) updateBlockTaskTree(msg protocol.TaskProgress) {
	// Get or create active tree block
	block := m.ensureActiveTreeBlock()

	// Find existing root node by TaskName
	var root *TaskNode
	for _, n := range block.TaskTree {
		if n.Name == msg.TaskName {
			root = n
			break
		}
	}

	// Create root if not found
	if root == nil {
		root = &TaskNode{
			ID:       msg.TaskName,
			Name:     msg.TaskName,
			Status:   "running",
			Expanded: true,
			Depth:    0,
		}
		block.TaskTree = append(block.TaskTree, root)
	}

	// Update root status from TaskProgress.Status
	if msg.Status == "done" || msg.Status == "failed" {
		root.Status = msg.Status
	}

	// Add new steps as children (deduplicated)
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
				Status: "done",
				Depth:  1,
			})
		}
	}
}

// finishActiveBlocks marks all active blocks as inactive (frozen)
func (m *Model) finishActiveBlocks() {
	for _, block := range m.Blocks {
		block.IsActive = false
	}
}

// cleanupEmptyBlocks removes empty tree blocks that have no nodes
// Called before rendering to prevent empty trees from being shown
func (m *Model) cleanupEmptyBlocks() {
	if len(m.Blocks) == 0 {
		return
	}

	// Filter out empty inactive tree blocks
	cleaned := make([]*HistoryBlock, 0, len(m.Blocks))
	for _, block := range m.Blocks {
		// Keep if not an empty inactive tree
		if block.Type != BlockAgentTree || block.IsActive || len(block.TaskTree) > 0 {
			cleaned = append(cleaned, block)
		}
	}
	m.Blocks = cleaned
}
