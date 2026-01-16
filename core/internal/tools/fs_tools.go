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
	// 0. Check AutoApproval settings (Always Proceed)
	if e.safeguard != nil && e.safeguard.AutoApproval != nil && e.safeguard.AutoApproval.Enabled {
		// Phase 11 Fix: If Auto-Approval is globally enabled (Act Mode), we allow ALL actions.
		// Previous granular logic caused false positives where "Act" mode was active but specific
		// flags were missing, causing telegram bugs.
		return nil

		/* Granular checks preserved for reference or future specific modes
		switch tool {
		case "execute_command":
			if e.safeguard.AutoApproval.ExecuteAllCommands {
				return nil
			}
		case "write_file", "replace_file_content", "apply_diff":
			if e.safeguard.AutoApproval.EditFiles {
				return nil
			}
		case "read_file", "list_dir", "codebase_search":
			if e.safeguard.AutoApproval.ReadFiles {
				return nil
			}
		case "browser_open", "browser_click", "browser_type":
			if e.safeguard.AutoApproval.UseBrowser {
				return nil
			}
		}
		*/
	}

	// 1. Check persistent permissions (Phase 15)
	if e.safeguard != nil && e.safeguard.PermissionStore != nil {
		if e.safeguard.PermissionStore.IsAllowed(tool, path) {
			return nil // Auto-allowed
		}
	}

	// 2. Check mode context
	mode := e.modes.GetActiveMode()
	question := fmt.Sprintf("Mode: %s\n\nDo you allow Ricochet to perform the following action?\n\n%s", mode.Name, description)

	// 3. Ask User (Dual-Channel if Live Mode enabled)
	var response string
	var err error

	if e.livemode != nil && e.livemode.IsEnabled() {
		// Ether Mode: Ask via Telegram ONLY
		response, err = e.livemode.AskUserRemote(ctx, question)
	} else {
		// IDE Mode - ask via host popup only
		response, err = e.host.AskUser(question)
	}

	if err != nil {
		return fmt.Errorf("failed to get user consent: %w", err)
	}

	// 4. Handle Response
	resp := strings.ToLower(strings.TrimSpace(response))

	// Handle various positive responses
	if resp == "yes" || resp == "y" || resp == "approve" || resp == "ok" {
		return nil
	}

	// Handle "Always" variations
	if strings.Contains(resp, "always") {
		// "always allow", "always proceed", "always"
		if e.safeguard != nil && e.safeguard.PermissionStore != nil {
			err := e.safeguard.PermissionStore.AddRule(safeguard.PermissionRule{
				Tool:   tool,
				Path:   path,
				Action: "allow",
				Scope:  safeguard.ScopeProject,
			})
			if err != nil {
				// Log but allow once
				fmt.Printf("Warning: failed to save permission: %v\n", err)
			}
		}
		return nil
	}

	return fmt.Errorf("action was rejected by user")
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

	// Use NativeExecutor as receiver to access host methods
	return sb.String(), nil
}

func (e *NativeExecutor) ReplaceFileContent(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Path               string `json:"path"`
		TargetContent      string `json:"TargetContent"`
		ReplacementContent string `json:"ReplacementContent"`
		// Aliases for compatibility
		TargetFile string `json:"TargetFile"`
	}
	// Try parsing both casings to be safe
	if err := json.Unmarshal(args, &payload); err != nil {
		// Fallback for lowerCamelCase args
		var payloadLower struct {
			Path               string `json:"path"`
			TargetContent      string `json:"targetContent"`
			ReplacementContent string `json:"replacementContent"`
			TargetFile         string `json:"targetFile"`
		}
		if err2 := json.Unmarshal(args, &payloadLower); err2 != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		payload.Path = payloadLower.Path
		if payload.Path == "" {
			payload.Path = payloadLower.TargetFile
		}
		payload.TargetContent = payloadLower.TargetContent
		payload.ReplacementContent = payloadLower.ReplacementContent
	}

	// Handle alias if Path is empty
	if payload.Path == "" {
		payload.Path = payload.TargetFile
	}

	if payload.Path == "" {
		return "", fmt.Errorf("Path or TargetFile is required")
	}

	if payload.TargetContent == "" {
		return "", fmt.Errorf("TargetContent cannot be empty")
	}

	// Dynamic Mode check
	if allowed, msg := e.modes.CanAccessFile(payload.Path); !allowed {
		return "", fmt.Errorf("permission denied: %s", msg)
	}

	// Verify file exists and read it
	contentBytes, err := e.host.ReadFile(payload.Path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	content := string(contentBytes)

	// Check if target exists
	if !strings.Contains(content, payload.TargetContent) {
		return "", fmt.Errorf("TargetContent not found in file. Please ensure exact match including whitespace.")
	}

	// Verify uniqueness
	if strings.Count(content, payload.TargetContent) > 1 {
		return "", fmt.Errorf("TargetContent found multiple times. Please provide more context to make it unique.")
	}

	// Perform replacement
	newContent := strings.Replace(content, payload.TargetContent, payload.ReplacementContent, 1)

	// Delegate to WriteFile logic to handle consents and checkpoints
	// We call WriteFile but we must be careful about double-consent?
	// WriteFile asks for consent.
	// But ReplaceFileContent invocation implies we want to perform this specific action.
	// We can manually call ensuresConset here with "replace_file_content" tool name.
	// BUT WriteFile calls ensureConsent for "write_file".
	// If we call e.WriteFile, it will ask for "write_file" permission.
	// It's better to implement the logic here directly or refactor.
	// Let's implement directly to use correct tool name "replace_file_content".

	// INTERACTIVE CONSENT
	if err := e.ensureConsent(ctx, "replace_file_content", payload.Path, fmt.Sprintf("Replace content in file: %s", payload.Path)); err != nil {
		return "", err
	}

	// CHECKPOINT
	if e.safeguard != nil {
		msg := fmt.Sprintf("Checkpoint before replace_file_content in %s", payload.Path)
		if _, err := e.safeguard.CreateCheckpoint(msg); err != nil {
			return "", fmt.Errorf("failed to create checkpoint: %w", err)
		}
	} else {
		// Fallback backup
		if err := safeguard.Backup(e.host.GetCWD() + "/" + payload.Path); err != nil {
			return "", fmt.Errorf("safeguard backup failed: %w", err)
		}
	}

	// WRITE
	if err := e.host.WriteFile(payload.Path, []byte(newContent)); err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}

	return "File updated successfully", nil
}
