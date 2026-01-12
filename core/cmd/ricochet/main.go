package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/codegraph"
	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/livemode"
	"github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/prompts"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/server"
)

var (
	// State is now managed by server.Handler, but we keep initial config here
	// to pass to the handler constructor.
	cfg            *agent.Config
	liveModeConfig *livemode.Config
	settingsStore  *config.Store
	outputMu       sync.Mutex

	// Server Hub
	wsHub *WsHub
)

// StdioWriter implements server.ResponseWriter for Stdio
type StdioWriter struct{}

func (w *StdioWriter) Send(msg interface{}) error {
	sendMessage(msg)
	return nil
}

// WsWriter implements server.ResponseWriter for WebSocket (broadcasts to specific conn or all)
type WsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *WsWriter) Send(msg interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(msg)
}

// BroadcastWriter implements server.ResponseWriter for broadcasting to all clients
type BroadcastWriter struct {
	hub *WsHub
}

func (w *BroadcastWriter) Send(msg interface{}) error {
	w.hub.Broadcast(msg)
	return nil
}

type WsHub struct {
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func NewWsHub() *WsHub {
	return &WsHub{
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *WsHub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
			log.Printf("Client connected. Total: %d", len(h.clients))
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.clientsMu.Unlock()
			log.Printf("Client disconnected. Total: %d", len(h.clients))
		case <-ctx.Done():
			return
		}
	}
}

func (h *WsHub) Broadcast(msg interface{}) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for client := range h.clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("Error broadcasting to client: %v", err)
			// Don't unregister here to avoid deadlock, let the reader loop handle disconnect
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for local dev
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	log.SetPrefix("[ricochet-core] ")
	log.SetOutput(os.Stderr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Get current working directory for context
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Failed to get cwd: %v", err)
		cwd = "."
	}

	// Initialize Settings Store
	var errStore error
	settingsStore, errStore = config.NewStore()
	if errStore != nil {
		log.Printf("Warning: Failed to initialize settings store: %v", errStore)
	}

	settings := settingsStore.Get()

	// Initialize default config (will be updated via settings)
	cfg = &agent.Config{
		Provider: agent.ProviderConfig{
			Provider: settings.Provider.Provider,
			Model:    settings.Provider.Model,
			APIKey:   settings.Provider.APIKey,
		},
		SystemPrompt:    prompts.BuildSystemPrompt(cwd),
		MaxTokens:       4096, // Max tokens for response
		ContextWindow:   128000,
		EnableCodeIndex: settings.Context.EnableCodeIndex,
	}

	// Configure Embedding Provider if one is specified
	if settings.Provider.EmbeddingProvider != "" {
		embKey := settings.Provider.APIKeys[settings.Provider.EmbeddingProvider]
		if embKey == "" && settings.Provider.Provider == settings.Provider.EmbeddingProvider {
			embKey = settings.Provider.APIKey
		}

		cfg.EmbeddingProvider = &agent.ProviderConfig{
			Provider: settings.Provider.EmbeddingProvider,
			Model:    settings.Provider.EmbeddingModel,
			APIKey:   embKey,
		}
	}

	// Initialize Live Mode config (will be updated via settings)
	liveModeConfig = &livemode.Config{
		TelegramToken:  settings.LiveMode.TelegramToken,
		TelegramChatID: settings.LiveMode.TelegramChatID,
		AllowedUserIDs: []int64{},
	}

	// Check for flags
	args := os.Args[1:]
	isServer := false
	port := "5555"
	isStdio := false

	for i := 0; i < len(args); i++ {
		if args[i] == "--server" {
			isServer = true
		} else if args[i] == "--port" && i+1 < len(args) {
			port = args[i+1]
			i++
		} else if args[i] == "--stdio" {
			isStdio = true
		}
	}

	if isServer {
		runServerMode(ctx, cwd, port)
	} else if isStdio {
		runStdioMode(ctx, cwd)
	} else {
		// Default to MCP mode if no args, or handle as needed
		runMCPMode(ctx)
	}
}

