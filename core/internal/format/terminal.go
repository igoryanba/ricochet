package format

import (
	"strings"
)

// ProcessTerminalOutput handles terminal control characters like \r and \b.
// It simplifies progress bars and spinners for better chat display.
func ProcessTerminalOutput(input string) string {
	if !strings.ContainsAny(input, "\r\b") {
		return input
	}

	lines := strings.Split(input, "\n")
	processedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			processedLines = append(processedLines, "")
			continue
		}

		processedLines = append(processedLines, processLine(line))
	}

	return strings.Join(processedLines, "\n")
}

func processLine(line string) string {
	var b strings.Builder
	runes := []rune(line)
	cursor := 0

	// We use a slice of runes to represent the current line state for easy overwriting
	output := make([]rune, 0, len(runes))

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\r':
			cursor = 0
		case '\b':
			if cursor > 0 {
				cursor--
			}
		default:
			if cursor < len(output) {
				output[cursor] = r
			} else {
				output = append(output, r)
			}
			cursor++
		}
	}

	for _, r := range output {
		b.WriteRune(r)
	}
	return b.String()
}
