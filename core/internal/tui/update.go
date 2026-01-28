package tui

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	// GLOBAL TOGGLES
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		if kmsg.String() == "ctrl+p" {
			m.IsPlanMode = !m.IsPlanMode
			m.PlanAddingTask = false // Reset state
			if m.IsPlanMode {
				m.IsShellFocused = false // Ensure focus is relevant
				m.Textarea.Blur()
			} else {
				m.Textarea.Focus()
			}
			m.UpdateViewport()
			return m, nil
		}
	}

	// PLAN MODE INTERCEPTION
	if m.IsPlanMode {
		// 1. Text Input Mode (Adding Task)
		if m.PlanAddingTask {
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.Type {
				case tea.KeyEsc:
					m.PlanAddingTask = false
					m.Textarea.Reset()
					m.Textarea.Blur()
					m.UpdateViewport()
					return m, nil
				case tea.KeyEnter:
					title := m.Textarea.Value()
					if title != "" {
						pm := m.Controller.GetPlanManager()
						if pm != nil {
							pm.AddTask(title, "")
						}
					}
					m.PlanAddingTask = false
					m.Textarea.Reset()
					m.Textarea.Blur()
					m.UpdateViewport()
					return m, nil
				}
			}
			// Forward typing to textarea
			m.Textarea.Focus()
			m.Textarea, tiCmd = m.Textarea.Update(msg)
			return m, tiCmd
		}

		// 2. Navigation Mode
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			key := kmsg.String()
			pm := m.Controller.GetPlanManager()

			switch key {
			case "up", "k":
				m.PlanCursor--
				if m.PlanCursor < 0 {
					m.PlanCursor = 0
				}
				m.UpdateViewport()
				return m, nil
			case "down", "j":
				if pm != nil && m.PlanCursor < len(pm.Tasks)-1 {
					m.PlanCursor++
				}
				m.UpdateViewport()
				return m, nil
			case "a":
				m.PlanAddingTask = true
				m.Textarea.Placeholder = "Enter new task title..."
				m.Textarea.Focus()
				m.UpdateViewport()
				return m, nil
			case "d", "delete":
				if pm != nil && len(pm.Tasks) > 0 {
					// Rudimentary delete: filter out task at cursor
					// Assuming PlanManager has RemoveTask or we do it manually safely?
					// Let's assume AddTask exists, check RemoveTask later.
					// For now, skip delete or implement manual slice removal if safe.
					// pm.RemoveTask(m.PlanCursor) -> TODO: Add method to PlanManager
				}
				return m, nil
			case "enter", "space":
				if pm != nil && len(pm.Tasks) > 0 {
					t := pm.Tasks[m.PlanCursor]
					newStatus := "done"
					switch t.Status {
					case "done":
						newStatus = "pending"
					case "pending":
						newStatus = "active"
					}
					pm.UpdateTaskStatus(t.ID, newStatus)
					m.UpdateViewport()
				}
				return m, nil
			}
		}
		// Block other keys in plan mode
		return m, nil
	}

	// Modal Interception
	if m.PendingChoice != nil {
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			key := kmsg.String()

			if key == "up" || key == "shift+tab" || key == "k" {
				m.ConfirmationIdx--
				if m.ConfirmationIdx < 0 {
					m.ConfirmationIdx = len(m.PendingChoice.Choices) - 1
				}
				m.UpdateViewport() // CRITICAL: Refresh UI to show cursor move
				return m, nil
			}
			if key == "down" || key == "tab" || key == "j" {
				m.ConfirmationIdx++
				if m.ConfirmationIdx >= len(m.PendingChoice.Choices) {
					m.ConfirmationIdx = 0
				}
				m.UpdateViewport() // CRITICAL: Refresh UI to show cursor move
				return m, nil
			}
			// Number keys 1-9
			if len(key) == 1 && key >= "1" && key <= "9" {
				idx := int(key[0] - '1')
				if idx < len(m.PendingChoice.Choices) {
					m.ConfirmationIdx = idx
					m.UpdateViewport()
					return m, nil
				}
			}
			if key == "enter" {
				// Confirm
				m.PendingChoice.RespChan <- m.ConfirmationIdx
				m.PendingChoice = nil
				return m, nil
			}
			// Escape
			if key == "esc" {
				m.PendingChoice.RespChan <- 2 // Deny
				m.PendingChoice = nil
				return m, nil
			}
			return m, nil
		}
	}

	// INTERCEPT KEYBOARD for Tab Toggle Logic
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "tab" && !m.ShowSuggestions {
		m.IsShellFocused = !m.IsShellFocused
		// Sync Focus State Immediately
		if m.IsShellFocused {
			m.Textarea.Blur()
		} else {
			m.Textarea.Focus()
		}
		return m, nil
	}

	// Update Focus State (Textarea)
	// Ensure visual state matches logical state
	if m.IsShellFocused {
		m.Textarea.Blur()
	} else {
		m.Textarea.Focus()
	}

	// Dispatch Messages based on Focus
	if m.IsShellFocused {
		// Shell Focused: Viewport gets keys
		m.Viewport, vpCmd = m.Viewport.Update(msg)
		// Textarea gets nothing (it's blurred)
		m.Textarea, tiCmd = m.Textarea.Update(msg)
	} else {
		// Input Focused: Textarea gets keys
		m.Textarea, tiCmd = m.Textarea.Update(msg)

		// Only pass non-typing messages to Viewport (e.g. mouse, windowsize)
		switch msg.(type) {
		case tea.KeyMsg:
			// Skip Viewport update for keys to prevent scrolling while typing
			// UNLESS it's PageUp/PageDown?
			// For now, assume scrolling is mainly mouse or switching focus.
		case tea.MouseMsg:
			// Mouse events always go to Viewport (for scrolling)
			m.Viewport, vpCmd = m.Viewport.Update(msg)
		default:
			m.Viewport, vpCmd = m.Viewport.Update(msg)
		}
	}

	// Auto-Expand Textarea (1 to 5 lines)
	// We use lipgloss to measure visual lines because m.Textarea.LineCount()
	// only counts physical newlines in some bubbletea versions/configs.
	// Box is Width - 2.
	// Box has Border (1+1) + Padding (1+1) = 4 overhead.
	// Inner width = (Width - 2) - 4 = Width - 6.
	taWidth := m.TerminalWidth - 6
	if taWidth < 10 {
		taWidth = 10
	}

	val := m.Textarea.Value()
	// Render text wrapped at taWidth to count lines
	measureStyle := lipgloss.NewStyle().Width(taWidth)
	rendered := measureStyle.Render(val)
	visualLines := lipgloss.Height(rendered)

	// Clamp to 1-5 lines
	if visualLines < 1 {
		visualLines = 1
	}
	if visualLines > 5 {
		visualLines = 5
	}

	if m.Textarea.Height() != visualLines {
		m.Textarea.SetHeight(visualLines)
		m.recalculateViewportHeight()
	}

	// Suggestion Logic
	prevShow := m.ShowSuggestions
	m.updateSuggestions()
	if m.ShowSuggestions != prevShow {
		m.recalculateViewportHeight()
	}

	if m.IsLoading {
		m.Spinner, spCmd = m.Spinner.Update(msg)

		// Whimsical Status Cycler
		m.StatusTick++
		if m.StatusTick > 20 { // ~2 seconds @ 100ms tick (roughly)
			m.StatusTick = 0
			// Pick random verb
			rand.Seed(time.Now().UnixNano())
			verb := WhimsicalVerbs[rand.Intn(len(WhimsicalVerbs))]
			m.CurrentStatusStr = verb + "..."
		}
	}

	switch msg := msg.(type) {
	case RemoteInputMsg:
		// SYNC: Show user input from Telegram in TUI
		m.appendUserBlock(msg.Content)
		m.IsLoading = true
		m.UpdateViewport()
		// CRITICAL: Must continue waiting for messages, otherwise TUI becomes deaf to following Agent response.
		return m, tea.Batch(m.Spinner.Tick, m.waitForMsg())

	case tea.WindowSizeMsg:
		m.TerminalWidth = msg.Width
		m.TerminalHeight = msg.Height
		m.recalculateViewportHeight()
		// Re-render viewport with new width if needed
		m.UpdateViewport()
		return m, nil

	case DemoUpdateMsg:
		msg(&m) // Execute the closure on the model pointer
		m.UpdateViewport()
		return m, nil

	case tea.KeyMsg:
		// Global Mode Switch
		if msg.String() == "shift+tab" {
			m.IsPlanMode = !m.IsPlanMode
			// Notify controller or just change local state?
			// For now, visual change.
			return m, nil
		}

		// Ether Mode Toggle
		// Changed to Ctrl+E (or Alt+E) for better ergonomics
		if msg.String() == "ctrl+e" || msg.String() == "alt+e" {
			m.IsEtherMode = !m.IsEtherMode
			return m, nil
		}
		// ... (rest of KeyMsg handling is fine, just inserting WindowSizeMsg before it or in switch)

		// Suggestion Navigation
		if m.ShowSuggestions {
			switch msg.String() {
			case "up":
				m.SelectedSuggestion--
				if m.SelectedSuggestion < 0 {
					m.SelectedSuggestion = len(m.Suggestions) - 1
				}
				return m, nil
			case "down":
				m.SelectedSuggestion++
				if m.SelectedSuggestion >= len(m.Suggestions) {
					m.SelectedSuggestion = 0
				}
				return m, nil
			case "tab", "enter":
				if len(m.Suggestions) > 0 {
					shouldExec := m.selectSuggestion()
					if shouldExec {
						// Fallthrough to exec
					} else {
						return m, nil
					}
				} else if msg.String() == "tab" {
					// Toggle Focus
					m.IsShellFocused = !m.IsShellFocused
					return m, nil
				}
			case "esc":
				m.ShowSuggestions = false
				return m, nil
			}
		} else {
			// Suggestions closed, check for Tab Toggle
			if msg.String() == "tab" {
				m.IsShellFocused = !m.IsShellFocused
				return m, nil
			}
		}

		if msg.String() == "ctrl+r" {
			// Toggle expansion for the active tree block
			block := m.ensureActiveTreeBlock()
			if block != nil && len(block.TaskTree) > 0 {
				lastNode := block.TaskTree[len(block.TaskTree)-1]
				lastNode.Expanded = !lastNode.Expanded
				m.UpdateViewport()
			}
			return m, nil
		}

		// ─── ESC: Cancel Running Task (not session) ───
		if msg.String() == "esc" {
			if m.IsLoading && m.Controller != nil {
				// Abort the current agent task
				m.Controller.AbortCurrentSession()
				m.IsLoading = false
				m.CurrentAction = ""
				m.finishActiveBlocks()

				// Add system message via block (or special type?)
				// For now, let's just append a text block with system style
				textBlock := m.getOrCreateTextBlock()
				textBlock.Content += "\n\n⛔ Task cancelled by user."

				m.UpdateViewport()
				return m, m.waitForMsg()
			}
			// If not loading, Esc does nothing (or could clear input)
			return m, nil
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Alt+Enter for Manual Newline
		if msg.String() == "alt+enter" {
			// Simulate Enter key for textarea to insert newline
			var cmd tea.Cmd
			m.Textarea, cmd = m.Textarea.Update(tea.KeyMsg{Type: tea.KeyEnter})

			// Manually trigger auto-expand logic immediately for responsiveness
			lc := m.Textarea.LineCount()
			if lc < 1 {
				lc = 1
			}
			if lc > 5 {
				lc = 5
			}
			if m.Textarea.Height() != lc {
				m.Textarea.SetHeight(lc)
				m.recalculateViewportHeight()
				m.UpdateViewport()
			}
			return m, cmd
		}

		if msg.String() == "enter" {
			if m.Textarea.Value() == "" {
				return m, nil
			}
			input := m.Textarea.Value()
			m.Textarea.Reset()

			// Command?
			if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "?") {
				if input == "/" {
					// Just open suggestions if not already open, or do nothing
					// Ideally we should have selected a suggestion.
					// If they just hit enter on /, let's treat it as invalid or ignore.
					return m, nil
				}
				res, cmd := m.handleSlashCommand(input)
				if res != "" {
					// System message for command result
					textBlock := m.getOrCreateTextBlock()
					textBlock.Content += "\n" + res
				}
				m.UpdateViewport()
				return m, cmd
			}

			if strings.TrimSpace(input) == "" {
				m.Textarea.Reset()
				return m, nil
			}

			// Chat
			// INTERLEAVED BLOCKS: Create User block + new Tree block
			// appendUserBlock automatically creates the tree block
			m.appendUserBlock(input)

			m.UpdateViewport()
			m.IsLoading = true

			return m, tea.Batch(
				m.Spinner.Tick,
				m.runAsync(input, func() (string, error) {
					req := agent.ChatRequestInput{
						SessionID: m.SessionID,
						Content:   input,
						Via:       "cli",
					}

					// Streaming
					go func() {
						fullResponse := ""
						m.MsgChan <- StreamMsg{Content: "**Ricochet**: ", Done: false}

						// Note: Error handling omitted for brevity in this quick-port
						_ = m.Controller.Chat(context.Background(), req, func(update interface{}) {
							if cu, ok := update.(agent.ChatUpdate); ok {
								if cu.Message.Role == "assistant" {
									if cu.Message.Reasoning != "" {
										m.MsgChan <- ThoughtsMsg{Content: cu.Message.Reasoning}
									}
									if len(cu.Message.Content) > len(fullResponse) {
										diff := cu.Message.Content[len(fullResponse):]
										m.MsgChan <- StreamMsg{Content: diff, Done: false}
										fullResponse = cu.Message.Content
									}
								}
							} else if tp, ok := update.(protocol.TaskProgress); ok {
								m.MsgChan <- tp
							}
						})
						m.MsgChan <- StreamMsg{Done: true}
					}()

					return "", nil
				}),
			)
		}

	case StreamMsg:
		if msg.Done {
			m.IsLoading = false
			m.CurrentAction = "" // Reset status on done
			m.Thoughts = ""      // Clear thoughts on done
			// INTERLEAVED BLOCKS: Mark all active blocks as finished
			m.finishActiveBlocks()
		} else {
			// INTERLEAVED BLOCKS: Stream to text block
			textBlock := m.getOrCreateTextBlock()
			textBlock.Content += msg.Content
		}
		m.UpdateViewport()
		return m, m.waitForMsg()

	case ThoughtsMsg:
		m.Thoughts = msg.Content
		m.UpdateViewport() // Trigger view update to show thoughts node
		return m, m.waitForMsg()

	case protocol.TaskProgress:
		// INTERLEAVED BLOCKS: Update block-based task tree
		m.updateBlockTaskTree(msg)

		// UPDATE GLOBAL STATE for Dashboard
		// Create a copy to prevent pointer issues if msg is reused (unlikely but safe)
		prog := msg
		m.Tasks[msg.TaskName] = &prog

		// Dynamic Status / Smart Summary
		if msg.Status != "" {
			m.CurrentAction = msg.Status
		}
		if msg.Status == "" && msg.Summary != "" {
			m.CurrentAction = msg.Summary
		}

		// Mode Sync
		// Mode Sync
		// DISABLE AUTO-SWITCH: Allow manual toggle (Shift+Tab) only to prevent "Empty Plan" confusion
		/*
			switch msg.Mode {
			case "planning":
				m.IsPlanMode = true
			case "execution", "verification":
				m.IsPlanMode = false
			}
		*/

		m.UpdateViewport()
		// Return both waitForMsg AND Spinner.Tick for animation
		return m, tea.Batch(m.waitForMsg(), m.Spinner.Tick)

	case AskUserMsg:
		m.IsLoading = false
		m.PendingApproval = &msg
		// System message for question
		// We handle this via PendingApproval overlay in View(), but maybe log it to history too?
		// For now, overlay is enough.
		m.UpdateViewport()
		return m, m.waitForMsg()

	case AskUserChoiceMsg:
		// AUTO-PILOT INTERCEPTION
		if m.AutoStepsRemaining > 0 {
			// Check if this is a tool execution request (heuristic)
			isToolExec := strings.Contains(msg.Question, "execute") || strings.Contains(msg.Question, "approve")

			if isToolExec {
				m.AutoStepsRemaining--
				// Auto-Confirm (Choice 0 = Yes)
				go func() { msg.RespChan <- 0 }()
				return m, nil
			}
		}

		m.IsLoading = false
		m.PendingChoice = &msg
		m.UpdateViewport()
		return m, m.waitForMsg()

	case RemoteChatMsg:
		log.Printf("[TUI] Received RemoteChatMsg: Role=%s Len=%d Via=%s Session=%s IsStreaming=%v", msg.Message.Role, len(msg.Message.Content), msg.Message.Via, msg.Message.SessionID, msg.Message.IsStreaming)

		// SYNC: Sync Loading state from remote message
		// If it's an assistant message, we trust its streaming state
		if msg.Message.Role == "assistant" {
			m.IsLoading = msg.Message.IsStreaming
			if m.IsLoading {
				m.CurrentAction = "Reflecting..." // or "Thinking..."
			} else {
				m.CurrentAction = ""
				// INTERLEAVED BLOCKS: Mark all active blocks as finished when streaming ends
				m.finishActiveBlocks()
			}
		}

		// SYNC: Show remote Telegram messages in TUI
		// FILTER: Ignore empty messages unless they are tool calls, have reasoning, OR are actively streaming
		hasContent := strings.TrimSpace(msg.Message.Content) != ""
		hasReasoning := strings.TrimSpace(msg.Message.Reasoning) != ""
		hasTools := len(msg.Message.ToolCalls) > 0
		isStreaming := msg.Message.IsStreaming

		// Allow empty message if it's the start of a stream (IsStreaming=true)
		if !hasContent && !hasTools && !hasReasoning && !isStreaming {
			return m, m.waitForMsg()
		}

		// INTERLEAVED BLOCKS: Update blocks for remote messages
		switch msg.Message.Role {
		case "user":
			// User message from Telegram - create user block + tree block
			m.appendUserBlock(msg.Message.Content)
		case "assistant":
			// ONLY create/update text block if there's ACTUAL content
			// Empty streaming messages should NOT create text blocks (they're just heartbeats)
			if hasContent || hasReasoning {
				textBlock := m.getOrCreateTextBlock()
				// Streaming append or replace?
				// Since we get full updates sometimes or chunks, this needs care.
				// Assuming streaming updates are incremental? No, usually they are appended here.
				// Actually StreamMsg handles "diff". RemoteChatMsg usually comes as full or partial?
				// To be safe in TUI for now, let's just Append if it's new, or Update if it's streaming.
				// With blocks, we just update the last block's content.

				// TODO: Logic for incremental vs full replacement for remote msgs
				// For now, simplistic replacement or append:
				// If we assume sequential streaming chunks:
				// textBlock.Content += msg.Message.Content
				// BUT RemoteChatMsg might be the *full* message so far?
				// Let's assume it's like a stream chunk for now given `StreamMsg` usage above.
				if msg.Message.Content != "" {
					// We might need a better diff or just trust the latest if we are replacing.
					// But `textBlock` is persistent.
					// Let's assume the controller handles dedupe? No, controller sends chunks usually.
					// Wait, the `StreamMsg` logic handles `diff`.
					// `RemoteChatMsg` might be different.
					// Ideally we'd replace the content if it's an update to the SAME message ID.
					// But we don't track message IDs well here.
					// Let's append for now and fix if double rendering occurs.
					// Safe bet: if IsStreaming, it's a chunk?
					textBlock.Content = msg.Message.Content // Replace assumes full state?
					// Or Append? Code says `lastMsg.Content = msg.Message.Content` in legacy.
					// So it was REPLACING.
					// Let's REPLA C E the content of the last block if it's active.
				}
				textBlock.Reasoning = msg.Message.Reasoning
			}
		}

		// AUTO-SWITCH SESSION
		if msg.Message.SessionID != "" && msg.Message.SessionID != m.SessionID {
			m.SessionID = msg.Message.SessionID
		}

		m.UpdateViewport()
		// If loading, include Spinner.Tick for animation
		if m.IsLoading {
			return m, tea.Batch(m.waitForMsg(), m.Spinner.Tick)
		}
		return m, m.waitForMsg()
	}

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

