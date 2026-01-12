package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/igoryan-dao/ricochet/cmd/cli/client"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	sessionID  string
)

var rootCmd = &cobra.Command{
	Use:   "ricochet-cli",
	Short: "Terminal client for Ricochet Core",
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Run: func(cmd *cobra.Command, args []string) {
		startChat()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "localhost:5555", "Address of Ricochet Core Daemon")
	rootCmd.PersistentFlags().StringVarP(&sessionID, "session", "i", "cli-default", "Session ID to use")
	rootCmd.AddCommand(chatCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startChat() {
	c := client.NewClient(serverAddr)

	// Channels to manage sync flow if needed, though we are event driven
	responseDone := make(chan bool)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	c.OnConnected = func() {
		fmt.Printf("🔌 Connected to Ricochet Daemon at %s\n", serverAddr)
		// Create session if needed or just start
		c.SendCommand("create_session", nil) // Just strict fire
	}

	c.OnMessage = func(msg protocol.RPCMessage) {
		switch msg.Type {
		case "chat_update":
			// Streaming update handled in override below

		case "response":
			// Request finished
			if msg.ID != nil {
				// Verify it matches our last request?
				responseDone <- true
			}

		case "error":
			fmt.Printf("Error: %v\n", msg)
		}
	}

	// Override OnMessage to handle streaming better
	var lastLength int
	var fullContent strings.Builder

	c.OnMessage = func(msg protocol.RPCMessage) {
		switch msg.Type {
		case "chat_update":
			// Extract content
			var payload map[string]interface{}
			_ = json.Unmarshal(msg.Payload, &payload)

			if content, ok := payload["message"].(string); ok {
				// Basic incremental printing
				if len(content) > lastLength {
					chunk := content[lastLength:]
					fmt.Print(chunk)
					fullContent.WriteString(chunk)
					lastLength = len(content)
				}
			}

		case "response":
			// Done
			// Render Markdown of the full message
			out, err := renderer.Render(fullContent.String())
			if err == nil {
				// Print pretty version
				fmt.Println("\n" + strings.Repeat("─", 50))
				fmt.Print(out)
			} else {
				fmt.Println()
			}

			fmt.Println(strings.Repeat("─", 50))
			lastLength = 0
			fullContent.Reset()
			responseDone <- true
		}
	}

	if err := c.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		fmt.Println("Make sure 'ricochet --server' is running!")
		return
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("💬 Ricochet CLI Ready. Type your message (Ctrl+C to quit)")
	fmt.Println(strings.Repeat("─", 50))

	for {
		fmt.Print("You > ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "quit" || text == "exit" {
			return
		}
		if text == "" {
			continue
		}

		// Reset state
		lastLength = 0
		fullContent.Reset()

		// Send
		c.SendCommand("chat_message", map[string]string{
			"content":    text,
			"session_id": sessionID,
			"via":        "cli",
		})

		// Wait for completion
		<-responseDone
	}
}
