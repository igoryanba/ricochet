package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/igoryan-dao/ricochet/internal/bridge"
	"github.com/igoryan-dao/ricochet/internal/bridge/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Session represents a connected local agent
type Session struct {
	ID     string
	Stream proto.BridgeService_ConnectServer
	Done   chan struct{}
}

// CloudBridge is the central server component of Ricochet v2
type CloudBridge struct {
	proto.UnimplementedBridgeServiceServer
	upgrader websocket.Upgrader
	port     int
	tgToken  string
	tgBot    *bot.Bot

	sessionsMu sync.RWMutex
	sessions   map[string]*Session // sessionID -> Session
	chatsMu    sync.RWMutex
	chatToSess map[int64]string // chatID -> sessionID
}

func NewCloudBridge(port int, tgToken string) *CloudBridge {
	return &CloudBridge{
		port:       port,
		tgToken:    tgToken,
		sessions:   make(map[string]*Session),
		chatToSess: make(map[int64]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *CloudBridge) Start(ctx context.Context) error {
	// Initialize Telegram Bot
	opts := []bot.Option{
		bot.WithDefaultHandler(s.defaultHandler),
	}
	b, err := bot.New(s.tgToken, opts...)
	if err != nil {
		return fmt.Errorf("failed to create tg bot: %w", err)
	}
	s.tgBot = b

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/webhook/telegram", b.WebhookHandler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("🚀 Ricochet Cloud Bridge starting on :%d...", s.port)

	// Start bot in long polling or webhook mode
	// For testing, we might want long polling, but for cloud it's webhooks.
	go b.Start(ctx)

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server.ListenAndServe()
}

func (s *CloudBridge) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	text := update.Message.Text

	// Handle /link command
	if strings.HasPrefix(text, "/link ") {
		sessionID := strings.TrimSpace(strings.TrimPrefix(text, "/link "))
		s.sessionsMu.RLock()
		_, ok := s.sessions[sessionID]
		s.sessionsMu.RUnlock()

		if ok {
			s.chatsMu.Lock()
			s.chatToSess[chatID] = sessionID
			s.chatsMu.Unlock()
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      fmt.Sprintf("✅ <b>Success!</b> Your Telegram chat is now linked to Ricochet Agent: <code>%s</code>", sessionID),
				ParseMode: models.ParseModeHTML,
			})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      "❌ <b>Session not found.</b> Make sure your local agent is running and connected.",
				ParseMode: models.ParseModeHTML,
			})
		}
		return
	}

	log.Printf("📩 Received Telegram message from %d: %s", chatID, text)

	// Routing logic
	s.chatsMu.RLock()
	sessionID, ok := s.chatToSess[chatID]
	s.chatsMu.RUnlock()

	if !ok {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "⚠️ <b>Chat not linked.</b> Use <code>/link &lt;your_session_id&gt;</code> to connect your Ricochet Agent.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Send to Local Agent via bridge
	err := s.SendToSession(sessionID, &proto.BridgeEvent{
		Payload: &proto.BridgeEvent_IncomingMessage{
			IncomingMessage: &proto.IncomingMessage{
				ChatId:   chatID,
				Body:     text,
				Platform: "telegram",
			},
		},
	})

	if err != nil {
		log.Printf("❌ Failed to forward message to session %s: %v", sessionID, err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ <b>Agent offline.</b> Reconnecting...",
			ParseMode: models.ParseModeHTML,
		})
	}
}

func (s *CloudBridge) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	rwc := bridge.NewWebSocketRWC(conn)

	session, err := yamux.Server(rwc, nil)
	if err != nil {
		log.Printf("Yamux server error: %v", err)
		return
	}

	grpcServer := grpc.NewServer()
	proto.RegisterBridgeServiceServer(grpcServer, s)

	if err := grpcServer.Serve(session); err != nil {
		log.Printf("gRPC server error: %v", err)
	}
}

func (s *CloudBridge) Connect(stream proto.BridgeService_ConnectServer) error {
	// 0. Verify Auth Secret from Metadata
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return fmt.Errorf("missing metadata")
	}

	secrets := md.Get("x-bridge-secret")
	expectedSecret := os.Getenv("RICOCHET_BRIDGE_SECRET")

	if len(secrets) == 0 || secrets[0] != expectedSecret {
		log.Printf("⛔ Unauthorized connection attempt!")
		return fmt.Errorf("unauthorized")
	}

	// 1. Initial Handshake / Identity
	event, err := stream.Recv()
	if err != nil {
		return err
	}

	sessionID := event.GetSessionId()
	if sessionID == "" {
		return fmt.Errorf("missing session_id in handshake")
	}

	log.Printf("🔌 Local Agent connected: %s", sessionID)

	sess := &Session{
		ID:     sessionID,
		Stream: stream,
		Done:   make(chan struct{}),
	}

	s.sessionsMu.Lock()
	s.sessions[sessionID] = sess
	s.sessionsMu.Unlock()

	defer func() {
		s.sessionsMu.Lock()
		delete(s.sessions, sessionID)
		s.sessionsMu.Unlock()
		log.Printf("🔌 Local Agent disconnected: %s", sessionID)
	}()

	// 2. Main Event Loop (Receiving from Local Agent)
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		// Handle events from client (log, relay, etc.)
		if hb := event.GetHeartbeat(); hb != nil {
			// update last seen?
		}

		if out := event.GetOutgoingMessage(); out != nil {
			log.Printf("[%s] Outgoing Message to %d: %s", sessionID, out.ChatId, out.Body)
			// Send to Telegram API from cloud
			s.tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
				ChatID:    out.ChatId,
				Text:      out.Body,
				ParseMode: models.ParseModeHTML, // Default to HTML as per Ricochet v1
			})
		}
	}
}

func (s *CloudBridge) Broadcast(event *proto.BridgeEvent) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for _, sess := range s.sessions {
		sess.Stream.Send(event)
	}
}

func (s *CloudBridge) SendToSession(sessionID string, event *proto.BridgeEvent) error {
	s.sessionsMu.RLock()
	sess, ok := s.sessions[sessionID]
	s.sessionsMu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	return sess.Stream.Send(event)
}
