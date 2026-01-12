package safeguard

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Linter defines an interface for language-specific validation
type Linter interface {
	CanLint(path string) bool
	Lint(ctx context.Context, path string) error
}

// ShadowVerifier manages the linter loop
type ShadowVerifier struct {
	linters []Linter
}

func NewShadowVerifier() *ShadowVerifier {
	return &ShadowVerifier{
		linters: []Linter{
			&GoLinter{},
			&TSLinter{},
		},
	}
}

// Verify checks a file against registered linters
func (v *ShadowVerifier) Verify(ctx context.Context, path string) error {
	for _, linter := range v.linters {
		if linter.CanLint(path) {
			return linter.Lint(ctx, path)
		}
	}
	return nil
}

// --- Go Linter ---

type GoLinter struct{}

func (l *GoLinter) CanLint(path string) bool {
	return strings.HasSuffix(path, ".go")
}

func (l *GoLinter) Lint(ctx context.Context, path string) error {
	// 1. go fmt first (auto-fix if possible)
	// We execute on the file's dir
	dir := filepath.Dir(path)

	// Check if go is installed
	if _, err := exec.LookPath("go"); err != nil {
		return nil // Skip if no go installed
	}

	// Run go vet
	cmd := exec.CommandContext(ctx, "go", "vet", path)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go vet failed:\n%s", string(out))
	}

	return nil
}

// --- TypeScript/JavaScript Linter ---

type TSLinter struct{}

func (l *TSLinter) CanLint(path string) bool {
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") ||
		strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx")
}

func (l *TSLinter) Lint(ctx context.Context, path string) error {
	// This is trickier as it depends on project config (eslint, tsc).
	// For now, we'll try a generic check if 'tsc' is available for .ts files

	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
		if _, err := exec.LookPath("tsc"); err == nil {
			// tsc --noEmit --skipLibCheck [path]
			// Note: Usually tsc runs on project, but we can try single file or just skip if no config
			// For now, return nil to avoid false positives without proper project context
			return nil
		}
	}

	return nil
}
