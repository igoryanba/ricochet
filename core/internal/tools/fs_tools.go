package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/safeguard"
)

func (e *NativeExecutor) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(e.host.GetCWD(), path), nil
}

func (e *NativeExecutor) ListDir(args json.RawMessage) (string, error) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	infos, err := e.host.ListDir(payload.Path)
	if err != nil {
		return "", fmt.Errorf("list dir: %w", err)
	}

	var result string
	for _, info := range infos {
		typeStr := "file"
		if info.IsDir {
			typeStr = "dir"
		}
		result += fmt.Sprintf("%s (%s)\n", info.Name, typeStr)
	}

	if result == "" {
		return "(empty directory)", nil
	}
	return result, nil
}

func (e *NativeExecutor) ReadFile(args json.RawMessage) (string, error) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	content, err := e.host.ReadFile(payload.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}

func (e *NativeExecutor) WriteFile(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Dynamic Mode check
	if allowed, msg := e.modes.CanAccessFile(payload.Path); !allowed {
		return "", fmt.Errorf("permission denied: %s", msg)
	}

	// INTERACTIVE CONSENT (Phase 11)
	if err := e.ensureConsent(ctx, "write_file", payload.Path, fmt.Sprintf("Write to file: %s", payload.Path)); err != nil {
		return "", err
	}

	// SAFETY: Create checkpoint before writing
	if e.safeguard != nil {
		msg := fmt.Sprintf("Checkpoint before writing to %s", payload.Path)
		if _, err := e.safeguard.CreateCheckpoint(msg); err != nil {
			return "", fmt.Errorf("failed to create safeguard checkpoint: %w", err)
		}
	} else {
		// Fallback to simple backup if safeguard not initialized (e.g. tests)
		if err := safeguard.Backup(e.host.GetCWD() + "/" + payload.Path); err != nil {
			return "", fmt.Errorf("safeguard backup failed: %w", err)
		}
	}

	if err := e.host.WriteFile(payload.Path, []byte(payload.Content)); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// PHASE 11: Shadow Workspace (Linter Loop)
	// Verify the written file immediately
	if e.shadowVerifier != nil {
		if err := e.shadowVerifier.Verify(ctx, payload.Path); err != nil {
			// We return an error to force the agent to fix it.
			// But we clarify that the file WAS written.
			return "", fmt.Errorf("file written, but failed verification: %w. Please fix the code", err)
		}
	}

	return "File written successfully", nil
}

func (e *NativeExecutor) ensureConsent(ctx context.Context, tool, path, description string) error {
	// 1. Check persistent permissions (Phase 15)
	if e.safeguard != nil && e.safeguard.PermissionStore != nil {
		if e.safeguard.PermissionStore.IsAllowed(tool, path) {
			return nil // Auto-allowed
		}
	}

	// 2. Check mode context
	mode := e.modes.GetActiveMode()
	question := fmt.Sprintf("Mode: %s\n\nDo you allow Ricochet to perform the following action?\n\n%s", mode.Name, description)

	// 3. Ask User
	var response string
	var err error

	if e.livemode != nil && e.livemode.IsEnabled() {
		// Ether Mode - ask via Telegram
		response, err = e.livemode.AskUserRemote(ctx, question)
	} else {
		// IDE Mode - ask via host popup
		response, err = e.host.AskUser(question)
	}

	if err != nil {
		return fmt.Errorf("failed to get user consent: %w", err)
	}

	// 4. Handle Response
	resp := strings.ToLower(response)
	if resp == "always allow" {
		if e.safeguard != nil && e.safeguard.PermissionStore != nil {
			err := e.safeguard.PermissionStore.AddRule(safeguard.PermissionRule{
				Tool:   tool,
				Path:   path,
				Action: "allow",
				Scope:  safeguard.ScopeProject,
			})
			if err != nil {
				return fmt.Errorf("failed to save permission: %w", err)
			}
			return nil
		}
		// If no safeguard store, treat as single "yes"
		return nil
	}

	if resp != "yes" {
		return fmt.Errorf("action was rejected by user")
	}

	return nil
}

func (e *NativeExecutor) CodebaseSearch(ctx context.Context, args json.RawMessage) (string, error) {
	if e.indexer == nil {
		return "", fmt.Errorf("code indexing is not enabled or indexer not initialized")
	}

	var payload struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if payload.Limit <= 0 {
		payload.Limit = 5
	}

	results, err := e.indexer.Search(ctx, payload.Query, payload.Limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No relevant code sections found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Semantic search results for '%s':\n\n", payload.Query))
	for _, res := range results {
		sb.WriteString(fmt.Sprintf("--- %s (Lines %d-%d, Score: %.2f) ---\n",
			res.Document.FilePath, res.Document.LineStart, res.Document.LineEnd, res.Score))
		sb.WriteString(res.Document.Content)
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}
