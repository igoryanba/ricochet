package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Workflow represents a user-defined automation workflow
type Workflow struct {
	Command     string   `json:"command"`     // e.g. "/release"
	Description string   `json:"description"` // e.g. "Prepare release"
	Content     string   `json:"content"`     // Raw markdown content
	Steps       []string `json:"steps"`       // Parsed steps (if applicable)
}

// Manager handles loading and retrieving workflows
type Manager struct {
	cwd       string
	mu        sync.RWMutex
	workflows map[string]Workflow
	Hooks     *HookManager
}

func NewManager(cwd string) *Manager {
	return &Manager{
		cwd:       cwd,
		workflows: make(map[string]Workflow),
		Hooks:     NewHookManager(cwd),
	}
}

// LoadWorkflows scans .agent/workflows/*.md and parses them
func (m *Manager) LoadWorkflows() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// reset
	m.workflows = make(map[string]Workflow)

	workflowDir := filepath.Join(m.cwd, ".agent", "workflows")
	if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
		return nil // No workflows defined yet
	}

	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(workflowDir, entry.Name())
		wf, err := m.parseWorkflow(path)
		if err != nil {
			// Log error but continue loading others
			fmt.Printf("Failed to parse workflow %s: %v\n", entry.Name(), err)
			continue
		}

		m.workflows[wf.Command] = wf
	}

	return nil
}

func (m *Manager) GetWorkflows() []Workflow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []Workflow
	for _, wf := range m.workflows {
		list = append(list, wf)
	}
	return list
}

func (m *Manager) GetWorkflow(command string) (Workflow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wf, ok := m.workflows[command]
	return wf, ok
}

// parseWorkflow reads a markdown file and extracts metadata
// Format expectation:
// ---
// description: My Workflow
// ---
// Steps...
func (m *Manager) parseWorkflow(path string) (Workflow, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, err
	}

	filename := filepath.Base(path)
	basename := strings.TrimSuffix(filename, filepath.Ext(filename))
	command := "/" + basename

	wf := Workflow{
		Command: command,
		Content: string(content),
		Steps:   []string{},
	}

	// Simple Frontmatter Parser
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inFrontmatter := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				continue
			}

			// Key: Value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Strip quotes
				val = strings.Trim(val, `"'`)

				if key == "description" {
					wf.Description = val
				}
			}
		}
	}

	// Fallback description if missing
	if wf.Description == "" {
		wf.Description = fmt.Sprintf("Run %s workflow", basename)
	}

	return wf, nil
}
