package format

import (
	"testing"
)

func TestProcessTerminalOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "simple carriage return",
			input:    "loading...\rdone      ",
			expected: "done      ",
		},
		{
			name:     "partial carriage return overwrite",
			input:    "processing 10%\rprocessing 20%",
			expected: "processing 20%",
		},
		{
			name:     "multiple carriage returns",
			input:    "step 1\rstep 2\rstep 3",
			expected: "step 3",
		},
		{
			name:     "backspace",
			input:    "abc\bde",
			expected: "abde",
		},
		{
			name:     "backspace at start",
			input:    "\baaa",
			expected: "aaa",
		},
		{
			name:     "multi-line with carriage returns",
			input:    "line 1\rline one\nline 2\rline two",
			expected: "line one\nline two",
		},
		{
			name:     "real-world-ish progress bar",
			input:    "Downloading: [=>    ] 20%\rDownloading: [==>   ] 40%\rDownloading: [===>  ] 60%",
			expected: "Downloading: [===>  ] 60%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProcessTerminalOutput(tt.input)
			if got != tt.expected {
				t.Errorf("ProcessTerminalOutput(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
