package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/igoryan-dao/ricochet/internal/format"
	"github.com/igoryan-dao/ricochet/internal/state"
	"github.com/igoryan-dao/ricochet/internal/whisper"
)

// Callback data constants
const (
	CallbackChatHistory = "chat_history"
	CallbackNewChat     = "new_chat"
)

// Bot wraps Telegram bot with message handling
type Bot struct {
	bot            *bot.Bot
	token          string
	allowedUserIDs map[int64]bool
	state          *state.Manager

	// Channel for receiving user responses (generic)
	responseCh chan *UserResponse

	// Channel for callback events
	callbackCh chan *CallbackEvent

	// Active session per chat (chatID -> SessionUUID)
	activeMu       sync.Mutex
	activeSessions map[int64]string

	// Pending questions awaiting answers (chatID -> response channel)
	pendingMu sync.Mutex
	pending   map[int64]chan string

	// Session specific channels (SessionUUID -> response channel)
	sessionMu        sync.Mutex
	sessionResponses map[string]chan string

	// Buffer for messages when no one is listening (SessionUUID -> []string)
	unreadMu       sync.Mutex
	unreadMessages map[string][]string

	// Whisper transcriber
	transcriber *whisper.Transcriber
}

// UserResponse represents a message from user
type UserResponse struct {
	ChatID    int64
	Text      string
	SessionID string // Optional, if linked to a session
}

// CallbackEvent represents a button click
type CallbackEvent struct {
	ChatID    int64
	UserID    int64
	Data      string
	MessageID int
}

// New creates a new Telegram bot
func New(token string, allowedIDs []int64, stateMgr *state.Manager) (*Bot, error) {
	allowed := make(map[int64]bool)
	for _, id := range allowedIDs {
		allowed[id] = true
	}

	b := &Bot{
		token:            token,
		allowedUserIDs:   allowed,
		state:            stateMgr,
		responseCh:       make(chan *UserResponse, 100),
		callbackCh:       make(chan *CallbackEvent, 100),
		activeSessions:   stateMgr.GetActiveSessions(),
		pending:          make(map[int64]chan string),
		sessionResponses: make(map[string]chan string),
		unreadMessages:   make(map[string][]string),
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.handleUpdate),
	}

	if token != "" {
		tgBot, err := bot.New(token, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
		b.bot = tgBot
	} else {
		log.Println("⚠️ Telegram token is empty. Bot will operate in bridge-only mode.")
	}

	return b, nil
}

// GetUnreadMessages returns and clears unread messages for a session
func (b *Bot) GetUnreadMessages(sessionID string) []string {
	b.unreadMu.Lock()
	defer b.unreadMu.Unlock()

	msgs := b.unreadMessages[sessionID]
	delete(b.unreadMessages, sessionID)
	return msgs
}

// Start begins long polling
func (b *Bot) Start(ctx context.Context) {
	if b.bot == nil {
		log.Println("⚠️ Telegram bot start skipped (no token). Waiting for Cloud Bridge events...")
		<-ctx.Done()
		return
	}
	log.Println("Starting Telegram bot...")
	b.bot.Start(ctx)
}

// handleUpdate processes all incoming updates
func (b *Bot) handleUpdate(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	// Handle callback queries (button clicks)
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, tgBot, update.CallbackQuery)
		return
	}

	// Handle regular messages
	if update.Message != nil {
		if update.Message.Voice != nil {
			b.handleVoice(ctx, tgBot, update.Message)
		} else {
			b.handleMessage(ctx, tgBot, update.Message)
		}
	}
}

// handleCallback processes button clicks
func (b *Bot) handleCallback(ctx context.Context, tgBot *bot.Bot, callback *models.CallbackQuery) {
	chatID := callback.Message.Message.Chat.ID
	userID := callback.From.ID

	// Check if user is allowed
	if len(b.allowedUserIDs) > 0 && !b.allowedUserIDs[userID] {
		log.Printf("Unauthorized callback from user %d", userID)
		return
	}

	// Answer callback to remove loading state
	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	log.Printf("Callback received: %s from chat %d", callback.Data, chatID)

	// Send to callback channel
	b.callbackCh <- &CallbackEvent{
		ChatID:    chatID,
		UserID:    userID,
		Data:      callback.Data,
		MessageID: callback.Message.Message.ID,
	}
}