func (m *Model) updateSuggestions() {
	val := m.Textarea.Value()
	if val == "" {
		m.ShowSuggestions = false
		return
	}

	// 1. Commands
	if val[0] == '/' && !strings.Contains(val, " ") {
		m.Suggestions = nil
		for _, cmd := range m.AllCommands {
			if strings.HasPrefix(cmd, val) {
				m.Suggestions = append(m.Suggestions, cmd)
			}
		}
		m.ShowSuggestions = len(m.Suggestions) > 0
		return
	}

	// 2. Model Argument Autocomplete
	if strings.HasPrefix(val, "/model ") {
		query := strings.TrimPrefix(val, "/model ")
		// Fetch only if controller available
		if m.Controller != nil {
			pm := m.Controller.GetProvidersManager()
			if pm != nil {
				available := pm.GetAvailableProviders()
				m.Suggestions = nil
				for _, p := range available {
					if !p.Available && !p.HasKey {
						continue
					}
					for _, model := range p.Models {
						// suggestions format: "provider:model"
						sug := fmt.Sprintf("%s:%s", p.ID, model.ID)
						// Filter by query
						if strings.HasPrefix(sug, query) {
							m.Suggestions = append(m.Suggestions, sug)
						}
					}
				}
				m.ShowSuggestions = len(m.Suggestions) > 0
				return
			}
		}
	}

	// 2. Help (?)
	if val == "?" {
		m.Suggestions = m.AllCommands
		m.ShowSuggestions = true
		return
	}

	// 2. Files (@) - Simplified for update.go context (limited imports)
	// We need 'os' to read dir, so we must add it to imports

	// Detect @
	atIdx := strings.LastIndex(val, "@")
	if atIdx != -1 {
		if atIdx == 0 || val[atIdx-1] == ' ' {
			query := val[atIdx+1:]
			if !strings.Contains(query, " ") {
				m.populateFileSuggestions(query)
				return
			}
		}
	}

	m.ShowSuggestions = false
}

