package discord

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/igoryan-dao/ricochet/internal/format"
	"github.com/igoryan-dao/ricochet/internal/state"
)

// Bot wraps Discord bot with message handling
type Bot struct {
	session *discordgo.Session
	guildID string // Optional: restrict to specific guild
	state   *state.Manager

	// Channel for receiving user responses
	responseCh chan *UserResponse

	// Active session per channel (channelID -> SessionUUID)
	activeMu       sync.Mutex
	activeSessions map[string]string

	// Session specific channels (SessionUUID -> response channel)
	sessionMu        sync.Mutex
	sessionResponses map[string]chan string

	// Buffer for messages when no one is listening
	unreadMu       sync.Mutex
	unreadMessages map[string][]string
}

// UserResponse represents a message from user
type UserResponse struct {
	ChannelID string
	UserID    string
	Text      string
	SessionID string
}

// New creates a new Discord bot
func New(token string, guildID string, stateMgr *state.Manager) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	b := &Bot{
		session:          session,
		guildID:          guildID,
		state:            stateMgr,
		responseCh:       make(chan *UserResponse, 100),
		activeSessions:   make(map[string]string),
		sessionResponses: make(map[string]chan string),
		unreadMessages:   make(map[string][]string),
	}

	// Register handlers
	session.AddHandler(b.handleMessage)
	session.AddHandler(b.handleReady)

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	// Load active sessions from state
	if stateMgr != nil {
		active := stateMgr.GetDiscordActiveSessions()
		for channelID, sessionID := range active {
			b.activeSessions[channelID] = sessionID
		}
	}

	return b, nil
}

// Start opens connection to Discord
func (b *Bot) Start() error {
	log.Println("Starting Discord bot...")
	return b.session.Open()
}

// Stop closes connection
func (b *Bot) Stop() error {
	log.Println("Stopping Discord bot...")
	return b.session.Close()
}

// handleReady logs when bot is connected
func (b *Bot) handleReady(_ *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Discord bot connected as %s#%s", r.User.Username, r.User.Discriminator)
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Ignore messages from other guilds if restricted
	if b.guildID != "" && m.GuildID != b.guildID {
		return
	}

	text := m.Content
	channelID := m.ChannelID

	// Handle commands
	if strings.HasPrefix(text, "/ricochet") || strings.HasPrefix(text, "!ricochet") {
		b.handleCommand(s, m)
		return
	}

	// Route to active session
	b.activeMu.Lock()
	sessionID := b.activeSessions[channelID]
	b.activeMu.Unlock()

	if sessionID != "" {
		b.SendToSession(sessionID, text)
		return
	}

	// Buffer or send to general channel
	b.responseCh <- &UserResponse{
		ChannelID: channelID,
		UserID:    m.Author.ID,
		Text:      text,
	}
}

// handleCommand processes bot commands
func (b *Bot) handleCommand(_ *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.Fields(m.Content)
	if len(parts) < 2 {
		b.SendMessage(context.Background(), m.ChannelID, "📡 **Ricochet Discord** — AI Agent Bridge\n\nCommands:\n• `/ricochet status` — Show active session\n• `/ricochet activate <session>` — Activate a session")
		return
	}

	switch parts[1] {
	case "status":
		b.activeMu.Lock()
		sessionID := b.activeSessions[m.ChannelID]
		b.activeMu.Unlock()

		if sessionID == "" {
			b.SendMessage(context.Background(), m.ChannelID, "📭 No active session in this channel")
		} else {
			b.SendMessage(context.Background(), m.ChannelID, fmt.Sprintf("✅ Active session: `%s`", sessionID[:8]))
		}

	case "activate":
		if len(parts) < 3 {
			b.SendMessage(context.Background(), m.ChannelID, "Usage: `/ricochet activate <session_id>`")
			return
		}
		sessionID := parts[2]
		b.SetActiveSession(m.ChannelID, sessionID)
		b.SendMessage(context.Background(), m.ChannelID, fmt.Sprintf("📍 Session `%s` activated for this channel", sessionID[:8]))

	default:
		b.SendMessage(context.Background(), m.ChannelID, "Unknown command. Try `/ricochet` for help.")
	}
}

