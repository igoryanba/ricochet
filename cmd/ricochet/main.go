package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/igoryan-dao/ricochet/internal/bridge"
	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/discord"
	"github.com/igoryan-dao/ricochet/internal/install"
	"github.com/igoryan-dao/ricochet/internal/mcp"
	"github.com/igoryan-dao/ricochet/internal/state"
	"github.com/igoryan-dao/ricochet/internal/telegram"
	"github.com/igoryan-dao/ricochet/internal/whisper"
)

func main() {
	// 1. Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			handleInstall()
			return
		case "help":
			printHelp()
			return
		}
	}

	// 2. Check if running interactively (double-clicked exe)
	// If stdin is not a pipe/MCP connection, show help instead of hanging
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// Running in terminal/interactive mode without subcommand
		// This happens when user double-clicks the exe on Windows
		printInteractiveHelp()
		return
	}

	// 3. Default mode: Run MCP Server (stdin/stdout for IDE)
	runServer()
}

func handleInstall() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Error: TELEGRAM_BOT_TOKEN environment variable is required for installation.")
	}

	executable, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}

	fmt.Println("🚀 Ricochet Universal Installer")
	fmt.Println("-------------------------------")

	err = install.Install(token, executable)
	if err != nil {
		log.Fatalf("Installation failed: %v", err)
	}

	fmt.Println("\n✨ Ricochet is now configured in all detected AI tools!")
	fmt.Println("Please restart your IDE or AI CLI (Cursor, Claude Desktop, etc.) to apply changes.")
}

func printHelp() {
	fmt.Println("Ricochet - The Bridge between AI Agents and Telegram")
	fmt.Println("\nUsage:")
	fmt.Println("  ricochet          Run MCP Server (stdio mode)")
	fmt.Println("  ricochet install  Automatically configure MCP in Cursor, Claude Desktop, Claude Code, etc.")
	fmt.Println("  ricochet help     Show this help")
}

func printInteractiveHelp() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           🚀 Ricochet - Control IDE from Telegram            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("  Ricochet is an MCP server that bridges your IDE with Telegram.")
	fmt.Println("  It runs automatically when your IDE starts.")
	fmt.Println("")
	fmt.Println("  📦 INSTALLATION:")
	fmt.Println("  ────────────────")
	fmt.Println("  1. Set your Telegram bot token:")
	fmt.Println("     export TELEGRAM_BOT_TOKEN=\"your_token_here\"")
	fmt.Println("")
	fmt.Println("  2. Run the installer:")
	fmt.Println("     ./ricochet install")
	fmt.Println("")
	fmt.Println("  3. Restart your IDE (Cursor, Claude Code, etc.)")
	fmt.Println("")
	fmt.Println("  📖 MANUAL:")
	fmt.Println("  ──────────")
	fmt.Println("  ricochet install  - Configure MCP in all detected IDEs")
	fmt.Println("  ricochet help     - Show command help")
	fmt.Println("")
	fmt.Println("  🌐 GitHub: https://github.com/igoryanba/ricochet")
	fmt.Println("")
	fmt.Println("Press Enter to exit...")
	fmt.Scanln()
}