// SendToSession sends a message to a session listener or buffers it
func (b *Bot) SendToSession(sessionID string, text string) {
	b.sessionMu.Lock()
	ch, ok := b.sessionResponses[sessionID]
	b.sessionMu.Unlock()

	if ok {
		select {
		case ch <- text:
			return
		default:
			// Full, buffer it
		}
	}

	b.unreadMu.Lock()
	b.unreadMessages[sessionID] = append(b.unreadMessages[sessionID], text)
	b.unreadMu.Unlock()
	log.Printf("Message buffered for session %s: %s", sessionID, text)
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(ctx context.Context, _ *bot.Bot, message *models.Message) {
	chatID := message.Chat.ID
	userID := message.From.ID

	if len(b.allowedUserIDs) > 0 && !b.allowedUserIDs[userID] {
		log.Printf("Unauthorized access attempt from user %d", userID)
		return
	}

	text := message.Text
	if strings.HasPrefix(text, "/start") {
		b.sendWelcomeMenu(ctx, chatID)
		return
	}

	b.activeMu.Lock()
	sessionID := b.activeSessions[chatID]
	b.activeMu.Unlock()

	if sessionID != "" {
		b.SendToSession(sessionID, text)
		return
	}

	// 2. Fallback to generic pending questions (legacy/general)
	b.pendingMu.Lock()
	if ch, ok := b.pending[chatID]; ok {
		ch <- text
		delete(b.pending, chatID)
		b.pendingMu.Unlock()
		return
	}
	b.pendingMu.Unlock()

	// 3. Send to response channel for general processing
	b.responseCh <- &UserResponse{
		ChatID:    chatID,
		Text:      text,
		SessionID: sessionID,
	}
}

// handleVoice processes incoming voice messages
func (b *Bot) handleVoice(ctx context.Context, tgBot *bot.Bot, message *models.Message) {
	chatID := message.Chat.ID
	userID := message.From.ID

	if len(b.allowedUserIDs) > 0 && !b.allowedUserIDs[userID] {
		log.Printf("Unauthorized voice access attempt from user %d", userID)
		return
	}

	if b.transcriber == nil {
		b.SendMessage(ctx, chatID, "⚠️ Голосовое управление не настроено (отсутствует Transcriber).")
		return
	}

	b.SendMessage(ctx, chatID, "🎙 _Обрабатываю голосовое сообщение..._")
	b.SendTyping(ctx, chatID)

	// 1. Get file info
	file, err := tgBot.GetFile(ctx, &bot.GetFileParams{
		FileID: message.Voice.FileID,
	})
	if err != nil {
		b.SendMessage(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении файла: %v", err))
		return
	}

	// 2. Download file
	homeDir, _ := os.UserHomeDir()
	oggPath := filepath.Join(homeDir, ".ricochet", "tmp", message.Voice.FileID+".ogg")

	os.MkdirAll(filepath.Dir(oggPath), 0755)

	if err := b.downloadFile(ctx, file.FilePath, oggPath); err != nil {
		b.SendMessage(ctx, chatID, fmt.Sprintf("❌ Ошибка при скачивании файла: %v", err))
		return
	}
	defer os.Remove(oggPath)

	// 3. Transcribe
	text, err := b.transcriber.Transcribe(oggPath)
	if err != nil {
		b.SendMessage(ctx, chatID, fmt.Sprintf("❌ Ошибка транскрипции: %v", err))
		return
	}

	if text == "" {
		b.SendMessage(ctx, chatID, "🤔 Не удалось распознать речь.")
		return
	}

	b.SendMessage(ctx, chatID, fmt.Sprintf("📝 _Текст_: %s", text))

	// 4. Route to session
	b.activeMu.Lock()
	sessionID := b.activeSessions[chatID]
	b.activeMu.Unlock()

	if sessionID != "" {
		b.SendToSession(sessionID, "[Voice Message]: "+text)
	} else {
		b.responseCh <- &UserResponse{
			ChatID:    chatID,
			Text:      "[Voice Message]: " + text,
			SessionID: sessionID,
		}
	}
}

// downloadFile downloads a file from Telegram servers
func (b *Bot) downloadFile(_ context.Context, tgFilePath, localPath string) error {
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, tgFilePath)

	resp, err := http.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// SetTranscriber sets the whisper transcriber
func (b *Bot) SetTranscriber(t *whisper.Transcriber) {
	b.transcriber = t
}

// SendMessageAndTrack sends a message and returns its ID for later editing
func (b *Bot) SendMessageAndTrack(ctx context.Context, chatID int64, text string) (int, error) {
	formatted := format.ToTelegramHTML(text)
	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      formatted,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

// EditMessage edits an existing message by ID
func (b *Bot) EditMessage(ctx context.Context, chatID int64, messageID int, newText string) error {
	_, err := b.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      format.ToTelegramHTML(newText),
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SendPhoto sends an image to a chat
func (b *Bot) SendPhoto(ctx context.Context, chatID int64, photoPath string, caption string) error {
	file, err := os.Open(photoPath)
	if err != nil {
		return fmt.Errorf("failed to open photo: %w", err)
	}
	defer file.Close()

	_, err = b.bot.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:    chatID,
		Photo:     &models.InputFileUpload{Filename: filepath.Base(photoPath), Data: file},
		Caption:   format.ToTelegramHTML(caption),
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SendVoice sends a voice message (audio file) to a chat
func (b *Bot) SendVoice(ctx context.Context, chatID int64, audioPath string) error {
	file, err := os.Open(audioPath)
	if err != nil {
		return fmt.Errorf("failed to open audio: %w", err)
	}
	defer file.Close()

	_, err = b.bot.SendVoice(ctx, &bot.SendVoiceParams{
		ChatID: chatID,
		Voice:  &models.InputFileUpload{Filename: filepath.Base(audioPath), Data: file},
	})
	return err
}

// SendCodeBlock sends a nicely formatted code block
func (b *Bot) SendCodeBlock(ctx context.Context, chatID int64, language, code string) error {
	text := fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", language, format.EscapeHTML(code))
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SetActiveSession sets the active session for a chat
func (b *Bot) SetActiveSession(chatID int64, sessionID string) {
	b.activeMu.Lock()
	b.activeSessions[chatID] = sessionID
	b.activeMu.Unlock()

	if b.state != nil {
		if err := b.state.SetActiveSession(chatID, sessionID); err != nil {
			log.Printf("Failed to save active session state: %v", err)
		}
	}
	log.Printf("Active session for chat %d set to %s", chatID, sessionID)
}

// GetActiveSession returns the active session ID for a chat
func (b *Bot) GetActiveSession(chatID int64) string {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()
	return b.activeSessions[chatID]
}

// RegisterSessionHandler registers a channel for a specific session's response
func (b *Bot) RegisterSessionHandler(sessionID string, ch chan string) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	b.sessionResponses[sessionID] = ch
}

// UnregisterSessionHandler removes a session handler
func (b *Bot) UnregisterSessionHandler(sessionID string) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	delete(b.sessionResponses, sessionID)
}

// sendWelcomeMenu sends the main menu with inline buttons
func (b *Bot) sendWelcomeMenu(ctx context.Context, chatID int64) {
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 История чатов", CallbackData: CallbackChatHistory},
				{Text: "➕ Новый чат", CallbackData: CallbackNewChat},
			},
		},
	}

	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "👋 Добро пожаловать в Ricochet!\n\nВыберите действие:",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("Failed to send welcome menu: %v", err)
	}
}

