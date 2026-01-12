package safeguard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type PermissionScope string

const (
	ScopeGlobal  PermissionScope = "global"
	ScopeProject PermissionScope = "project"

	ZoneDanger   TrustZone = 0 // "God Mode"
	ZoneSafe     TrustZone = 1 // Default
	ZoneReadOnly TrustZone = 2 // Analysis only
)

type TrustZone int

// ZoneConfig maps tools to their minimum required zone (Lower zone = Higher trust required)
var toolZoneMap = map[string]TrustZone{
	"execute_command": ZoneDanger,   // Dangerous
	"write_file":      ZoneSafe,     // Safe (Project only)
	"execute_python":  ZoneSafe,     // Safe (Sandboxed - theoretically)
	"read_file":       ZoneReadOnly, // Read only
	"list_dir":        ZoneReadOnly,
	"codebase_search": ZoneReadOnly,
	"browser_open":    ZoneReadOnly,
}

type PermissionRule struct {
	Tool   string          `json:"tool"`
	Path   string          `json:"path,omitempty"` // Regex or exact path
	Action string          `json:"action"`         // "allow", "deny"
	Scope  PermissionScope `json:"scope"`
}

type Permissions struct {
	Rules []PermissionRule `json:"rules"`
}

type PermissionStore struct {
	mu          sync.RWMutex
	path        string
	permissions *Permissions
}

// NewPermissionStore creates a store for persistent permissions
func NewPermissionStore() (*PermissionStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ricochet")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	store := &PermissionStore{
		path: filepath.Join(configDir, "permissions.json"),
		permissions: &Permissions{
			Rules: []PermissionRule{},
		},
	}

	if err := store.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load permissions: %w", err)
		}
		// If file doesn't exist, save default
		if err := store.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default permissions: %w", err)
		}
	}

	return store, nil
}

func (s *PermissionStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var perms Permissions
	if err := json.Unmarshal(data, &perms); err != nil {
		return fmt.Errorf("failed to parse permissions.json: %w", err)
	}

	s.permissions = &perms
	return nil
}

// Save writes permissions to disk. NOTE: Caller must hold lock if needed.
func (s *PermissionStore) Save() error {
	// Removed internal locking to prevent deadlock since AddRule calls this while holding Lock
	data, err := json.MarshalIndent(s.permissions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *PermissionStore) AddRule(rule PermissionRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicates? For now just append
	s.permissions.Rules = append(s.permissions.Rules, rule)
	return s.Save() // Auto-save
}

func (s *PermissionStore) IsAllowed(tool string, path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, rule := range s.permissions.Rules {
		if rule.Tool == tool && rule.Action == "allow" {
			if rule.Path == "" || rule.Path == path { // Simple match for MVP
				return true
			}
			// Regex match could be added here
		}
	}
	return false
}

// CheckZonePermission checks if a tool is allowed in the given zone
func CheckZonePermission(zone TrustZone, tool string) error {
	requiredZone, ok := toolZoneMap[tool]
	if !ok {
		// Unknown tools are treated as Dangerous by default?
		// Or Safe by default? Let's say Safe (1) if it's passive, but we don't know.
		// BETTER: Default to ZoneReadOnly (2) unless listed.
		// Actually, let's default to ZoneSafe for unknown tools to be permissive during dev,
		// but blocked in ReadOnly.
		requiredZone = ZoneSafe
	}

	// Zone 0 (Danger) < Zone 1 (Safe) < Zone 2 (ReadOnly)
	// User Zone must be <= Required Zone to be allowed.
	// Example: User=0 (Danger), Required=0 (Danger) -> OK
	// Example: User=1 (Safe), Required=0 (Danger) -> DENIED
	// Example: User=2 (ReadOnly), Required=1 (Safe) -> DENIED

	if zone > requiredZone {
		return fmt.Errorf("tool '%s' requires trust zone %d, but current zone is %d", tool, requiredZone, zone)
	}
	return nil
}
