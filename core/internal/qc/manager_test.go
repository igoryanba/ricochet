package qc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestQCManager_GoProject(t *testing.T) {
	// Setup errors-ridden project
	tmpDir, err := os.MkdirTemp("", "qc_test_fail")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create BROKEN main.go
	brokenCode := `package main
	func main() {
		fmt.Println("Hello" // Missing parenthesis and import
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(brokenCode), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(tmpDir)

	// Test Detection
	cmd := mgr.detectCommand()
	if cmd != "go build ./..." {
		t.Errorf("Expected 'go build ./...', got '%s'", cmd)
	}

	// Test Failure
	ctx := context.Background()
	res, err := mgr.RunCheck(ctx)
	if err != nil {
		t.Fatalf("RunCheck failed unexpectedly: %v", err)
	}

	if res.Success {
		t.Error("Expected QC to fail for broken code, but it passed")
	}
	t.Logf("Got expected failure output: %s", res.Output)

	// Fix code
	validCode := `package main
	import "fmt"
	func main() {
		fmt.Println("Hello")
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(validCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Test Success
	res, err = mgr.RunCheck(ctx)
	if err != nil {
		t.Fatalf("RunCheck failed: %v", err)
	}
	if !res.Success {
		t.Errorf("Expected QC to pass for valid code, failed with: %s", res.Output)
	}
}
