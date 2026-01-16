package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindApplicableSkills(t *testing.T) {
	// Setup temporary directory with skills
	tmpDir, err := os.MkdirTemp("", "skills_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .agent/skills structure
	skillsDir := filepath.Join(tmpDir, ".agent", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create skill-rules.json
	rulesJSON := `{
		"backend-skill": {
			"type": "domain",
			"enforcement": "suggest",
			"promptTriggers": {
				"keywords": ["controller", "service"],
				"intentPatterns": ["create.*endpoint"]
			},
			"fileTriggers": {
				"pathPatterns": ["**/*.go"]
			}
		},
		"frontend-skill": {
			"type": "domain",
			"enforcement": "suggest",
			"promptTriggers": {
				"keywords": ["react", "component"]
			},
			"fileTriggers": {
				"pathPatterns": ["**/*.tsx"]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(skillsDir, "skill-rules.json"), []byte(rulesJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create dummy markdown files
	if err := os.WriteFile(filepath.Join(skillsDir, "backend-skill.md"), []byte("# Backend Rules"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize Manager
	opt := NewManager(tmpDir)
	if err := opt.LoadSkills(); err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	tests := []struct {
		name        string
		prompt      string
		activeFiles []string
		wantSkill   string
	}{
		{
			name:        "Keyword Trigger (backend)",
			prompt:      "I need to fix the auth controller",
			activeFiles: nil,
			wantSkill:   "backend-skill",
		},
		{
			name:        "Regex Trigger (backend)",
			prompt:      "Let's create a new user endpoint",
			activeFiles: nil,
			wantSkill:   "backend-skill",
		},
		{
			name:        "File Trigger (backend)",
			prompt:      "Fix this bug",
			activeFiles: []string{"/path/to/main.go"},
			wantSkill:   "backend-skill",
		},
		{
			name:        "Keyword Trigger (frontend)",
			prompt:      "Update the button component",
			activeFiles: nil,
			wantSkill:   "frontend-skill",
		},
		{
			name:        "No Match",
			prompt:      "Hello world",
			activeFiles: []string{"README.md"},
			wantSkill:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skills := opt.FindApplicableSkills(tt.prompt, tt.activeFiles)

			if tt.wantSkill == "" {
				if len(skills) > 0 {
					t.Errorf("Expected no skills, got %d", len(skills))
				}
			} else {
				if len(skills) == 0 {
					t.Errorf("Expected skill %s, got none", tt.wantSkill)
					return
				}
				found := false
				for _, s := range skills {
					if s.Name == tt.wantSkill {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected skill %s not found in results", tt.wantSkill)
				}
			}
		})
	}
}
