package format

import (
	"fmt"
	"regexp"
	"strings"
)

// ToTelegramHTML converts Markdown text to Telegram-compatible HTML
func ToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	// 0. Handle tables before escaping (detect |---| or similar)
	text = processTables(text)

	// 1. Handle code blocks (triple backticks) - preserve them from further processing
	codeBlocks := make(map[string]string)
	codeBlockRegex := regexp.MustCompile("(?s)```([a-zA-Z]*)\n?(.*?)```")
	text = codeBlockRegex.ReplaceAllStringFunc(text, func(m string) string {
		match := codeBlockRegex.FindStringSubmatch(m)
		lang := match[1]
		content := match[2]

		id := fmt.Sprintf("{CB-%d}", len(codeBlocks))
		escaped := EscapeHTML(content)
		if lang != "" {
			codeBlocks[id] = fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", lang, escaped)
		} else {
			codeBlocks[id] = fmt.Sprintf("<pre><code>%s</code></pre>", escaped)
		}
		return id
	})

	// 2. Handle inline code (single backticks)
	inlineCode := make(map[string]string)
	inlineRegex := regexp.MustCompile("`([^`]+)`")
	text = inlineRegex.ReplaceAllStringFunc(text, func(m string) string {
		match := inlineRegex.FindStringSubmatch(m)
		id := fmt.Sprintf("{IL-%d}", len(inlineCode))
		inlineCode[id] = fmt.Sprintf("<code>%s</code>", EscapeHTML(match[1]))
		return id
	})

	// 3. Escape the rest of HTML
	text = EscapeHTML(text)

	// 4. Handle Headers (allow icons before #)
	headerRegex := regexp.MustCompile(`(?m)^(.*?)#{1,6}\s+(.*)$`)
	text = headerRegex.ReplaceAllString(text, "$1<b>$2</b>")

	// 5. Build Bold Pattern (**bold**)
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	text = boldRegex.ReplaceAllString(text, "<b>$1</b>")

	// 6. Build Italic Pattern (*italic* or _italic_)
	italicRegex1 := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicRegex1.ReplaceAllString(text, "<i>$1</i>")
	// For underscores, require non-alphanumeric boundaries to avoid matching inside words
	italicRegex2 := regexp.MustCompile(`\b_([^_]+)_\b`)
	text = italicRegex2.ReplaceAllString(text, "<i>$1</i>")

	// 7. Strikethrough (~~strike~~)
	strikeRegex := regexp.MustCompile(`~~([^~]+)~~`)
	text = strikeRegex.ReplaceAllString(text, "<s>$1</s>")

	// 8. Underline (__underline__)
	underlineRegex := regexp.MustCompile(`__([^_]+)__`)
	text = underlineRegex.ReplaceAllString(text, "<u>$1</u>")

	// 9. Spoiler (||spoiler||)
	spoilerRegex := regexp.MustCompile(`\|\|([^|]+)\|\|`)
	text = spoilerRegex.ReplaceAllString(text, "<tg-spoiler>$1</tg-spoiler>")

	// 10. Links ([text](url))
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRegex.ReplaceAllString(text, "<a href=\"$2\">$1</a>")

	// 11. Blockquotes (> quote)
	text = processBlockquotes(text)

	// 12. Lists
	bulletRegex := regexp.MustCompile(`(?m)^[\s]*[-*+][\s]+(.*)$`)
	text = bulletRegex.ReplaceAllString(text, "• $1")

	// 13. Restore code blocks
	for id, block := range codeBlocks {
		text = strings.ReplaceAll(text, id, block)
	}
	for id, code := range inlineCode {
		text = strings.ReplaceAll(text, id, code)
	}

	return text
}

// ToDiscordMarkdown ensures text is clean Markdown (minimal escaping)
func ToDiscordMarkdown(text string) string {
	// Discord handles Markdown natively, so we just return it.
	// We might want to strip HTML if it accidentally got in.
	stripHTML := regexp.MustCompile("<[^>]*>")
	return stripHTML.ReplaceAllString(text, "")
}

// EscapeHTML escapes HTML special characters
func EscapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

func processBlockquotes(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	inQuote := false
	var quoteBuffer []string

	for _, line := range lines {
		if strings.HasPrefix(line, "&gt; ") || strings.HasPrefix(line, "> ") {
			if !inQuote {
				inQuote = true
			}
			content := strings.TrimPrefix(strings.TrimPrefix(line, "&gt; "), "> ")
			quoteBuffer = append(quoteBuffer, content)
		} else {
			if inQuote {
				result = append(result, "<blockquote>"+strings.Join(quoteBuffer, "\n")+"</blockquote>")
				quoteBuffer = nil
				inQuote = false
			}
			result = append(result, line)
		}
	}
	if inQuote {
		result = append(result, "<blockquote>"+strings.Join(quoteBuffer, "\n")+"</blockquote>")
	}

	return strings.Join(result, "\n")
}

func processTables(text string) string {
	// Simple table detection: lines starting and ending with | and having a separator |---|
	lines := strings.Split(text, "\n")
	var result []string
	var tableBuffer []string
	inTable := false

	tableSep := regexp.MustCompile(`^[|\s\-:]{3,}$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			inTable = true
			tableBuffer = append(tableBuffer, line)
		} else if inTable && tableSep.MatchString(trimmed) {
			tableBuffer = append(tableBuffer, line)
		} else {
			if inTable {
				// Wrap table in <pre>
				result = append(result, "```\n"+strings.Join(tableBuffer, "\n")+"\n```")
				tableBuffer = nil
				inTable = false
			}
			result = append(result, line)
		}
	}
	if inTable {
		result = append(result, "```\n"+strings.Join(tableBuffer, "\n")+"\n```")
	}

	return strings.Join(result, "\n")
}
