package agent

import (
	"testing"
)

func TestIsHelpQuery(t *testing.T) {
	agent := NewHelpAgent()

	tests := []struct {
		input    string
		expected bool
	}{
		{"How do I configure settings?", true},
		{"what is ricochet?", true},
		{"/help", true},
		{"?", true},
		{"Write a function to add numbers", false},
		{"Fix this bug", false},
		{"settings for auto-approval", true},
	}

	for _, tt := range tests {
		if got := agent.IsHelpQuery(tt.input); got != tt.expected {
			t.Errorf("IsHelpQuery(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