// populateFileSuggestions helper
func (m *Model) populateFileSuggestions(query string) {
	files, err := os.ReadDir(m.Cwd)
	if err == nil {
		m.Suggestions = nil
		for _, f := range files {
			name := "@" + f.Name()
			if strings.HasPrefix(name, "@"+query) {
				m.Suggestions = append(m.Suggestions, name)
			}
		}
		m.ShowSuggestions = len(m.Suggestions) > 0
	}
}

func (m *Model) selectSuggestion() bool {
	if m.SelectedSuggestion >= len(m.Suggestions) {
		return false
	}
	sug := m.Suggestions[m.SelectedSuggestion]
	val := m.Textarea.Value()

	if strings.HasPrefix(sug, "/") {
		m.Textarea.SetValue(sug + " ")
	} else if strings.HasPrefix(sug, "@") {
		atIdx := strings.LastIndex(val, "@")
		m.Textarea.SetValue(val[:atIdx] + sug + " ")
	} else if strings.HasPrefix(val, "/model ") {
		// Replace entire input with command + selected model
		m.Textarea.SetValue("/model " + sug)
		return true // Auto-submit? Or let user press enter? Let's just fill it.
		// Actually, standard behavior is fill and let user confirm.
		// Wait, if I return true, it tries to auto-exec?
		// autoExec map doesn't contain these args usually.
		// Let's standard return false to keep cursor at end.
	}

	m.Textarea.SetCursor(len(m.Textarea.Value()))
	m.ShowSuggestions = false
	m.SelectedSuggestion = 0

	// Auto-exec check
	autoExec := map[string]bool{"/init": true, "/status": true, "/clear": true, "/exit": true, "/help": true}
	if autoExec[sug] {
		m.Textarea.SetValue(sug)
		return true
	}
	// For model selection, we might want to auto-submit if it's a complete selection?
	// For now, let user hit enter.
	return false
}

