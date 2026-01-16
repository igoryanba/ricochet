package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

const defaultOpenAIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements Provider for OpenAI and compatible APIs (OpenRouter)
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o"
	}
	if baseURL == "" {
		baseURL = defaultOpenAIURL
	} else {
		// Ensure the URL ends with /chat/completions
		if !strings.HasSuffix(baseURL, "/chat/completions") {
			baseURL = strings.TrimSuffix(baseURL, "/") + "/chat/completions"
		}
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}
}

func (p *OpenAIProvider) Name() string {
	if strings.Contains(p.baseURL, "openrouter") {
		return "openrouter"
	}
	return "openai"
}

// openaiRequest is the OpenAI API request format
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"` // DeepSeek R1
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

// openaiResponse is the OpenAI API response format
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Chat performs a non-streaming chat completion
func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	openaiReq := p.buildRequest(req, false)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := doRequest(ctx, "POST", p.baseURL, p.headers(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", openaiResp.Error.Message)
	}

	return p.parseResponse(&openaiResp), nil
}

type openaiEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openaiEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (p *OpenAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embedURL := strings.Replace(p.baseURL, "/chat/completions", "/embeddings", 1)

	req := openaiEmbedRequest{
		Model: "text-embedding-3-small", // Default embedding model
		Input: texts,
	}

	body, _ := json.Marshal(req)
	resp, err := doRequest(ctx, "POST", embedURL, p.headers(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI Embed error %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp openaiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}

	result := make([][]float32, len(embedResp.Data))
	for i, d := range embedResp.Data {
		result[i] = d.Embedding
	}
	return result, nil
}

// ChatStream performs a streaming chat completion
func (p *OpenAIProvider) ChatStream(ctx context.Context, req *ChatRequest, callback StreamCallback) error {
	openaiReq := p.buildRequest(req, true)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := doRequest(ctx, "POST", p.baseURL, p.headers(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return p.processStream(resp.Body, callback)
}

func (p *OpenAIProvider) headers() map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.apiKey,
	}

	// OpenRouter specific headers
	if strings.Contains(p.baseURL, "openrouter") {
		headers["HTTP-Referer"] = "https://ricochet.dev"
		headers["X-Title"] = "Ricochet"
	}

	return headers
}

func (p *OpenAIProvider) buildRequest(req *ChatRequest, stream bool) *openaiRequest {
	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	// Add system message if present
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		// Handle tool results
		if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				messages = append(messages, openaiMessage{
					Role:       "tool",
					Content:    tr.Content,
					ToolCallID: tr.ToolUseID,
				})
			}
			continue
		}

		// Handle tool calls from assistant
		if len(msg.ToolUse) > 0 {
			toolCalls := make([]openaiToolCall, 0, len(msg.ToolUse))
			for _, tu := range msg.ToolUse {
				toolCalls = append(toolCalls, openaiToolCall{
					ID:   tu.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tu.Name,
						Arguments: string(tu.Input),
					},
				})
			}
			messages = append(messages, openaiMessage{
				Role:             msg.Role,
				Content:          msg.Content,
				ReasoningContent: msg.ReasoningContent, // DeepSeek R1 requires this for tool calls
				ToolCalls:        toolCalls,
			})
			continue
		}

		// Simple message
		messages = append(messages, openaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	tools := make([]openaiTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, openaiTool{
			Type: "function",
			Function: struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters"`
			}{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return &openaiRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		Stream:      stream,
	}
}

func (p *OpenAIProvider) parseResponse(resp *openaiResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{
			ID:    resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]

	var toolCalls []protocol.ToolUseBlock
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, protocol.ToolUseBlock{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return &ChatResponse{
		ID:         resp.ID,
		Model:      resp.Model,
		Content:    choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: choice.FinishReason,
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

// openaiStreamChunk is a streaming response chunk
type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string                 `json:"role,omitempty"`
			Content          string                 `json:"content,omitempty"`
			ReasoningContent string                 `json:"reasoning_content,omitempty"`
			ToolCalls        []openaiStreamToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// openaiStreamToolCall is a tool call in a streaming response (includes Index)
type openaiStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

func (p *OpenAIProvider) processStream(reader io.Reader, callback StreamCallback) error {
	scanner := bufio.NewScanner(reader)

	toolCallBuffers := make(map[int]*struct {
		id   string
		name string
		args strings.Builder
	})

	var inReasoning bool

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Close reasoning if still open
			if inReasoning {
				callback(&StreamChunk{
					Type:  "content_block_delta",
					Delta: "\n</thinking>\n\n",
				})
				inReasoning = false
			}
			callback(&StreamChunk{Type: "message_stop"})
			break
		}

		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Handle reasoning delta (DeepSeek R1/V3)
		if choice.Delta.ReasoningContent != "" {
			if !inReasoning {
				log.Printf("[OpenAI] Starting reasoning block, first chunk: %q", choice.Delta.ReasoningContent[:min(50, len(choice.Delta.ReasoningContent))])
				callback(&StreamChunk{
					Type:  "content_block_delta",
					Delta: "<thinking>\n",
				})
				inReasoning = true
			}
			// Send both: Delta for UI display, ReasoningDelta for storage
			callback(&StreamChunk{
				Type:           "content_block_delta",
				Delta:          choice.Delta.ReasoningContent,
				ReasoningDelta: choice.Delta.ReasoningContent,
			})
		}

		// Handle content delta
		if choice.Delta.Content != "" {
			if inReasoning {
				callback(&StreamChunk{
					Type:  "content_block_delta",
					Delta: "\n</thinking>\n\n",
				})
				inReasoning = false
			}
			callback(&StreamChunk{
				Type:  "content_block_delta",
				Delta: choice.Delta.Content,
			})
		}

		// Handle tool calls
		for _, tc := range choice.Delta.ToolCalls {
			if inReasoning {
				callback(&StreamChunk{
					Type:  "content_block_delta",
					Delta: "\n</thinking>\n\n",
				})
				inReasoning = false
			}

			if _, ok := toolCallBuffers[tc.Index]; !ok {
				toolCallBuffers[tc.Index] = &struct {
					id   string
					name string
					args strings.Builder
				}{
					id:   tc.ID,
					name: tc.Function.Name,
				}
			}

			buf := toolCallBuffers[tc.Index]
			if tc.ID != "" {
				buf.id = tc.ID
			}
			if tc.Function.Name != "" {
				buf.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				buf.args.WriteString(tc.Function.Arguments)
			}
		}

		// Handle finish reason
		if choice.FinishReason != "" {
			if inReasoning {
				callback(&StreamChunk{
					Type:  "content_block_delta",
					Delta: "\n</thinking>\n\n",
				})
				inReasoning = false
			}

			// Emit any buffered tool calls
			for _, buf := range toolCallBuffers {
				callback(&StreamChunk{
					Type: "tool_use",
					ToolUse: &protocol.ToolUseBlock{
						ID:    buf.id,
						Name:  buf.name,
						Input: json.RawMessage(buf.args.String()),
					},
				})
			}

			callback(&StreamChunk{
				Type:       "message_delta",
				StopReason: choice.FinishReason,
			})
		}
	}

	return scanner.Err()
}
