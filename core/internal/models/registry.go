package models

// ModelInfo contains information about an AI model
type ModelInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	ContextWindow int     `json:"contextWindow"`
	InputPrice    float64 `json:"inputPrice"`  // per 1M tokens
	OutputPrice   float64 `json:"outputPrice"` // per 1M tokens
	IsFree        bool    `json:"isFree"`
	SupportsTools bool    `json:"supportsTools"`
	Description   string  `json:"description,omitempty"`
}

// ProviderInfo contains provider details and available models
type ProviderInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	HasKey      bool        `json:"hasKey"`
	IsAvailable bool        `json:"isAvailable"`
	Models      []ModelInfo `json:"models"`
}

// Registry holds all available providers and models
var Registry = map[string]ProviderInfo{
	"gemini": {
		ID:   "gemini",
		Name: "Google Gemini",
		Models: []ModelInfo{
			{ID: "gemini-3-flash", Name: "Gemini 3 Flash", Provider: "gemini", ContextWindow: 1000000, InputPrice: 0, OutputPrice: 0, IsFree: true, SupportsTools: true, Description: "Fast, free tier"},
			{ID: "gemini-3-pro", Name: "Gemini 3 Pro", Provider: "gemini", ContextWindow: 1000000, InputPrice: 1.25, OutputPrice: 5.0, IsFree: false, SupportsTools: true, Description: "Flagship model"},
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: "gemini", ContextWindow: 1000000, InputPrice: 0.075, OutputPrice: 0.30, IsFree: false, SupportsTools: true},
		},
	},
	"anthropic": {
		ID:   "anthropic",
		Name: "Anthropic (Claude)",
		Models: []ModelInfo{
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", ContextWindow: 200000, InputPrice: 3.0, OutputPrice: 15.0, IsFree: false, SupportsTools: true, Description: "Latest Sonnet"},
			{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Provider: "anthropic", ContextWindow: 200000, InputPrice: 15.0, OutputPrice: 75.0, IsFree: false, SupportsTools: true, Description: "Most powerful"},
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic", ContextWindow: 200000, InputPrice: 3.0, OutputPrice: 15.0, IsFree: false, SupportsTools: true},
		},
	},
	"openai": {
		ID:   "openai",
		Name: "OpenAI",
		Models: []ModelInfo{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextWindow: 128000, InputPrice: 2.5, OutputPrice: 10.0, IsFree: false, SupportsTools: true, Description: "Latest GPT-4"},
			{ID: "gpt-4o-mini", Name: "GPT-4o mini", Provider: "openai", ContextWindow: 128000, InputPrice: 0.15, OutputPrice: 0.60, IsFree: false, SupportsTools: true, Description: "Fast & cheap"},
			{ID: "o1", Name: "o1", Provider: "openai", ContextWindow: 128000, InputPrice: 15.0, OutputPrice: 60.0, IsFree: false, SupportsTools: false, Description: "Reasoning model"},
		},
	},
	"xai": {
		ID:   "xai",
		Name: "xAI (Grok)",
		Models: []ModelInfo{
			{ID: "grok-code-fast-1", Name: "Grok Code Fast", Provider: "xai", ContextWindow: 128000, InputPrice: 0.15, OutputPrice: 0.60, IsFree: false, SupportsTools: true, Description: "Optimized for coding"},
			{ID: "grok-4", Name: "Grok 4", Provider: "xai", ContextWindow: 128000, InputPrice: 3.0, OutputPrice: 15.0, IsFree: false, SupportsTools: true, Description: "Latest flagship"},
			{ID: "grok-3-fast", Name: "Grok 3 Fast", Provider: "xai", ContextWindow: 128000, InputPrice: 1.0, OutputPrice: 5.0, IsFree: false, SupportsTools: true},
		},
	},
	"deepseek": {
		ID:   "deepseek",
		Name: "DeepSeek",
		Models: []ModelInfo{
			{ID: "deepseek-chat", Name: "DeepSeek V3.2", Provider: "deepseek", ContextWindow: 128000, InputPrice: 0.27, OutputPrice: 1.10, IsFree: false, SupportsTools: true, Description: "Latest V3.2 model"},
			{ID: "deepseek-reasoner", Name: "DeepSeek R1", Provider: "deepseek", ContextWindow: 64000, InputPrice: 0.55, OutputPrice: 2.19, IsFree: false, SupportsTools: true, Description: "Reasoning model"},
			{ID: "deepseek-coder", Name: "DeepSeek Coder", Provider: "deepseek", ContextWindow: 64000, InputPrice: 0.14, OutputPrice: 0.28, IsFree: false, SupportsTools: true, Description: "Code optimized"},
		},
	},
	"minimax": {
		ID:   "minimax",
		Name: "MiniMax",
		Models: []ModelInfo{
			{ID: "MiniMax-M2.1", Name: "MiniMax M2.1", Provider: "minimax", ContextWindow: 200000, InputPrice: 0.5, OutputPrice: 2.0, IsFree: false, SupportsTools: true, Description: "200K context"},
			{ID: "MiniMax-Text-02", Name: "MiniMax Text 02", Provider: "minimax", ContextWindow: 1000000, InputPrice: 1.0, OutputPrice: 4.0, IsFree: false, SupportsTools: true, Description: "1M context"},
		},
	},
	"moonshot": {
		ID:   "moonshot",
		Name: "Moonshot AI (Kimi)",
		Models: []ModelInfo{
			{ID: "moonshot-v1-8k", Name: "Kimi 8k", Provider: "moonshot", ContextWindow: 8000, InputPrice: 0.15, OutputPrice: 0.60, IsFree: false, SupportsTools: true, Description: "Standard context"},
			{ID: "moonshot-v1-32k", Name: "Kimi 32k", Provider: "moonshot", ContextWindow: 32000, InputPrice: 0.30, OutputPrice: 1.20, IsFree: false, SupportsTools: true, Description: "Long context"},
			{ID: "moonshot-v1-128k", Name: "Kimi 128k", Provider: "moonshot", ContextWindow: 128000, InputPrice: 0.60, OutputPrice: 2.40, IsFree: false, SupportsTools: true, Description: "Ultra long context"},
		},
	},
	"zhipu": {
		ID:   "zhipu",
		Name: "Zhipu AI (GLM)",
		Models: []ModelInfo{
			{ID: "glm-4-plus", Name: "GLM-4 Plus", Provider: "zhipu", ContextWindow: 128000, InputPrice: 5.0, OutputPrice: 10.0, IsFree: false, SupportsTools: true, Description: "Latest flagship"},
			{ID: "glm-4-flash", Name: "GLM-4 Flash", Provider: "zhipu", ContextWindow: 128000, InputPrice: 0.01, OutputPrice: 0.01, IsFree: true, SupportsTools: true, Description: "Extremely fast & cheap"},
			{ID: "glm-4-air", Name: "GLM-4 Air", Provider: "zhipu", ContextWindow: 128000, InputPrice: 0.10, OutputPrice: 0.10, IsFree: false, SupportsTools: true, Description: "Balanced performance"},
		},
	},
	"openrouter": {
		ID:   "openrouter",
		Name: "OpenRouter",
		Models: []ModelInfo{
			{ID: "anthropic/claude-sonnet-4", Name: "Claude Sonnet 4", Provider: "openrouter", ContextWindow: 200000, InputPrice: 3.0, OutputPrice: 15.0, IsFree: false, SupportsTools: true},
			{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", ContextWindow: 128000, InputPrice: 2.5, OutputPrice: 10.0, IsFree: false, SupportsTools: true},
			{ID: "google/gemini-3-flash", Name: "Gemini 3 Flash", Provider: "openrouter", ContextWindow: 1000000, InputPrice: 0, OutputPrice: 0, IsFree: true, SupportsTools: true},
		},
	},
}

