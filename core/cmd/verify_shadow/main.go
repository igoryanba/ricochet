package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/igoryan-dao/ricochet/internal/codegraph"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/index"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/safeguard"
	"github.com/igoryan-dao/ricochet/internal/tools"
)

func main() {
	cwd, _ := os.Getwd()
	fmt.Printf("Verifying Shadow Workspace (Linter Loop) in %s\n", cwd)

	// 1. Setup Executor
	fmt.Println("Initializing Host...")
	h := host.NewNativeHost(cwd)
	fmt.Println("Initializing Modes...")
	m := modes.NewManager(cwd) // Defaults to Code mode (Zone 1)
	fmt.Println("Initializing Safeguard...")
	sg, err := safeguard.NewManager(cwd)
	if err != nil {
		fmt.Printf("Safeguard init failed: %v\n", err)
		return
	}
	fmt.Println("Safeguard Initialized.")
	// We need 'safeguard' to be initialized for ShadowVerifier to work?
	// No, ShadowVerifier is in NativeExecutor struct, initialized in NewNativeExecutor.

	// Create Executor
	fmt.Println("Creating Executor...")
	exec := tools.NewNativeExecutor(h, m, sg, nil, &index.Indexer{}, &codegraph.Service{})
	fmt.Println("Executor Created.")

	// Bypass consent by adding permission rule
	if sg.PermissionStore != nil {
		fmt.Println("Adding permission rule...")
		err := sg.PermissionStore.AddRule(safeguard.PermissionRule{
			Tool:   "write_file",
			Path:   "shadow_test_gen.go",
			Action: "allow",
			Scope:  safeguard.ScopeProject,
		})
		if err != nil {
			fmt.Printf("AddRule failed: %v\n", err)
		}
		fmt.Println("Permission rule added.")
	} else {
		fmt.Println("PermissionStore is nil!")
	}

	ctx := context.Background()
	testFile := filepath.Join(cwd, "shadow_test_gen.go")
	defer os.Remove(testFile)

	// 2. Test Case: Invalid Go Code
	fmt.Println("\n--- Test 1: Writing Invalid Code ---")
	invalidCode := `package main
	func main() {
		fmt.Println("Hello") // Missing import
	}
	`
	args, _ := json.Marshal(map[string]string{
		"path":    "shadow_test_gen.go",
		"content": invalidCode,
	})

	_, err = exec.Execute(ctx, "write_file", json.RawMessage(args))
	if err == nil {
		fmt.Println("❌ FAILED: Executor accepted invalid code!")
		os.Exit(1)
	} else {
		fmt.Printf("✅ SUCCESS: Executor rejected invalid code.\nError: %v\n", err)
	}

	// 3. Test Case: Valid Go Code
	fmt.Println("\n--- Test 2: Writing Valid Code ---")
	validCode := `package main
	import "fmt"
	func main() {
		fmt.Println("Hello")
	}
	`
	argsValid, _ := json.Marshal(map[string]string{
		"path":    "shadow_test_gen.go",
		"content": validCode,
	})

	res, err := exec.Execute(ctx, "write_file", json.RawMessage(argsValid))
	if err != nil {
		fmt.Printf("❌ FAILED: Executor rejected valid code: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ SUCCESS: Valid code written. Result: %s\n", res)
}
