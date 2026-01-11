package format

import (
	"fmt"
	"strings"
)

// FormatFileChange formats a file change notification for display
func FormatFileChange(action, filePath, summary string) string {
	var emoji string
	switch action {
	case "created":
		emoji = "📄"
	case "modified":
		emoji = "✏️"
	case "deleted":
		emoji = "🗑️"
	default:
		emoji = "📝"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s **File %s**\n", emoji, action))
	sb.WriteString(fmt.Sprintf("`%s`", filePath))
	if summary != "" {
		sb.WriteString(fmt.Sprintf("\n\n%s", summary))
	}
	return sb.String()
}

// FormatProgress formats a progress update for display
func FormatProgress(stage, description string, progressPercent int) string {
	var emoji string
	switch stage {
	case "planning":
		emoji = "📋"
	case "execution":
		emoji = "⚙️"
	case "verification":
		emoji = "✅"
	case "completed":
		emoji = "🎉"
	default:
		emoji = "🔄"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s **%s**\n", emoji, strings.Title(stage)))
	sb.WriteString(description)

	if progressPercent > 0 {
		sb.WriteString(fmt.Sprintf("\n\nProgress: %d%%", progressPercent))
	}

	return sb.String()
}

// FormatSummary formats a summary for display
func FormatSummary(title string, bulletPoints []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **%s**\n\n", title))

	for _, point := range bulletPoints {
		sb.WriteString(fmt.Sprintf("• %s\n", point))
	}

	return sb.String()
}

// FormatCodeBlock formats code with syntax highlighting for Telegram
func FormatCodeBlock(code, language string) string {
	if language != "" {
		return fmt.Sprintf("```%s\n%s\n```", language, code)
	}
	return fmt.Sprintf("```\n%s\n```", code)
}

// FormatError formats an error message
func FormatError(title, message string) string {
	return fmt.Sprintf("❌ **%s**\n\n%s", title, message)
}

// FormatWarning formats a warning message
func FormatWarning(title, message string) string {
	return fmt.Sprintf("⚠️ **%s**\n\n%s", title, message)
}

// FormatSuccess formats a success message
func FormatSuccess(title, message string) string {
	return fmt.Sprintf("✅ **%s**\n\n%s", title, message)
}

// EscapeHTML escapes special characters for Telegram HTML
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ToTelegramHTML converts simple markdown to Telegram HTML format
func ToTelegramHTML(text string) string {
	// Escape HTML first
	text = EscapeHTML(text)

	// Convert multiline code blocks ```text``` to <pre>text</pre>
	for {
		start := strings.Index(text, "```")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+3:], "```")
		if end == -1 {
			break
		}
		// Extract language if present (e.g. ```go\n...)
		content := text[start+3 : start+3+end]
		if newlineIdx := strings.Index(content, "\n"); newlineIdx != -1 {
			// Skip the first line if it looks like a language identifier
			// But check if it's not just a newline
			if newlineIdx > 0 {
				content = content[newlineIdx+1:]
			} else {
				content = content[1:]
			}
		}

		text = text[:start] + "<pre>" + content + "</pre>" + text[start+3+end+3:]
	}

	// Convert markdown bold **text** to HTML <b>text</b>
	for {
		start := strings.Index(text, "**")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			break
		}
		text = text[:start] + "<b>" + text[start+2:start+2+end] + "</b>" + text[start+2+end+2:]
	}

	// Convert markdown italic _text_ to HTML <i>text</i>
	for {
		start := strings.Index(text, "_")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "_")
		if end == -1 {
			break
		}
		text = text[:start] + "<i>" + text[start+1:start+1+end] + "</i>" + text[start+1+end+1:]
	}

	// Convert markdown code `text` to HTML <code>text</code>
	for {
		start := strings.Index(text, "`")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			break
		}
		text = text[:start] + "<code>" + text[start+1:start+1+end] + "</code>" + text[start+1+end+1:]
	}

	return text
}

// ToDiscordMarkdown ensures text is safe for Discord
func ToDiscordMarkdown(text string) string {
	// Discord supports markdown natively, so we mostly pass it through.
	// We could escape specific things if needed, but for AI output it's usually desired.
	return text
}
