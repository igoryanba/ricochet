package prompts

import (
	"fmt"
	"strings"
)

// EphemeralContext holds dynamic state for generating ephemeral messages
type EphemeralContext struct {
	Mode             string // "planning", "execution", "verification"
	HasPlan          bool
	HasActiveTask    bool
	ToolCallCount    int
	LastToolFailed   bool
	IsInTaskMode     bool
	ArtifactsCreated []string
}

// BuildEphemeralMessage generates a conditional reminder based on current context
// This is inserted at the END of the message history to maximize adherence
func BuildEphemeralMessage(ctx EphemeralContext) string {
	var reminders []string

	// Mode-specific reminders
	switch strings.ToLower(ctx.Mode) {
	case "planning":
		if !ctx.HasPlan && ctx.IsInTaskMode {
			reminders = append(reminders, `<planning_reminder>
CRITICAL: You are in PLANNING mode but have not created an implementation plan yet.
You MUST:
1. Create implementation_plan.md artifact with your technical approach
2. Use notify_user to request review before proceeding to EXECUTION
3. DO NOT start making code changes until the plan is approved
</planning_reminder>`)
		} else if ctx.HasPlan {
			reminders = append(reminders, `<planning_reminder>
You are in PLANNING mode. Continue refining your implementation plan.
Remember to use notify_user when ready for review.
</planning_reminder>`)
		}

	case "execution":
		if !ctx.HasPlan && ctx.IsInTaskMode {
			reminders = append(reminders, `<execution_reminder>
CRITICAL: You are in EXECUTION mode but no implementation plan was created.
Consider switching to PLANNING mode first to design your approach.
If this is a simple task, proceed with caution and communicate your steps clearly.
</execution_reminder>`)
		} else {
			reminders = append(reminders, `<execution_reminder>
You are in EXECUTION mode. Follow your implementation plan.
Remember to:
- Use replace_file_content for targeted edits (NOT write_file for full overwrites)
- Create checkpoints before destructive operations
- Communicate progress clearly to the user
</execution_reminder>`)
		}

	case "verification":
		reminders = append(reminders, `<verification_reminder>
You are in VERIFICATION mode. Test your changes thoroughly.
After verification:
1. Create walkthrough.md to document what was accomplished
2. Include proof of testing (screenshots, command outputs)
3. Report any issues found and fixes applied
</verification_reminder>`)
	}

	// Task mode reminder
	if ctx.IsInTaskMode && ctx.ToolCallCount > 5 {
		reminders = append(reminders, `<progress_reminder>
You have made multiple tool calls. Remember to:
- Update task.md to track your progress
- Use task_boundary to update task status and summary
- Keep the user informed of your progress
</progress_reminder>`)
	}

	// Tool failure recovery
	if ctx.LastToolFailed {
		reminders = append(reminders, `<error_recovery_reminder>
Your last tool call failed. Remember to:
- Read and understand the error message carefully
- Check your arguments and paths
- Don't repeat the same failing approach
- Consider an alternative strategy
</error_recovery_reminder>`)
	}

	// Artifact management
	if len(ctx.ArtifactsCreated) > 0 {
		artifactList := strings.Join(ctx.ArtifactsCreated, ", ")
		reminders = append(reminders, fmt.Sprintf(`<artifact_reminder>
You have created artifacts: %s
Remember to keep them updated as you make progress.
CRITICAL: Artifacts should be concise and user-focused.
</artifact_reminder>`, artifactList))
	}

	// Communication reminder (always shown when in task mode)
	if ctx.IsInTaskMode {
		reminders = append(reminders, `<communication_reminder>
IMPORTANT: Explain your reasoning and approach BEFORE executing tools.
Users appreciate understanding your thought process.
Use the thinking block or explain in your response.
</communication_reminder>`)
	}

	// Combine all reminders
	if len(reminders) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n<EPHEMERAL_MESSAGE>\n")
	sb.WriteString("The following are system-injected reminders based on your current context.\n")
	sb.WriteString("Pay close attention to these guidelines.\n\n")
	for _, reminder := range reminders {
		sb.WriteString(reminder)
		sb.WriteString("\n")
	}
	sb.WriteString("</EPHEMERAL_MESSAGE>")

	return sb.String()
}

// BuildToolSpecificReminder generates reminders for specific tool patterns
func BuildToolSpecificReminder(toolName string, consecutiveFailures int) string {
	if consecutiveFailures >= 2 {
		return fmt.Sprintf(`<tool_reminder>
WARNING: Tool '%s' has failed %d times consecutively.
Consider:
- Reviewing the tool's requirements and arguments
- Checking if you have the necessary permissions
- Trying a different approach to solve the problem
</tool_reminder>`, toolName, consecutiveFailures)
	}
	return ""
}
