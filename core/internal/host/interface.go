package host

import (
	"context"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// Host defines the interface for environment-specific operations.
// This allows Ricochet to run in different hosts (VSCode, JetBrains, Terminal)
// by providing a consistent interface for OS and UI interactions.
type Host interface {
	// File System operations
	GetCWD() string
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	ListDir(path string) ([]FileInfo, error)

	// Terminal operations
	ExecuteCommand(ctx context.Context, command string, background bool) (CommandResult, error)
	GetCommandStatus(id string) (CommandStatus, bool)

	// UI / Interaction
	ShowMessage(level string, text string)
	AskUser(question string) (string, error)
	SendMessage(msg protocol.RPCMessage)
	SendRequest(method string, payload interface{}) (interface{}, error)
}

// CommandStatus represents the current state of a command
type CommandStatus struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	LogFile string `json:"log_file,omitempty"`
}

// FileInfo represents basic file metadata
type FileInfo struct {
	Name  string
	Size  int64
	IsDir bool
}

// CommandResult represents the outcome of a command execution
type CommandResult struct {
	ID     string // Unique ID for the command
	Output string // Immediate output (if not background)
	Error  error
}
