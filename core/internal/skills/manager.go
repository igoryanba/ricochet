package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// SkillRule defines when to trigger a specific skill
type SkillRule struct {
	Name           string        `json:"-"` // Key in JSON
	Type           string        `json:"type"`
	Enforcement    string        `json:"enforcement"` // suggest, force
	Priority       string        `json:"priority"`
	PromptTriggers TriggerConfig `json:"promptTriggers"`
	FileTriggers   TriggerConfig `json:"fileTriggers"`
	Content        string        `json:"-"` // Loaded content from the distinct file
	Description    string        `json:"description,omitempty"`
}

type TriggerConfig struct {
	Keywords        []string `json:"keywords,omitempty"`
	IntentPatterns  []string `json:"intentPatterns,omitempty"`
	PathPatterns    []string `json:"pathPatterns,omitempty"`
	ContentPatterns []string `json:"contentPatterns,omitempty"`
}

type Manager struct {
	mu     sync.RWMutex
	cwd    string
	skills map[string]*SkillRule
}

func NewManager(cwd string) *Manager {
	return &Manager{
		cwd:    cwd,
		skills: make(map[string]*SkillRule),
	}
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Enforcement string `yaml:"enforcement"`
	Priority    string `yaml:"priority"`
	Triggers    struct {
		Keywords        []string `yaml:"keywords,omitempty"`
		IntentPatterns  []string `yaml:"intentPatterns,omitempty"`
		PathPatterns    []string `yaml:"pathPatterns,omitempty"`
		ContentPatterns []string `yaml:"contentPatterns,omitempty"`
	} `yaml:"triggers"`
}

// LoadSkills loads the skill-rules.json and associated markdown files
func (m *Manager) LoadSkills() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rulesPath := filepath.Join(m.cwd, ".agent", "skills", "skill-rules.json")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return nil // No skills defined, totally fine
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("read skill rules: %w", err)
	}

	var rulesMap map[string]*SkillRule
	if err := json.Unmarshal(data, &rulesMap); err != nil {
		return fmt.Errorf("parse skill rules: %w", err)
	}

	m.skills = make(map[string]*SkillRule)
	for name, rule := range rulesMap {
		rule.Name = name

		// Load the actual skill content (e.g., .agent/skills/backend-dev-guidelines.md)
		// We assume the skill name maps to a markdown file
		skillPath := filepath.Join(m.cwd, ".agent", "skills", name+".md")
		if content, err := os.ReadFile(skillPath); err == nil {
			rule.Content = string(content)
		} else {
			// If no specific file, maybe content is in description or just a stub
			rule.Content = rule.Description
		}

		m.skills[name] = rule
	}

	// Load embedded skills
	for _, skill := range PluginDevSkills() {
		m.skills[skill.Name] = &SkillRule{
			Name:           skill.Name,
			Description:    skill.Description,
			Type:           "embedded",
			Enforcement:    skill.Enforcement,
			Content:        skill.Content,
			PromptTriggers: skill.Triggers,
		}
	}

	// ─── Phase 19: Dynamic Project Skills ───
	m.loadDynamicSkills()

	return nil
}

func (m *Manager) loadDynamicSkills() {
	skillsDir := filepath.Join(m.cwd, ".ricochet", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return // Directory might not exist, that's fine
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(skillsDir, skillName, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		content := string(data)
		rule := &SkillRule{
			Name: skillName,
			Type: "dynamic",
		}

		// Check for YAML frontmatter
		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content, "---", 3)
			if len(parts) >= 3 {
				var fm skillFrontmatter
				if err := yaml.Unmarshal([]byte(parts[1]), &fm); err == nil {
					if fm.Name != "" {
						rule.Name = fm.Name
					}
					rule.Description = fm.Description
					rule.Enforcement = fm.Enforcement
					rule.Priority = fm.Priority
					rule.PromptTriggers.Keywords = fm.Triggers.Keywords
					rule.PromptTriggers.IntentPatterns = fm.Triggers.IntentPatterns
					rule.FileTriggers.PathPatterns = fm.Triggers.PathPatterns
					rule.FileTriggers.ContentPatterns = fm.Triggers.ContentPatterns
					rule.Content = strings.TrimSpace(parts[2])
				}
			}
		}

		if rule.Content == "" {
			rule.Content = content
		}

		m.skills[rule.Name] = rule
	}
}

// FindApplicableSkills returns skills that match the context
func (m *Manager) FindApplicableSkills(prompt string, activeFiles []string) []*SkillRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*SkillRule
	seen := make(map[string]bool)

	for _, rule := range m.skills {
		if seen[rule.Name] {
			continue
		}

		matched := false

		// 1. Check Keywords
		for _, kw := range rule.PromptTriggers.Keywords {
			if strings.Contains(strings.ToLower(prompt), strings.ToLower(kw)) {
				matched = true
				break
			}
		}

		// 2. Check Intent Patterns (Regex)
		if !matched {
			for _, pat := range rule.PromptTriggers.IntentPatterns {
				if re, err := regexp.Compile("(?i)" + pat); err == nil {
					if re.MatchString(prompt) {
						matched = true
						break
					}
				}
			}
		}

		// 3. Check File Paths
		if !matched && len(activeFiles) > 0 {
			for _, pat := range rule.FileTriggers.PathPatterns {
				for _, file := range activeFiles {
					// Handle ** prefix manually since filepath.Match doesn't support it
					checkPat := pat
					if strings.HasPrefix(pat, "**/") {
						checkPat = strings.TrimPrefix(pat, "**/")
						// If file base matches the pattern
						// e.g. **/*.go matches /foo/bar/baz.go if baz.go matches *.go
						if match, _ := filepath.Match(checkPat, filepath.Base(file)); match {
							matched = true
							break
						}
					}

					// Standard glob matching
					if match, _ := filepath.Match(pat, file); match {
						matched = true
						break
					}
					// Also try matching relative path if pattern contains /
					rel, _ := filepath.Rel(m.cwd, file)
					if match, _ := filepath.Match(pat, rel); match {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		if matched {
			matches = append(matches, rule)
			seen[rule.Name] = true
		}
	}

	return matches
}
