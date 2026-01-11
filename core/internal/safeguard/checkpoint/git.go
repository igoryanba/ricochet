package checkpoint

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Timer tracks performance metrics
type Timer struct {
	start time.Time
}

// GitManager handles shadow git operations
type GitManager struct {
	cwd        string
	shadowPath string
	mu         sync.Mutex
}

// NewGitManager creates a new git manager
func NewGitManager(cwd string, shadowBasePath string) (*GitManager, error) {
	// Hash CWD to create unique shadow path
	// For simplicity, we'll use a sanitized path string as ID for now
	// In production, use proper hashing
	cwdHash := fmt.Sprintf("%x", sha256.Sum256([]byte(cwd)))
	shadowPath := filepath.Join(shadowBasePath, "shadow-"+cwdHash[:8])

	return &GitManager{
		cwd:        cwd,
		shadowPath: shadowPath,
	}, nil
}

// Init initializes the shadow repository
func (g *GitManager) Init() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. Create shadow directory
	if err := os.MkdirAll(g.shadowPath, 0755); err != nil {
		return fmt.Errorf("failed to create shadow dir: %w", err)
	}

	// 2. Initialize bare repo if not exists
	gitDir := filepath.Join(g.shadowPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = g.shadowPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init failed: %s: %w", out, err)
		}

		// Configure to ignore file modes to reduce noise
		configCmd := exec.Command("git", "config", "core.fileMode", "false")
		configCmd.Dir = g.shadowPath
		configCmd.Run()
	}

	return nil
}

// Commit creates a checkpoint
func (g *GitManager) Commit(message string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. Add all files from CWD explicitly
	// We use --work-tree to operate on CWD while keeping .git in shadowPath

	// Prepare git command environment
	// git --git-dir=... --work-tree=... add .
	addCmd := exec.Command("git", "--git-dir="+filepath.Join(g.shadowPath, ".git"), "--work-tree="+g.cwd, "add", ".")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %s: %w", out, err)
	}

	// 2. Commit
	commitCmd := exec.Command("git", "--git-dir="+filepath.Join(g.shadowPath, ".git"), "--work-tree="+g.cwd, "commit", "-m", message, "--allow-empty")
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %s: %w", out, err)
	}

	// 3. Get hash
	revCmd := exec.Command("git", "--git-dir="+filepath.Join(g.shadowPath, ".git"), "rev-parse", "HEAD")
	out, err := revCmd.Output()
	if err != nil {
		return "", fmt.Errorf("get hash failed: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Restore resets CWD to a specific commit
func (g *GitManager) Restore(commitHash string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// git --git-dir=... --work-tree=... reset --hard <hash>
	cmd := exec.Command("git", "--git-dir="+filepath.Join(g.shadowPath, ".git"), "--work-tree="+g.cwd, "reset", "--hard", commitHash)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %s: %w", out, err)
	}

	// Clean untracked files to ensure full restore
	cleanCmd := exec.Command("git", "--git-dir="+filepath.Join(g.shadowPath, ".git"), "--work-tree="+g.cwd, "clean", "-fd")
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean failed: %s: %w", out, err)
	}

	return nil
}
