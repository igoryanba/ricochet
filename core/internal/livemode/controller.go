package livemode

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/state"
	"github.com/igoryan-dao/ricochet/internal/telegram"
)

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

	// Callback for emitting activity events to extension
	onActivity func(EtherActivity)

	// Callback for forwarding chat updates to IDE
	onChatUpdate func(agent.ChatUpdate)
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

// Enable starts Live Mode
func (c *Controller) Enable(ctx context.Context) (*Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.enabled {
		return c.getStatusLocked(), nil
	}

	if c.tgBot == nil {
		// Allow enabling for UI demo purposes, but set status to not connected
		log.Println("⚠️ Telegram bot not configured. Enabling Live Mode in offline/demo state.")
		c.enabled = true
		return c.getStatusLocked(), nil
	}

	// Create cancellable context for the listener
	listenerCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.enabled = true

	// Start Telegram bot in background
	go c.tgBot.Start(listenerCtx)

	// Start message listener
	go c.listenForMessages(listenerCtx)

	// Notify user
	if c.chatID != 0 {
		c.tgBot.SendMessage(ctx, c.chatID, "🟢 **Live Mode Enabled**\n\nYou can now send messages here to control Ricochet!")
	}

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

	// Cancel listener
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	c.enabled = false

	// Notify user
	if c.chatID != 0 && c.tgBot != nil {
		c.tgBot.SendMessage(ctx, c.chatID, "🔴 **Live Mode Disabled**\n\nReturning control to IDE.")
	}

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

	if c.agent == nil {
		c.tgBot.SendMessage(ctx, resp.ChatID, "⚠️ Agent not configured")
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

	err := c.agent.Chat(ctx, agent.ChatRequestInput{
		SessionID: "default", // Shared session with IDE
		Content:   resp.Text,
		Via:       "telegram",
	}, func(update agent.ChatUpdate) {
		// Forward updates to IDE with via field
		update.Message.Via = "telegram"
		c.emitChatUpdate(update)

		// Forward to Telegram
		content := update.Message.Content
		if content == "" || content == currentContent {
			return
		}

		// Only send partial updates for non-streaming or final messages
		// If streaming, only update every few chars or after significant time
		if update.Message.IsStreaming && len(content)-len(currentContent) < 20 {
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

	switch callback.Data {
	case telegram.CallbackNewChat:
		if c.agent != nil {
			c.agent.ClearSession(fmt.Sprintf("telegram-%d", callback.ChatID))
		}
		c.tgBot.SendMessage(ctx, callback.ChatID, "🆕 New chat started!")

	case telegram.CallbackChatHistory:
		c.tgBot.SendMessage(ctx, callback.ChatID, "📋 Chat history feature coming soon...")
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
func (c *Controller) emitChatUpdate(update agent.ChatUpdate) {
	c.mu.RLock()
	fn := c.onChatUpdate
	c.mu.RUnlock()

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
	return tgBot.AskUser(ctx, chatID, question)
}
