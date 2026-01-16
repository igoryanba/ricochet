package qc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CheckResult struct {
	Success bool
	Output  string
	Command string
}

type Manager struct {
	cwd string
}

func NewManager(cwd string) *Manager {
	return &Manager{cwd: cwd}
}

// RunCheck detects the project type and runs the appropriate verify command
func (m *Manager) RunCheck(ctx context.Context) (*CheckResult, error) {
	cmdStr := m.detectCommand()
	if cmdStr == "" {
		return &CheckResult{Success: true, Output: "No QC command detected for this project type"}, nil
	}

	// Safety: Timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	parts := strings.Fields(cmdStr)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = m.cwd

	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		// Command failed
		return &CheckResult{
			Success: false,
			Output:  output,
			Command: cmdStr,
		}, nil
	}

	return &CheckResult{
		Success: true,
		Output:  output,
		Command: cmdStr,
	}, nil
}

func (m *Manager) detectCommand() string {
	// 1. Check for Ricochet specific QC script
	if _, err := os.Stat(filepath.Join(m.cwd, ".agent", "qc.sh")); err == nil {
		return "./.agent/qc.sh"
	}

	// 2. Go
	if _, err := os.Stat(filepath.Join(m.cwd, "go.mod")); err == nil {
		return "go build ./..."
	}

	// 3. Node/TS
	if _, err := os.Stat(filepath.Join(m.cwd, "package.json")); err == nil {
		// Prefer user-defined 'lint' or 'build' script?
		// For safety, maybe just tsc if available?
		// Let's stick to a safe default if 'npm test' is too heavy.
		// 'npm run type-check' is common.
		// For now, let's look for known scripts or just skip to avoid running heavy builds unexpectedly.
		// Actually, Hardcore mode implies running the build.
		return "npm run build"
	}

	// 4. Rust
	if _, err := os.Stat(filepath.Join(m.cwd, "Cargo.toml")); err == nil {
		return "cargo check"
	}

	return ""
}
