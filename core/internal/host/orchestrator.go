package host

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/igoryan-dao/ricochet/internal/format"
	"github.com/igoryan-dao/ricochet/internal/paths"
)

type CommandLabel string

const (
	StatusRunning   CommandLabel = "running"
	StatusCompleted CommandLabel = "completed"
	StatusFailed    CommandLabel = "failed"
)

const (
	// MaxBufferSize is the maximum amount of output we'll keep in memory for the chat.
	// If output exceeds this, it is truncated in the response, but the full output remains in the log file.
	MaxBufferSize = 10 * 1024 // 10KB (reduced from 5MB for better chat performance, matching Cline's philosophy)
)

type CommandState struct {
	ID        string       `json:"id"`
	Command   string       `json:"command"`
	Status    CommandLabel `json:"status"`
	Output    string       `json:"output,omitempty"`
	Error     string       `json:"error,omitempty"`
	LogFile   string       `json:"log_file,omitempty"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time,omitempty"`
}

type CommandOrchestrator struct {
	cwd      string
	commands map[string]*CommandState
	mu       sync.RWMutex
}

func NewCommandOrchestrator(cwd string) *CommandOrchestrator {
	return &CommandOrchestrator{
		cwd:      cwd,
		commands: make(map[string]*CommandState),
	}
}

func (o *CommandOrchestrator) Execute(ctx context.Context, shellCmd string, background bool) (*CommandState, error) {
	id := uuid.New().String()
	state := &CommandState{
		ID:        id,
		Command:   shellCmd,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}

	o.mu.Lock()
	o.commands[id] = state
	o.mu.Unlock()

	// Ensure log directory exists in the global storage
	logDir := paths.GetLogDir(o.cwd)
	if err := paths.EnsureDir(logDir); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", id))
	state.LogFile = logFilePath

	// Start command
	// Note: We don't use CommandContext for background tasks if we want them to outlive the tool call context,
	// but usually tool calls have a reasonable timeout. For background, we might want a separate context.
	var cmdCtx context.Context
	var cancel context.CancelFunc
	if background {
		// For background commands, we use a background context to avoid being killed when the tool call returns.
		cmdCtx, cancel = context.WithCancel(context.Background())
		_ = cancel // In a real system, we'd store cancel to allow killing the process
	} else {
		cmdCtx = ctx
	}

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", shellCmd)
	cmd.Dir = o.cwd

	if background {
		go o.runCommand(cmd, state)
		return state, nil
	}

	o.runCommand(cmd, state)
	return state, nil
}

func (o *CommandOrchestrator) runCommand(cmd *exec.Cmd, state *CommandState) {
	logFile, err := os.Create(state.LogFile)
	if err != nil {
		o.mu.Lock()
		state.Status = StatusFailed
		state.Error = fmt.Sprintf("failed to create log file: %v", err)
		o.mu.Unlock()
		return
	}
	defer logFile.Close()

	var buf bytes.Buffer
	// MultiWriter to handle both in-memory buffer (for chat) and file log
	mw := io.MultiWriter(logFile, &buf)

	cmd.Stdout = mw
	cmd.Stderr = mw

	err = cmd.Run()

	o.mu.Lock()
	defer o.mu.Unlock()

	state.EndTime = time.Now()
	if err != nil {
		state.Status = StatusFailed
		state.Error = err.Error()
	} else {
		state.Status = StatusCompleted
	}

	output := buf.String()
	// Apply terminal output polish for chat display
	cleanOutput := format.ProcessTerminalOutput(output)

	if len(cleanOutput) > MaxBufferSize {
		state.Output = cleanOutput[:MaxBufferSize] + "\n... (output truncated, see log file for full output: " + state.LogFile + ")"
	} else {
		state.Output = cleanOutput
	}
}

func (o *CommandOrchestrator) GetStatus(id string) (*CommandState, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	state, ok := o.commands[id]
	return state, ok
}

func (o *CommandOrchestrator) ListCommands() []*CommandState {
	o.mu.RLock()
	defer o.mu.RUnlock()
	res := make([]*CommandState, 0, len(o.commands))
	for _, v := range o.commands {
		res = append(res, v)
	}
	return res
}
