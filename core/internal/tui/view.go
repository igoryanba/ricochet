package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/igoryan-dao/ricochet/internal/tui/style"
)

func (m Model) View() string {
	if m.TerminalWidth <= 0 {
		return "Initializing..."
	}

	// 1. Header
	header := RenderDashboard(m)

	// 2. Viewport (Content)
	viewport := m.Viewport.View()

	// 3. Footer (Status + Mode)
	footer := RenderStatusBar(m)

	// 4. Input
	input := m.Textarea.View()

	// 5. Suggestions (Optional Overlay)
	suggestions := ""
	if m.ShowSuggestions {
		suggestions = m.renderSuggestions()
	}

	// COMPOSITION:
	bottom := lipgloss.JoinVertical(lipgloss.Left, footer, input)

	if suggestions != "" {
		bottom = lipgloss.JoinVertical(lipgloss.Left, suggestions, bottom)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		viewport,
		bottom,
	)
}

func (m *Model) UpdateViewport() {
	var sb strings.Builder

	// ========================
	// INTERLEAVED BLOCKS RENDERING (Claude Code-style)
	// ========================
	// Render blocks sequentially: User → Tree → Text → Tree
	// Each tree is isolated to its block, not a monolithic global tree.

	// Clean up empty blocks before rendering
	m.cleanupEmptyBlocks()

	if len(m.Blocks) > 0 {
		for _, block := range m.Blocks {
			switch block.Type {
			case BlockUserQuery:
				// User message block
				renderedContent, _ := m.Renderer.Render(block.Content)
				renderedContent = strings.TrimSpace(renderedContent)
				bulletStyle := style.UserStyle.Width(2).Align(lipgloss.Center)
				formattedMsg := lipgloss.JoinHorizontal(lipgloss.Top,
					bulletStyle.Render(style.BulletUser),
					" ",
					style.UserStyle.Render(renderedContent),
				)
				sb.WriteString(formattedMsg + "\n\n")

			case BlockAgentTree:
				// Tool execution tree block
				if len(block.TaskTree) > 0 {
					sb.WriteString(RenderTaskTree(block.TaskTree, "", m.Spinner, block.IsActive))
					sb.WriteString("\n")
				}

			case BlockAgentText:
				// Agent text response block
				hasContent := strings.TrimSpace(block.Content) != ""
				hasReasoning := strings.TrimSpace(block.Reasoning) != ""

				if hasContent || hasReasoning {
					// Render reasoning first (if any)
					if hasReasoning {
						reasoningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#767676")).Italic(true)
						reasoningContent, _ := m.Renderer.Render(block.Reasoning)
						reasoningContent = strings.TrimSpace(reasoningContent)
						sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
							style.AgentStyle.Width(2).Align(lipgloss.Center).Render(""),
							" ",
							reasoningStyle.Render(reasoningContent),
						) + "\n")
					}

					// Render content
					renderedContent, _ := m.Renderer.Render(block.Content)
					renderedContent = strings.TrimSpace(renderedContent)
					bulletStyle := style.AgentStyle.Width(2).Align(lipgloss.Center)
					formattedMsg := lipgloss.JoinHorizontal(lipgloss.Top,
						bulletStyle.Render(style.BulletAgent),
						" ",
						style.AgentStyle.Render(renderedContent),
					)
					sb.WriteString(formattedMsg + "\n\n")
				}
			}
		}

		// Active loading indicator if last block is active tree with no text following
		if m.IsLoading {
			lastBlock := m.Blocks[len(m.Blocks)-1]
			if lastBlock.Type == BlockAgentTree && lastBlock.IsActive {
				// Spinner is already in the tree via RenderTaskTree, we're good
			} else if lastBlock.Type == BlockAgentText && strings.TrimSpace(lastBlock.Content) == "" {
				// Empty text block while loading - show thinking
				status := "Thinking..."
				if m.CurrentStatusStr != "" {
					status = m.CurrentStatusStr
				} else if m.CurrentAction != "" {
					status = m.CurrentAction
				}
				bullet := m.Spinner.View()
				bulletStyle := style.AgentStyle.Width(2).Align(lipgloss.Center)
				formattedMsg := lipgloss.JoinHorizontal(lipgloss.Top,
					bulletStyle.Render(bullet),
					" ",
					style.ThinkingStyle.Render(status),
				)
				sb.WriteString(formattedMsg + "\n")
			}
		}
	}

	// (Legacy activeStream section removed - now handled by BlockAgentText)

	// 4. Pending Thoughts/Spinner/Gate (Bottom Overlay)
	if m.PendingChoice != nil {
		// BATCH CONFIRMATION / CHOICE UI
		var choices strings.Builder
		for i, c := range m.PendingChoice.Choices {
			cursor := " "
			styleOpt := style.SystemStyle
			if i == m.ConfirmationIdx {
				cursor = ">"
				styleOpt = style.UserStyle // Highlight
			}
			choices.WriteString(fmt.Sprintf("%s %s\n", cursor, styleOpt.Render(c)))
		}

		box := style.BoxStyle.BorderForeground(style.BurntOrange).Render(
			style.UserStyle.Bold(true).Render("❓ "+m.PendingChoice.Question) + "\n\n" +
				choices.String() + "\n" +
				style.SystemStyle.Render("[Up/Down] Select  [Enter] Confirm  [Esc] Deny"),
		)
		sb.WriteString("\n" + box + "\n")
	} else if m.PendingApproval != nil {
		diffView := ""
		if m.PendingApproval.Diff != "" {
			diffView = "\n\n" + RenderDiff(m.PendingApproval.Diff)
		}

		box := style.BoxStyle.BorderForeground(style.Yellow).Render(
			style.WarningStyle.Render("⚠ INTERCEPTION: SENSITIVE ACTION") + "\n\n" +
				m.PendingApproval.Question +
				diffView + "\n\n" +
				style.SystemStyle.Render("[Y] Confirm  [N] Deny  [A] Always Allow"),
		)
		sb.WriteString("\n" + box + "\n")
	} else if m.IsLoading {
		// NOTE: Thinking status is now ONLY shown in StatusBar to prevent duplication.
		// Transient thoughts still render here if any.
		if m.Thoughts != "" {
			lines := strings.Split(m.Thoughts, "\n")
			for _, line := range lines {
				if line != "" {
					sb.WriteString(style.SystemStyle.Render("  │ "+line) + "\n")
				}
			}
		}
	}

	m.Viewport.SetContent(sb.String())
	m.Viewport.GotoBottom()
}

