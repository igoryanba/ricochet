package agent

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// InjectionProcessor handles @file and !cmd expansions
type InjectionProcessor struct {
	cwd string
}

// NewInjectionProcessor creates a new processor
func NewInjectionProcessor(cwd string) *InjectionProcessor {
	return &InjectionProcessor{cwd: cwd}
}

// Process expands injections in the input text
func (p *InjectionProcessor) Process(input string) (string, []string) {
	result := input
	var infoMessages []string

	// 1. Process @path/to/file
	// Regex matches @ followed by a path-like string (letters, numbers, dots, slashes, underscores, hyphens)
	fileRegex := regexp.MustCompile(`@([a-zA-Z0-9\./\-_]+)`)
	matches := fileRegex.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := match[1]
		content, err := os.ReadFile(path)
		if err != nil {
			infoMessages = append(infoMessages, fmt.Sprintf("⚠️ Could not read file @%s: %v", path, err))
			continue
		}

		// Append to the end of the message as a context block
		injection := fmt.Sprintf("\n\n---\n**Content of @%s:**\n```\n%s\n```\n---", path, string(content))
		result += injection
		infoMessages = append(infoMessages, fmt.Sprintf("📎 Injected file content: @%s", path))
	}

	// 2. Process !command
	// Regex matches ! followed by a shell command (until end of line or specific delimiters)
	// For simplicity, let's use !{cmd} if possible, or just !cmd until space?
	// Gemini uses !{cmd}, let's support both for UX.
	shellRegex := regexp.MustCompile(`!\{([^}]+)\}`)
	shellMatches := shellRegex.FindAllStringSubmatch(input, -1)

	for _, match := range shellMatches {
		if len(match) < 2 {
			continue
		}
		cmdStr := match[1]

		// Execute command
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			continue
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = p.cwd
		output, err := cmd.CombinedOutput()

		status := "success"
		if err != nil {
			status = fmt.Sprintf("failed: %v", err)
		}

		injection := fmt.Sprintf("\n\n---\n**Output of !{%s} (%s):**\n```\n%s\n```\n---", cmdStr, status, string(output))
		result += injection
		infoMessages = append(infoMessages, fmt.Sprintf("🖥️ Injected command output: !{%s}", cmdStr))
	}

	return result, infoMessages
}
