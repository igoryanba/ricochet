package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	"github.com/igoryan-dao/ricochet/internal/bridge"
	"github.com/igoryan-dao/ricochet/internal/bridge/proto"
	"github.com/igoryan-dao/ricochet/internal/discord"
	"github.com/igoryan-dao/ricochet/internal/sessions"
	"github.com/igoryan-dao/ricochet/internal/state"
	"github.com/igoryan-dao/ricochet/internal/telegram"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// getArgs extracts arguments from request as map[string]any
func getArgs(request mcp.CallToolRequest) map[string]any {
	if args, ok := request.Params.Arguments.(map[string]any); ok {
		return args
	}
	return make(map[string]any)
}

// Server wraps MCP server with Telegram bridge
type Server struct {
	mcpServer    *server.MCPServer
	tgBot        *telegram.Bot
	discordBot   *discord.Bot
	bridgeClient *bridge.Client
	state        *state.Manager
	sessionsMgr  *sessions.Manager
	chatID       int64 // Primary chat for notifications
}

// NewServer creates a new MCP server with Telegram tools
func NewServer(tgBot *telegram.Bot, discordBot *discord.Bot, stateMgr *state.Manager) *Server {
	s := &Server{
		tgBot:       tgBot,
		discordBot:  discordBot,
		state:       stateMgr,
		sessionsMgr: sessions.NewManager(),
		chatID:      stateMgr.GetPrimaryChatID(),
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"ricochet",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	// Register components
	s.registerTools(mcpServer)
	s.registerResources(mcpServer)
	s.registerPrompts(mcpServer)

	s.mcpServer = mcpServer
	return s
}

// SetBridge registers the Cloud Bridge client
func (s *Server) SetBridge(c *bridge.Client) {
	s.bridgeClient = c
	go s.listenBridge()
}

func (s *Server) listenBridge() {
	log.Println("Listening for events from Cloud Bridge...")
	for event := range s.bridgeClient.Incoming() {
		if msg := event.GetIncomingMessage(); msg != nil {
			log.Printf("Bridge: Received message from %s: %s", msg.Platform, msg.Body)
			// Route to session
			if event.SessionId != "" {
				s.tgBot.SendToSession(event.SessionId, msg.Body)
			}
		}
	}
}

// registerTools adds all MCP tools
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	// Tool: notify - Send a notification to the user
	notifyTool := mcp.NewTool("notify",
		mcp.WithDescription("Send a notification message to the user via Telegram"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("The message to send"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID to link this notification to"),
		),
	)
	mcpServer.AddTool(notifyTool, s.handleNotify)

	// Tool: ask - Ask a question and wait for response
	askTool := mcp.NewTool("ask",
		mcp.WithDescription("Ask the user a question via Telegram and wait for their response"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to ask"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID to route the response back to this specific agent"),
		),
	)
	mcpServer.AddTool(askTool, s.handleAsk)

	// Tool: confirm_dangerous - Confirm a potentially dangerous command
	confirmTool := mcp.NewTool("confirm_dangerous",
		mcp.WithDescription("Ask user to confirm a potentially dangerous command before execution"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The command that needs confirmation"),
		),
		mcp.WithString("reason",
			mcp.Description("Why this command might be dangerous"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(confirmTool, s.handleConfirmDangerous)

	// Tool: set_chat - Set the primary chat ID for notifications
	setChatTool := mcp.NewTool("set_chat",
		mcp.WithDescription("Set the primary Telegram chat ID for receiving notifications"),
		mcp.WithNumber("chat_id",
			mcp.Required(),
			mcp.Description("The Telegram chat ID"),
		),
	)
	mcpServer.AddTool(setChatTool, s.handleSetChat)

	// Tool: get_unread_messages - Get unread messages for a session
	getUnreadTool := mcp.NewTool("get_unread_messages",
		mcp.WithDescription("Get unread messages buffered for a specific session"),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session UUID to check"),
		),
	)
	mcpServer.AddTool(getUnreadTool, s.handleGetUnreadMessages)

	// Tool: heartbeat - Signal that session is active
	heartbeatTool := mcp.NewTool("heartbeat",
		mcp.WithDescription("Signal that this session/agent is active and ready to receive commands"),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session UUID"),
		),
	)
	mcpServer.AddTool(heartbeatTool, s.handleHeartbeat)

	// Tool: wait_for_command - Wait for next command from Telegram
	waitTool := mcp.NewTool("wait_for_command",
		mcp.WithDescription("Block execution and wait for the next command/message from the active Telegram session. Use this to enter Remote Standby mode."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session UUID to wait on"),
		),
		mcp.WithNumber("timeout_minutes",
			mcp.Description("Optional timeout in minutes (default 30)"),
		),
	)
	mcpServer.AddTool(waitTool, s.handleWaitForCommand)

	// Tool: exec_command - Run a system command (Headless Mode)
	execTool := mcp.NewTool("exec_command",
		mcp.WithDescription("Execute a system command or launch an AI agent (e.g. 'npm test' or 'claude -p ...'). Useful for Headless mode."),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The full command to execute"),
		),
		mcp.WithBoolean("background",
			mcp.Description("Whether to run the command in background and notify later"),
		),
	)
	mcpServer.AddTool(execTool, s.handleExecCommand)

	// Tool: file_changed - Notify about file modifications (Rich UX)
	fileChangedTool := mcp.NewTool("file_changed",
		mcp.WithDescription("Send a structured notification about file changes to Telegram. Use this to show users what files were modified."),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the modified file"),
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action: 'created', 'modified', 'deleted'"),
		),
		mcp.WithString("summary",
			mcp.Description("Brief description of changes"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(fileChangedTool, s.handleFileChanged)

	// Tool: update_progress - Send progress stage updates (Rich UX)
	progressTool := mcp.NewTool("update_progress",
		mcp.WithDescription("Send a progress update to Telegram. Use to show current work stage."),
		mcp.WithString("stage",
			mcp.Required(),
			mcp.Description("Stage: 'planning', 'execution', 'verification', 'completed'"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("What you are currently doing"),
		),
		mcp.WithNumber("progress_percent",
			mcp.Description("Optional progress percentage (0-100)"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(progressTool, s.handleUpdateProgress)

	// Tool: send_summary - Send a brief summary to Telegram (Rich UX)
	summaryTool := mcp.NewTool("send_summary",
		mcp.WithDescription("Send a brief summary of completed work to Telegram. Use instead of full response for mobile-friendly updates."),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Summary title"),
		),
		mcp.WithString("bullet_points",
			mcp.Required(),
			mcp.Description("Comma-separated list of key points"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(summaryTool, s.handleSendSummary)

	// Tool: send_image - Send an image/screenshot to Telegram
	sendImageTool := mcp.NewTool("send_image",
		mcp.WithDescription("Send an image or screenshot to Telegram. Useful for showing UI changes or visual results."),
		mcp.WithString("image_path",
			mcp.Required(),
			mcp.Description("Absolute path to the image file"),
		),
		mcp.WithString("caption",
			mcp.Description("Optional caption for the image"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(sendImageTool, s.handleSendImage)

	// Tool: send_code_block - Send a formatted code block
	sendCodeTool := mcp.NewTool("send_code_block",
		mcp.WithDescription("Send a nicely formatted code block to Telegram with syntax highlighting."),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The code to send"),
		),
		mcp.WithString("language",
			mcp.Description("Programming language for syntax highlighting (e.g., 'go', 'python', 'javascript')"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID"),
		),
	)
	mcpServer.AddTool(sendCodeTool, s.handleSendCodeBlock)

	// Tool: browser_search - Perform a web search
	searchTool := mcp.NewTool("browser_search",
		mcp.WithDescription("Perform a web search and get top results from DuckDuckGo"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
	)
	mcpServer.AddTool(searchTool, s.handleBrowserSearch)

	// Tool: voice_reply - Send a voice message (TTS)
	voiceReplyTool := mcp.NewTool("voice_reply",
		mcp.WithDescription("Send a voice message (TTS) to the user. Useful for answering in a friendly way or providing updates."),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Text to convert to speech"),
		),
		mcp.WithString("session_id",
			mcp.Description("Optional session UUID to route to"),
		),
	)
	mcpServer.AddTool(voiceReplyTool, s.handleVoiceReply)
}

// registerResources adds MCP resources
func (s *Server) registerResources(mcpServer *server.MCPServer) {
	// Resource: ricochet://messages - Current unread messages
	messagesRes := mcp.NewResource("ricochet://messages", "List of unread messages from Telegram")
	mcpServer.AddResource(messagesRes, s.handleReadMessages)
}

// registerPrompts adds MCP prompts
func (s *Server) registerPrompts(mcpServer *server.MCPServer) {
	// Prompt: ricochet/instructions - Instructions for the agent
	instrPrompt := mcp.NewPrompt("ricochet/instructions",
		mcp.WithPromptDescription("Get instructions on how to use Ricochet for remote control"),
	)
	mcpServer.AddPrompt(instrPrompt, s.handleGetInstructions)
}

// updateHeartbeat updates session last seen if session_id is present
func (s *Server) updateHeartbeat(args map[string]any) {
	if sessionID, ok := args["session_id"].(string); ok && sessionID != "" && s.state != nil {
		s.state.UpdateHeartbeat(sessionID)
	}
}

// handleNotify sends a notification to the user
func (s *Server) handleNotify(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	message, ok := args["message"].(string)
	if !ok {
		return mcp.NewToolResultError("message parameter is required"), nil
	}

	sessionID, _ := args["session_id"].(string)

	text := fmt.Sprintf("📢 %s", message)

	// Buttons for activation
	var buttons [][]telegram.ButtonConfig
	if sessionID != "" {
		buttons = append(buttons, []telegram.ButtonConfig{
			{Text: "📍 Активировать этот чат", Data: "activate:" + sessionID},
		})
	}

	if err := s.sendMessage(ctx, sessionID, text, buttons); err != nil {
		log.Printf("Failed to send notification: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to send: %v", err)), nil
	}

	return mcp.NewToolResultText("Notification sent successfully"), nil
}

// sendMessage sends a message to the user, routing through bridge if available
func (s *Server) sendMessage(ctx context.Context, sessionID, text string, buttons [][]telegram.ButtonConfig) error {
	if s.bridgeClient != nil {
		return s.bridgeClient.Send(&proto.BridgeEvent{
			SessionId: sessionID,
			Payload: &proto.BridgeEvent_OutgoingMessage{
				OutgoingMessage: &proto.OutgoingMessage{
					ChatId:   s.chatID,
					Body:     text,
					Platform: "telegram",
				},
			},
		})
	}

	if sessionID != "" {
		if s.discordBot != nil {
			activeDiscord := s.state.GetDiscordActiveSessions()
			for channelID, sessID := range activeDiscord {
				if sessID == sessionID {
					s.discordBot.SendTyping(ctx, channelID)
					return s.discordBot.SendMessage(ctx, channelID, text)
				}
			}
		}
		s.tgBot.SendTyping(ctx, s.chatID)
		if len(buttons) > 0 {
			return s.tgBot.SendMessageWithButtons(ctx, s.chatID, text, buttons)
		}
		return s.tgBot.SendMessage(ctx, s.chatID, text)
	}

	if s.chatID == 0 {
		return fmt.Errorf("chat_id not set")
	}
	s.tgBot.SendTyping(ctx, s.chatID)
	return s.tgBot.SendMessage(ctx, s.chatID, text)
}

// resolveChannel finds where the session is active and returns bot and channelID
func (s *Server) resolveChannel(sessionID string) (tg *telegram.Bot, dg *discord.Bot, tgChatID int64, dgChannelID string) {
	if sessionID != "" {
		// Try Discord
		if s.discordBot != nil {
			activeDiscord := s.state.GetDiscordActiveSessions()
			for channelID, sessID := range activeDiscord {
				if sessID == sessionID {
					return nil, s.discordBot, 0, channelID
				}
			}
		}
	}
	// Default to Telegram
	return s.tgBot, nil, s.chatID, ""
}

// handleAsk asks a question and waits for response
func (s *Server) handleAsk(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	question, ok := args["question"].(string)
	if !ok {
		return mcp.NewToolResultError("question parameter is required"), nil
	}

	sessionID, _ := args["session_id"].(string)

	if s.chatID == 0 {
		return mcp.NewToolResultError("chat_id not set. Use set_chat tool first or send a message to the bot."), nil
	}

	// If we have a sessionID, register a specific handler
	if sessionID != "" {
		// Try Discord first
		if s.discordBot != nil {
			activeDiscord := s.state.GetDiscordActiveSessions()
			for channelID, sessID := range activeDiscord {
				if sessID == sessionID {
					// Check Discord buffer
					unread := s.discordBot.GetUnreadMessages(sessionID)
					if len(unread) > 0 {
						return mcp.NewToolResultText(unread[len(unread)-1]), nil
					}

					respCh := make(chan string, 1)
					s.discordBot.RegisterSessionHandler(sessionID, respCh)
					defer s.discordBot.UnregisterSessionHandler(sessionID)

					text := fmt.Sprintf("❓ %s", question)
					s.discordBot.SendTyping(ctx, channelID)
					if err := s.discordBot.SendMessage(ctx, channelID, text); err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("failed to send to Discord: %v", err)), nil
					}

					select {
					case resp := <-respCh:
						return mcp.NewToolResultText(resp), nil
					case <-time.After(10 * time.Minute):
						return mcp.NewToolResultError("timeout waiting for Discord response"), nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
			}
		}

		// Fallback to Telegram
		unread := s.tgBot.GetUnreadMessages(sessionID)
		if len(unread) > 0 {
			return mcp.NewToolResultText(unread[len(unread)-1]), nil
		}

		respCh := make(chan string, 1)
		s.tgBot.RegisterSessionHandler(sessionID, respCh)
		defer s.tgBot.UnregisterSessionHandler(sessionID)

		// Set this session as active for the chat automatically if nothing else is active
		if s.tgBot.GetActiveSession(s.chatID) == "" {
			s.tgBot.SetActiveSession(s.chatID, sessionID)
		}

		text := fmt.Sprintf("❓ %s", question)
		s.tgBot.SendTyping(ctx, s.chatID)

		// Send question with "Activate" button if it's not the active session
		var buttons [][]telegram.ButtonConfig
		if s.tgBot.GetActiveSession(s.chatID) != sessionID {
			buttons = append(buttons, []telegram.ButtonConfig{
				{Text: "🔗 Начать отвечать здесь", Data: "activate:" + sessionID},
			})
		}

		if len(buttons) > 0 {
			s.tgBot.SendMessageWithButtons(ctx, s.chatID, text, buttons)
		} else {
			s.tgBot.SendMessage(ctx, s.chatID, text)
		}

		// Wait for response from session channel
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case response := <-respCh:
			return mcp.NewToolResultText(response), nil
		}
	}

	// Legacy fallback: generic AskUser
	response, err := s.tgBot.AskUser(ctx, s.chatID, fmt.Sprintf("❓ %s", question))
	if err != nil {
		log.Printf("Failed to ask user: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to ask: %v", err)), nil
	}

	return mcp.NewToolResultText(response), nil
}

// handleConfirmDangerous asks for confirmation of a dangerous command
func (s *Server) handleConfirmDangerous(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	command, ok := args["command"].(string)
	if !ok {
		return mcp.NewToolResultError("command parameter is required"), nil
	}

	reason, _ := args["reason"].(string)
	sessionID, _ := args["session_id"].(string)
	if reason == "" {
		reason = "This command may have destructive side effects"
	}

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	question := fmt.Sprintf("⚠️ *Dangerous Command Confirmation*\n\n```\n%s\n```\n\n%s\n\nReply 'yes' to confirm, anything else to cancel.", command, reason)

	var response string
	if dg != nil {
		dg.SendTyping(ctx, channelID)
		dg.SendMessage(ctx, channelID, question)
		respCh := make(chan string, 1)
		dg.RegisterSessionHandler(sessionID, respCh)
		defer dg.UnregisterSessionHandler(sessionID)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case response = <-respCh:
			// obtained
		case <-time.After(5 * time.Minute):
			return mcp.NewToolResultError("confirmation timed out"), nil
		}
	} else {
		tg.SendTyping(ctx, chatID)
		respCh := make(chan string, 1)
		tg.RegisterSessionHandler(sessionID, respCh)
		defer tg.UnregisterSessionHandler(sessionID)

		tg.SendMessageWithButtons(ctx, chatID, question, [][]telegram.ButtonConfig{
			{
				{Text: "✅ Confirm", Data: "confirm_yes:" + sessionID},
				{Text: "❌ Cancel", Data: "confirm_no:" + sessionID},
			},
		})

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case response = <-respCh:
			// obtained
		case <-time.After(5 * time.Minute):
			return mcp.NewToolResultError("confirmation timed out"), nil
		}
	}

	if response == "yes" || response == "Yes" || response == "YES" || response == "да" || response == "Да" || response == "confirm_yes" {
		return mcp.NewToolResultText("confirmed"), nil
	}

	return mcp.NewToolResultText("cancelled"), nil
}

// handleSetChat sets the primary chat ID
func (s *Server) handleSetChat(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	chatIDFloat, ok := args["chat_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("chat_id parameter is required"), nil
	}

	s.chatID = int64(chatIDFloat)
	log.Printf("Chat ID set to: %d", s.chatID)

	// Persist to state
	if s.state != nil {
		if err := s.state.SetPrimaryChatID(s.chatID); err != nil {
			log.Printf("Failed to save primary chat ID: %v", err)
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("Chat ID set to %d", s.chatID)), nil
}

// handleGetUnreadMessages returns buffered messages for a session
func (s *Server) handleGetUnreadMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewToolResultError("session_id parameter is required"), nil
	}

	var messages []string
	if s.discordBot != nil {
		messages = append(messages, s.discordBot.GetUnreadMessages(sessionID)...)
	}
	messages = append(messages, s.tgBot.GetUnreadMessages(sessionID)...)

	if len(messages) == 0 {
		return mcp.NewToolResultText("No unread messages"), nil
	}

	return mcp.NewToolResultText(strings.Join(messages, "\n")), nil
}

// handleHeartbeat updates session last seen time
func (s *Server) handleHeartbeat(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewToolResultError("session_id parameter is required"), nil
	}

	if s.state != nil {
		if err := s.state.UpdateHeartbeat(sessionID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to update heartbeat: %v", err)), nil
		}
	}

	return mcp.NewToolResultText("Heartbeat received"), nil
}

// handleWaitForCommand blocks until a message is received from Telegram
func (s *Server) handleWaitForCommand(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewToolResultError("session_id parameter is required"), nil
	}

	timeoutMinutes := 30.0
	if t, ok := args["timeout_minutes"].(float64); ok {
		timeoutMinutes = t
	}

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	// 1. Check for unread messages first
	var unread []string
	if dg != nil {
		unread = dg.GetUnreadMessages(sessionID)
	} else {
		unread = tg.GetUnreadMessages(sessionID)
	}

	if len(unread) > 0 {
		return mcp.NewToolResultText(strings.Join(unread, "\n")), nil
	}

	// 2. Register handler and wait
	respCh := make(chan string, 1)
	if dg != nil {
		dg.RegisterSessionHandler(sessionID, respCh)
		defer dg.UnregisterSessionHandler(sessionID)

		// Auto-activate for Discord if not active
		if dg.GetActiveSession(channelID) == "" {
			dg.SetActiveSession(channelID, sessionID)
		}

		dg.SendMessage(ctx, channelID, "💤 **Agent in standby.** Send next command when ready.")
	} else {
		tg.RegisterSessionHandler(sessionID, respCh)
		defer tg.UnregisterSessionHandler(sessionID)

		// Auto-activate for Telegram if not active
		if tg.GetActiveSession(chatID) == "" {
			tg.SetActiveSession(chatID, sessionID)
		}

		tg.SendMessage(ctx, chatID, "💤 **Агент перешел в режим ожидания.** Пришлите следующую команду, когда будете готовы.")
	}

	log.Printf("Session %s entering wait mode...", sessionID)

	timeout := time.Duration(timeoutMinutes) * time.Minute
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return mcp.NewToolResultText("Wait timed out after " + fmt.Sprintf("%.0f", timeoutMinutes) + " minutes"), nil
	case resp := <-respCh:
		return mcp.NewToolResultText(resp), nil
	}
}

// handleExecCommand executes a system command
func (s *Server) handleExecCommand(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	command, ok := args["command"].(string)
	if !ok {
		return mcp.NewToolResultError("command parameter is required"), nil
	}

	background, _ := args["background"].(bool)
	sessionID, _ := args["session_id"].(string)

	if s.chatID == 0 {
		return mcp.NewToolResultError("chat_id not set"), nil
	}

	runCmd := func() {
		log.Printf("Executing command: %s", command)
		// Basic shell execution
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()

		status := "✅ Success"
		if err != nil {
			status = fmt.Sprintf("❌ Error: %v", err)
		}

		resultText := fmt.Sprintf("💻 **Command Execution Result**\n\n`%s`\n\n**Status:** %s\n\n**Output:**\n```\n%s\n```",
			command, status, string(output))

		// If result is too long, truncate it for Telegram
		if len(resultText) > 4000 {
			resultText = resultText[:3900] + "\n... (truncated)"
		}

		if sessionID != "" {
			s.tgBot.SendToSession(sessionID, resultText)
		} else {
			s.tgBot.SendMessage(context.Background(), s.chatID, resultText)
		}
	}

	if background {
		go runCmd()
		return mcp.NewToolResultText("Command started in background"), nil
	}

	runCmd()
	return mcp.NewToolResultText("Command executed successfully. Check Telegram for output."), nil
}

// handleFileChanged sends structured file change notification
func (s *Server) handleFileChanged(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	filePath, ok := args["file_path"].(string)
	if !ok {
		return mcp.NewToolResultError("file_path parameter is required"), nil
	}

	action, ok := args["action"].(string)
	if !ok {
		return mcp.NewToolResultError("action parameter is required"), nil
	}

	summary, _ := args["summary"].(string)
	sessionID, _ := args["session_id"].(string)

	if s.chatID == 0 {
		return mcp.NewToolResultError("chat_id not set"), nil
	}

	// Format action icon
	actionIcon := "📝"
	switch action {
	case "created":
		actionIcon = "➕"
	case "deleted":
		actionIcon = "🗑️"
	case "modified":
		actionIcon = "✏️"
	}

	text := fmt.Sprintf("%s **%s**: `%s`", actionIcon, action, filePath)
	if summary != "" {
		text += fmt.Sprintf("\n   └─ %s", summary)
	}

	if sessionID != "" {
		s.tgBot.SendToSession(sessionID, text)
	} else {
		s.tgBot.SendMessage(ctx, s.chatID, text)
	}

	return mcp.NewToolResultText("File change notification sent"), nil
}

// handleUpdateProgress sends progress stage update
func (s *Server) handleUpdateProgress(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	stage, ok := args["stage"].(string)
	if !ok {
		return mcp.NewToolResultError("stage parameter is required"), nil
	}

	description, ok := args["description"].(string)
	if !ok {
		return mcp.NewToolResultError("description parameter is required"), nil
	}

	progressPercent, _ := args["progress_percent"].(float64)
	sessionID, _ := args["session_id"].(string)

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	// Format stage icon
	stageIcon := "🔄"
	switch stage {
	case "planning":
		stageIcon = "📋"
	case "execution":
		stageIcon = "🛠️"
	case "verification":
		stageIcon = "✅"
	case "completed":
		stageIcon = "🎉"
	}

	text := fmt.Sprintf("%s **%s**: %s", stageIcon, strings.ToUpper(stage), description)
	if progressPercent > 0 {
		text += fmt.Sprintf(" (%d%%)", int(progressPercent))
	}

	if dg != nil {
		dg.SendTyping(ctx, channelID)
		dg.SendMessage(ctx, channelID, text)
	} else {
		tg.SendTyping(ctx, chatID)
		tg.SendMessage(ctx, chatID, text)
	}

	return mcp.NewToolResultText("Progress update sent"), nil
}

// handleSendSummary sends a brief summary
func (s *Server) handleSendSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	title, ok := args["title"].(string)
	if !ok {
		return mcp.NewToolResultError("title parameter is required"), nil
	}

	bulletPoints, ok := args["bullet_points"].(string)
	if !ok {
		return mcp.NewToolResultError("bullet_points parameter is required"), nil
	}

	sessionID, _ := args["session_id"].(string)

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	// Format bullet points
	points := strings.Split(bulletPoints, ",")
	var formattedPoints []string
	for _, p := range points {
		formattedPoints = append(formattedPoints, fmt.Sprintf("• %s", strings.TrimSpace(p)))
	}

	text := fmt.Sprintf("📊 **%s**\n\n%s", title, strings.Join(formattedPoints, "\n"))

	if dg != nil {
		dg.SendTyping(ctx, channelID)
		dg.SendMessage(ctx, channelID, text)
	} else {
		tg.SendTyping(ctx, chatID)
		tg.SendMessage(ctx, chatID, text)
	}

	return mcp.NewToolResultText("Summary sent"), nil
}

// handleReadMessages handles reading the unread messages resource
func (s *Server) handleReadMessages(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	sessionID := ""
	if strings.Contains(request.Params.URI, "session_id=") {
		parts := strings.Split(request.Params.URI, "session_id=")
		if len(parts) > 1 {
			sessionID = parts[1]
		}
	}

	if sessionID == "" && s.chatID != 0 {
		sessionID = s.tgBot.GetActiveSession(s.chatID)
	}

	if sessionID == "" {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				Text: "No active session found. Connect to a session first.",
				URI:  request.Params.URI,
			},
		}, nil
	}

	var messages []string
	if s.discordBot != nil {
		messages = append(messages, s.discordBot.GetUnreadMessages(sessionID)...)
	}
	messages = append(messages, s.tgBot.GetUnreadMessages(sessionID)...)

	text := "No unread messages."
	if len(messages) > 0 {
		text = strings.Join(messages, "\n")
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			Text:     text,
			URI:      request.Params.URI,
			MIMEType: "text/plain",
		},
	}, nil
}

