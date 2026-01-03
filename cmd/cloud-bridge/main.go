package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/igoryan-dao/ricochet/internal/bridge/server"
)

func main() {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid PORT: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Cloud Bridge shutting down...")
		cancel()
	}()

	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	bridgeServer := server.NewCloudBridge(port, tgToken)
	if err := bridgeServer.Start(ctx); err != nil {
		log.Fatalf("Bridge server failed: %v", err)
	}
}
