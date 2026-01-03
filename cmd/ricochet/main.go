package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	// 2. Default mode: Run MCP Server
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

	// Initialize Whisper transcriber
	// Check for env vars (useful for Docker), fallback to local Mac paths
	whisperPath := os.Getenv("WHISPER_PATH")
	if whisperPath == "" {
		whisperPath = "/Users/igoryan_dao/Ricochet/third_party/whisper.cpp/build/bin/whisper-cli"
	}

	modelPath := os.Getenv("WHISPER_MODEL_PATH")
	if modelPath == "" {
		modelPath = "/Users/igoryan_dao/Ricochet/third_party/whisper.cpp/models/ggml-base.bin"
	}

	log.Printf("Initializing Whisper with binary: %s, model: %s", whisperPath, modelPath)
	transcriber, err := whisper.NewTranscriber(whisperPath, modelPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize Whisper: %v. Voice commands will be disabled.", err)
	} else {
		tgBot.SetTranscriber(transcriber)
		log.Println("Whisper transcriber initialized successfully")
	}

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