// runStdioMode runs as sidecar process communicating with extension via stdio
func runStdioMode(ctx context.Context, cwd string) {
	log.Println("Starting in stdio mode...")

	stdioHost := host.NewStdioHost(cwd)
	modesManager := modes.NewManager(cwd)
	mcpHub := mcp.NewHub(cwd)
	cg := codegraph.NewService()

	modesManager.SetOnModeChange(func(slug string) {
		sendMessage(protocol.RPCMessage{
			Type:    "mode_changed",
			Payload: protocol.EncodeRPC(map[string]string{"mode": slug}),
		})
	})

	// Initialize Handler
	// We pass nil for ProvidersManager initially, Handler handles lazy load if needed
	handler := server.NewHandler(
		ctx,
		cfg,
		liveModeConfig,
		settingsStore,
		stdioHost,
		modesManager,
		mcpHub,
		cg,
		nil,
	)
	writer := &StdioWriter{}

	// Send ready message
	sendMessage(protocol.RPCMessage{Type: "ready", Payload: protocol.EncodeRPC(map[string]string{"version": "0.1.0"})})

	// Read messages from stdin
	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		var msg protocol.RPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle response type directly in loop (Host specific)
		if msg.Type == "response" {
			var payload string
			// For RawMessage, we just pass the raw bytes or unmarshal if needed
			// Let's assume HandleResponse takes string for now as it's for AskUser
			_ = json.Unmarshal(msg.Payload, &payload)

			// Handle ID which could be string or float64 from extension
			var idStr string
			switch v := msg.ID.(type) {
			case string:
				idStr = v
			case float64:
				idStr = fmt.Sprintf("%.0f", v)
			}

			stdioHost.HandleResponse(idStr, payload)
			continue
		}

		// Process message via Handler
		go handler.HandleMessage(msg, writer)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

// runServerMode runs as a WebSocket server (Dawn of the Daemon)
func runServerMode(ctx context.Context, cwd, port string) {
	log.Printf("Starting in Server Mode on port %s...", port)

	// Server Host acts conceptually different than StdioHost,
	// it might just log or broadcast UI requests.
	// For now, let's reuse StdioHost logic but using logging
	headlessHost := host.NewStdioHost(cwd)

	modesManager := modes.NewManager(cwd)
	mcpHub := mcp.NewHub(cwd)
	cg := codegraph.NewService()

	wsHub = NewWsHub()
	go wsHub.Run(ctx)

	handler := server.NewHandler(
		ctx,
		cfg,
		liveModeConfig,
		settingsStore,
		headlessHost,
		modesManager,
		mcpHub,
		cg,
		nil,
	)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}

		wsHub.register <- conn

		wsWriter := &WsWriter{conn: conn}

		// Read Loop
		go func() {
			defer func() {
				wsHub.unregister <- conn
			}()

			for {
				var msg protocol.RPCMessage
				err := conn.ReadJSON(&msg)
				if err != nil {
					log.Printf("Read error: %v", err)
					break
				}

				// Ensure ID is string (JS JSON often sends numbers)

				// Handle response
				if msg.Type == "response" {
					continue
				}

				// Special handling for Chat Message to broadcast updates
				if msg.Type == "chat_message" {
					// We want updates to go to EVERYONE, not just the caller
					broadcastWriter := &BroadcastWriter{hub: wsHub}
					handler.HandleMessage(msg, broadcastWriter)
				} else {
					// Other requests (get_state, etc) go back to caller only
					handler.HandleMessage(msg, wsWriter)
				}
			}
		}()
	})

	log.Printf("Listening on :%s", port)
	server := &http.Server{Addr: ":" + port, Handler: nil}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Wait for shutdown trigger
	<-ctx.Done()

	ctxShut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctxShut)
}

// runMCPMode runs as MCP server (for Claude Code, Cursor, etc.)
func runMCPMode(ctx context.Context) {
	log.Println("Starting in MCP mode...")
	log.Println("MCP server not yet integrated into unified binary")
	<-ctx.Done()
}

func sendMessage(msg interface{}) {
	outputMu.Lock()
	defer outputMu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}
	fmt.Printf("%s\n", data)
}
