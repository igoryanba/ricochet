package context

import (
	"encoding/json"
	"testing"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

func TestPruneMessages_PreservesToolCallPairs(t *testing.T) {
	wm := NewWindowManager(1000) // Small token limit to force pruning

	// Create messages with a tool call/result pair
	messages := []protocol.Message{
		{Role: "user", Content: "Initial task"},
		{Role: "assistant", Content: "I'll help", ToolUse: []protocol.ToolUseBlock{
			{ID: "tool_123", Name: "read_file", Input: json.RawMessage(`{"path": "/test.txt"}`)},
		}},
		{Role: "user", Content: "", ToolResults: []protocol.ToolResultBlock{
			{ToolUseID: "tool_123", Content: "File content here"},
		}},
		{Role: "assistant", Content: "Done!"},
	}

	pruned := wm.PruneMessages(messages, "System prompt")

	// The pruned messages should either contain both the assistant with tool_calls
	// AND the user with tool_results, OR neither of them
	var hasToolCalls bool
	var hasToolResults bool
	toolCallIDs := make(map[string]bool)
	toolResultIDs := make(map[string]bool)

	for _, msg := range pruned {
		if msg.Role == "assistant" && len(msg.ToolUse) > 0 {
			hasToolCalls = true
			for _, tu := range msg.ToolUse {
				toolCallIDs[tu.ID] = true
			}
		}
		if msg.Role == "user" && len(msg.ToolResults) > 0 {
			hasToolResults = true
			for _, tr := range msg.ToolResults {
				toolResultIDs[tr.ToolUseID] = true
			}
		}
	}

	// If we have tool results, we must have the corresponding tool calls
	if hasToolResults {
		for resultID := range toolResultIDs {
			if !toolCallIDs[resultID] {
				t.Errorf("Orphaned tool result found: %s has no corresponding tool_call in the pruned messages", resultID)
			}
		}
	}

	t.Logf("Pruned %d -> %d messages", len(messages), len(pruned))
	t.Logf("hasToolCalls: %v, hasToolResults: %v", hasToolCalls, hasToolResults)
}

func TestPruneMessages_RemovesOrphanedToolResults(t *testing.T) {
	wm := &WindowManager{
		MaxTokens: 500, // Very small limit
		Settings: &ContextSettings{
			SlidingWindowSize: 5,
		},
	}

	// Create a scenario where pruning would orphan a tool result
	// by having many messages between the tool call and result
	messages := []protocol.Message{
		{Role: "user", Content: "Initial task message that is quite long to consume tokens"},
		{Role: "assistant", Content: "First planning response", ToolUse: []protocol.ToolUseBlock{
			{ID: "tool_old", Name: "old_tool", Input: json.RawMessage(`{}`)},
		}},
		{Role: "user", Content: "", ToolResults: []protocol.ToolResultBlock{
			{ToolUseID: "tool_old", Content: "Old result that will be orphaned"},
		}},
		{Role: "assistant", Content: "Second response with more text to consume tokens"},
		{Role: "user", Content: "Another user message to add more tokens"},
		{Role: "assistant", Content: "Third response", ToolUse: []protocol.ToolUseBlock{
			{ID: "tool_new", Name: "new_tool", Input: json.RawMessage(`{}`)},
		}},
		{Role: "user", Content: "", ToolResults: []protocol.ToolResultBlock{
			{ToolUseID: "tool_new", Content: "New result"},
		}},
		{Role: "assistant", Content: "Final response"},
	}

	pruned := wm.PruneMessages(messages, "System prompt")

	// Verify no orphaned tool results
	toolCallIDs := make(map[string]bool)
	for _, msg := range pruned {
		if msg.Role == "assistant" && len(msg.ToolUse) > 0 {
			for _, tu := range msg.ToolUse {
				toolCallIDs[tu.ID] = true
			}
		}
	}

	for _, msg := range pruned {
		if msg.Role == "user" && len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				if !toolCallIDs[tr.ToolUseID] {
					t.Errorf("Orphaned tool result: %s", tr.ToolUseID)
				}
			}
		}
	}

	t.Logf("Pruned %d -> %d messages", len(messages), len(pruned))
}
