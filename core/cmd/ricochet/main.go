package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/checkpoints"
	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/livemode"
	"github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/prompts"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/whisper"
)

var (
	agentController    *agent.Controller
	liveModeController *livemode.Controller
	checkpointService  *checkpoints.CheckpointService
	providersManager   *config.ProvidersManager
	outputMu           sync.Mutex
	cfg                *agent.Config
	liveModeConfig     *livemode.Config
	settingsStore      *config.Store
	globalCtx          context.Context
	audioBuffer        []byte
	audioMu            sync.Mutex
	transcriber        *whisper.Transcriber
	stdioHost          *host.StdioHost
	modesManager       *modes.Manager
	mcpHub             *mcp.Hub
	initMu             sync.Mutex
)

type Response struct {
	ID      interface{} `json:"id,omitempty"`
	Type    string      `json:"type,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
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
		MaxTokens:       4096,   // Max tokens for response
		ContextWindow:   128000, // Context window for DeepSeek (will be updated when model changes)
		EnableCodeIndex: settings.Context.EnableCodeIndex,
	}

	// Initialize Live Mode config (will be updated via settings)
	liveModeConfig = &livemode.Config{
		TelegramToken:  settings.LiveMode.TelegramToken,
		TelegramChatID: settings.LiveMode.TelegramChatID,
		AllowedUserIDs: []int64{}, // Empty = allow all (bot is protected by token)
	}
	globalCtx = ctx

	// Check for --stdio mode
	if len(os.Args) > 1 && os.Args[1] == "--stdio" {
		runStdioMode(ctx)
	} else {
		runMCPMode(ctx)
	}
}

// runStdioMode runs as sidecar process communicating with extension via stdio
func runStdioMode(ctx context.Context) {
	log.Println("Starting in stdio mode...")

	cwd, _ := os.Getwd()
	stdioHost = host.NewStdioHost(cwd)
	modesManager = modes.NewManager(cwd)
	mcpHub = mcp.NewHub(cwd)
	modesManager.SetOnModeChange(func(slug string) {
		sendMessage(protocol.RPCMessage{
			Type:    "mode_changed",
			Payload: protocol.EncodeRPC(map[string]string{"mode": slug}),
		})
	})

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

		// Ensure controller is initialized with StdioHost if not already
		if agentController == nil {
			initMu.Lock()
			if agentController == nil {
				var err error
				agentController, err = agent.NewController(cfg, agent.ControllerOptions{
					Host:             stdioHost,
					Modes:            modesManager,
					McpHub:           mcpHub,
					ProvidersManager: providersManager,
				})
				if err != nil {
					log.Printf("Failed to initialize controller: %v", err)
				} else if liveModeController != nil {
					agentController.SetLiveMode(liveModeController)
				}
			}
			initMu.Unlock()
		}

		// Handle response type directly in loop
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

		go handleMessage(ctx, msg)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

// runMCPMode runs as MCP server (for Claude Code, Cursor, etc.)
func runMCPMode(ctx context.Context) {
	log.Println("Starting in MCP mode...")

	// TODO: Import and run MCP server from internal/mcp
	log.Println("MCP server not yet integrated into unified binary")

	<-ctx.Done()
}

// handleMessage processes incoming messages
func handleMessage(ctx context.Context, msg protocol.RPCMessage) {
	switch msg.Type {
	case "get_state":
		var payload struct {
			SessionID string `json:"session_id"`
		}
		json.Unmarshal(msg.Payload, &payload)
		sessionID := payload.SessionID
		if sessionID == "" {
			sessionID = "default"
		}

		if agentController != nil {
			state := agentController.GetState(sessionID)
			sendMessage(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "state",
				Payload: protocol.EncodeRPC(state),
			})
		} else {
			sendMessage(protocol.RPCMessage{
				ID:   msg.ID,
				Type: "state",
				Payload: protocol.EncodeRPC(map[string]interface{}{
					"messages":        []interface{}{},
					"liveModeEnabled": false,
				}),
			})
		}

	case "list_sessions":
		if agentController != nil {
			sessions := agentController.ListSessions()
			sendMessage(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "session_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"sessions": sessions}),
			})
		} else {
			sendMessage(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "session_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"sessions": []interface{}{}}),
			})
		}

	case "create_session":
		if agentController == nil {
			var err error
			agentController, err = agent.NewController(cfg, agent.ControllerOptions{
				ProvidersManager: providersManager,
			})
			if err != nil {
				sendMessage(Response{ID: msg.ID, Error: err.Error()})
				return
			}
		}
		session := agentController.CreateSession()
		sendMessage(protocol.RPCMessage{
			ID:      msg.ID,
			Type:    "session_created",
			Payload: protocol.EncodeRPC(session),
		})

	case "delete_session":
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}
		if agentController != nil {
			agentController.DeleteSession(payload.SessionID)
		}
		sendMessage(Response{ID: msg.ID, Type: "session_deleted"})

	case "chat_message":
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
			return
		}

		log.Printf("Received chat message: %s", payload.Content)

		// Check if we have API key configured
		if cfg.Provider.APIKey == "" {
			sendMessage(protocol.RPCMessage{
				ID:    msg.ID,
				Type:  "response",
				Error: "⚠️ API key not configured. Please go to Settings and add your API key (DeepSeek, Gemini, etc.).",
			})
			return
		}

		// Ensure controller is initialized
		if agentController == nil {
			initMu.Lock()
			if agentController == nil {
				var err error
				log.Printf("Initializing agent controller with provider %s (%s)", cfg.Provider.Provider, cfg.Provider.Model)
				agentController, err = agent.NewController(cfg, agent.ControllerOptions{
					Host:             stdioHost,
					Modes:            modesManager,
					McpHub:           mcpHub,
					ProvidersManager: providersManager,
				})
				if err != nil {
					initMu.Unlock()
					sendMessage(protocol.RPCMessage{
						ID:    msg.ID,
						Type:  "response",
						Error: fmt.Sprintf("Failed to initialize AI provider: %v", err),
					})
					return
				}
				if liveModeController != nil {
					agentController.SetLiveMode(liveModeController)
				}
			}
			initMu.Unlock()
		}

		// Stream chat response
		var fullPayload struct {
			Content   string `json:"content"`
			SessionID string `json:"session_id"`
			Via       string `json:"via"`
		}
		if err := json.Unmarshal(msg.Payload, &fullPayload); err != nil {
			sendMessage(protocol.RPCMessage{ID: msg.ID, Error: "Invalid payload: " + err.Error()})
			return
		}

		sessionID := fullPayload.SessionID
		if sessionID == "" {
			sessionID = "default"
		}

		err := agentController.Chat(ctx, agent.ChatRequestInput{
			SessionID: sessionID,
			Content:   fullPayload.Content,
			Via:       fullPayload.Via,
		}, func(update agent.ChatUpdate) {
			// Forward update to extension
			sendMessage(protocol.RPCMessage{
				Type: "chat_update",
				Payload: protocol.EncodeRPC(map[string]interface{}{
					"message": update.Message,
				}),
			})
		})

		if err != nil {
			log.Printf("Chat error: %v", err)
			sendMessage(protocol.RPCMessage{
				ID:    msg.ID,
				Type:  "response",
				Error: err.Error(),
			})
		} else {
			// CRITICAL: Send final response to resolve extension promise
			sendMessage(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "response",
				Payload: protocol.EncodeRPC(map[string]interface{}{"status": "done"}),
			})
		}

	case "get_models":
		log.Println("get_models: Starting...")

		// Initialize providers manager if not done
		if providersManager == nil {
			configPath := config.FindConfigFile()
			log.Printf("get_models: Config path: %s", configPath)
			pm, err := config.NewProvidersManager(configPath)
			if err != nil {
				log.Printf("get_models: Error creating ProvidersManager: %v", err)
			}
			providersManager = pm
		}

		if providersManager == nil {
			log.Println("get_models: ERROR - providersManager is nil!")
			sendMessage(protocol.RPCMessage{
				ID:   msg.ID,
				Type: "response",
				Payload: protocol.EncodeRPC(map[string]interface{}{
					"providers": []interface{}{},
				}),
			})
			return
		}

		// Set user key from settings if available
		if settingsStore != nil {
			s := settingsStore.Get()
			if s.Provider.APIKey != "" && s.Provider.Provider != "" {
				providersManager.SetUserKey(s.Provider.Provider, s.Provider.APIKey)
			}
		}

		// Get all providers with availability
		providers := providersManager.GetAvailableProviders()
		log.Printf("get_models: Got %d providers", len(providers))

		sendMessage(protocol.RPCMessage{
			ID:   msg.ID,
			Type: "response",
			Payload: protocol.EncodeRPC(map[string]interface{}{
				"providers": providers,
			}),
		})

	case "get_settings":
		if settingsStore == nil {
			sendMessage(Response{ID: msg.ID, Error: "settings store not initialized"})
			return
		}
		s := settingsStore.Get()
		settings := map[string]interface{}{
			"provider":       s.Provider.Provider,
			"model":          s.Provider.Model,
			"apiKeys":        s.Provider.APIKeys,
			"telegramToken":  s.LiveMode.TelegramToken,
			"telegramChatId": s.LiveMode.TelegramChatID,
			"context":        s.Context,
			"auto_approval":  s.AutoApproval,
			"theme":          s.Theme,
		}
		sendMessage(Response{ID: msg.ID, Type: "settings_loaded", Payload: settings})

	case "save_settings":
		var payload struct {
			APIKeys        map[string]string            `json:"apiKeys"`
			Provider       string                       `json:"provider"`
			Model          string                       `json:"model"`
			TelegramChatID int64                        `json:"telegramChatId"`
			TelegramToken  string                       `json:"telegramToken"`
			Context        *config.ContextSettings      `json:"context,omitempty"`
			AutoApproval   *config.AutoApprovalSettings `json:"auto_approval,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
			return
		}

		// Properly disable old controllers before clearing them
		if liveModeController != nil {
			log.Println("Disabling old Live Mode controller...")
			liveModeController.Disable(globalCtx)
			liveModeController = nil
		}
		agentController = nil

		log.Printf("Updating settings: provider=%s, model=%s", payload.Provider, payload.Model)

		if settingsStore != nil {
			err := settingsStore.Update(func(s *config.Settings) {
				if len(payload.APIKeys) > 0 {
					if s.Provider.APIKeys == nil {
						s.Provider.APIKeys = make(map[string]string)
					}
					for providerID, key := range payload.APIKeys {
						if key != "" {
							s.Provider.APIKeys[providerID] = key
							if providersManager != nil {
								providersManager.SetUserKey(providerID, key)
							}
						}
					}
					if activeKey, ok := s.Provider.APIKeys[payload.Provider]; ok {
						cfg.Provider.APIKey = activeKey
					}
				}
				if payload.Provider != "" {
					s.Provider.Provider = payload.Provider
					cfg.Provider.Provider = payload.Provider
				}
				if payload.Model != "" {
					s.Provider.Model = payload.Model
					cfg.Provider.Model = payload.Model
				}
				if payload.TelegramToken != "" {
					s.LiveMode.TelegramToken = payload.TelegramToken
					liveModeConfig.TelegramToken = payload.TelegramToken
				}
				if payload.TelegramChatID != 0 {
					s.LiveMode.TelegramChatID = payload.TelegramChatID
					liveModeConfig.TelegramChatID = payload.TelegramChatID
				}
				if payload.Context != nil {
					s.Context = *payload.Context
					cfg.EnableCodeIndex = s.Context.EnableCodeIndex
					if s.Context.SlidingWindowSize > 0 {
						// This is for sliding window pruning, but usually we use tokens.
						// If user specified a limit in tokens elsewhere we should use it.
					}
				}
				if payload.AutoApproval != nil {
					s.AutoApproval = *payload.AutoApproval
				}
				s.LiveMode.Enabled = s.LiveMode.TelegramToken != ""
			})
			if err != nil {
				log.Printf("Failed to save settings: %v", err)
			}
		}

		// Update runtime config with active provider's key
		if len(payload.APIKeys) > 0 {
			if key, ok := payload.APIKeys[payload.Provider]; ok {
				cfg.Provider.APIKey = key
			}
		}
		if payload.Provider != "" {
			cfg.Provider.Provider = payload.Provider
		}
		if payload.Model != "" {
			cfg.Provider.Model = payload.Model
		}

		// Update Live Mode config
		if payload.TelegramToken != "" {
			liveModeConfig.TelegramToken = payload.TelegramToken
		}
		if payload.TelegramChatID != 0 {
			liveModeConfig.TelegramChatID = payload.TelegramChatID
			// Don't restrict by user/chat ID - bot is already protected by token
			liveModeConfig.AllowedUserIDs = []int64{}
		}

		// Reinitialize agent controller with new config
		var err error
		agentController, err = agent.NewController(cfg, agent.ControllerOptions{
			ProvidersManager: providersManager,
		})
		if err != nil {
			sendMessage(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
			return
		}

		sendMessage(protocol.RPCMessage{
			ID:   msg.ID,
			Type: "settings_saved",
			Payload: protocol.EncodeRPC(map[string]interface{}{
				"success":           true,
				"liveModeAvailable": liveModeConfig.TelegramToken != "",
			}),
		})

	case "set_live_mode":
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		log.Printf("Live mode set to: %v", payload.Enabled)
		initMu.Lock()

		// Initialize Live Mode controller if not done (allow without token for demo mode)
		if liveModeController == nil {
			var err error
			liveModeController, err = livemode.New(liveModeConfig, agentController)
			if err != nil {
				log.Printf("Failed to create Live Mode controller: %v", err)
				sendMessage(Response{
					ID:   msg.ID,
					Type: "live_mode_status",
					Payload: map[string]interface{}{
						"enabled":      false,
						"error":        err.Error(),
						"connectedVia": nil,
					},
				})
				return
			}

			// Set up activity callback to forward events to extension
			liveModeController.SetOnActivity(func(activity livemode.EtherActivity) {
				sendMessage(Response{
					Type: "ether_activity",
					Payload: map[string]interface{}{
						"stage":    activity.Stage,
						"source":   activity.Source,
						"username": activity.Username,
						"preview":  activity.Preview,
					},
				})
			})

			// Set up chat update callback to forward messages to extension
			liveModeController.SetOnChatUpdate(func(update agent.ChatUpdate) {
				sendMessage(Response{
					Type:    "chat_update",
					Payload: update,
				})
			})

			if agentController != nil {
				agentController.SetLiveMode(liveModeController)
			}
		}
		initMu.Unlock()

		if liveModeController == nil {
			sendMessage(Response{
				ID:   msg.ID,
				Type: "live_mode_status",
				Payload: map[string]interface{}{
					"enabled":      false,
					"error":        "Telegram not configured. Please add your Telegram token in Settings.",
					"connectedVia": nil,
				},
			})
			return
		}

		// Toggle live mode
		var status *livemode.Status
		var err error
		if payload.Enabled {
			status, err = liveModeController.Enable(globalCtx)
		} else {
			status, err = liveModeController.Disable(globalCtx)
		}

		if err != nil {
			log.Printf("Live mode error: %v", err)
			sendMessage(Response{
				ID:   msg.ID,
				Type: "live_mode_status",
				Payload: map[string]interface{}{
					"enabled":      false,
					"error":        err.Error(),
					"connectedVia": nil,
				},
			})
			return
		}

		sendMessage(Response{
			ID:      msg.ID,
			Type:    "live_mode_status",
			Payload: status,
		})

	case "audio_start":
		audioMu.Lock()
		audioBuffer = nil
		audioMu.Unlock()
		log.Println("Voice recording started")

	case "audio_chunk":
		var payload struct {
			Data   string `json:"data"`
			Format string `json:"format"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		data, err := protocol.DecodeBase64(payload.Data)
		if err != nil {
			log.Printf("Failed to decode audio chunk: %v", err)
			return
		}
		audioMu.Lock()
		audioBuffer = append(audioBuffer, data...)
		audioMu.Unlock()

	case "audio_stop":
		log.Println("Voice recording stopped, transcribing...")
		audioMu.Lock()
		buffer := audioBuffer
		audioBuffer = nil
		audioMu.Unlock()

		if len(buffer) == 0 {
			return
		}

		go func() {
			// Save to temp file
			tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("ricochet_voice_%d.webm", os.Getpid()))
			if err := os.WriteFile(tmpFile, buffer, 0644); err != nil {
				log.Printf("Failed to save voice temp file: %v", err)
				return
			}
			defer os.Remove(tmpFile)

			// Transcribe (Try OpenAI API if local transcriber is not available)
			var text string
			var err error

			if transcriber != nil {
				text, err = transcriber.Transcribe(tmpFile)
			} else if os.Getenv("OPENAI_API_KEY") != "" {
				t := whisper.NewOpenAICloudTranscriber(os.Getenv("OPENAI_API_KEY"))
				text, err = t.Transcribe(tmpFile)
			} else {
				sendMessage(Response{
					Type: "show_message",
					Payload: map[string]string{
						"level": "warning",
						"text":  "Voice transcription failed: No transcriber configured (local or OpenAI).",
					},
				})
				return
			}

			if err != nil {
				log.Printf("Transcription error: %v", err)
				return
			}

			log.Printf("Transcribed: %s", text)

			// Send to agent as if it was a chat message
			if text != "" && agentController != nil {
				// Inject [Voice] prefix so the agent knows it was a voice command
				agentController.Chat(globalCtx, agent.ChatRequestInput{
					SessionID: "default",
					Content:   "[Voice]: " + text,
				}, func(update agent.ChatUpdate) {
					sendMessage(protocol.RPCMessage{
						Type: "chat_update",
						Payload: protocol.EncodeRPC(map[string]interface{}{
							"message": update.Message,
						}),
					})
				})
			}
		}()

	case "clear_chat":
		log.Println("Clearing chat")
		if agentController != nil {
			var payload struct {
				SessionID string `json:"session_id"`
			}
			json.Unmarshal(msg.Payload, &payload)
			sid := payload.SessionID
			if sid == "" {
				sid = "default"
			}
			agentController.ClearSession(sid)
		}
		sendMessage(Response{ID: msg.ID, Type: "chat_cleared"})

	case "checkpoint_init":
		// Get workspace and storage dirs
		cwd, _ := os.Getwd()
		homeDir, _ := os.UserHomeDir()
		storageDir := filepath.Join(homeDir, ".ricochet")

		// Create checkpoint service with task ID from payload (or default)
		var payload struct {
			TaskID string `json:"taskId"`
		}
		json.Unmarshal(msg.Payload, &payload)
		if payload.TaskID == "" {
			payload.TaskID = "default"
		}

		checkpointService = checkpoints.NewCheckpointService(payload.TaskID, cwd, storageDir)
		if err := checkpointService.Init(); err != nil {
			sendMessage(Response{ID: msg.ID, Error: fmt.Sprintf("checkpoint init failed: %v", err)})
			return
		}

		sendMessage(Response{
			ID:   msg.ID,
			Type: "checkpoint_initialized",
			Payload: map[string]interface{}{
				"baseHash":  checkpointService.BaseHash(),
				"workspace": cwd,
			},
		})

	case "checkpoint_save":
		if checkpointService == nil {
			sendMessage(Response{ID: msg.ID, Error: "checkpoint service not initialized"})
			return
		}

		var payload struct {
			Message string `json:"message"`
		}
		json.Unmarshal(msg.Payload, &payload)

		hash, err := checkpointService.Save(payload.Message)
		if err != nil {
			sendMessage(Response{ID: msg.ID, Error: fmt.Sprintf("checkpoint save failed: %v", err)})
			return
		}

		sendMessage(Response{
			ID:   msg.ID,
			Type: "checkpoint_saved",
			Payload: map[string]interface{}{
				"hash":    hash,
				"message": payload.Message,
			},
		})

	case "checkpoint_restore":
		if checkpointService == nil {
			sendMessage(Response{ID: msg.ID, Error: "checkpoint service not initialized"})
			return
		}

		var payload struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		if err := checkpointService.Restore(payload.Hash); err != nil {
			sendMessage(Response{ID: msg.ID, Error: fmt.Sprintf("checkpoint restore failed: %v", err)})
			return
		}

		sendMessage(Response{
			ID:   msg.ID,
			Type: "checkpoint_restored",
			Payload: map[string]interface{}{
				"hash": payload.Hash,
			},
		})

	case "checkpoint_list":
		if checkpointService == nil {
			sendMessage(Response{ID: msg.ID, Error: "checkpoint service not initialized"})
			return
		}

		sendMessage(Response{
			ID:   msg.ID,
			Type: "checkpoint_list",
			Payload: map[string]interface{}{
				"checkpoints": checkpointService.List(),
				"baseHash":    checkpointService.BaseHash(),
			},
		})

	case "get_all_settings":
		settings := settingsStore.Get()
		sendMessage(Response{
			ID:   msg.ID,
			Type: "settings",
			Payload: map[string]interface{}{
				"provider":      settings.Provider,
				"live_mode":     settings.LiveMode,
				"context":       settings.Context,
				"auto_approval": settings.AutoApproval,
				"theme":         settings.Theme,
			},
		})

	case "update_auto_approval":
		var payload struct {
			Enabled             *bool `json:"enabled,omitempty"`
			ReadFiles           *bool `json:"read_files,omitempty"`
			ReadFilesExternal   *bool `json:"read_files_external,omitempty"`
			EditFiles           *bool `json:"edit_files,omitempty"`
			EditFilesExternal   *bool `json:"edit_files_external,omitempty"`
			ExecuteSafeCommands *bool `json:"execute_safe_commands,omitempty"`
			ExecuteAllCommands  *bool `json:"execute_all_commands,omitempty"`
			UseBrowser          *bool `json:"use_browser,omitempty"`
			UseMCP              *bool `json:"use_mcp,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		if err := settingsStore.Update(func(s *config.Settings) {
			if payload.Enabled != nil {
				s.AutoApproval.Enabled = *payload.Enabled
			}
			if payload.ReadFiles != nil {
				s.AutoApproval.ReadFiles = *payload.ReadFiles
			}
			if payload.ReadFilesExternal != nil {
				s.AutoApproval.ReadFilesExternal = *payload.ReadFilesExternal
			}
			if payload.EditFiles != nil {
				s.AutoApproval.EditFiles = *payload.EditFiles
			}
			if payload.EditFilesExternal != nil {
				s.AutoApproval.EditFilesExternal = *payload.EditFilesExternal
			}
			if payload.ExecuteSafeCommands != nil {
				s.AutoApproval.ExecuteSafeCommands = *payload.ExecuteSafeCommands
			}
			if payload.ExecuteAllCommands != nil {
				s.AutoApproval.ExecuteAllCommands = *payload.ExecuteAllCommands
			}
			if payload.UseBrowser != nil {
				s.AutoApproval.UseBrowser = *payload.UseBrowser
			}
			if payload.UseMCP != nil {
				s.AutoApproval.UseMCP = *payload.UseMCP
			}
		}); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		settings := settingsStore.Get()
		sendMessage(Response{
			ID:   msg.ID,
			Type: "settings_updated",
			Payload: map[string]interface{}{
				"auto_approval": settings.AutoApproval,
			},
		})

	case "update_context_settings":
		var payload struct {
			AutoCondense         *bool `json:"auto_condense,omitempty"`
			CondenseThreshold    *int  `json:"condense_threshold,omitempty"`
			ShowContextIndicator *bool `json:"show_context_indicator,omitempty"`
			EnableCheckpoints    *bool `json:"enable_checkpoints,omitempty"`
			CheckpointOnWrites   *bool `json:"checkpoint_on_writes,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		if err := settingsStore.Update(func(s *config.Settings) {
			if payload.AutoCondense != nil {
				s.Context.AutoCondense = *payload.AutoCondense
			}
			if payload.CondenseThreshold != nil {
				s.Context.CondenseThreshold = *payload.CondenseThreshold
			}
			if payload.ShowContextIndicator != nil {
				s.Context.ShowContextIndicator = *payload.ShowContextIndicator
			}
			if payload.EnableCheckpoints != nil {
				s.Context.EnableCheckpoints = *payload.EnableCheckpoints
			}
			if payload.CheckpointOnWrites != nil {
				s.Context.CheckpointOnWrites = *payload.CheckpointOnWrites
			}
		}); err != nil {
			sendMessage(Response{ID: msg.ID, Error: err.Error()})
			return
		}

		settings := settingsStore.Get()
		sendMessage(Response{
			ID:   msg.ID,
			Type: "settings_updated",
			Payload: map[string]interface{}{
				"context": settings.Context,
			},
		})

	default:
		sendMessage(Response{ID: msg.ID, Error: fmt.Sprintf("unknown message type: %s", msg.Type)})
	}
}

// sendMessage writes any JSON serializable value to stdout (thread-safe)
func sendMessage(v interface{}) {
	outputMu.Lock()
	defer outputMu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}
	log.Printf("SENDING RESPONSE: %s", string(data))
	fmt.Println(string(data))
	os.Stdout.Sync()
}
