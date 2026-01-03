package sessions

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session represents an Antigravity conversation session
type Session struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Workspace      string    `json:"workspace"`
	WorkspaceDir   string    `json:"workspace_dir"`
	UpdatedAt      time.Time `json:"updated_at"`
	HasTask        bool      `json:"has_task"`
	HasPlan        bool      `json:"has_plan"`
	HasWalkthrough bool      `json:"has_walkthrough"`
}

// ArtifactMetadata represents the metadata.json structure
type ArtifactMetadata struct {
	ArtifactType string `json:"artifactType"`
	Summary      string `json:"summary"`
	UpdatedAt    string `json:"updatedAt"`
}

// Manager handles session discovery and reading
type Manager struct {
	antigravityDir string
}

// NewManager creates a new session manager
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	return &Manager{
		antigravityDir: filepath.Join(homeDir, ".gemini", "antigravity"),
	}
}

// GetSessions returns all sessions sorted by last update time
func (m *Manager) GetSessions(limit int) ([]Session, error) {
	brainDir := filepath.Join(m.antigravityDir, "brain")

	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, err
	}

	var sessions []Session

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sessionDir := filepath.Join(brainDir, sessionID)

		session := Session{
			ID:        sessionID,
			Workspace: "Global", // Default
		}

		// Get directory modification time
		info, err := entry.Info()
		if err == nil {
			session.UpdatedAt = info.ModTime()
		}

		// Try to read artifacts to find workspace and information
		m.enrichSession(sessionDir, &session)

		// Only include sessions with some content
		if session.HasTask || session.HasPlan || session.HasWalkthrough {
			if session.Title == "" {
				session.Title = "Session " + sessionID[:8]
			}
			sessions = append(sessions, session)
		}
	}

	// Sort by UpdatedAt descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// Limit results
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// enrichSession reads artifacts to fill session details
func (m *Manager) enrichSession(sessionDir string, session *Session) {
	// 1. Check for artifacts
	artifacts := []string{"walkthrough.md", "implementation_plan.md", "task.md"}
	for _, art := range artifacts {
		path := filepath.Join(sessionDir, art)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		switch art {
		case "task.md":
			session.HasTask = true
			if session.Title == "" {
				session.Title = extractTitle(string(content))
			}
		case "implementation_plan.md":
			session.HasPlan = true
		case "walkthrough.md":
			session.HasWalkthrough = true
		}

		// Try to detect workspace from file links
		if session.Workspace == "Global" || session.Workspace == "" {
			ws := detectWorkspace(string(content))
			if ws != "" {
				session.Workspace = ws
			}
		}

		// Read metadata if available
		metaPath := path + ".metadata.json"
		if meta, err := readMetadata(metaPath); err == nil {
			if session.Summary == "" || (art == "walkthrough.md" && len(meta.Summary) > 50) {
				session.Summary = meta.Summary
			}
			if t, err := time.Parse(time.RFC3339, meta.UpdatedAt); err == nil {
				if t.After(session.UpdatedAt) {
					session.UpdatedAt = t
				}
			}
		}
	}
}

// detectWorkspace looks for file:/// paths to find the project root name
func detectWorkspace(content string) string {
	// Look for file:///Users/ pattern
	idx := strings.Index(content, "file:///Users/")
	if idx == -1 {
		return ""
	}

	// Skip file:///Users/ (14 chars)
	path := content[idx+14:]

	// We want to find the part after the username: /Users/username/PROJECT_NAME/
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}

	// parts[0] is username, parts[1] is project or Documents/Desktop
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		p = strings.ReplaceAll(p, "%20", " ")
		if p == "" || p == "Documents" || p == "Desktop" || p == "Downloads" || p == "Source" || p == "Work" {
			continue
		}
		// Found something that looks like a project name
		if endIdx := strings.Index(p, ")"); endIdx != -1 {
			p = p[:endIdx]
		}
		return p
	}

	return ""
}

// GetSession returns a single session by ID
func (m *Manager) GetSession(id string) (*Session, error) {
	sessions, err := m.GetSessions(0)
	if err != nil {
		return nil, err
	}

	for _, s := range sessions {
		if s.ID == id {
			return &s, nil
		}
	}

	return nil, fs.ErrNotExist
}

// extractTitle extracts title from markdown content (first # heading or task name)
func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// readMetadata reads and parses a metadata.json file
func readMetadata(path string) (*ArtifactMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta ArtifactMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// WorkspaceGroup holds sessions for a single workspace
type WorkspaceGroup struct {
	Name     string
	Sessions []Session
	Latest   time.Time
}

// GroupByWorkspace groups sessions and sorts groups by latest activity
func (m *Manager) GroupByWorkspace(sessions []Session) []WorkspaceGroup {
	groupsMap := make(map[string]*WorkspaceGroup)

	for _, s := range sessions {
		name := s.Workspace
		if name == "" {
			name = "Global"
		}

		group, ok := groupsMap[name]
		if !ok {
			group = &WorkspaceGroup{Name: name}
			groupsMap[name] = group
		}

		group.Sessions = append(group.Sessions, s)
		if s.UpdatedAt.After(group.Latest) {
			group.Latest = s.UpdatedAt
		}
	}

	var groups []WorkspaceGroup
	for _, g := range groupsMap {
		groups = append(groups, *g)
	}

	// Sort groups by latest activity (most recent workspace first)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Latest.After(groups[j].Latest)
	})

	return groups
}

// FormatTimeAgo returns human-readable time difference
func FormatTimeAgo(t time.Time) string {
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "только что"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return formatPlural(mins, "минуту", "минуты", "минут") + " назад"
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return formatPlural(hours, "час", "часа", "часов") + " назад"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return formatPlural(days, "день", "дня", "дней") + " назад"
	default:
		return t.Format("02.01.2006")
	}
}

// formatPlural returns correct Russian plural form with number
func formatPlural(n int, one, few, many string) string {
	var form string
	n10 := n % 10
	n100 := n % 100

	if n10 == 1 && n100 != 11 {
		form = one
	} else if n10 >= 2 && n10 <= 4 && (n100 < 10 || n100 >= 20) {
		form = few
	} else {
		form = many
	}

	return fmt.Sprintf("%d %s", n, form)
}