func runServer() {
	standalone := flag.Bool("standalone", false, "Run in standalone mode (Telegram only, no MCP)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		cloudURL := os.Getenv("RICOCHET_CLOUD_URL")
		if cloudURL == "" {
			log.Fatalf("Failed to load config: %v. Either .env with token or RICOCHET_CLOUD_URL must be provided.", err)
		}
		// If cloudURL is present, we can proceed with empty config (token-less)
		cfg = &config.Config{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Initialize state manager
	stateMgr, err := state.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize state manager: %v", err)
	}

	tgBot, err := telegram.New(cfg.TelegramToken, cfg.AllowedUserIDs, stateMgr)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Initialize Whisper transcriber (optional feature)
	initializeWhisper(tgBot)

	// Initialize Discord bot if token is provided
	var discordBot *discord.Bot
	if cfg.DiscordToken != "" {
		db, err := discord.New(cfg.DiscordToken, cfg.DiscordGuildID, stateMgr)
		if err != nil {
			log.Printf("Warning: Failed to create Discord bot: %v. Discord integration disabled.", err)
		} else {
			discordBot = db
			if err := discordBot.Start(); err != nil {
				log.Printf("Warning: Failed to start Discord bot: %v", err)
			} else {
				log.Println("Discord bot started successfully")
				defer discordBot.Stop()
			}
		}
	}

	// Initialize MCP server with Telegram & Discord bridge
	mcpServer := mcp.NewServer(tgBot, discordBot, stateMgr)

	// Cloud Bridge Integration (Ricochet v2)
	cloudURL := os.Getenv("RICOCHET_CLOUD_URL")
	if cloudURL != "" {
		log.Printf("☁️ Cloud Bridge Mode enabled: %s", cloudURL)

		// Use machine hostname or a unique ID for the session
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "local-agent"
		}

		bridgeClient := bridge.NewClient(cloudURL, hostname)
		if err := bridgeClient.Start(ctx); err != nil {
			log.Printf("⚠️ Failed to start Cloud Bridge: %v", err)
		} else {
			log.Println("✅ Connected to Ricochet Cloud Bridge")
			mcpServer.SetBridge(bridgeClient)
			defer bridgeClient.Close()
		}
	}

	// Start Telegram bot in background
	go tgBot.Start(ctx)

	if *standalone {
		// Run in standalone mode (for testing)
		if err := mcpServer.RunStandalone(ctx); err != nil {
			log.Fatalf("Standalone error: %v", err)
		}
	} else {
		// Run MCP server (stdio mode)
		if err := mcpServer.Run(ctx); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
	}
}

// initializeWhisper attempts to initialize Whisper transcriber
// This is an optional feature - if it fails, voice commands are disabled but the app continues
func initializeWhisper(tgBot *telegram.Bot) {
	whisperPath := os.Getenv("WHISPER_PATH")
	modelPath := os.Getenv("WHISPER_MODEL_PATH")

	// Try to find Whisper if not explicitly set
	if whisperPath == "" {
		whisperPath = findWhisperBinary()
		if whisperPath == "" {
			log.Println("Whisper not found. Voice transcription disabled. Set WHISPER_PATH to enable.")
			return
		}
	}

	// Model path is required if binary is set
	if modelPath == "" {
		log.Println("WHISPER_MODEL_PATH not set. Voice transcription disabled.")
		return
	}

	// Validate paths exist
	if _, err := os.Stat(whisperPath); os.IsNotExist(err) {
		log.Printf("Warning: Whisper binary not found at %s. Voice commands disabled.", whisperPath)
		return
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Printf("Warning: Whisper model not found at %s. Voice commands disabled.", modelPath)
		return
	}

	// Check if FFmpeg is available (required for voice message conversion)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Println("Warning: FFmpeg not found. Voice transcription requires FFmpeg. Install it to enable voice commands.")
		return
	}

	// Try to initialize transcriber
	log.Printf("Initializing Whisper with binary: %s, model: %s", whisperPath, modelPath)
	transcriber, err := whisper.NewTranscriber(whisperPath, modelPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize Whisper: %v. Voice commands disabled.", err)
		return
	}

	tgBot.SetTranscriber(transcriber)
	log.Println("Whisper transcriber initialized successfully")
}

// findWhisperBinary looks for whisper-cli in common locations
func findWhisperBinary() string {
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		candidates = []string{
			"/usr/local/bin/whisper-cli",
			"/opt/homebrew/bin/whisper-cli",
			homeDir + "/Ricochet/third_party/whisper.cpp/build/bin/whisper-cli",
		}
	case "linux":
		candidates = []string{
			"/usr/local/bin/whisper-cli",
			"/usr/bin/whisper-cli",
		}
	case "windows":
		candidates = []string{
			"C:\\Program Files\\whisper\\whisper-cli.exe",
			"C:\\whisper\\whisper-cli.exe",
		}
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
