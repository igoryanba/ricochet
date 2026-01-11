package eval

import (
	"time"
)

// TestCase defines a single evaluation scenario
type TestCase struct {
	ID           string     `json:"id"`
	Description  string     `json:"description"`
	InitialState State      `json:"initial_state"`
	Prompt       string     `json:"prompt"`
	Expected     Assertions `json:"expected"`
	Config       *Config    `json:"config,omitempty"`
}

// State represents the initial environment (files, env vars)
type State struct {
	Files map[string]string `json:"files,omitempty"`
}

// Assertions are checks to run after the agent finishes
type Assertions struct {
	Files      map[string]FileAssertion `json:"files,omitempty"`
	Tools      []string                 `json:"tools,omitempty"`       // Expected tools to be called
	ErrorCount int                      `json:"error_count,omitempty"` // Expected number of tool errors
}

// FileAssertion defines rules for verifying file content
type FileAssertion struct {
	Exists      bool   `json:"exists"`
	Contains    string `json:"contains,omitempty"`
	Exact       string `json:"exact,omitempty"`
	NoSubstring string `json:"no_substring,omitempty"`
}

// Config allows overriding default agent settings for a test
type Config struct {
	Model     string `json:"model,omitempty"`
	MaxTurns  int    `json:"max_turns,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// Result represents the outcome of a test case
type Result struct {
	TestCaseID string        `json:"test_case_id"`
	Success    bool          `json:"success"`
	Turns      int           `json:"turns"`
	Tokens     int           `json:"tokens"`
	Duration   time.Duration `json:"duration"`
	Errors     []string      `json:"errors,omitempty"`
	Logs       []string      `json:"logs,omitempty"`
}

// Summary is a collection of results
type Summary struct {
	Total    int           `json:"total"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Duration time.Duration `json:"duration"`
	Results  []Result      `json:"results"`
}
