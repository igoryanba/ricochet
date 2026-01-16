package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HookDefinition represents a set of commands to run for a specific event
type HookConfig struct {
	OnStart    []string `json:"on_start,omitempty"`
	OnShutdown []string `json:"on_shutdown,omitempty"`
	OnSession  []string `json:"on_session_created,omitempty"`
}

type HookManager struct {
	cwd    string
	config HookConfig
	mu     sync.RWMutex
}

func NewHookManager(cwd string) *HookManager {
	return &HookManager{
		cwd: cwd,
	}
}

// LoadHooks reads .agent/hooks.json
func (m *HookManager) LoadHooks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset config
	m.config = HookConfig{}

	path := filepath.Join(m.cwd, ".agent", "hooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No hooks defined, that's fine
		}
		return err
	}

	if err := json.Unmarshal(data, &m.config); err != nil {
		return fmt.Errorf("failed to parse hooks.json: %w", err)
	}

	return nil
}

// Trigger executes the hooks for a given event
func (m *HookManager) Trigger(event string) {
	m.mu.RLock()
	var commands []string
	switch event {
	case "on_start":
		commands = m.config.OnStart
	case "on_shutdown":
		commands = m.config.OnShutdown
	case "on_session_created":
		commands = m.config.OnSession
	}
	m.mu.RUnlock()

	if len(commands) == 0 {
		return
	}

	log.Printf("[Hooks] Triggering event: %s (%d commands)", event, len(commands))

	go func() {
		for _, cmdStr := range commands {
			if err := m.executeCommand(cmdStr); err != nil {
				log.Printf("[Hooks] Error executing '%s': %v", cmdStr, err)
			}
		}
	}()
}

func (m *HookManager) executeCommand(cmdStr string) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	head := parts[0]
	args := parts[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, head, args...)
	cmd.Dir = m.cwd
	// We deliberately ignore output for now to avoid polluting logs,
	// or we could log debug only.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (output: %s)", err, string(out))
	}

	log.Printf("[Hooks] Executed: %s", cmdStr)
	return nil
}
