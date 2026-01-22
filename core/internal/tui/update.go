package tui

import (
	"context"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

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

	m.Textarea, tiCmd = m.Textarea.Update(msg)
	m.Viewport, vpCmd = m.Viewport.Update(msg)

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
		m.Messages = append(m.Messages, DisplayMessage{Role: "user", Content: msg.Content, Style: "user"})
		m.TaskTree = nil
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
				}
			case "esc":
				m.ShowSuggestions = false
				return m, nil
			}
		}

		if msg.String() == "ctrl+r" {
			// Simple toggle for the last active node (deepest running)
			// Matches Claude Code's "Collapse/Expand" hotkey.
			if len(m.TaskTree) > 0 {
				m.TaskTree[len(m.TaskTree)-1].Expanded = !m.TaskTree[len(m.TaskTree)-1].Expanded
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
				m.TaskTree = nil
				m.Messages = append(m.Messages, DisplayMessage{
					Role:    "system",
					Content: "⛔ Task cancelled by user.",
					Style:   "system",
				})
				m.UpdateViewport()
				return m, m.waitForMsg()
			}
			// If not loading, Esc does nothing (or could clear input)
			return m, nil
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
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
					m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: res, Style: "system"})
				}
				m.UpdateViewport()
				return m, cmd
			}

			if strings.TrimSpace(input) == "" {
				m.Textarea.Reset()
				return m, nil
			}

			// Chat
			m.Messages = append(m.Messages, DisplayMessage{Role: "user", Content: input, Style: "user"})

			// INTERLEAVED BLOCKS: Create User block + new Tree block
			// appendUserBlock automatically creates the tree block
			m.appendUserBlock(input)

			// Reset legacy TaskState for new turn (kept for compatibility)
			m.TaskTree = nil

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
		// Check for last message (legacy)
		if len(m.Messages) == 0 || m.Messages[len(m.Messages)-1].Style != "agent" {
			m.Messages = append(m.Messages, DisplayMessage{Role: "assistant", Content: "", Style: "agent"})
		}

		if msg.Done {
			m.IsLoading = false
			m.CurrentAction = "" // Reset status on done
			m.Thoughts = ""      // Clear thoughts on done
			// INTERLEAVED BLOCKS: Mark all active blocks as finished
			m.finishActiveBlocks()
		} else {
			m.Messages[len(m.Messages)-1].Content += msg.Content
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

		// Legacy: Tree View Logic (kept for compatibility)
		m.updateTaskTree(msg)

		// Dynamic Status / Smart Summary
		if msg.Status != "" {
			m.CurrentAction = msg.Status
		}
		if msg.Status == "" && msg.Summary != "" {
			m.CurrentAction = msg.Summary
		}

		// Smart Tool Result Parsing for Tree
		if len(msg.Steps) > 0 {
			// Check last step for tool results
			// lastStep := msg.Steps[len(msg.Steps)-1]
			// e.g. "Read 78 lines" or "Found 3 files"

			// This relies on the controller sending formatted steps.
			// Ideally we'd parse the Meta from the task node if we had it.
			// For now, let's assume the controller does its job or we parse here if needed.
		}

		// Mode Sync
		switch msg.Mode {
		case "planning":
			m.IsPlanMode = true
		case "execution", "verification":
			m.IsPlanMode = false
		}

		m.UpdateViewport()
		// Return both waitForMsg AND Spinner.Tick for animation
		return m, tea.Batch(m.waitForMsg(), m.Spinner.Tick)

	case AskUserMsg:
		m.IsLoading = false
		m.PendingApproval = &msg
		m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "❓ " + msg.Question, Style: "system"})
		m.UpdateViewport()
		return m, m.waitForMsg()

	case AskUserChoiceMsg:
		// AUTO-PILOT INTERCEPTION
		if m.AutoStepsRemaining > 0 {
			// Check if this is a tool execution request (heuristic)
			// The prompt normally puts "The agent wants to execute..."
			isToolExec := strings.Contains(msg.Question, "execute") || strings.Contains(msg.Question, "approve")

			if isToolExec {
				m.AutoStepsRemaining--
				// Auto-Confirm (Choice 0 = Yes)
				go func() { msg.RespChan <- 0 }()

				// Optional: Show a flash message or log it?
				// m.Messages = append(m.Messages, DisplayMessage{Role: "system", Content: "🟣 Auto-Pilot: Approved tool execution.", Style: "system"})

				// Don't change U I state, just return
				return m, nil
			}
		}

		m.IsLoading = false
		m.PendingChoice = &msg // Logic already exists in Update(msg) to handle key inputs for this
		// We don't append to history here, we show a modal?
		// Actually, let's append the question to history so context is clear?
		// Or just let the modal handle it.
		// The modal interception logic at top of Update() handles the keys.
		// We just need to trigger the view.
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
		// FILTER: Ignore empty messages unless they are tool calls, have reasoning, OR are actively streaming (priming the bubble)
		hasContent := strings.TrimSpace(msg.Message.Content) != ""
		hasReasoning := strings.TrimSpace(msg.Message.Reasoning) != ""
		hasTools := len(msg.Message.ToolCalls) > 0
		isStreaming := msg.Message.IsStreaming

		// Allow empty message if it's the start of a stream (IsStreaming=true)
		if !hasContent && !hasTools && !hasReasoning && !isStreaming {
			// CRITICAL: Must continue waiting for messages even if we ignore this one.
			// Returning nil command kills the listener loop.
			return m, m.waitForMsg()
		}

		style := "user"
		if msg.Message.Role == "assistant" {
			style = "agent"
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
				textBlock.Content = msg.Message.Content
				textBlock.Reasoning = msg.Message.Reasoning
			}
		}

		// LEGACY: Also update Messages array for backward compatibility
		shouldAppend := true
		if len(m.Messages) > 0 {
			lastMsg := &m.Messages[len(m.Messages)-1]
			// If last message is same role and styles match, AND content is a prefix match (streaming update)
			// OR if old content is empty (start of stream)
			if lastMsg.Role == msg.Message.Role && lastMsg.Style == style {
				// We assume it's the same message stream if it's the last one.
				// For more robustness we could check ID, but ID might change or be missing in some flows.
				// Let's stick to the existing "append vs update" logic but make it robust for empty starts.
				lastMsg.Content = msg.Message.Content
				lastMsg.Reasoning = msg.Message.Reasoning
				shouldAppend = false
			}
		}

		if shouldAppend {
			m.Messages = append(m.Messages, DisplayMessage{
				Role:      msg.Message.Role,
				Content:   msg.Message.Content,
				Reasoning: msg.Message.Reasoning,
				Style:     style,
			})
		}

		// AUTO-SWITCH SESSION: If the remote message belongs to a different session, switch to it.
		// This ensures the shell "follows" the Telegram conversation.
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

	headerH := 3
	footerH := 1
	inputH := 3
	safetyMargin := 1 // Prevent scroll flicker

	layoutReserved := headerH + footerH + inputH + safetyMargin

	if m.ShowSuggestions {
		layoutReserved += 7
	}

	// Calculate Viewport Height
	vpHeight := m.TerminalHeight - layoutReserved
	if vpHeight < 5 {
		vpHeight = 5 // Min height to prevent panic/ugliness
	}

	m.Viewport.Height = vpHeight
	m.Viewport.Width = m.TerminalWidth   // Ensure width is synced
	m.Textarea.SetWidth(m.TerminalWidth) // Ensure input width is synced
}
