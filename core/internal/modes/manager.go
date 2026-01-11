package modes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type Manager struct {
	cwd          string
	activeMode   string
	customModes  map[string]Mode
	onModeChange func(slug string)
	mu           sync.RWMutex
	loader       *Loader
	lastModTime  time.Time
}

func (m *Manager) SetOnModeChange(fn func(slug string)) {
	m.onModeChange = fn
}

func NewManager(cwd string) *Manager {
	m := &Manager{
		cwd:         cwd,
		activeMode:  "code", // Default mode
		customModes: make(map[string]Mode),
		loader:      &Loader{},
	}
	m.LoadFromProject()
	m.StartWatcher()
	return m
}

func (m *Manager) StartWatcher() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		configPath := filepath.Join(m.cwd, ".ricochet", "modes.yaml")

		for range ticker.C {
			info, err := os.Stat(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					// File deleted? clear modes?
					// For now, keep last known good state or do nothing
					continue
				}
				continue
			}

			if info.ModTime().After(m.lastModTime) {
				// File changed, reload
				m.LoadFromProject()
			}
		}
	}()
}

func (m *Manager) LoadFromProject() {
	configPath := filepath.Join(m.cwd, ".ricochet", "modes.yaml")

	info, err := os.Stat(configPath)
	if err != nil {
		return
	}

	cfg, err := m.loader.Load(configPath)
	if err != nil {
		fmt.Printf("Warning: Failed to parse .ricochet/modes.yaml: %v\n", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear old project modes
	m.customModes = make(map[string]Mode)

	for _, mode := range cfg.CustomModes {
		mode.Source = "project"
		m.customModes[mode.Slug] = mode
	}

	m.lastModTime = info.ModTime()

	// If active mode was updated, trigger callback
	if m.onModeChange != nil {
		// Just notify current mode again to refresh context
		m.onModeChange(m.activeMode)
	}
}

func (m *Manager) GetActiveMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if mode, ok := m.customModes[m.activeMode]; ok {
		return mode
	}

	// Fallback to builtin
	for _, mode := range BuiltinModes {
		if mode.Slug == m.activeMode {
			return mode
		}
	}

	return BuiltinModes[0] // Default to 'code'
}

func (m *Manager) SetMode(slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate slug exists
	found := false
	for _, mode := range BuiltinModes {
		if mode.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		if _, ok := m.customModes[slug]; ok {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("mode %s not found", slug)
	}

	m.activeMode = slug
	if m.onModeChange != nil {
		m.onModeChange(slug)
	}
	return nil
}

// CanAccessFile checks if the current mode is allowed to edit the given file
func (m *Manager) CanAccessFile(path string) (bool, string) {
	mode := m.GetActiveMode()
	if len(mode.FileRestrictions) == 0 {
		return true, ""
	}

	for _, rest := range mode.FileRestrictions {
		matched, _ := regexp.MatchString(rest.Regex, path)
		if matched {
			return true, ""
		}
	}

	return false, fmt.Sprintf("Mode %s is restricted to: %s", mode.Name, m.getRestrictionsSummary(mode))
}

func (m *Manager) getRestrictionsSummary(mode Mode) string {
	var summary []string
	for _, r := range mode.FileRestrictions {
		summary = append(summary, r.Description)
	}
	return filepath.Join(summary...)
}
