package safeguard

import (
	"fmt"

	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/paths"
	"github.com/igoryan-dao/ricochet/internal/safeguard/checkpoint"
)

// Manager handles all safeguard operations including checkpoints
type Manager struct {
	gitManager      *checkpoint.GitManager
	PermissionStore *PermissionStore
	CurrentZone     TrustZone
	AutoApproval    *config.AutoApprovalSettings
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

// SetAutoApproval updates the auto-approval settings
func (m *Manager) SetAutoApproval(settings *config.AutoApprovalSettings) {
	m.AutoApproval = settings
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
	// Check Auto-Approval checks first
	if m.AutoApproval != nil && m.AutoApproval.Enabled {
		switch tool {
		case "execute_command":
			if m.AutoApproval.ExecuteAllCommands {
				return nil
			}
			if m.AutoApproval.ExecuteSafeCommands && IsSafeCommand(tool) { // Need to verify IsSafeCommand or equivalent logic
				return nil
			}
		case "read_file", "list_dir", "codebase_search":
			if m.AutoApproval.ReadFiles {
				return nil
			}
		case "write_file", "replace_file_content", "apply_diff":
			if m.AutoApproval.EditFiles {
				return nil
			}
		case "browser_open", "browser_click", "browser_type":
			if m.AutoApproval.UseBrowser {
				return nil
			}
		}
	}

	return CheckZonePermission(m.CurrentZone, tool)
}

// Helper to check if a command is generally safe (simple heuristic)
// Since we don't have the command args here, we just return false for now unless we change signature.
// For now, we rely on 'ExecuteAllCommands' for the "Always Proceed" button which is the user's issue.
func IsSafeCommand(tool string) bool {
	// Refactor: We might needs args to determine if command is safe (e.g. ls vs rm)
	// But CheckPermission only takes tool name.
	// We will trust the Zone system for finer grained control if AutoApproval is off.
	return false
}
