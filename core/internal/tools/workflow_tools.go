package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func (e *NativeExecutor) GetWorkflows(ctx context.Context, args json.RawMessage) (string, error) {
	// 1. Reload first to pick up any changes
	if e.workflows != nil {
		if err := e.workflows.LoadWorkflows(); err != nil {
			return "", fmt.Errorf("failed to reload workflows: %w", err)
		}

		wfs := e.workflows.GetWorkflows()

		// Return JSON string for structured parsing by UI
		data, err := json.Marshal(wfs)
		if err != nil {
			return "", fmt.Errorf("marshal error: %w", err)
		}

		return string(data), nil
	}

	return "[]", nil
}
