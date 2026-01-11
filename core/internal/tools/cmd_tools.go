package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func (e *NativeExecutor) ExecuteCommand(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Command    string `json:"command"`
		Background bool   `json:"background"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// INTERACTIVE CONSENT (Phase 11)
	actionDesc := fmt.Sprintf("Execute command: %s", payload.Command)
	if payload.Background {
		actionDesc += " (in background)"
	}

	if !IsSafeCommand(payload.Command) {
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
