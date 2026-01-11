package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager handles project-specific rules discovery and loading
type Manager struct {
	cwd string
}

func NewManager(cwd string) *Manager {
	return &Manager{cwd: cwd}
}

// GetRules loads all .md files from .ricochet/rules and returns them as a single string
func (m *Manager) GetRules() string {
	rulesDir := filepath.Join(m.cwd, ".ricochet", "rules")
	files, err := os.ReadDir(rulesDir)
	if err != nil {
		// No rules directory or error reading it
		return ""
	}

	var sb strings.Builder
	hasRules := false
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
			if !hasRules {
				sb.WriteString("\n\n### Project-Specific Rules\n")
				hasRules = true
			}
			content, err := os.ReadFile(filepath.Join(rulesDir, f.Name()))
			if err == nil {
				sb.WriteString(fmt.Sprintf("\n#### Rule: %s\n%s\n", f.Name(), string(content)))
			}
		}
	}
	return sb.String()
}
