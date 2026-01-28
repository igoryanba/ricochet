package tui

import (
	"fmt"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/tui/style"
)

// RenderPlan renders the interactive plan editor
func RenderPlan(m Model) string {
	pm := m.Controller.GetPlanManager()
	if pm == nil {
		return "Plan Manager not initialized."
	}

	// Safe read? PlanManager likely needs locking if we read directly.
	// But for UI rendering often we race or need a View method.
	// Let's rely on the slice copy if possible, or just read.
	// The slice is exported.
	tasks := pm.Tasks

	var s strings.Builder

	s.WriteString(style.HeaderStyle.Render(" PLAN EDITOR (Ctrl+P to Exit) "))
	s.WriteString("\n\n")

	if len(tasks) == 0 {
		s.WriteString(style.SubtleStyle.Render("  No tasks in plan. Press 'a' to add one."))
		s.WriteString("\n")
	}

	for i, t := range tasks {
		cursor := "  "
		if m.PlanCursor == i {
			cursor = "> "
		}

		statusIcon := "[ ]"
		statusColor := style.SubtleStyle

		switch t.Status {
		case "done":
			statusIcon = "[x]"
			statusColor = style.SuccessStyle
		case "active":
			statusIcon = "[>]"
			statusColor = style.AccentStyle
		case "failed":
			statusIcon = "[!]"
			statusColor = style.ErrorStyle
		case "pending":
			statusIcon = "[ ]"
		}

		// Dependency string
		deps := ""
		if len(t.Dependencies) > 0 {
			deps = style.SubtleStyle.Render(fmt.Sprintf(" (Deps: %s)", strings.Join(t.Dependencies, ", ")))
		}

		retryStr := ""
		if t.RetryCount > 0 {
			retryColor := style.WarningStyle
			if t.RetryCount >= t.MaxRetries {
				retryColor = style.ErrorStyle
			}
			retryStr = retryColor.Render(fmt.Sprintf(" ↺ %d/%d", t.RetryCount, t.MaxRetries))
		}

		line := fmt.Sprintf("%s%s %s%s%s", cursor, statusIcon, t.Title, deps, retryStr)

		if m.PlanCursor == i {
			line = style.SelectedStyle.Render(line)
		} else {
			line = statusColor.Render(line)
		}

		s.WriteString(line)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(style.SubtleStyle.Render("  [a] Add  [d] Delete  [enter] Toggle Status  [up/down] Navigate"))

	if m.PlanAddingTask {
		s.WriteString("\n\n")
		s.WriteString(style.AccentStyle.Render("  New Task: > ") + m.Textarea.View())
	}

	return s.String()
}