// SendMessage sends a message to a channel
func (b *Bot) SendMessage(ctx context.Context, channelID string, text string) error {
	formatted := format.ToDiscordMarkdown(text)
	_, err := b.session.ChannelMessageSend(channelID, formatted)
	return err
}

// SendPhoto sends an image to a Discord channel
func (b *Bot) SendPhoto(ctx context.Context, channelID string, photoPath string, caption string) error {
	file, err := os.Open(photoPath)
	if err != nil {
		return fmt.Errorf("failed to open photo: %w", err)
	}
	defer file.Close()

	_, err = b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: caption,
		Files: []*discordgo.File{
			{
				Name:        filepath.Base(photoPath),
				ContentType: "image/png", // Discord usually detects it, but better specified
				Reader:      file,
			},
		},
	})
	return err
}

// SendVoice sends an audio file to a Discord channel
func (b *Bot) SendVoice(ctx context.Context, channelID string, audioPath string) error {
	file, err := os.Open(audioPath)
	if err != nil {
		return fmt.Errorf("failed to open audio: %w", err)
	}
	defer file.Close()

	_, err = b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:        filepath.Base(audioPath),
				ContentType: "audio/wav",
				Reader:      file,
			},
		},
	})
	return err
}

// SendCodeBlock sends a formatted code block to Discord
func (b *Bot) SendCodeBlock(ctx context.Context, channelID string, language, code string) error {
	formatted := fmt.Sprintf("```%s\n%s\n```", language, code)
	_, err := b.session.ChannelMessageSend(channelID, formatted)
	return err
}

// SendToSession routes message to a specific session
func (b *Bot) SendToSession(sessionID, text string) {
	b.sessionMu.Lock()
	ch, ok := b.sessionResponses[sessionID]
	b.sessionMu.Unlock()

	if ok {
		select {
		case ch <- text:
			log.Printf("Message sent to session %s", sessionID)
		default:
			log.Printf("Session %s channel full, buffering", sessionID)
			b.bufferMessage(sessionID, text)
		}
		return
	}

	b.bufferMessage(sessionID, text)
}

func (b *Bot) bufferMessage(sessionID, text string) {
	b.unreadMu.Lock()
	b.unreadMessages[sessionID] = append(b.unreadMessages[sessionID], text)
	b.unreadMu.Unlock()
	log.Printf("Message buffered for session %s", sessionID)
}

// SetActiveSession sets the active session for a channel
func (b *Bot) SetActiveSession(channelID, sessionID string) {
	b.activeMu.Lock()
	b.activeSessions[channelID] = sessionID
	b.activeMu.Unlock()
	log.Printf("Active session for channel %s set to %s", channelID, sessionID)

	if b.state != nil {
		if err := b.state.SetDiscordActiveSession(channelID, sessionID); err != nil {
			log.Printf("Failed to save Discord session state: %v", err)
		}
	}
}

// GetActiveSession returns the active session for a channel
func (b *Bot) GetActiveSession(channelID string) string {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()
	return b.activeSessions[channelID]
}

// RegisterSessionHandler registers a channel for session responses
func (b *Bot) RegisterSessionHandler(sessionID string, ch chan string) {
	b.sessionMu.Lock()
	b.sessionResponses[sessionID] = ch
	b.sessionMu.Unlock()
}

// UnregisterSessionHandler removes session handler
func (b *Bot) UnregisterSessionHandler(sessionID string) {
	b.sessionMu.Lock()
	delete(b.sessionResponses, sessionID)
	b.sessionMu.Unlock()
}

// GetUnreadMessages returns and clears buffered messages
func (b *Bot) GetUnreadMessages(sessionID string) []string {
	b.unreadMu.Lock()
	defer b.unreadMu.Unlock()
	msgs := b.unreadMessages[sessionID]
	delete(b.unreadMessages, sessionID)
	return msgs
}

// GetResponseChannel returns the general response channel
func (b *Bot) GetResponseChannel() <-chan *UserResponse {
	return b.responseCh
}

// SendTyping shows typing indicator
func (b *Bot) SendTyping(ctx context.Context, channelID string) {
	b.session.ChannelTyping(channelID)
}
