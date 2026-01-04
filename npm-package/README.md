# Ricochet MCP 🚀

**Control your AI coding agents from Telegram**

Ricochet bridges your IDE (Cursor, Claude Code, Windsurf, Antigravity) with Telegram. Get notifications, answer questions, and send commands — all from your phone.

## ✨ Features

- 📱 **Get notified** when your agent finishes a task
- 🎤 **Send voice messages** — Ricochet transcribes them using Whisper
- ✅ **Approve dangerous commands** (`rm -rf`, `git push`) from your phone
- 💬 **Answer agent questions** without touching your keyboard

## 📦 Installation

### Claude Code (One Command!)

```bash
claude mcp add ricochet -- npx -y ricochet-mcp
```

### Cursor

Add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "ricochet": {
      "command": "npx",
      "args": ["-y", "ricochet-mcp"],
      "env": {
        "TELEGRAM_BOT_TOKEN": "your_token_here"
      }
    }
  }
}
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ricochet": {
      "command": "npx",
      "args": ["-y", "ricochet-mcp"],
      "env": {
        "TELEGRAM_BOT_TOKEN": "your_token_here"
      }
    }
  }
}
```

## 🔑 Setup

1. Open [@BotFather](https://t.me/BotFather) in Telegram
2. Send `/newbot` and follow instructions
3. Copy your bot token
4. Add to your MCP config (see above)
5. Restart your IDE
6. Send `/start` to your bot!

## 🎤 Voice Control

Send a voice message in Telegram → Ricochet transcribes it → Your agent receives the command.

**Requirements**: FFmpeg (`brew install ffmpeg`)

## 📄 License

MIT © Igor Pryimak

## 🔗 Links

- [GitHub](https://github.com/igoryanba/ricochet)
- [Telegram](https://t.me/igoryan34)
