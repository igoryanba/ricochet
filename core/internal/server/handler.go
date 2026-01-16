package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/checkpoints"
	"github.com/igoryan-dao/ricochet/internal/codegraph"
	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/host"
	"github.com/igoryan-dao/ricochet/internal/livemode"
	"github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/modes"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/whisper"
	"github.com/igoryan-dao/ricochet/internal/workflow"
)

// ResponseWriter interface allows different transports (Stdio, WS) to send responses
type ResponseWriter interface {
	Send(msg interface{}) error
}

// Handler manages the application state and processes RPC messages
type Handler struct {
	Agent          *agent.Controller
	LiveMode       *livemode.Controller
	Checkpoint     *checkpoints.CheckpointService
	Providers      *config.ProvidersManager
	Config         *agent.Config
	LiveModeConfig *livemode.Config
	Settings       *config.Store
	Host           host.Host // StdioHost or other
	Modes          *modes.Manager
	McpHub         *mcp.Hub
	Codegraph      *codegraph.Service
	Workflows      *workflow.Manager
	Transcriber    *whisper.Transcriber
	AudioBuffer    []byte
	AudioMu        sync.Mutex
	InitMu         sync.Mutex // Protects lazy init of Agent
	GlobalCtx      context.Context
}

// NewHandler creates a new handler with initial state
func NewHandler(
	ctx context.Context,
	cfg *agent.Config,
	liveCfg *livemode.Config,
	settings *config.Store,
	host host.Host,
	modes *modes.Manager,
	mcp *mcp.Hub,
	cg *codegraph.Service,
	wm *workflow.Manager,
	pm *config.ProvidersManager,
	liveCtrl *livemode.Controller,
) *Handler {
	return &Handler{
		GlobalCtx:      ctx,
		Config:         cfg,
		LiveModeConfig: liveCfg,
		LiveMode:       liveCtrl,
		Settings:       settings,
		Host:           host,
		Modes:          modes,
		McpHub:         mcp,
		Codegraph:      cg,
		Workflows:      wm,
		Providers:      pm,
	}
}

