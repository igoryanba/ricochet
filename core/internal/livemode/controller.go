package livemode

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/state"
	"github.com/igoryan-dao/ricochet/internal/telegram"
)

type contextKey string

const chatIDKey contextKey = "chatID"

// Controller manages Live Mode - bridging Telegram/Discord with the AI agent
type Controller struct {
	mu sync.RWMutex

	enabled  bool
	tgBot    *telegram.Bot
	agent    *agent.Controller
	stateMgr *state.Manager
	chatID   int64 // Primary Telegram chat ID for notifications

	// Cancellation for the listener goroutine
	cancel context.CancelFunc

	// Callback for status updates
	onStatusUpdate func(Status)

	// Callback for emitting activity events to extension
	onActivity func(EtherActivity)

	// Callback for forwarding chat updates to IDE
	onChatUpdate func(agent.ChatUpdate)

	// Throttling for streaming updates to prevent webview crash
	lastChatUpdateTime time.Time
}

// Config holds Live Mode configuration
type Config struct {
	TelegramToken  string  `json:"telegram_token"`
	TelegramChatID int64   `json:"telegram_chat_id"`
	AllowedUserIDs []int64 `json:"allowed_user_ids"`
}

// Status represents the current Live Mode status
type Status struct {
	Enabled      bool   `json:"enabled"`
	ConnectedVia string `json:"connectedVia,omitempty"` // "telegram", "discord", or nil
	LastActivity string `json:"lastActivity,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
}

// EtherActivity represents real-time activity for UI mirroring
type EtherActivity struct {
	Stage    string `json:"stage"`  // receiving, processing, responding
	Source   string `json:"source"` // telegram, discord
	Username string `json:"username,omitempty"`
	Preview  string `json:"preview,omitempty"` // First 50 chars of message
}

// broadcastStatus notifies listeners about status change
func (c *Controller) broadcastStatus() {
	status := c.GetStatus()
	c.mu.RLock()
	fn := c.onStatusUpdate
	c.mu.RUnlock()
	if fn != nil {
		fn(*status)
	}
}

// SetOnStatusUpdate sets the callback for status updates
func (c *Controller) SetOnStatusUpdate(fn func(Status)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStatusUpdate = fn
}

// New creates a new Live Mode controller
func New(cfg *Config, agentCtrl *agent.Controller) (*Controller, error) {
	// Create state manager
	stateMgr, err := state.NewManager()
	if err != nil {
		log.Printf("Warning: Failed to create state manager: %v", err)
		// Continue without persistence
	}

	ctrl := &Controller{
		agent:    agentCtrl,
		stateMgr: stateMgr,
		chatID:   cfg.TelegramChatID,
	}

	// Create Telegram bot if token provided
	if cfg.TelegramToken != "" {
		// AllowedIDs empty = allow all (bot is protected by token)
		tgBot, err := telegram.New(cfg.TelegramToken, cfg.AllowedUserIDs, stateMgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create telegram bot: %w", err)
		}
		ctrl.tgBot = tgBot
	}

	return ctrl, nil
}

// Start begins the background poller (must be called once)
func (c *Controller) Start(ctx context.Context) {
	c.mu.Lock()
	if c.tgBot == nil {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// Start Telegram bot in background
	go c.tgBot.Start(ctx)

	// Start message listener
	go c.listenForMessages(ctx)

	log.Println("Live Mode background poller started")
}

// Enable starts Live Mode
func (c *Controller) Enable(ctx context.Context) (*Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.enabled {
		return c.getStatusLocked(), nil
	}

	c.enabled = true

	// Notify user
	if c.chatID != 0 && c.tgBot != nil {
		c.tgBot.SendMessage(ctx, c.chatID, "🟢 **Live Mode Enabled**\n\nYou can now send messages here to control Ricochet!")
	}

	c.broadcastStatus()
	log.Println("Live Mode enabled")

	return c.getStatusLocked(), nil
}

// Disable stops Live Mode
func (c *Controller) Disable(ctx context.Context) (*Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return c.getStatusLocked(), nil
	}

	c.enabled = false

	// Notify user
	if c.chatID != 0 && c.tgBot != nil {
		c.tgBot.SendMessage(ctx, c.chatID, "🔴 **Live Mode Disabled**\n\nReturning control to IDE.")
	}

	c.broadcastStatus()
	log.Println("Live Mode disabled")

	return c.getStatusLocked(), nil
}

// Toggle toggles Live Mode on/off
func (c *Controller) Toggle(ctx context.Context) (*Status, error) {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if enabled {
		return c.Disable(ctx)
	}
	return c.Enable(ctx)
}

// GetStatus returns current Live Mode status
func (c *Controller) GetStatus() *Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getStatusLocked()
}

func (c *Controller) getStatusLocked() *Status {
	status := &Status{
		Enabled: c.enabled,
	}
	if c.enabled {
		if c.tgBot != nil {
			status.ConnectedVia = "telegram"
		} else {
			status.ConnectedVia = "offline (demo)"
		}
	}
	return status
}

// listenForMessages handles incoming Telegram messages and forwards to agent
func (c *Controller) listenForMessages(ctx context.Context) {
	if c.tgBot == nil {
		return
	}

	responseCh := c.tgBot.GetResponseChannel()
	callbackCh := c.tgBot.GetCallbackChannel()

	for {
		select {
		case <-ctx.Done():
			return

		case resp := <-responseCh:
			if resp == nil {
				continue
			}
			// Handle async so we don't block on agent.Chat()
			go c.handleTelegramMessage(ctx, resp)

		case callback := <-callbackCh:
			if callback == nil {
				continue
			}
			c.handleTelegramCallback(ctx, callback)
		}
	}
}

// handleTelegramMessage processes incoming Telegram messages
func (c *Controller) handleTelegramMessage(ctx context.Context, resp *telegram.UserResponse) {
	log.Printf("Live Mode received message from chat %d: %s", resp.ChatID, resp.Text)

	// Auto-Enable if disabled
	if !c.IsEnabled() {
		log.Println("Live Mode triggering AUTO-ENABLE via Telegram")
		c.mu.Lock()
		c.enabled = true
		c.mu.Unlock()

		c.tgBot.SendMessage(ctx, resp.ChatID, "🟢 **Ricochet Activated!**\n\nBridging to IDE...")
		c.broadcastStatus()
	}

	if c.agent == nil {
		c.tgBot.SendMessage(ctx, resp.ChatID, "⚠️ Agent not configured")
		return
	}

	// Handle /stop command
	if resp.Text == "/stop" {
		c.Disable(ctx)
		return
	}

	// Handle /new command
	if resp.Text == "/new" {
		c.agent.ClearSession("default") // Assuming default session for now
		c.tgBot.SendMessage(ctx, resp.ChatID, "🆕 **New Session Started**")
		return
	}

	// Emit receiving activity
	c.emitActivity("receiving", "telegram", resp.Username, resp.Text)

	// Forward user message to IDE
	c.emitChatUpdate(agent.ChatUpdate{
		Message: agent.ChatMessage{
			ID:        fmt.Sprintf("tg-%d-%d", resp.ChatID, resp.MessageID),
			Role:      "user",
			Content:   resp.Text,
			Timestamp: resp.Timestamp,
			Via:       "telegram",
			Username:  resp.Username,
		},
	})

	// Send typing indicator
	c.tgBot.SendTyping(ctx, resp.ChatID)

	// Emit processing activity
	c.emitActivity("processing", "telegram", resp.Username, "")

	// Stream response to Telegram
	var currentMsgID int
	var currentContent string

	// Handle /sessions command
	if resp.Text == "/sessions" {
		sessions := c.agent.ListSessions()
		var views []telegram.SessionView
		for _, s := range sessions {
			views = append(views, telegram.SessionView{
				ID:        s.ID,
				TotalCost: s.TotalCost,
			})
		}
		c.tgBot.SendSessionList(ctx, resp.ChatID, views)
		return
	}

	// Resolve Session ID
	sessionID := c.tgBot.GetActiveSession(resp.ChatID)
	if sessionID == "" {
		sessionID = "default"
	}

	// Inject ChatID into context so tools (AskUserRemote) know where to reply
	chatCtx := context.WithValue(ctx, chatIDKey, resp.ChatID)

	err := c.agent.Chat(chatCtx, agent.ChatRequestInput{
		SessionID: sessionID,
		Content:   resp.Text,
		Via:       "telegram",
	}, func(update interface{}) {
		// Only handle ChatUpdate for now
		chatUpdate, ok := update.(agent.ChatUpdate)
		if !ok {
			return
		}

		// Forward updates to IDE with via field
		chatUpdate.Message.Via = "telegram"
		c.emitChatUpdate(chatUpdate)

		// Forward to Telegram
		content := chatUpdate.Message.Content
		if content == "" || content == currentContent {
			return
		}

		// Only send partial updates for non-streaming or final messages
		// If streaming, only update every few chars or after significant time
		if chatUpdate.Message.IsStreaming && len(content)-len(currentContent) < 20 {
			return
		}

		currentContent = content

		// Truncate for Telegram's 4096 char limit
		if len(content) > 4000 {
			content = content[:4000] + "..."
		}

		if currentMsgID == 0 {
			// First message - send new
			msgID, err := c.tgBot.SendMessageAndTrack(ctx, resp.ChatID, content)
			if err != nil {
				log.Printf("Failed to send message: %v", err)
				return
			}
			currentMsgID = msgID
		} else {
			// Update message
			c.tgBot.EditMessage(ctx, resp.ChatID, currentMsgID, content)
		}
	})

	// Emit responding activity (done)
	c.emitActivity("responding", "telegram", resp.Username, "")

	if err != nil {
		c.tgBot.SendMessage(ctx, resp.ChatID, fmt.Sprintf("❌ Error: %v", err))
	}
}

// handleTelegramCallback processes button clicks
func (c *Controller) handleTelegramCallback(ctx context.Context, callback *telegram.CallbackEvent) {
	log.Printf("Live Mode received callback: %s from chat %d", callback.Data, callback.ChatID)

	// Session Switching
	if strings.HasPrefix(callback.Data, "session:") {
		sessionID := strings.TrimPrefix(callback.Data, "session:")
		c.tgBot.SetActiveSession(callback.ChatID, sessionID)
		c.tgBot.SendMessage(ctx, callback.ChatID, fmt.Sprintf("✅ **Switched to session:** `%s`", sessionID))

		// Show recent history
		if c.agent != nil {
			if session := c.agent.GetSession(sessionID); session != nil {
				msgs := session.StateHandler.GetMessages()
				count := len(msgs)
				if count > 0 {
					start := count - 6
					if start < 0 {
						start = 0
					}
					var history strings.Builder
					history.WriteString("📜 **Recent Context:**\n\n")

					for _, m := range msgs[start:] {
						if m.Role == "system" {
							continue
						}
						// Skip tool use/results to keep it clean, or maybe show a summary
						if m.Role == "tool" {
							continue
						}

						icon := "👤"
						if m.Role == "assistant" {
							icon = "🤖"
						}

						content := m.Content
						if len(content) > 200 {
							content = content[:200] + "..."
						}
						// If content is empty (e.g. pure tool call), skip
						if strings.TrimSpace(content) == "" {
							continue
						}

						history.WriteString(fmt.Sprintf("%s **%s**: %s\n\n", icon, strings.Title(m.Role), content))
					}
					c.tgBot.SendMessage(ctx, callback.ChatID, history.String())
				}
			}
		}
		return
	}

	switch callback.Data {
	case telegram.CallbackNewChat:
		if c.agent != nil {
			s := c.agent.CreateSession()
			c.tgBot.SetActiveSession(callback.ChatID, s.ID)
			c.tgBot.SendMessage(ctx, callback.ChatID, fmt.Sprintf("🆕 **New Session Started:** `%s`\n\nI am ready. What would you like to build?", s.ID))
		}

	case telegram.CallbackChatHistory:
		if c.agent != nil {
			sessions := c.agent.ListSessions()
			var views []telegram.SessionView
			for _, s := range sessions {
				views = append(views, telegram.SessionView{
					ID:        s.ID,
					TotalCost: s.TotalCost,
				})
			}
			c.tgBot.SendSessionList(ctx, callback.ChatID, views)
		} else {
			c.tgBot.SendMessage(ctx, callback.ChatID, "⚠️ Agent not ready.")
		}
	}
}

// SetAgent sets the agent controller (for deferred initialization)
func (c *Controller) SetAgent(agent *agent.Controller) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agent = agent
}

// SetChatID sets the primary Telegram chat ID
func (c *Controller) SetChatID(chatID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chatID = chatID
}

// SetOnActivity sets the callback for activity events
func (c *Controller) SetOnActivity(fn func(EtherActivity)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onActivity = fn
}

// SetOnChatUpdate sets the callback for forwarding chat updates to IDE
func (c *Controller) SetOnChatUpdate(fn func(agent.ChatUpdate)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChatUpdate = fn
}

// emitActivity sends an activity event to the extension
func (c *Controller) emitActivity(stage, source, username, preview string) {
	c.mu.RLock()
	fn := c.onActivity
	c.mu.RUnlock()

	if fn != nil {
		// Truncate preview to 50 chars
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		fn(EtherActivity{
			Stage:    stage,
			Source:   source,
			Username: username,
			Preview:  preview,
		})
	}
}

// emitChatUpdate forwards a chat update to the IDE
// Includes throttling for streaming updates to prevent webview overflow
func (c *Controller) emitChatUpdate(update agent.ChatUpdate) {
	c.mu.Lock()
	fn := c.onChatUpdate
	lastTime := c.lastChatUpdateTime
	now := time.Now()

	// Throttle streaming updates to max 4/second (250ms interval)
	// Final messages (IsStreaming=false) bypass throttle
	const throttleInterval = 250 * time.Millisecond
	if update.Message.IsStreaming && now.Sub(lastTime) < throttleInterval {
		c.mu.Unlock()
		return
	}
	c.lastChatUpdateTime = now
	c.mu.Unlock()

	if fn != nil {
		fn(update)
	}
}

// IsEnabled returns true if Live Mode is currently active
func (c *Controller) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// AskUserRemote sends an approval request to Telegram and waits for response
// This is used for tool consent when the user is controlling via Ether Mode
func (c *Controller) AskUserRemote(ctx context.Context, question string) (string, error) {
	c.mu.RLock()
	enabled := c.enabled
	tgBot := c.tgBot
	chatID := c.chatID
	c.mu.RUnlock()

	if !enabled {
		return "", fmt.Errorf("live mode not enabled")
	}

	if tgBot == nil {
		return "", fmt.Errorf("telegram bot not configured")
	}

	if chatID == 0 {
		return "", fmt.Errorf("telegram chat ID not set")
	}

	// Use the bot's AskUser method which handles inline buttons
	// Prefer context chatID if available (dynamic routing)
	var response string
	var err error
	if ctxChatID, ok := ctx.Value(chatIDKey).(int64); ok {
		response, err = tgBot.AskUser(ctx, ctxChatID, question)
	} else {
		// Fallback to default configured ChatID
		response, err = tgBot.AskUser(ctx, chatID, question)
	}

	// Emit activity to notify UI about the approval
	if err == nil && response != "" {
		var status string
		switch response {
		case "yes":
			status = "✅ Approved via Telegram"
		case "no":
			status = "❌ Rejected via Telegram"
		case "always allow":
			status = "🛡️ Always Allow enabled via Telegram"
		default:
			status = "Received: " + response
		}
		c.emitActivity("approved", "telegram", "", status)
	}

	return response, err
}
