package agent

import (
	"context"
	"fmt"
	"log"
	"strings" // Added missing import

	"github.com/igoryan-dao/ricochet/internal/prompts"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// ContextManager handles the context window and compaction strategies
type ContextManager struct {
	provider      Provider
	contextWindow int
	maxOutput     int
	threshold     float64 // Percentage (0.0 - 1.0) to trigger compaction
}

// NewContextManager creates a new context manager
func NewContextManager(provider Provider, contextWindow int, maxOutput int) *ContextManager {
	if contextWindow <= 0 {
		contextWindow = 128000 // Default safe fallback
	}
	if maxOutput <= 0 {
		maxOutput = 4096
	}

	return &ContextManager{
		provider:      provider,
		contextWindow: contextWindow,
		maxOutput:     maxOutput,
		threshold:     0.7, // Trigger at 70% usage
	}
}

// EstimateTokens provides a rough estimation of tokens (char/4)
// Precise counting would require model-specific tokenizers which is heavy.
func (cm *ContextManager) EstimateTokens(messages []protocol.Message) int {
	var count int
	for _, msg := range messages {
		count += len(msg.Content) / 4
		for _, tool := range msg.ToolUse {
			count += len(tool.Input) / 4
		}
		for _, res := range msg.ToolResults {
			count += len(res.Content) / 4
		}
	}
	return count
}

// ShouldCompact checks if the current history exceeds the threshold
func (cm *ContextManager) ShouldCompact(messages []protocol.Message) bool {
	usage := cm.EstimateTokens(messages)
	limit := int(float64(cm.contextWindow) * cm.threshold)

	should := usage > limit
	if should {
		log.Printf("[ContextManager] Usage: %d / %d (Threshold: %.2f). Compaction triggered.", usage, cm.contextWindow, cm.threshold)
	}
	return should
}

// Compact performs the recursive summarization
func (cm *ContextManager) Compact(ctx context.Context, history []protocol.Message, model string) ([]protocol.Message, error) {
	log.Printf("[ContextManager] Compacting %d messages...", len(history))

	// 1. Prepare history for summarizer
	// Convert protocol messages to a format suitable for the prompt
	var conversationText strings.Builder
	for i, msg := range history {
		// Skip system messages for the summary input, focus on the flow
		if msg.Role == "system" {
			continue
		}
		conversationText.WriteString(fmt.Sprintf("[%d] %s: %s\n", i, strings.ToUpper(msg.Role), msg.Content))

		if len(msg.ToolUse) > 0 {
			for _, tc := range msg.ToolUse {
				conversationText.WriteString(fmt.Sprintf("  [TOOL CALL] %s(%s)\n", tc.Name, string(tc.Input)))
			}
		}
		if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				conversationText.WriteString(fmt.Sprintf("  [TOOL RESULT] %s\n", tr.Content))
			}
		}

		conversationText.WriteString("\n")
	}

	// 2. Create Compaction Request
	req := &ChatRequest{
		Model: model,
		Messages: []protocol.Message{
			{Role: "system", Content: prompts.CompactionSystemPrompt},
			{Role: "user", Content: "Current Conversation History:\n" + conversationText.String()},
		},
		MaxTokens: 4000, // Enough for a detailed summary
	}

	// 3. Execute
	resp, err := cm.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("compaction failed: %w", err)
	}

	summary := resp.Content
	log.Printf("[ContextManager] Compaction complete. Summary length: %d chars", len(summary))

	// 4. Construct New History
	// Strategy: [System Prompt] + [Summary] + [Last 5 Messages (Context Tail)]

	// Keep the original system prompt(s)
	var newHistory []protocol.Message
	for _, msg := range history {
		if msg.Role == "system" {
			newHistory = append(newHistory, msg)
		}
	}

	// Add Summary (as a system note or user injection)
	// OpenCode uses a user injection to "prime" the model.
	// We'll use a System message to be authoritative.
	newHistory = append(newHistory, protocol.Message{
		Role:    "system",
		Content: "=== PREVIOUS CONTEXT SUMMARY ===\n" + summary + "\n================================",
	})

	// Add Tail (Last 5 messages) to keep immediate flow
	// Be careful not to duplicate if history was short (though ShouldCompact checks size)
	tailSize := 5
	if len(history) > tailSize {
		// Find start index, ignoring top-level system prompts if possible,
		// but simple slicing is safer.
		startIdx := len(history) - tailSize
		// Ensure we don't split a tool call and its result?
		// Ideally yes. For now, simple slice.
		tail := history[startIdx:]
		newHistory = append(newHistory, tail...)
	} else {
		// Fallback (shouldn't happen given threshold)
		newHistory = append(newHistory, history...)
	}

	return newHistory, nil
}