// handleGetInstructions returns system instructions for the agent
func (s *Server) handleGetInstructions(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	instructions := `### CRITICAL: REMOTE CONTROL MODE
You are an AI agent operating in "Remote Standby" mode via Ricochet. 

### RULES OF OPERATION:
1. **STRICT STANDBY**: You MUST NEVER finish your turn without calling 'wait_for_command' if you have completed the user's request. This is the ONLY way to stay connected and receive further instructions from Telegram/Discord.
2. **ACTIVE LOGGING**: Always use 'update_progress' or 'notify' to tell the user what you are doing. Don't work in silence.
3. **MESSENGER FIRST**: If the user asks a question via 'wait_for_command' or 'ask', respond back to the messenger using 'notify' or 'send_summary' before entering standby again.
4. **SESSION MANAGEMENT**: Use the provided sessionID from the context to ensure messages reach the correct chat.`

	return &mcp.GetPromptResult{
		Description: "Ricochet Remote Control Instructions",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: instructions,
				},
			},
		},
	}, nil
}

// Run starts the MCP server in stdio mode
func (s *Server) Run(ctx context.Context) error {
	log.Println("Starting Ricochet MCP server...")

	// Start background listener for Telegram events
	go s.listenForEvents(ctx)

	log.Println("Starting MCP server in stdio mode...")
	return server.ServeStdio(s.mcpServer)
}

