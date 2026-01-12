package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/tools"
)

func main() {
	script := `
import sys
print("Hello from Python!")
print("Stderr output", file=sys.stderr)
`
	ctx := context.Background()
	output, err := tools.ExecutePython(ctx, script)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return
	}

	if strings.Contains(output, "Hello from Python!") && strings.Contains(output, "Stderr output") {
		fmt.Println("SUCCESS: Python retrieval works.")
		fmt.Printf("Output:\n%s\n", output)
	} else {
		fmt.Printf("FAILED: Unexpected output: %s\n", output)
	}
}
