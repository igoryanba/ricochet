package terminal

import (
	"fmt"
	"os"
)

// AgentState represents the current state of the agent for terminal title display
type AgentState string

const (
	// StateReady indicates the agent is idle and waiting for input
	StateReady AgentState = "◇ Ready"
	// StateWorking indicates the agent is actively processing
	StateWorking AgentState = "✦ Working…"
	// StateActionRequired indicates user approval is needed
	StateActionRequired AgentState = "✋ Action Required"
)

// currentState tracks the current terminal title state
var currentState AgentState = StateReady

// SetTerminalTitle updates the terminal title with the agent state
// Uses ANSI escape sequence: \033]0;TITLE\007
func SetTerminalTitle(state AgentState) {
	currentState = state
	// Skip if not a TTY (e.g., piped output, CI environment)
	if !isTTY() {
		return
	}

	// OSC (Operating System Command) sequence for setting terminal title
	fmt.Fprintf(os.Stdout, "\033]0;Ricochet %s\007", state)
}

// GetCurrentState returns the current terminal title state
func GetCurrentState() AgentState {
	return currentState
}

// ResetTerminalTitle resets the terminal title to default
func ResetTerminalTitle() {
	if !isTTY() {
		return
	}
	fmt.Fprintf(os.Stdout, "\033]0;Ricochet\007")
}

// isTTY checks if stdout is a terminal
func isTTY() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
