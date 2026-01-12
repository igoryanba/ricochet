package tools

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ExecutePython runs a python script in a temporary isolated file and returns its stdout/stderr
func ExecutePython(ctx context.Context, script string) (string, error) {
	// 1. Create a temp directory if not exists
	tempDir := filepath.Join(os.TempDir(), "ricochet_exec")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 2. Create a unique temp file
	filename := fmt.Sprintf("script_%d.py", time.Now().UnixNano())
	filePath := filepath.Join(tempDir, filename)

	if err := ioutil.WriteFile(filePath, []byte(script), 0644); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}
	// Cleanup on exit
	defer os.Remove(filePath)

	// 3. Prepare command
	// We use "python3" as the default command. Ensure python3 is in PATH.
	cmd := exec.CommandContext(ctx, "python3", filePath)

	// 4. Run and capture output
	// We combine stdout and stderr to give the full picture
	output, err := cmd.CombinedOutput()

	result := string(output)

	if err != nil {
		// If the process failed, we still want the output (syntax errors, exceptions)
		// But we also append the error message
		if result == "" {
			return "", fmt.Errorf("python execution failed: %v", err)
		}
		return fmt.Sprintf("%s\n\n(Execution failed with code: %v)", result, err), nil
	}

	if result == "" {
		return "(No output)", nil
	}

	return result, nil
}
