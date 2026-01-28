package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// Provider represents an AI provider (Anthropic, OpenAI, OpenRouter)
type Provider interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest, callback StreamCallback) error
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
}

// StreamCallback is called for each streaming chunk
type StreamCallback func(chunk *StreamChunk) error

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model        string             `json:"model"`
	Messages     []protocol.Message `json:"messages"`
	MaxTokens    int                `json:"max_tokens,omitempty"`
	Temperature  float64            `json:"temperature,omitempty"`
	Tools        []protocol.Tool    `json:"tools,omitempty"`
	SystemPrompt string             `json:"system,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID         string                  `json:"id"`
	Model      string                  `json:"model"`
	Content    string                  `json:"content"`
	ToolCalls  []protocol.ToolUseBlock `json:"tool_calls,omitempty"`
	StopReason string                  `json:"stop_reason"`
	Usage      Usage                   `json:"usage"`
}

// Usage represents token usage
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Type           string                 `json:"type"` // content_block_delta, message_stop, etc.
	Delta          string                 `json:"delta,omitempty"`
	ReasoningDelta string                 `json:"reasoning_delta,omitempty"` // DeepSeek R1 reasoning
	ToolUse        *protocol.ToolUseBlock `json:"tool_use,omitempty"`
	StopReason     string                 `json:"stop_reason,omitempty"`
}

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	Provider     string `json:"provider"` // anthropic, openai, openrouter
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url,omitempty"` // For custom endpoints
	Organization string `json:"organization,omitempty"`
	Project      string `json:"project,omitempty"`
}

// NewProvider creates a provider based on config
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "anthropic":
		return NewAnthropicProvider(cfg.APIKey, cfg.Model), nil
	case "openai":
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.Organization, cfg.Project), nil
	case "openrouter":
		baseURL := "https://openrouter.ai/api/v1"
		if cfg.BaseURL != "" {
			baseURL = cfg.BaseURL
		}
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, baseURL, "", ""), nil // OpenRouter doesn't use standard Org/Project headers
	case "xai":
		return NewXAIProvider(cfg.APIKey, cfg.Model), nil
	case "gemini":
		return NewGeminiProvider(cfg.APIKey, cfg.Model), nil
	case "minimax":
		return NewMinimaxProvider(cfg.APIKey, cfg.Model), nil
	case "deepseek":
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, "https://api.deepseek.com/v1", "", ""), nil
	case "mistral":
		baseURL := "https://api.mistral.ai/v1"
		if cfg.BaseURL != "" {
			baseURL = cfg.BaseURL
		}
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, baseURL, "", ""), nil
	case "zhipu", "glm":
		baseURL := "https://api.z.ai/api/paas/v4"
		if cfg.BaseURL != "" {
			baseURL = cfg.BaseURL
		}
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, baseURL, "", ""), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// httpClient is a shared HTTP client with a long timeout for AI requests
var httpClient = &http.Client{
	Timeout: 10 * time.Minute,
	Transport: &http.Transport{
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

// doRequest performs an HTTP request and returns the response with retry logic
func doRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	// Read body into buffer for retries
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}

	retryDelay := 1 * time.Second
	maxRetries := 3

	for i := 0; i <= maxRetries; i++ {
		// Re-create reader
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return nil, err
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			// Check for transient errors or context cancellation that IS NOT the user hitting stop
			// Actually, if context is canceled, we should probably stop unless we are detached?
			// But here we rely on the passed context.
			// Ideally the caller manages context context.
			// If error is "context canceled", we check if we should retry?
			// Usually no, but if it's a network timeout (Client.Timeout exceeded while in Do), it returns error.

			// We optimize for "Network Flake".
			if i < maxRetries {
				log.Printf("[Network] Request failed: %v. Retrying in %v...", err, retryDelay)
				time.Sleep(retryDelay)
				retryDelay *= 2
				continue
			}
			return nil, err
		}

		// Check for 5xx errors
		if resp.StatusCode >= 500 {
			if i < maxRetries {
				log.Printf("[Network] API returned %d. Retrying in %v...", resp.StatusCode, retryDelay)
				resp.Body.Close()
				time.Sleep(retryDelay)
				retryDelay *= 2
				continue
			}
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}
