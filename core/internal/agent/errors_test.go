package agent

import (
	"errors"
	"testing"
)

func TestTranslateError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "Authentication 401",
			err:      errors.New("API error 401: Unauthorized"),
			expected: "⚠️ Authentication error: Please check your API key in Settings.",
		},
		{
			name:     "Rate Limit 429",
			err:      errors.New("API error 429: Too many requests"),
			expected: "⏳ Rate limit exceeded: Please wait a moment or try a different provider/model.",
		},
		{
			name:     "Max Tokens Case",
			err:      errors.New("API error 400: Invalid max_tokens value, the valid range of max_tokens is [1, 8192]"),
			expected: "🛑 Parameter error: The selected model does not support this request length. Try shortening your context or selecting a different model.",
		},
		{
			name:     "Timeout",
			err:      errors.New("context deadline exceeded"),
			expected: "🌐 Connection timeout: Check your internet connection or the AI provider's status page.",
		},
		{
			name:     "Connection Refused",
			err:      errors.New("dial tcp: lookup api.openai.com: no such host"),
			expected: "🌐 Network error: Cannot reach the AI server. Check your internet or proxy settings.",
		},
		{
			name:     "Insufficient Balance",
			err:      errors.New("insufficient_balance: your account credit is too low"),
			expected: "💰 Insufficient balance: Please check your AI provider account credits.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateError(tt.err)
			if got != tt.expected {
				t.Errorf("TranslateError() = %q, want %q", got, tt.expected)
			}
		})
	}
}
