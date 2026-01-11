package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// GetGlobalDir returns the root Ricochet directory in the user's home (~/.ricochet)
func GetGlobalDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ricochet")
}

// GetWorkspaceHash returns a short SHA256 hash of the absolute workspace path
func GetWorkspaceHash(workspaceRoot string) string {
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		abs = workspaceRoot
	}
	hash := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(hash[:8])
}

// GetSessionDir returns the global session directory for a specific workspace
func GetSessionDir(workspaceRoot string) string {
	hash := GetWorkspaceHash(workspaceRoot)
	return filepath.Join(GetGlobalDir(), "sessions", hash)
}

// GetLogDir returns the global log directory for a specific workspace
func GetLogDir(workspaceRoot string) string {
	hash := GetWorkspaceHash(workspaceRoot)
	return filepath.Join(GetGlobalDir(), "logs", hash)
}

// GetTmpDir returns the global temporary directory
func GetTmpDir() string {
	return filepath.Join(GetGlobalDir(), "tmp")
}

// GetShadowGitDir returns the global shadow git directory for a workspace
func GetShadowGitDir(workspaceRoot string) string {
	hash := GetWorkspaceHash(workspaceRoot)
	return filepath.Join(GetGlobalDir(), "shadow-git", hash)
}

// EnsureDir creates the directory and all parents if they don't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
