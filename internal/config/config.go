package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	TelegramToken  string
	AllowedUserIDs []int64
	DiscordToken   string
	DiscordGuildID string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	cfg := &Config{
		TelegramToken:  token,
		AllowedUserIDs: []int64{},
		DiscordToken:   os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordGuildID: os.Getenv("DISCORD_GUILD_ID"),
	}

	// Parse allowed user IDs (comma-separated)
	if userIDs := os.Getenv("ALLOWED_USER_IDS"); userIDs != "" {
		for _, idStr := range strings.Split(userIDs, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid user ID %q: %w", idStr, err)
			}
			cfg.AllowedUserIDs = append(cfg.AllowedUserIDs, id)
		}
	}

	return cfg, nil
}