// GetAvailableModels returns models for a provider with key status
func GetAvailableModels(providerID string, hasKey bool) []ModelInfo {
	provider, ok := Registry[providerID]
	if !ok {
		return nil
	}

	models := make([]ModelInfo, len(provider.Models))
	copy(models, provider.Models)

	return models
}

// GetAllProviders returns all providers with availability status
func GetAllProviders(keyMap map[string]string) []ProviderInfo {
	providers := make([]ProviderInfo, 0, len(Registry))

	for id, p := range Registry {
		p.HasKey = keyMap[id] != ""
		p.IsAvailable = p.HasKey
		providers = append(providers, p)
	}

	return providers
}

// GetModelByID finds a model across all providers
func GetModelByID(modelID string) *ModelInfo {
	for _, provider := range Registry {
		for _, model := range provider.Models {
			if model.ID == modelID {
				return &model
			}
		}
	}
	return nil
}

// DefaultKeys are built-in API keys (loaded from .env.keys)
var DefaultKeys = map[string]string{}

// SetDefaultKey sets a default API key for a provider
func SetDefaultKey(provider, key string) {
	DefaultKeys[provider] = key
}

// HasDefaultKey checks if a default key exists for provider
func HasDefaultKey(provider string) bool {
	return DefaultKeys[provider] != ""
}