// HandleMessage processes a single RPC message
func (h *Handler) HandleMessage(msg protocol.RPCMessage, writer ResponseWriter) {
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

		if h.Agent != nil {
			state := h.Agent.GetState(sessionID)
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "state",
				Payload: protocol.EncodeRPC(state),
			})
		} else {
			writer.Send(protocol.RPCMessage{
				ID:   msg.ID,
				Type: "state",
				Payload: protocol.EncodeRPC(map[string]interface{}{
					"messages":        []interface{}{},
					"liveModeEnabled": false,
				}),
			})
		}

	case "list_sessions":
		if h.Agent != nil {
			sessions := h.Agent.ListSessions()
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "session_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"sessions": sessions}),
			})
		} else {
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "session_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"sessions": []interface{}{}}),
			})
		}

	case "create_session":
		if h.Agent == nil {
			if err := h.lazyInitAgent(); err != nil {
				writer.Send(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
				return
			}
		}
		session := h.Agent.CreateSession()
		writer.Send(protocol.RPCMessage{
			ID:      msg.ID,
			Type:    "session_created",
			Payload: protocol.EncodeRPC(session),
		})

	case "delete_session":
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			writer.Send(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
			return
		}
		if h.Agent != nil {
			h.Agent.DeleteSession(payload.SessionID)
		}
		writer.Send(protocol.RPCMessage{ID: msg.ID, Type: "session_deleted"})

	case "abort_chat":
		log.Printf("Received abort_chat request")
		if h.Agent != nil {
			h.Agent.AbortCurrentSession()
		}
		writer.Send(protocol.RPCMessage{ID: msg.ID, Type: "aborted", Payload: protocol.EncodeRPC(map[string]bool{"success": true})})

	case "chat_message":
		var payload struct {
			Content string `json:"content"`
		}
		// First pass to get content for logging
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("Received chat message: %s", payload.Content)

		if h.Config.Provider.APIKey == "" {
			writer.Send(protocol.RPCMessage{
				ID:    msg.ID,
				Type:  "response",
				Error: "⚠️ API key not configured. Please go to Settings and add your API key.",
			})
			return
		}

		if h.Agent == nil {
			if err := h.lazyInitAgent(); err != nil {
				writer.Send(protocol.RPCMessage{
					ID:    msg.ID,
					Type:  "response",
					Error: fmt.Sprintf("Failed to initialize AI provider: %v", err),
				})
				return
			}
		}

		var fullPayload struct {
			Content   string `json:"content"`
			SessionID string `json:"session_id"`
			Via       string `json:"via"`
		}
		if err := json.Unmarshal(msg.Payload, &fullPayload); err != nil {
			writer.Send(protocol.RPCMessage{ID: msg.ID, Error: "Invalid payload: " + err.Error()})
			return
		}

		sessionID := fullPayload.SessionID
		if sessionID == "" {
			sessionID = "default"
		}

		err := h.Agent.Chat(h.GlobalCtx, agent.ChatRequestInput{
			SessionID: sessionID,
			Content:   fullPayload.Content,
			Via:       fullPayload.Via,
		}, func(update interface{}) {
			switch u := update.(type) {
			case agent.ChatUpdate:
				writer.Send(protocol.RPCMessage{
					Type: "chat_update",
					Payload: protocol.EncodeRPC(map[string]interface{}{
						"message": u.Message,
					}),
				})
			case protocol.TaskProgress:
				writer.Send(protocol.RPCMessage{
					Type:    "task_progress",
					Payload: protocol.EncodeRPC(u),
				})
			}
		})

		if err != nil {
			log.Printf("Chat error: %v", err)
			writer.Send(protocol.RPCMessage{
				ID:    msg.ID,
				Type:  "response",
				Error: err.Error(),
			})
		} else {
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "response",
				Payload: protocol.EncodeRPC(map[string]interface{}{"status": "done"}),
			})
		}

	case "get_models":
		if h.Providers == nil {
			// Lazy init providers if needed
			configPath := config.FindConfigFile()
			pm, err := config.NewProvidersManager(configPath)
			if err != nil {
				log.Printf("get_models: Error creating ProvidersManager: %v", err)
			}
			h.Providers = pm
		}

		if h.Providers == nil {
			writer.Send(protocol.RPCMessage{
				ID:   msg.ID,
				Type: "response",
				Payload: protocol.EncodeRPC(map[string]interface{}{
					"providers": []interface{}{},
				}),
			})
			return
		}

		if h.Settings != nil {
			s := h.Settings.Get()
			if s.Provider.APIKey != "" && s.Provider.Provider != "" {
				h.Providers.SetUserKey(s.Provider.Provider, s.Provider.APIKey)
			}
		}

		providers := h.Providers.GetAvailableProviders()
		writer.Send(protocol.RPCMessage{
			ID:   msg.ID,
			Type: "response",
			Payload: protocol.EncodeRPC(map[string]interface{}{
				"providers": providers,
			}),
		})

	case "get_workflows":
		if h.Workflows != nil {
			// Reload to ensure fresh data
			h.Workflows.LoadWorkflows()
			wfs := h.Workflows.GetWorkflows()
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "workflows_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"workflows": wfs}),
			})
		} else {
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "workflows_list",
				Payload: protocol.EncodeRPC(map[string]interface{}{"workflows": []interface{}{}}),
			})
		}

	case "get_settings":
		if h.Settings == nil {
			writer.Send(protocol.RPCMessage{ID: msg.ID, Error: "settings store not initialized"})
			return
		}
		s := h.Settings.Get()
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
		writer.Send(protocol.RPCMessage{ID: msg.ID, Type: "settings_loaded", Payload: protocol.EncodeRPC(settings)})

	case "save_settings":
		h.handleSaveSettings(msg, writer)

	case "set_live_mode":
		h.handleSetLiveMode(msg, writer)

	case "get_live_mode_status":
		if h.LiveMode != nil {
			status := h.LiveMode.GetStatus()
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "live_mode_status",
				Payload: protocol.EncodeRPC(status),
			})
		} else {
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "live_mode_status",
				Payload: protocol.EncodeRPC(map[string]interface{}{"enabled": false, "error": "Live Mode not initialized"}),
			})
		}
	}
}

func (h *Handler) lazyInitAgent() error {
	h.InitMu.Lock()
	defer h.InitMu.Unlock()

	if h.Agent != nil {
		return nil
	}

	if h.Settings != nil {
		s := h.Settings.Get()
		h.Config.AutoApproval = &s.AutoApproval
	}

	log.Printf("Initializing agent controller with provider %s (%s)", h.Config.Provider.Provider, h.Config.Provider.Model)
	var err error
	h.Agent, err = agent.NewController(h.Config, agent.ControllerOptions{
		Host:             h.Host,
		Modes:            h.Modes,
		McpHub:           h.McpHub,
		ProvidersManager: h.Providers,
		Codegraph:        h.Codegraph,
		WorkflowManager:  h.Workflows,
	})
	if err != nil {
		return err
	}
	if h.LiveMode != nil {
		h.Agent.SetLiveMode(h.LiveMode)
		h.LiveMode.SetAgent(h.Agent)
	}
	return nil
}

