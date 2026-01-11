package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (e *NativeExecutor) BrowserOpen(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := e.browser.Navigate(ctx, payload.URL); err != nil {
		return "", fmt.Errorf("failed to open URL: %w", err)
	}

	return fmt.Sprintf("Successfully opened %s", payload.URL), nil
}

func (e *NativeExecutor) BrowserScreenshot(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := e.browser.Screenshot(ctx, payload.URL)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Save screenshot
	screenshotDir := filepath.Join(e.host.GetCWD(), ".ricochet", "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create screenshot directory: %w", err)
	}

	filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	path := filepath.Join(screenshotDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	return fmt.Sprintf("Screenshot captured and saved to %s", path), nil
}

func (e *NativeExecutor) BrowserClick(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		URL      string `json:"url"`
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := e.browser.Click(ctx, payload.URL, payload.Selector); err != nil {
		return "", fmt.Errorf("failed to click element: %w", err)
	}

	return fmt.Sprintf("Successfully clicked %s on %s", payload.Selector, payload.URL), nil
}

func (e *NativeExecutor) BrowserType(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		URL      string `json:"url"`
		Selector string `json:"selector"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if err := e.browser.Type(ctx, payload.URL, payload.Selector, payload.Text); err != nil {
		return "", fmt.Errorf("failed to type text: %w", err)
	}

	return fmt.Sprintf("Successfully typed text into %s on %s", payload.Selector, payload.URL), nil
}
