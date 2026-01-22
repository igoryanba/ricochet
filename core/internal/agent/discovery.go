package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectScanner handles automated project analysis
type ProjectScanner struct {
	cwd        string
	controller *Controller
}

// NewProjectScanner creates a new scanner
func NewProjectScanner(cwd string, controller *Controller) *ProjectScanner {
	return &ProjectScanner{
		cwd:        cwd,
		controller: controller,
	}
}

// ScanProject gathers key project info for initialization
func (s *ProjectScanner) ScanProject(ctx context.Context) (string, error) {
	var summary strings.Builder
	summary.WriteString("## Project Discovery Report\n\n")

	// 1. Directory Structure (Top level)
	entries, _ := os.ReadDir(s.cwd)
	summary.WriteString("### Directory Structure (Root):\n")
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".ricochet" {
			continue
		}
		prefix := "📄 "
		if e.IsDir() {
			prefix = "📁 "
		}
		summary.WriteString(fmt.Sprintf("%s%s\n", prefix, name))
	}
	summary.WriteString("\n")

	// 2. Core Config Files
	coreFiles := []string{"README.md", "go.mod", "package.json", "docker-compose.yml", "Makefile"}
	summary.WriteString("### Core Manifests:\n")
	for _, f := range coreFiles {
		path := filepath.Join(s.cwd, f)
		if _, err := os.Stat(path); err == nil {
			summary.WriteString(fmt.Sprintf("- Found `%s`\n", f))
		}
	}
	summary.WriteString("\n")

	// 3. Ask LLM to summarize
	prompt := fmt.Sprintf(`I have analyzed the project at %s. 
Here is its structure:
%s

Please provide a concise (1-2 paragraphs) overview of what this project does and its core architecture. 
This summary will be saved in the project's long-term memory.`, s.cwd, summary.String())

	description, err := s.controller.Execute(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM analysis failed: %w", err)
	}

	return description, nil
}