// RunStandalone runs only Telegram bot (for testing without MCP)
func (s *Server) RunStandalone(ctx context.Context) error {
	log.Println("Running in standalone mode (Telegram only)...")
	s.listenForEvents(ctx)
	return nil
}

// listenForEvents handles messages and button callbacks
func (s *Server) listenForEvents(ctx context.Context) {
	respCh := s.tgBot.GetResponseChannel()
	callbackCh := s.tgBot.GetCallbackChannel()

	var discordRespCh <-chan *discord.UserResponse
	if s.discordBot != nil {
		discordRespCh = s.discordBot.GetResponseChannel()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case resp := <-respCh:
			if s.chatID == 0 {
				s.chatID = resp.ChatID
				log.Printf("Auto-set primary Telegram chat ID to: %d", s.chatID)
				if s.state != nil {
					s.state.SetPrimaryChatID(s.chatID)
				}
			}
			log.Printf("[Telegram] Received message from chat %d: %s", resp.ChatID, resp.Text)

		case resp := <-discordRespCh:
			log.Printf("[Discord] Received message from channel %s: %s", resp.ChannelID, resp.Text)
			// Discord doesn't use s.chatID (primary chat), it routes primarily by sessionID
			// which is already handled inside discordBot.handleMessage via GetActiveSession

		case cb := <-callbackCh:
			s.handleCallback(ctx, cb)
		}
	}
}

