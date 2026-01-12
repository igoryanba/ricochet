package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/context/handoff"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

func main() {
	// Mock Generator
	mockGen := func(ctx context.Context, prompt string) (string, error) {
		if strings.Contains(prompt, "CONVERSATION HISTORY:") {
			return "# SPEC.md\n\n## Goal\nTest goal.\n", nil
		}
		return "", fmt.Errorf("prompt missing history")
	}

	service := handoff.NewService(mockGen)

	msgs := []protocol.Message{
		{Role: "user", Content: "I want to build a rocket."},
		{Role: "assistant", Content: "Okay, let's design the engine."},
	}

	spec, err := service.GenerateSpec(context.Background(), msgs)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return
	}

	if strings.Contains(spec, "# SPEC.md") {
		fmt.Println("SUCCESS: Handoff prompt generation and callback working.")
		fmt.Printf("Generated Spec:\n%s\n", spec)
	} else {
		fmt.Printf("FAILED: Unexpected output: %s\n", spec)
	}
}
