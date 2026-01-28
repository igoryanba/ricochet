package terminal

import (
	"os"
	"testing"
)

func TestAgentStateConstants(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected string
	}{
		{StateReady, "◇ Ready"},
		{StateWorking, "✦ Working…"},
		{StateActionRequired, "✋ Action Required"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.expected {
			t.Errorf("AgentState %v = %q, want %q", tt.state, string(tt.state), tt.expected)
		}
	}
}

func TestGetCurrentState(t *testing.T) {
	// Reset to known state
	currentState = StateReady

	got := GetCurrentState()
	if got != StateReady {
		t.Errorf("GetCurrentState() = %v, want %v", got, StateReady)
	}
}

func TestSetTerminalTitle_UpdatesState(t *testing.T) {
	// Reset state
	currentState = StateReady

	// SetTerminalTitle should update internal state even if not TTY
	SetTerminalTitle(StateWorking)

	if currentState != StateWorking {
		t.Errorf("currentState after SetTerminalTitle = %v, want %v", currentState, StateWorking)
	}
}

func TestIsTTY(t *testing.T) {
	// When running in test environment, typically not a TTY
	// Just ensure it doesn't panic
	result := isTTY()
	_ = result // We can't assert the value as it depends on environment
}

func TestSetTerminalTitle_NonTTY(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout

	// Create a pipe (not a TTY)
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Should not panic even when not TTY
	SetTerminalTitle(StateActionRequired)

	// Restore
	os.Stdout = oldStdout
	w.Close()
	r.Close()

	// State should still be updated internally
	if currentState != StateActionRequired {
		t.Errorf("currentState = %v, want %v", currentState, StateActionRequired)
	}
}