func (h *Handler) handleSaveSettings(msg protocol.RPCMessage, writer ResponseWriter) {
	var payload struct {
		APIKeys           map[string]string            `json:"apiKeys"`
		Provider          string                       `json:"provider"`
		Model             string                       `json:"model"`
		EmbeddingProvider string                       `json:"embeddingProvider"`
		EmbeddingModel    string                       `json:"embeddingModel"`
		TelegramChatID    int64                        `json:"telegramChatId"`
		TelegramToken     string                       `json:"telegramToken"`
		Context           *config.ContextSettings      `json:"context,omitempty"`
		AutoApproval      *config.AutoApprovalSettings `json:"auto_approval,omitempty"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		writer.Send(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
		return
	}

	// Do not destroy h.LiveMode here. It is running in the background (managed by main.go/Handler).
	// We will re-link the new Agent to it in lazyInitAgent.

	// Update LiveMode ChatID if changed
	if h.LiveMode != nil && payload.TelegramChatID != 0 {
		h.LiveMode.SetChatID(payload.TelegramChatID)
	}

	h.Agent = nil // Reset agent to re-init with new config

	if h.Settings != nil {
		h.Settings.Update(func(s *config.Settings) {
			if len(payload.APIKeys) > 0 {
				if s.Provider.APIKeys == nil {
					s.Provider.APIKeys = make(map[string]string)
				}
				for k, v := range payload.APIKeys {
					if v != "" {
						s.Provider.APIKeys[k] = v
						if h.Providers != nil {
							h.Providers.SetUserKey(k, v)
						}
					}
				}
				if activeKey, ok := s.Provider.APIKeys[payload.Provider]; ok {
					h.Config.Provider.APIKey = activeKey
				}
			}
			if payload.Provider != "" {
				s.Provider.Provider = payload.Provider
				h.Config.Provider.Provider = payload.Provider
			}
			if payload.Model != "" {
				s.Provider.Model = payload.Model
				h.Config.Provider.Model = payload.Model
			}
			if payload.EmbeddingProvider != "" {
				s.Provider.EmbeddingProvider = payload.EmbeddingProvider
			}
			if payload.EmbeddingModel != "" {
				s.Provider.EmbeddingModel = payload.EmbeddingModel
			}
			if payload.TelegramToken != "" {
				s.LiveMode.TelegramToken = payload.TelegramToken
				h.LiveModeConfig.TelegramToken = payload.TelegramToken
			}
			if payload.Context != nil {
				s.Context = *payload.Context
				h.Config.EnableCodeIndex = s.Context.EnableCodeIndex
			}
			if payload.AutoApproval != nil {
				s.AutoApproval = *payload.AutoApproval
			}
			s.LiveMode.Enabled = s.LiveMode.TelegramToken != ""
		})
	}

	// Updating runtime config logic (abbreviated, similar to main.go)
	if payload.Provider != "" {
		h.Config.Provider.Provider = payload.Provider
	}
	if payload.Model != "" {
		h.Config.Provider.Model = payload.Model
	}
	if payload.EmbeddingProvider != "" {
		// embedding config logic
		s := h.Settings.Get()
		embKey := s.Provider.APIKeys[payload.EmbeddingProvider]
		if embKey == "" && s.Provider.Provider == payload.EmbeddingProvider {
			embKey = s.Provider.APIKey
		}
		h.Config.EmbeddingProvider = &agent.ProviderConfig{
			Provider: payload.EmbeddingProvider,
			Model:    payload.EmbeddingModel,
			APIKey:   embKey,
		}
	}

	// Re-init agent
	if err := h.lazyInitAgent(); err != nil {
		writer.Send(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
		return
	}

	writer.Send(protocol.RPCMessage{
		ID:   msg.ID,
		Type: "settings_saved",
		Payload: protocol.EncodeRPC(map[string]interface{}{
			"success":           true,
			"liveModeAvailable": h.LiveModeConfig.TelegramToken != "",
		}),
	})
}

func (h *Handler) handleSetLiveMode(msg protocol.RPCMessage, writer ResponseWriter) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		writer.Send(protocol.RPCMessage{ID: msg.ID, Error: err.Error()})
		return
	}

	h.InitMu.Lock()
	if h.LiveMode == nil {
		// Should have been initialized in main.go, but if not (e.g. no token on startup but added later??)
		// We can try to init here, but it won't have callbacks wired unless we wire them.
		// For now, assume main.go handles it if token is present.
		// If token was added via settings save, we might need re-init.
		var err error
		h.LiveMode, err = livemode.New(h.LiveModeConfig, h.Agent)
		if err != nil {
			h.InitMu.Unlock()
			writer.Send(protocol.RPCMessage{
				ID:      msg.ID,
				Type:    "live_mode_status",
				Payload: protocol.EncodeRPC(map[string]interface{}{"enabled": false, "error": err.Error()}),
			})
			return
		}

		// Note: Callbacks might be missing if created here!
		// TODO: Ensure save_settings re-wires LiveMode properly.
	}

	if h.Agent != nil && h.LiveMode != nil {
		h.Agent.SetLiveMode(h.LiveMode)
		h.LiveMode.SetAgent(h.Agent)
	}
	h.InitMu.Unlock()

	// Toggle
	var status *livemode.Status
	var err error
	if payload.Enabled {
		status, err = h.LiveMode.Enable(h.GlobalCtx)
	} else {
		status, err = h.LiveMode.Disable(h.GlobalCtx)
	}

	if err != nil {
		writer.Send(protocol.RPCMessage{
			ID:      msg.ID,
			Type:    "live_mode_status",
			Payload: protocol.EncodeRPC(map[string]interface{}{"enabled": false, "error": err.Error()}),
		})
		return
	}

	writer.Send(protocol.RPCMessage{
		ID:      msg.ID,
		Type:    "live_mode_status",
		Payload: protocol.EncodeRPC(status),
	})
}
