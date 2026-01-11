package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Verifier struct {
	workspace string
}

func (v *Verifier) Verify(expected Assertions) []string {
	var errors []string

	// 1. Verify Files
	for relPath, assertion := range expected.Files {
		errs := v.verifyFile(relPath, assertion)
		errors = append(errors, errs...)
	}

	// Future: Verify Tool calls if needed by collecting them in runner

	return errors
}

func (v *Verifier) verifyFile(relPath string, a FileAssertion) []string {
	var errors []string
	absPath := filepath.Join(v.workspace, relPath)

	_, err := os.Stat(absPath)
	exists := err == nil

	if a.Exists && !exists {
		errors = append(errors, fmt.Sprintf("File %s: expected to exist, but was not found", relPath))
		return errors
	}

	if !a.Exists && exists {
		errors = append(errors, fmt.Sprintf("File %s: expected to NOT exist, but was found", relPath))
		return errors
	}

	if !exists {
		return errors
	}

	// Content checks
	content, err := os.ReadFile(absPath)
	if err != nil {
		errors = append(errors, fmt.Sprintf("File %s: failed to read content: %v", relPath, err))
		return errors
	}

	strContent := string(content)

	if a.Exact != "" && strContent != a.Exact {
		errors = append(errors, fmt.Sprintf("File %s: exact match failed", relPath))
	}

	if a.Contains != "" && !strings.Contains(strContent, a.Contains) {
		errors = append(errors, fmt.Sprintf("File %s: content does not contain %q", relPath, a.Contains))
	}

	if a.NoSubstring != "" && strings.Contains(strContent, a.NoSubstring) {
		errors = append(errors, fmt.Sprintf("File %s: content contains forbidden substring %q", relPath, a.NoSubstring))
	}

	return errors
}
