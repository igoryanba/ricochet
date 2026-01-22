package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DynamicHookConfig represents the structure of a hook rule file
type DynamicHookConfig struct {
	Name       string      `yaml:"name"`
	Enabled    bool        `yaml:"enabled"`
	Event      string      `yaml:"event"`
	Action     string      `yaml:"action"` // warn, block
	Pattern    string      `yaml:"pattern"`
	Conditions []Condition `yaml:"conditions"`
	Message    string      `yaml:"message"` // Detailed message to show
}

type Condition struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"` // regex_match, contains, etc
	Pattern  string `yaml:"pattern"`
}

type DynamicHookManager struct {
	hooks []DynamicHookConfig
	cwd   string
}

func NewDynamicHookManager(cwd string) *DynamicHookManager {
	return &DynamicHookManager{
		cwd: cwd,
	}
}

func (m *DynamicHookManager) LoadHooks() error {
	hooksDir := filepath.Join(m.cwd, ".ricochet", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return err
	}

	m.hooks = []DynamicHookConfig{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Support .yaml and .md (with frontmatter)
		if strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(hooksDir, entry.Name())
			hook, err := m.loadHookFile(path)
			if err != nil {
				fmt.Printf("Error loading hook %s: %v\n", entry.Name(), err)
				continue
			}
			if hook.Enabled {
				m.hooks = append(m.hooks, hook)
			}
		}
	}
	return nil
}

func (m *DynamicHookManager) loadHookFile(path string) (DynamicHookConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DynamicHookConfig{}, err
	}

	var config DynamicHookConfig
	var yamlContent []byte

	if strings.HasSuffix(path, ".md") {
		// Parse frontmatter
		contentStr := string(data)
		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr, "---\n", 3)
			if len(parts) >= 3 {
				yamlContent = []byte(parts[1])
				// The body is the message if not specified
				if config.Message == "" {
					config.Message = strings.TrimSpace(parts[2])
				}
			}
		}
	} else {
		yamlContent = data
	}

	if len(yamlContent) > 0 {
		if err := yaml.Unmarshal(yamlContent, &config); err != nil {
			return DynamicHookConfig{}, err
		}
	}

	// If message is still empty and it was an md file, we handled it above.
	// If it was yaml and message is empty, we leave it empty.

	return config, nil
}

// ListHooks returns all currently loaded and enabled hooks
func (m *DynamicHookManager) ListHooks() []DynamicHookConfig {
	return m.hooks
}

// CheckPreToolUse validates a tool call against the hooks
func (m *DynamicHookManager) CheckPreToolUse(toolName string, args map[string]interface{}) (string, error) {
	// Re-load hooks to be dynamic? Or cache?
	// For performance, maybe cache, but for "Hookify" experience (instant update), re-load or watch is better.
	// Let's re-load for now (low volume).
	_ = m.LoadHooks()

	for _, hook := range m.hooks {
		// Only check relevant events
		if hook.Event != "all" && hook.Event != "tool" && hook.Event != "bash" && hook.Event != "file" {
			continue
		}

		// Specific mapping for Ricochet tools
		if hook.Event == "bash" && toolName != "execute_command" {
			continue
		}
		if hook.Event == "file" && toolName != "replace_file_content" && toolName != "write_to_file" && toolName != "edit_file" {
			continue
		}

		matched := false

		// check simple pattern
		if hook.Pattern != "" {
			// Whatfield to check against pattern?
			// bash -> command line
			if toolName == "execute_command" {
				if cmd, ok := args["command"].(string); ok {
					if ruleMatches(cmd, "regex_match", hook.Pattern) {
						matched = true
					}
				}
			}
		}

		// check conditions
		for _, cond := range hook.Conditions {
			val := getFieldVal(toolName, args, cond.Field)
			if ruleMatches(val, cond.Operator, cond.Pattern) {
				matched = true
			} else {
				matched = false
				break // All conditions must match
			}
		}

		if matched {
			if hook.Action == "block" {
				return "", fmt.Errorf("Hook '%s' blocked execution: %s", hook.Name, hook.Message)
			}
			if hook.Action == "warn" {
				// return warning string
				return fmt.Sprintf("Hook Warning (%s): %s", hook.Name, hook.Message), nil
			}
		}
	}

	return "", nil
}

func getFieldVal(_ string, args map[string]interface{}, field string) string {
	// Map conceptual fields to actual args
	if field == "command" {
		if v, ok := args["command"].(string); ok {
			return v
		}
		if v, ok := args["CommandLine"].(string); ok {
			return v
		}
	}
	if field == "file_path" {
		if v, ok := args["file_path"].(string); ok {
			return v
		}
		if v, ok := args["TargetFile"].(string); ok {
			return v
		}
		if v, ok := args["AbsolutePath"].(string); ok {
			return v
		}
	}
	return ""
}

func ruleMatches(val string, op string, pattern string) bool {
	switch op {
	case "contains":
		return strings.Contains(val, pattern)
	case "regex_match":
		r, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return r.MatchString(val)
	default: // default to regex
		r, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return r.MatchString(val)
	}
}
