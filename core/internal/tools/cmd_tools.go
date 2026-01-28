package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/safeguard"
)

// Pattern to detect sed/awk commands that modify files
var fileModifyPattern = regexp.MustCompile(`(?i)^(sed|awk|perl)\s+.*[>|]\s*\S+\.`)

func (e *NativeExecutor) ExecuteCommand(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Command    string `json:"command"`
		Background bool   `json:"background"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Check for file modification commands (sed, awk) - redirect to replace_file_content
	cmd := strings.TrimSpace(payload.Command)
	if fileModifyPattern.MatchString(cmd) ||
		(strings.HasPrefix(cmd, "sed") && (strings.Contains(cmd, ">") || strings.Contains(cmd, "-i"))) {
		return "", fmt.Errorf("❌ DO NOT use sed/awk to modify files. Use 'replace_file_content' tool instead. This ensures proper diff visualization, checkpoints, and undo capability")
	}

	// INTERACTIVE CONSENT (Phase 11)
	actionDesc := fmt.Sprintf("Execute command: %s", payload.Command)
	if payload.Background {
		actionDesc += " (in background)"
	}

	// 1. Granular Permission Check (Phase 13)
	if e.safeguard != nil && e.safeguard.Permissions != nil {
		// We use CheckCommand from manager
		if err := e.safeguard.CheckCommand(strings.Split(payload.Command, " ")[0]); err != nil {
			return "", fmt.Errorf("safeguard: %w", err)
		}
	}

	if !safeguard.IsSafeCommand(payload.Command) {
		if err := e.ensureConsent(ctx, "execute_command", payload.Command, actionDesc); err != nil {
			return "", err
		}
	}

	res, err := e.host.ExecuteCommand(ctx, payload.Command, payload.Background)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	if payload.Background {
		return fmt.Sprintf("Command started in background. ID: %s\nUse command_status to check progress.", res.ID), nil
	}

	return res.Output, nil
}

func (e *NativeExecutor) GetCommandStatus(args json.RawMessage) (string, error) {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	status, ok := e.host.GetCommandStatus(payload.ID)
	if !ok {
		return "", fmt.Errorf("command not found: %s", payload.ID)
	}

	res, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal status: %w", err)
	}

	return string(res), nil
}
