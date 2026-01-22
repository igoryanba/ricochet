package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Checkpoint represents a snapshot of specific files at a point in time
type Checkpoint struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Files     map[string]string `json:"files"` // RelativePath -> Content
}

// CheckpointManager handles the persistence and retrieval of project snapshots
type CheckpointManager struct {
	projectRoot string
	storageDir  string
}

// NewCheckpointManager creates a new manager for the given project
func NewCheckpointManager(projectRoot string) *CheckpointManager {
	storageDir := filepath.Join(projectRoot, ".ricochet", "checkpoints")
	return &CheckpointManager{
		projectRoot: projectRoot,
		storageDir:  storageDir,
	}
}

// Save creates a new checkpoint of the specified files
func (m *CheckpointManager) Save(name string, filePaths []string) (string, error) {
	if err := os.MkdirAll(m.storageDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create checkpoints dir: %w", err)
	}

	checkpoint := &Checkpoint{
		ID:        uuid.New().String(),
		Name:      name,
		Timestamp: time.Now(),
		Files:     make(map[string]string),
	}

	for _, path := range filePaths {
		// Ensure path is relative to project root for portability
		relPath, err := filepath.Rel(m.projectRoot, path)
		if err != nil {
			relPath = path // fallback to absolute if not in root
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue // skip files that can't be read
		}
		checkpoint.Files[relPath] = string(content)
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	fileName := fmt.Sprintf("%s_%s.json", checkpoint.Timestamp.Format("20060102_150405"), checkpoint.ID[:8])
	targetPath := filepath.Join(m.storageDir, fileName)

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	return checkpoint.ID, nil
}

// List returns a list of all available checkpoints, sorted by timestamp descending
func (m *CheckpointManager) List() ([]Checkpoint, error) {
	if _, err := os.Stat(m.storageDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(m.storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoints dir: %w", err)
	}

	var checkpoints []Checkpoint
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.storageDir, entry.Name()))
		if err != nil {
			continue
		}

		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err == nil {
			checkpoints = append(checkpoints, cp)
		}
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	return checkpoints, nil
}

// Restore reverts files to the state captured in the specified checkpoint ID or Name
func (m *CheckpointManager) Restore(idOrName string) error {
	checkpoints, err := m.List()
	if err != nil {
		return err
	}

	var target *Checkpoint
	for _, cp := range checkpoints {
		if cp.ID == idOrName || cp.Name == idOrName || cp.ID[:8] == idOrName {
			target = &cp
			break
		}
	}

	if target == nil {
		return fmt.Errorf("checkpoint not found: %s", idOrName)
	}

	for relPath, content := range target.Files {
		absPath := filepath.Join(m.projectRoot, relPath)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for %s: %w", relPath, err)
		}

		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to restore file %s: %w", relPath, err)
		}
	}

	return nil
}
