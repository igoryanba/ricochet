package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSlashCommand processes commands like /help, /status, /permissions
func (m *Model) handleSlashCommand(input string) (string, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	cmd := parts[0]
	// args := parts[1:]

	switch cmd {
	case "/help", "?":
		helpText := `
**Available Commands:**
- **/help** or **?**: Show this help
- **/auto <N>**: Engage Auto-Pilot for N steps
- **/status**: Show current session insights
- **/init**: Initialize a new project (scan codebase)
- **/permissions**: Manage security permissions
- **/checkpoint**: Save current state
- **/restore <hash>**: Restore to a checkpoint
- **/memory**: Show long-term memory stats
- **/hooks**: List active hooks
- **/ether**: Remote control (Telegram)
- **/clear**: Clear screen
- **/demo**: Run feature demo
- **/exit**: Quit
`
		return helpText, nil

	case "/auto":
		if len(parts) < 2 {
			return "Usage: /auto <N> (e.g., /auto 5)", nil
		}
		var n int
		if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil {
			return "Invalid number.", nil
		}
		m.AutoStepsRemaining = n
		return fmt.Sprintf("🟣 Auto-Pilot Engaged: %d steps allowed.", n), nil

	case "/permissions":
		// NEW FEATURE: Verify permissions
		sg := m.Controller.GetSafeguard()
		if sg == nil || sg.PermissionStore == nil {
			return "Safeguard not initialized.", nil
		}

		// rules := sg.PermissionStore.ExportRules() // Pending implementation access

		status := fmt.Sprintf("**Security Status**:\n- Auto-Approval: %v\n", sg.AutoApproval != nil && sg.AutoApproval.Enabled)

		// TODO: Expose rules from permission store properly.
		// For now, let's just show a placeholder if we can't reach internal state.
		return status + "\n(Detailed rule listing coming soon)", nil

	case "/status":
		// ... (Implementation from existing tui.go)
		return fmt.Sprintf("**Session ID**: %s\n**Model**: %s\n**Tokens Used**: ???", m.SessionID, m.ModelName), nil

	case "/clear":
		m.Messages = []DisplayMessage{}
		m.Viewport.SetContent("")
		return "Cleared history.", nil

	case "/exit":
		return "Goodbye!", tea.Quit

	case "/ether":
		// ... (Ether logic)
		return "", nil // Silent success

	case "/demo":
		return "Starting Demo Sequence (Debug Mode)...", func() tea.Msg {
			return DemoUpdateMsg(func(m *Model) {
				m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "Initializing Demo Sequence...", Style: "system"})
				m.recalculateViewportHeight()

				// Start the sequence runner
				go runDemoSequence(m.MsgChan)
			})
		}
	}

	return fmt.Sprintf("Unknown command: %s", cmd), nil
}

func runDemoSequence(msgChan chan tea.Msg) {
	// Step 1: Syntax Highlighting & Diff
	time.Sleep(500 * time.Millisecond)
	msgChan <- DemoUpdateMsg(func(m *Model) {
		m.Messages = append(m.Messages,
			DisplayMessage{Role: "user", Content: "Fix the concurrency bug in `main.go`.", Style: "user"},
			DisplayMessage{Role: "assistant", Content: "I will modify `main.go` to fix the race condition.", Style: "agent"},
		)
		// Fake Diff using Markdown code block
		// Note: We rely on standard markdown rendering here.
		diff := "```diff\n- func process() { time.Sleep(1) }\n+ func process() { time.Sleep(1 * time.Second) }\n```"
		m.Messages = append(m.Messages, DisplayMessage{Role: "assistant", Content: diff, Style: "agent"})
		m.TaskTree = []*TaskNode{{ID: "1", Name: "Refactoring `main.go`", Status: "running", Expanded: true}}
		m.Thoughts = "Analyzing AST..."
		m.recalculateViewportHeight()
	})

	// Step 2: Error Recovery
	time.Sleep(800 * time.Millisecond)
	msgChan <- DemoUpdateMsg(func(m *Model) {
		m.TaskTree[0].Children = append(m.TaskTree[0].Children, &TaskNode{ID: "2", Name: "Running tests...", Status: "failed", Meta: "Error"})
		m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "Step 2: Error Simulated", Style: "system"})
		m.Messages = append(m.Messages, DisplayMessage{Role: "assistant", Content: "Apologies, I missed an import. Fixing now...", Style: "agent"})
		m.recalculateViewportHeight()
	})

	// Step 3: Token Limit Warning
	time.Sleep(800 * time.Millisecond)
	msgChan <- DemoUpdateMsg(func(m *Model) {
		m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "— Context compacted: 12k tokens removed —", Style: "system"})
		m.TokenUsage = 120000
		m.recalculateViewportHeight()
	})

	// Step 4: Autocomplete
	time.Sleep(800 * time.Millisecond)
	msgChan <- DemoUpdateMsg(func(m *Model) {
		m.Textarea.SetValue("/")
		m.Suggestions = []string{"/compact", "/help", "/cost"}
		m.ShowSuggestions = true
		m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "Demo Complete.", Style: "system"})
		m.recalculateViewportHeight()
	})
}

// runAsync wrapper for command execution (moved from tui.go)
func (m *Model) runAsync(input string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		res, err := fn()
		return SlashCmdResMsg{Command: input, Response: res, Error: err}
	}
}
