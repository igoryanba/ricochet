package workflow

// WorkflowDefinition represents the structured definition of a workflow
type WorkflowDefinition struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description" yaml:"description"`
	Steps       []WorkflowStep `json:"steps" yaml:"steps"`
}

// WorkflowStep represents a single unit of work in the orchestration engine
type WorkflowStep struct {
	ID          string         `json:"id" yaml:"id"`
	Description string         `json:"description" yaml:"description"`
	Action      string         `json:"action" yaml:"action"`           // Prompt for the agent
	Type        string         `json:"type" yaml:"type"`               // "agent", "user_input", "parallel"
	Interactive bool           `json:"interactive" yaml:"interactive"` // Pauses for user input
	Parallel    []WorkflowStep `json:"parallel" yaml:"parallel"`       // Sub-steps for parallel execution
	Timeout     int            `json:"timeout" yaml:"timeout"`         // Timeout in seconds
}

// ExecutionContext holds the runtime state of a workflow execution
type ExecutionContext struct {
	WorkflowID string                 `json:"workflow_id"`
	Variables  map[string]interface{} `json:"variables"`
	History    []StepResult           `json:"history"`
}

// StepResult captures the output of a step
type StepResult struct {
	StepID  string      `json:"step_id"`
	Output  string      `json:"output"`
	Status  string      `json:"status"` // "success", "failed", "skipped"
	Context interface{} `json:"context,omitempty"`
}