// -- Components --

func RenderDashboard(m Model) string {
	if m.TerminalWidth <= 0 {
		return ""
	}

	w := m.TerminalWidth - 10
	if w < 0 {
		w = 0
	}

	if w < 50 {
		return style.HeaderStyle.Width(w).Render("Ricochet v0.1.0 • " + m.ModelName)
	}

	logo := style.HeaderLabelStyle.Render("Ricochet v0.1.0")
	modelInfo := fmt.Sprintf("Model: %s", m.ModelName)

	separator := " • "
	if m.IsLoading {
		// Use Spinner as separator or prefix
		separator = fmt.Sprintf(" %s ", m.Spinner.View())
	}

	left := logo + separator + style.SystemStyle.Render(modelInfo)
	right := style.SystemStyle.Render("Run /help")

	spaceCount := w - lipgloss.Width(left) - lipgloss.Width(right)
	if spaceCount < 1 {
		spaceCount = 1
	}
	spacer := strings.Repeat(" ", spaceCount)

	return style.HeaderStyle.Width(w).Render(left + spacer + right)
}

func RenderTaskTree(nodes []*TaskNode, prefix string, spin spinner.Model, isLoading bool) string {
	var sb strings.Builder

	for i, node := range nodes {
		isRealLast := i == len(nodes)-1

		// Determine if we need to append a synthetic "Thinking..." tail AFTER this node (as a sibling)
		// This happens if:
		// 1. This is the last real node
		// 2. We are loading
		// 3. This node is NOT running (it's done)
		// 4. This node has NO children (it's a leaf).
		//    (If it has children, the tail appears inside the children list via recursion)
		wantsTail := isRealLast && isLoading && node.Status != "running" && len(node.Children) == 0

		// 1. Connector
		connector := "├─ "
		childPrefix := "│  "

		// If it's the real last node AND we don't want a tail, then it's visually last.
		if isRealLast && !wantsTail {
			connector = "└─ "
			childPrefix = "   "
		}

		// 2. Icon & Style
		icon := "○"
		if !isRealLast && node.Depth == 0 {
			icon = "●" // Root nodes filled
		}
		s := style.TreeStyle
		textStyle := style.SystemStyle

		switch node.Status {
		case "running":
			// ACTIVE SPINNER
			icon = spin.View()
			s = style.TreeActiveStyle   // Orange
			textStyle = style.UserStyle // Bold/Bright
		case "completed", "done":
			icon = "✓"
			s = lipgloss.NewStyle().Foreground(style.Green)
		case "failed":
			icon = "x"
			s = style.ErrorStyle
		}

		if node.Depth == 0 {
			textStyle = style.UserStyle.Bold(true)
		}

		// 3. Render Line
		meta := ""
		if node.Meta != "" {
			meta = " " + style.MetaStyle.Render(node.Meta)
		}

		hint := ""
		if !node.Expanded && (len(node.Children) > 0 || node.Meta != "") {
			hint = " " + style.MetaStyle.Render("(ctrl+r to expand)")
		}

		sb.WriteString(fmt.Sprintf("%s%s%s %s%s%s\n",
			style.SystemStyle.Render(prefix),
			style.SystemStyle.Render(connector),
			s.Render(icon),
			textStyle.Render(node.Name),
			meta,
			hint,
		))

		// 4. TERMINAL BLOCK (Command Output)
		if node.Result != "" {
			// Check status to ensure we only show finished results
			if node.Status == "done" || node.Status == "completed" || node.Status == "failed" {
				// Render the output block
				// We pass `prefix + childPrefix` to indent it as a child of this node
				// But wait, `childPrefix` depends on whether it's the last node.
				// `RenderTaskTree` calculates `childPrefix` above.
				// If we use `prefix + childPrefix`, it will align with children.
				sb.WriteString(RenderTerminalOutput(node.Result, prefix+childPrefix, node.Expanded))
			}
		}

		// 4. Recurse or Synthetic Tail
		if len(node.Children) > 0 {
			if node.Expanded {
				sb.WriteString(RenderTaskTree(node.Children, prefix+childPrefix, spin, isLoading))
			}
		} else if wantsTail {
			// Render the "Thinking..." tail as a SIBLING (Same prefix)
			tailConnector := "└─ "
			sb.WriteString(fmt.Sprintf("%s%s%s %s\n",
				style.SystemStyle.Render(prefix), // SAME PREFIX
				style.SystemStyle.Render(tailConnector),
				style.TreeActiveStyle.Render(spin.View()),
				style.ThinkingStyle.Render("Thinking..."),
			))
		}
	}
	return sb.String()
}

