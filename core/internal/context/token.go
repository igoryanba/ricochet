package context

import (
	"log"
	"sync"

	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/pkoukk/tiktoken-go"
)

// TokenFudgeFactor is a safety margin to account for differences in tokenization.
// With precise tokenization, we can reduce this from 1.2 to 1.05.
const TokenFudgeFactor = 1.05

var (
	tkm     *tiktoken.Tiktoken
	tkmOnce sync.Once
)

func getTokenizer() *tiktoken.Tiktoken {
	tkmOnce.Do(func() {
		var err error
		tkm, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			log.Printf("Warning: failed to load tiktoken encoding: %v. Falling back to heuristic.", err)
		}
	})
	return tkm
}

// EstimateTokens provides an estimation of tokens in a string.
// It uses tiktoken if available, otherwise falls back to a 1:4 heuristic.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	tokenizer := getTokenizer()
	if tokenizer != nil {
		tokens := tokenizer.Encode(text, nil, nil)
		return len(tokens)
	}

	// Rule of thumb: 1 token ~= 4 characters for English text.
	return len(text) / 4
}

// EstimateBudgetedTokens applies the safety margin to the estimation.
func EstimateBudgetedTokens(text string) int {
	return int(float64(EstimateTokens(text)) * TokenFudgeFactor)
}

// EstimateMessageTokens estimates tokens for a message, including content and tool calls.
func EstimateMessageTokens(msg protocol.Message) int {
	return EstimateMessageTokensWithFudge(msg, false)
}

// EstimateMessageBudgetedTokens estimates tokens for a message with fudge factor.
func EstimateMessageBudgetedTokens(msg protocol.Message) int {
	return EstimateMessageTokensWithFudge(msg, true)
}

// EstimateMessageTokensWithFudge is the internal implementation that handles the fudge factor.
func EstimateMessageTokensWithFudge(msg protocol.Message, applyFudge bool) int {
	est := EstimateTokens
	if applyFudge {
		est = EstimateBudgetedTokens
	}

	tokens := est(msg.Content)

	// Add tokens for tool calls
	for _, tc := range msg.ToolUse {
		tokens += est(tc.Name)
		tokens += est(string(tc.Input))
	}

	// Add tokens for tool results
	for _, tr := range msg.ToolResults {
		tokens += est(tr.Content)
	}

	// Add base message overhead (role, tags, etc.)
	// Approximately 4 tokens per message in ChatML or similar formats
	tokens += 4

	return tokens
}

// EstimateTotalTokens counts tokens for a list of messages.
func EstimateTotalTokens(messages []protocol.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageTokens(msg)
	}
	return total
}

// EstimateTotalBudgetedTokens counts tokens for a list of messages with fudge factor.
func EstimateTotalBudgetedTokens(messages []protocol.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageBudgetedTokens(msg)
	}
	return total
}
