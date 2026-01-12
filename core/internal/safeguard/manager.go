package safeguard

import (
	"fmt"

	"github.com/igoryan-dao/ricochet/internal/paths"
	"github.com/igoryan-dao/ricochet/internal/safeguard/checkpoint"
)

// Manager handles all safeguard operations including checkpoints
type Manager struct {
	gitManager      *checkpoint.GitManager
	PermissionStore *PermissionStore
	CurrentZone     TrustZone
}

// NewManager creates a new safeguard manager
func NewManager(cwd string) (*Manager, error) {
	// Use global storage or similar for shadow git
	// For now, we use ~/.ricochet/shadow-git
	shadowBasePath := paths.GetShadowGitDir(cwd)
	if err := paths.EnsureDir(shadowBasePath); err != nil {
		return nil, fmt.Errorf("failed to create shadow base dir: %w", err)
	}

	gitMgr, err := checkpoint.NewGitManager(cwd, shadowBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create git manager: %w", err)
	}

	// Initialize repo
	if err := gitMgr.Init(); err != nil {
		return nil, fmt.Errorf("failed to init shadow repo: %w", err)
	}

	permStore, err := NewPermissionStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission store: %w", err)
	}

	return &Manager{
		gitManager:      gitMgr,
		PermissionStore: permStore,
		CurrentZone:     ZoneSafe, // Default to Safe Zone
	}, nil
}

// CreateCheckpoint creates a checkpoint of the current state
func (m *Manager) CreateCheckpoint(message string) (string, error) {
	return m.gitManager.Commit(message)
}

// RestoreCheckpoint restores the state to a specific checkpoint
func (m *Manager) RestoreCheckpoint(commitHash string) error {
	return m.gitManager.Restore(commitHash)
}

// CheckPermission verifies if the tool execution is allowed in the current zone
func (m *Manager) CheckPermission(tool string) error {
	return CheckZonePermission(m.CurrentZone, tool)
}