// handleCallback processes button clicks
func (s *Server) handleCallback(ctx context.Context, cb *telegram.CallbackEvent) {
	// Set chat ID if not set
	if s.chatID == 0 {
		s.chatID = cb.ChatID
	}

	switch {
	case cb.Data == telegram.CallbackChatHistory:
		s.sendChatHistory(ctx, cb.ChatID)

	case cb.Data == telegram.CallbackNewChat:
		s.tgBot.SendMessage(ctx, cb.ChatID, "➕ Для создания нового чата откройте Antigravity и начните новый разговор с агентом.")

	case strings.HasPrefix(cb.Data, "session:"):
		sessionID := strings.TrimPrefix(cb.Data, "session:")
		s.sendSessionDetails(ctx, cb.ChatID, sessionID)

	case strings.HasPrefix(cb.Data, "activate:"):
		sessionID := strings.TrimPrefix(cb.Data, "activate:")
		s.tgBot.SetActiveSession(cb.ChatID, sessionID)

		online := s.tgBot.IsSessionOnline(sessionID)
		statusStr := "⚠️ Оффлайн (откройте этот проект в Antigravity)"
		if online {
			statusStr = "🟢 В сети (готов к работе)"
		}

		sess, _ := s.sessionsMgr.GetSession(sessionID)
		title := sessionID[:8]
		if sess != nil {
			title = sess.Title
		}

		msg := fmt.Sprintf("📍 **Сессия активирована:** %s\nСтатус: %s\n\n", title, statusStr)
		if online {
			msg += "Чтобы запустить агента, **просто напишите ваш вопрос или команду первым в этот чат**, и он мгновенно ответит!"
		} else {
			msg += "Чтобы начать работу, **откройте соответствующий Workspace в Antigravity на вашем компьютере**. После этого я смогу принимать команды."
		}
		s.tgBot.SendMessage(ctx, cb.ChatID, msg)

	case strings.HasPrefix(cb.Data, "confirm_yes:"):
		sessionID := strings.TrimPrefix(cb.Data, "confirm_yes:")
		s.tgBot.SendToSession(sessionID, "confirm_yes")

	case strings.HasPrefix(cb.Data, "confirm_no:"):
		sessionID := strings.TrimPrefix(cb.Data, "confirm_no:")
		s.tgBot.SendToSession(sessionID, "confirm_no")

	default:
		log.Printf("Unknown callback: %s", cb.Data)
	}
}

