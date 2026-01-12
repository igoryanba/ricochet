package main

import (
	"fmt"
	"os"

	"github.com/igoryan-dao/ricochet/internal/safeguard"
)

func main() {
	cwd, _ := os.Getwd()
	mgr, err := safeguard.NewManager(cwd)
	if err != nil {
		fmt.Printf("Error creating manager: %v\n", err)
		return
	}

	tests := []struct {
		name      string
		zone      safeguard.TrustZone
		tool      string
		expectErr bool
	}{
		// Danger Zone Tests
		{"Danger: ExecCmd", safeguard.ZoneDanger, "execute_command", false}, // Allowed
		{"Danger: Write", safeguard.ZoneDanger, "write_file", false},        // Allowed

		// Safe Zone Tests
		{"Safe: ExecCmd", safeguard.ZoneSafe, "execute_command", true}, // Denied (Needs Danger)
		{"Safe: Write", safeguard.ZoneSafe, "write_file", false},       // Allowed
		{"Safe: Read", safeguard.ZoneSafe, "read_file", false},         // Allowed

		// ReadOnly Zone Tests
		{"ReadOnly: Write", safeguard.ZoneReadOnly, "write_file", true}, // Denied
		{"ReadOnly: Read", safeguard.ZoneReadOnly, "read_file", false},  // Allowed
		{"ReadOnly: List", safeguard.ZoneReadOnly, "list_dir", false},   // Allowed
	}

	failed := false
	for _, tt := range tests {
		// Simulate Mode Switch
		mgr.CurrentZone = tt.zone

		err := mgr.CheckPermission(tt.tool)
		denied := err != nil

		if denied != tt.expectErr {
			fmt.Printf("FAILED: %s. Expected denied=%v, got %v (%v)\n", tt.name, tt.expectErr, denied, err)
			failed = true
		} else {
			fmt.Printf("PASSED: %s\n", tt.name)
		}
	}

	if failed {
		fmt.Println("Some tests failed.")
		os.Exit(1)
	}

	fmt.Println("ALL SECURITY CHECKS PASSED.")
}