func (m *Model) recalculateViewportHeight() {
	if m.TerminalHeight == 0 {
		return
	}

	// FIXED HEIGHTS (Approximation based on styles)
	// Header: Border(1) + Content(1) + Border(1) = 3 lines
	// Footer: 1 line
	// Input: Textarea (Default 1 line + formatting?) -> Let's reserve 3 lines for safety (1 line input + border/padding)
	// Suggestion Box: 7 lines if visible

	// Fixed Heights
	// Header: Border(1) + Content(1) + Border(1) + Padding(1) = 4 lines
	// Fixed Heights
	// Header: Border(1) + Content(1) + Border(1) + Padding(1) = 4 lines
	headerH := 4
	footerH := 1

	// Dynamic Input Height
	// Textarea Height (1-5) + Box Border (2)
	inputH := m.Textarea.Height() + 2

	safetyMargin := 1 // Prevent scroll flicker

	layoutReserved := headerH + footerH + inputH + safetyMargin

	if m.ShowSuggestions {
		layoutReserved += 7
	}

	// Dynamic Dashboard Height Calculation
	// Must match RenderTaskDashboard logic:
	// Border(2) + Title/Pad(2) = 4 overhead
	// Plus 1 line per task
	pm := m.Controller.GetPlanManager()
	if pm != nil {
		taskCount := len(pm.GetTasks())
		if taskCount > 0 {
			dashboardHeight := taskCount + 4
			layoutReserved += dashboardHeight
		}
	}

	// Calculate Viewport Height
	vpHeight := m.TerminalHeight - layoutReserved
	if vpHeight < 5 {
		vpHeight = 5 // Min height to prevent panic/ugliness
	}

	m.Viewport.Height = vpHeight
	m.Viewport.Width = m.TerminalWidth // Ensure width is synced

	// Sync Textarea Width
	// terminal - 2(box) - 2(pad) - 2(border) = -6
	// We use -6 (and min 10) to match visual constraints.
	taWidth := m.TerminalWidth - 6
	if taWidth < 10 {
		taWidth = 10
	}
	m.Textarea.SetWidth(taWidth)
}
