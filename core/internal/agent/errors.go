package agent

import (
	"fmt"
	"strings"
)

// TranslateError converts technical provider/system errors into user-friendly messages.
// It prioritizes actionable advice and hides cryptic backend details.
func TranslateError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// 1. Authentication errors
	if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") || strings.Contains(errMsg, "invalid_api_key") {
		return "⚠️ Authentication error: Please check your API key in Settings."
	}

	// 2. Rate limits
	if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "Rate limit") || strings.Contains(errMsg, "Too Many Requests") {
		return "⏳ Rate limit exceeded: Please wait a moment or try a different provider/model."
	}

	// 3. Context window / Max tokens errors
	if strings.Contains(errMsg, "max_tokens") || strings.Contains(errMsg, "context_length") || strings.Contains(errMsg, "too many tokens") {
		if strings.Contains(errMsg, "max_tokens") && strings.Contains(errMsg, "range") {
			return "🛑 Parameter error: The selected model does not support this request length. Try shortening your context or selecting a different model."
		}
		return "🛑 Context full: Too much data for this model. Try clearing chat history or compressing files."
	}

	// 4. Invalid model
	if strings.Contains(errMsg, "model_not_found") || strings.Contains(errMsg, "404") && strings.Contains(errMsg, "model") {
		return "🔍 Model not found: Check the model name in Settings or ensure it's available for your API key."
	}

	// 5. Network errors
	if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "timeout") {
		return "🌐 Connection timeout: Check your internet connection or the AI provider's status page."
	}
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
		return "🌐 Network error: Cannot reach the AI server. Check your internet or proxy settings."
	}

	// 6. Insufficient balance (OpenRouter/DeepSeek specific)
	if strings.Contains(errMsg, "insufficient_balance") || strings.Contains(errMsg, "credit") {
		return "💰 Insufficient balance: Please check your AI provider account credits."
	}

	// 7. General provider errors
	if strings.Contains(errMsg, "API error 500") || strings.Contains(errMsg, "Internal Server Error") {
		return "🛠 Internal AI Server Error: The provider is temporarily unavailable. Please try again later."
	}

	// Fallback for unknown errors - still try to be helpful
	return fmt.Sprintf("❌ An error occurred: %s\n\nIf this persists, try resetting settings or changing models.", errMsg)
}
