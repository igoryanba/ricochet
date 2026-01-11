package checkpoints

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CheckpointService provides workspace snapshot functionality using shadow git
type CheckpointService struct {
	taskID        string
	workspaceDir  string
	checkpointDir string
	dotGitDir     string
	initialized   bool
	baseHash      string
	checkpoints   []string
	mu            sync.Mutex
}

// NewCheckpointService creates a new checkpoint service
func NewCheckpointService(taskID, workspaceDir, storageDir string) *CheckpointService {
	checkpointDir := filepath.Join(storageDir, "tasks", taskID, "checkpoints")
	return &CheckpointService{
		taskID:        taskID,
		workspaceDir:  workspaceDir,
		checkpointDir: checkpointDir,
		dotGitDir:     filepath.Join(checkpointDir, ".git"),
		checkpoints:   []string{},
	}
}

// Init initializes the shadow git repository
func (s *CheckpointService) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	// Create checkpoint directory
	if err := os.MkdirAll(s.checkpointDir, 0755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}

	// Check if .git already exists
	if _, err := os.Stat(s.dotGitDir); os.IsNotExist(err) {
		// Initialize new git repo
		if err := s.runGit("init"); err != nil {
			return fmt.Errorf("git init: %w", err)
		}

		// Configure git
		if err := s.runGit("config", "core.worktree", s.workspaceDir); err != nil {
			return fmt.Errorf("git config worktree: %w", err)
		}
		if err := s.runGit("config", "commit.gpgSign", "false"); err != nil {
			return fmt.Errorf("git config gpg: %w", err)
		}
		if err := s.runGit("config", "user.name", "Ricochet"); err != nil {
			return fmt.Errorf("git config user.name: %w", err)
		}
		if err := s.runGit("config", "user.email", "ricochet@example.com"); err != nil {
			return fmt.Errorf("git config user.email: %w", err)
		}

		// Write exclude patterns
		if err := s.writeExcludeFile(); err != nil {
			return fmt.Errorf("write exclude: %w", err)
		}

		// Stage all and initial commit
		if err := s.runGit("add", ".", "--ignore-errors"); err != nil {
			// Ignore staging errors - some files may be unreadable
		}

		if err := s.runGit("commit", "-m", "initial commit", "--allow-empty"); err != nil {
			return fmt.Errorf("initial commit: %w", err)
		}
	}

	// Get base hash
	out, err := s.runGitOutput("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("get HEAD: %w", err)
	}
	s.baseHash = strings.TrimSpace(out)
	s.initialized = true

	return nil
}

// Save creates a checkpoint of current workspace state
func (s *CheckpointService) Save(message string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return "", fmt.Errorf("checkpoint service not initialized")
	}

	// Stage all changes
	_ = s.runGit("add", ".", "--ignore-errors")

	// Commit
	if message == "" {
		message = fmt.Sprintf("Checkpoint at %s", time.Now().Format(time.RFC3339))
	}

	if err := s.runGit("commit", "-m", message, "--allow-empty"); err != nil {
		// Check if there were no changes
		if strings.Contains(err.Error(), "nothing to commit") {
			return "", nil // No changes
		}
		return "", fmt.Errorf("commit: %w", err)
	}

	// Get new commit hash
	out, err := s.runGitOutput("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get HEAD after commit: %w", err)
	}

	hash := strings.TrimSpace(out)
	s.checkpoints = append(s.checkpoints, hash)

	return hash, nil
}

// Restore restores workspace to a previous checkpoint
func (s *CheckpointService) Restore(commitHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return fmt.Errorf("checkpoint service not initialized")
	}

	// Clean untracked files
	if err := s.runGit("clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}

	// Reset to commit
	if err := s.runGit("reset", "--hard", commitHash); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}

	// Update checkpoints list
	for i, cp := range s.checkpoints {
		if cp == commitHash {
			s.checkpoints = s.checkpoints[:i+1]
			break
		}
	}

	return nil
}

// GetDiff returns the diff between two commits or current state
func (s *CheckpointService) GetDiff(fromHash, toHash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return "", fmt.Errorf("checkpoint service not initialized")
	}

	args := []string{"diff", "--stat"}
	if toHash != "" {
		args = append(args, fmt.Sprintf("%s..%s", fromHash, toHash))
	} else {
		args = append(args, fromHash)
	}

	out, err := s.runGitOutput(args...)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}

	return out, nil
}

// List returns all checkpoint hashes
func (s *CheckpointService) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.checkpoints))
	copy(result, s.checkpoints)
	return result
}

// BaseHash returns the initial commit hash
func (s *CheckpointService) BaseHash() string {
	return s.baseHash
}

// IsInitialized returns whether the service is ready
func (s *CheckpointService) IsInitialized() bool {
	return s.initialized
}

// writeExcludeFile writes .git/info/exclude with common patterns
func (s *CheckpointService) writeExcludeFile() error {
	excludes := []string{
		"node_modules/",
		".git/",
		"__pycache__/",
		"*.pyc",
		".venv/",
		"venv/",
		".env",
		"*.log",
		".DS_Store",
		"dist/",
		"build/",
		"target/",
		".idea/",
		".vscode/",
		"*.swp",
		"*.swo",
	}

	infoDir := filepath.Join(s.dotGitDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return err
	}

	excludeFile := filepath.Join(infoDir, "exclude")
	return os.WriteFile(excludeFile, []byte(strings.Join(excludes, "\n")), 0644)
}

// runGit executes a git command in the checkpoint directory
func (s *CheckpointService) runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.checkpointDir

	// Sanitize environment - remove git vars that could interfere
	cmd.Env = s.sanitizedEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

// runGitOutput executes git and returns stdout
func (s *CheckpointService) runGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.checkpointDir
	cmd.Env = s.sanitizedEnv()

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%v: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(output), nil
}

// sanitizedEnv returns env without git-specific variables
func (s *CheckpointService) sanitizedEnv() []string {
	skipVars := map[string]bool{
		"GIT_DIR":                          true,
		"GIT_WORK_TREE":                    true,
		"GIT_INDEX_FILE":                   true,
		"GIT_OBJECT_DIRECTORY":             true,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_CEILING_DIRECTORIES":          true,
	}

	var env []string
	for _, e := range os.Environ() {
		key := strings.Split(e, "=")[0]
		if !skipVars[key] {
			env = append(env, e)
		}
	}
	return env
}