// sendChatHistory sends list of recent sessions grouped by workspace
func (s *Server) sendChatHistory(ctx context.Context, chatID int64) {
	sessionsList, err := s.sessionsMgr.GetSessions(15) // Limit to 15 recent sessions for bot UI
	if err != nil {
		log.Printf("Failed to get sessions: %v", err)
		s.tgBot.SendMessage(ctx, chatID, "❌ Не удалось загрузить историю сессий")
		return
	}

	groups := s.sessionsMgr.GroupByWorkspace(sessionsList)
	if len(groups) == 0 {
		s.tgBot.SendMessage(ctx, chatID, "📭 Нет сохранённых сессий (с планом или отчетом)")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 **Ваши сессии по проектам:**\n")

	var buttons [][]telegram.ButtonConfig

	for _, g := range groups {
		sb.WriteString("\n📂 **" + g.Name + "**\n")

		for _, sess := range g.Sessions {
			status := "🔄"
			if sess.HasWalkthrough {
				status = "✅"
			} else if sess.HasPlan {
				status = "📝"
			}

			timeAgo := sessions.FormatTimeAgo(sess.UpdatedAt)
			sb.WriteString(fmt.Sprintf("- %s %s (%s)\n", status, sess.Title, timeAgo))

			// Add button for this session
			buttons = append(buttons, []telegram.ButtonConfig{
				{
					Text: fmt.Sprintf("%s %s", status, sess.Title),
					Data: "session:" + sess.ID,
				},
			})
		}
	}

	sb.WriteString("\n─────────────────\n💡 Нажмите на кнопку ниже, чтобы увидеть детали сессии.")

	if err := s.tgBot.SendMessageWithButtons(ctx, chatID, sb.String(), buttons); err != nil {
		log.Printf("Failed to send chat history with buttons: %v", err)
		// Fallback to plain message if buttons fail (e.g. too many)
		s.tgBot.SendMessage(ctx, chatID, sb.String())
	}
}

// sendSessionDetails sends more info about a selected session
func (s *Server) sendSessionDetails(ctx context.Context, chatID int64, sessionID string) {
	sess, err := s.sessionsMgr.GetSession(sessionID)
	if err != nil {
		s.tgBot.SendMessage(ctx, chatID, "❌ Сессия не найдена")
		return
	}

	status := "В процессе"
	if sess.HasWalkthrough {
		status = "Завершена ✅"
	} else if sess.HasPlan {
		status = "Планирование 📝"
	}

	msg := fmt.Sprintf("📄 **Детали сессии**\n\n"+
		"**Название:** %s\n"+
		"**Проект:** %s\n"+
		"**Статус:** %s\n"+
		"**Обновлено:** %s\n\n"+
		"**Кратко:**\n%s\n\n"+
		"─────────────────\n"+
		"Чтобы продолжить эту работу, откройте Antigravity и выберите данный чат в Inbox.",
		sess.Title, sess.Workspace, status, sessions.FormatTimeAgo(sess.UpdatedAt), sess.Summary)

	s.tgBot.SendMessageWithButtons(ctx, chatID, msg, [][]telegram.ButtonConfig{
		{
			{Text: "📍 Активировать этот чат", Data: "activate:" + sessionID},
		},
		{
			{Text: "🔙 К списку", Data: telegram.CallbackChatHistory},
		},
	})
}

// handleSendImage sends an image to the user
func (s *Server) handleSendImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	imagePath, ok := args["image_path"].(string)
	if !ok {
		return mcp.NewToolResultError("image_path parameter is required"), nil
	}

	caption, _ := args["caption"].(string)
	sessionID, _ := args["session_id"].(string)

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	if dg != nil {
		if err := dg.SendPhoto(ctx, channelID, imagePath, caption); err != nil {
			log.Printf("Failed to send image to Discord: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to send image to Discord: %v", err)), nil
		}
		log.Printf("Image sent to Discord: %s (session: %s)", imagePath, sessionID)
		return mcp.NewToolResultText("Image sent successfully to Discord"), nil
	}

	if chatID == 0 {
		return mcp.NewToolResultError("chat_id not set"), nil
	}

	if err := tg.SendPhoto(ctx, chatID, imagePath, caption); err != nil {
		log.Printf("Failed to send image to Telegram: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send image to Telegram: %v", err)), nil
	}

	log.Printf("Image sent to Telegram: %s (session: %s)", imagePath, sessionID)
	return mcp.NewToolResultText("Image sent successfully to Telegram"), nil
}

