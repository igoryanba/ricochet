package protocol

import "encoding/json"

// Message represents a chat message
type Message struct {
	Role             string            `json:"role"` // user, assistant, system
	Content          string            `json:"content"`
	ReasoningContent string            `json:"reasoning_content,omitempty"` // DeepSeek R1 reasoning
	ToolUse          []ToolUseBlock    `json:"tool_use,omitempty"`
	ToolResults      []ToolResultBlock `json:"tool_results,omitempty"`
	Via              string            `json:"via,omitempty"` // Message source
}

// ToolUseBlock represents a tool call by the assistant
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlock represents the result of a tool execution
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// TodoStatus represents the status of a task
type TodoStatus string

const (
	TodoPending   TodoStatus = "pending"
	TodoCurrent   TodoStatus = "current"
	TodoCompleted TodoStatus = "completed"
)

// Todo represents a single unit of work in a task
type Todo struct {
	Text   string     `json:"text"`
	Status TodoStatus `json:"status"`
}

// ContextStatus represents context window usage for UI display
type ContextStatus struct {
	TokensUsed     int     `json:"tokens_used"`
	TokensMax      int     `json:"tokens_max"`
	Percentage     float64 `json:"percentage"`
	WasCondensed   bool    `json:"was_condensed,omitempty"`
	WasTruncated   bool    `json:"was_truncated,omitempty"`
	Summary        string  `json:"summary,omitempty"`
	CumulativeCost float64 `json:"cumulative_cost,omitempty"`
}

// Checkpoint represents a workspace snapshot for undo/restore functionality
type Checkpoint struct {
	Hash      string `json:"hash"`
	Message   string `json:"message,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	ToolName  string `json:"tool_name,omitempty"` // Which tool triggered this checkpoint
}

// Diagnostic represents a compiler/linter error or warning
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // Error, Warning, Information
}

// DefinitionLocation represents a symbol definition
type DefinitionLocation struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// TaskProgress represents structured task progress for UI display
type TaskProgress struct {
	TaskName string   `json:"task_name"`           // Header title
	Status   string   `json:"status"`              // Current step description
	Summary  string   `json:"summary,omitempty"`   // Overall summary
	Mode     string   `json:"mode,omitempty"`      // planning, execution, verification
	Steps    []string `json:"steps,omitempty"`     // Progress history
	Files    []string `json:"files,omitempty"`     // Files modified during task
	IsActive bool     `json:"is_active,omitempty"` // Whether task is still in progress
}
