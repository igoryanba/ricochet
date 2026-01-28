package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Manager handles git operations
type Manager struct {
	cwd string
}

// NewManager creates a new git manager
func NewManager(cwd string) *Manager {
	return &Manager{cwd: cwd}
}

// execute runs a git command
func (m *Manager) execute(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %v\nOutput: %s", args[0], err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// IsRepo checks if the current directory is a git repository
func (m *Manager) IsRepo() bool {
	_, err := m.execute("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Status returns the current git status
func (m *Manager) Status() (string, error) {
	return m.execute("status", "--short")
}

// Diff returns the staged and unstaged changes
func (m *Manager) Diff() (string, error) {
	// combine staged and unstaged diffs
	// staged
	staged, err := m.execute("diff", "--cached")
	if err != nil {
		return "", err
	}
	// unstaged
	unstaged, err := m.execute("diff")
	if err != nil {
		return "", err
	}

	if staged == "" && unstaged == "" {
		return "", nil
	}

	return fmt.Sprintf("=== Staged ===\n%s\n\n=== Unstaged ===\n%s", staged, unstaged), nil
}

// StageAll stages all changes
func (m *Manager) StageAll() error {
	_, err := m.execute("add", ".")
	return err
}

// Commit commits staged changes with a message
func (m *Manager) Commit(msg string) error {
	_, err := m.execute("commit", "-m", msg)
	return err
}