// handleSendCodeBlock sends a formatted code block to user
func (s *Server) handleSendCodeBlock(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	code, ok := args["code"].(string)
	if !ok {
		return mcp.NewToolResultError("code parameter is required"), nil
	}

	language, _ := args["language"].(string)
	if language == "" {
		language = "text"
	}

	sessionID, _ := args["session_id"].(string)

	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	if dg != nil {
		if err := dg.SendCodeBlock(ctx, channelID, language, code); err != nil {
			log.Printf("Failed to send code block to Discord: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to send code block to Discord: %v", err)), nil
		}
		log.Printf("Code block sent to Discord (session: %s, language: %s)", sessionID, language)
		return mcp.NewToolResultText("Code block sent successfully to Discord"), nil
	}

	if chatID == 0 {
		return mcp.NewToolResultError("chat_id not set"), nil
	}

	if err := tg.SendCodeBlock(ctx, chatID, language, code); err != nil {
		log.Printf("Failed to send code block to Telegram: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send code block to Telegram: %v", err)), nil
	}

	log.Printf("Code block sent to Telegram (session: %s, language: %s)", sessionID, language)
	return mcp.NewToolResultText("Code block sent successfully to Telegram"), nil
}

// handleBrowserSearch performs a web search using DuckDuckGo
func (s *Server) handleBrowserSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	query, ok := args["query"].(string)
	if !ok {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))
	resp, err := http.Get(searchURL)
	if err != nil {
		log.Printf("Search failed: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to perform search: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read search response: %v", err)), nil
	}

	// We return the raw JSON from DDG as it's quite clean and informative for an LLM
	return mcp.NewToolResultText(string(body)), nil
}