func RenderStatusBar(m Model) string {
	w := m.TerminalWidth
	s := style.FooterStyle

	mode := "ACT"
	modeStyle := style.ActStyle
	if m.IsPlanMode {
		mode = "PLAN"
		modeStyle = style.PlanStyle
	}

	modeIndicator := fmt.Sprintf("[%s]", mode)

	statusText := "Ready"
	if m.IsLoading {
		statusText = "Thinking..."
		if m.CurrentStatusStr != "" {
			statusText = m.CurrentStatusStr
		} else if m.CurrentAction != "" {
			statusText = m.CurrentAction
		}
	}

	// Use ThinkingStyle for loading state to verify visibility
	var left string
	if m.IsLoading {
		left = style.ThinkingStyle.Render(fmt.Sprintf(" %s ", statusText))
	} else {
		left = s.Render(fmt.Sprintf(" %s ", statusText))
	}

	etherProps := ""
	if m.IsEtherMode {
		etherProps = style.ActStyle.Render("[ETH] ") // Can use different color if needed
	}

	autoBadge := ""
	if m.AutoStepsRemaining > 0 {
		// Purple badge
		autoBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#9D65FF")).Bold(true).Render(fmt.Sprintf("[AUTO: %d] ", m.AutoStepsRemaining))
	}

	right := modeStyle.Render(modeIndicator + " ")

	spacer := strings.Repeat(" ", max(0, w-lipgloss.Width(left)-lipgloss.Width(right)-lipgloss.Width(etherProps)-lipgloss.Width(autoBadge)))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, left, spacer, autoBadge, etherProps, right)
}

func (m Model) renderSuggestions() string {
	if len(m.Suggestions) == 0 {
		return ""
	}
	s := ""
	for i, sug := range m.Suggestions {
		if i == m.SelectedSuggestion {
			s += style.UserStyle.Render("> "+sug) + "\n"
		} else {
			s += "  " + sug + "\n"
		}
	}
	return style.BoxStyle.Render(s)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RenderWelcomeContent(modelName, cwd string) (string, string) {
	logoStyle := lipgloss.NewStyle().Foreground(style.BurntOrange).Bold(true)

	textContent := fmt.Sprintf(`
Welcome to **Ricochet** (v0.1.0)
Model: *%s*
CWD: %s

Type **/help** for commands.
Type **?** for shortcuts.
`, modelName, cwd)

	return logoStyle.Render("Ricochet"), textContent
}

func RenderDiff(diff string) string {
	var sb strings.Builder
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") {
			sb.WriteString(lipgloss.NewStyle().Foreground(style.Green).Render(line) + "\n")
		} else if strings.HasPrefix(line, "-") {
			sb.WriteString(lipgloss.NewStyle().Foreground(style.Red).Render(line) + "\n")
		} else {
			sb.WriteString(style.SystemStyle.Render(line) + "\n")
		}
	}
	return sb.String()
}

func RenderTerminalOutput(text string, indent string, expanded bool) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}

	// Style: Dim Gray (#767676)
	termStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))
	borderStyle := style.SystemStyle // Use system style for borders

	var sb strings.Builder

	// Truncation logic (Vertical)
	displayLines := lines
	truncated := false
	if !expanded && len(lines) > 5 {
		displayLines = lines[:5]
		truncated = true
	}

	// Top Border
	sb.WriteString(borderStyle.Render(indent+"┌"+strings.Repeat("─", 60)) + "\n")

	for _, line := range displayLines {
		// Truncation logic (Horizontal - prevent overflow)
		safeLine := truncateString(line, 500)
		sb.WriteString(borderStyle.Render(indent+"│ ") + termStyle.Render(safeLine) + "\n")
	}

	if truncated {
		msg := fmt.Sprintf("... (%d lines hidden, ctrl+r to expand)", len(lines)-5)
		sb.WriteString(borderStyle.Render(indent+"│ ") + style.MetaStyle.Render(msg) + "\n")
	}

	// Bottom Border
	sb.WriteString(borderStyle.Render(indent+"└"+strings.Repeat("─", 60)) + "\n")

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