// SendMessage sends a notification to a chat
func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string) error {
	if b.bot == nil {
		log.Printf("⚠️ Bot.SendMessage called but local bot is not initialized (Bridge mode?)")
		return nil
	}
	formatted := format.ToTelegramHTML(text)
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      formatted,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SendMessageWithButtons sends a message with inline keyboard
func (b *Bot) SendMessageWithButtons(ctx context.Context, chatID int64, text string, buttons [][]ButtonConfig) error {
	if b.bot == nil {
		log.Printf("⚠️ Bot.SendMessageWithButtons called but local bot is not initialized")
		return nil
	}
	formatted := format.ToTelegramHTML(text)
	keyboard := make([][]models.InlineKeyboardButton, len(buttons))
	for i, row := range buttons {
		keyboard[i] = make([]models.InlineKeyboardButton, len(row))
		for j, btn := range row {
			keyboard[i][j] = models.InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: btn.Data,
			}
		}
	}

	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      formatted,
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
	return err
}

// ButtonConfig represents a button configuration
type ButtonConfig struct {
	Text string
	Data string
}

// AskUser sends a question and waits for response (generic legacy)
func (b *Bot) AskUser(ctx context.Context, chatID int64, question string) (string, error) {
	// Create response channel
	respCh := make(chan string, 1)

	b.pendingMu.Lock()
	b.pending[chatID] = respCh
	b.pendingMu.Unlock()

	// Send question
	if err := b.SendMessage(ctx, chatID, question); err != nil {
		b.pendingMu.Lock()
		delete(b.pending, chatID)
		b.pendingMu.Unlock()
		return "", err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		b.pendingMu.Lock()
		delete(b.pending, chatID)
		b.pendingMu.Unlock()
		return "", ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

// GetResponseChannel returns channel for incoming messages
func (b *Bot) GetResponseChannel() <-chan *UserResponse {
	return b.responseCh
}

// GetCallbackChannel returns channel for button clicks
func (b *Bot) GetCallbackChannel() <-chan *CallbackEvent {
	return b.callbackCh
}

// IsSessionOnline checks if there is a listener for this session
func (b *Bot) IsSessionOnline(sessionID string) bool {
	// 1. Check if there is a live registered handler
	b.sessionMu.Lock()
	_, hasHandler := b.sessionResponses[sessionID]
	b.sessionMu.Unlock()

	if hasHandler {
		return true
	}

	// 2. Check heartbeat in state
	if b.state != nil {
		return b.state.IsSessionActive(sessionID)
	}

	return false
}

// SendTyping sends a typing action to a chat
func (b *Bot) SendTyping(ctx context.Context, chatID int64) {
	b.bot.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: models.ChatActionTyping,
	})
}