// handleVoiceReply generates and sends a voice message using TTS
func (s *Server) handleVoiceReply(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	s.updateHeartbeat(args)

	text, ok := args["text"].(string)
	if !ok {
		return mcp.NewToolResultError("text parameter is required"), nil
	}

	sessionID, _ := args["session_id"].(string)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return mcp.NewToolResultError("OPENAI_API_KEY environment variable is not set. Voice reply requires an OpenAI API key."), nil
	}

	// 1. Resolve delivery channel
	tg, dg, chatID, channelID := s.resolveChannel(sessionID)

	// 2. Prepare TTS request (OpenAI)
	tempDir := filepath.Join(os.TempDir(), "ricochet_tts")
	os.MkdirAll(tempDir, 0755)
	outputPath := filepath.Join(tempDir, fmt.Sprintf("tts_%d.mp3", time.Now().UnixNano()))

	jsonData := fmt.Sprintf(`{"model": "tts-1", "input": "%s", "voice": "alloy"}`, strings.ReplaceAll(text, `"`, `\"`))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", strings.NewReader(jsonData))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create TTS request: %v", err)), nil
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("TTS API request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultError(fmt.Sprintf("TTS API returned error (%d): %s", resp.StatusCode, string(body))), nil
	}

	// 3. Save to file
	out, err := os.Create(outputPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create temp audio file: %v", err)), nil
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save audio file: %v", err)), nil
	}

	// 4. Send to user
	if dg != nil {
		if err := dg.SendVoice(ctx, channelID, outputPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to send voice to Discord: %v", err)), nil
		}
	} else {
		if chatID == 0 {
			return mcp.NewToolResultError("Telegram chat_id not set"), nil
		}
		if err := tg.SendVoice(ctx, chatID, outputPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to send voice to Telegram: %v", err)), nil
		}
	}

	log.Printf("Voice reply sent (session: %s, text size: %d)", sessionID, len(text))
	return mcp.NewToolResultText("Voice reply sent successfully"), nil
}
