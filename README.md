# Ricochet 🚀

**Control your AI coding agents from Telegram**

Ricochet is an MCP server that bridges your IDE (Cursor, Claude Code, Windsurf, Antigravity) with Telegram and Discord. Get notifications, answer questions, and send commands — all from your phone.

[![npm version](https://img.shields.io/npm/v/ricochet-mcp.svg)](https://www.npmjs.com/package/ricochet-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)

---

## ✨ What can you do?

- 📱 **Get notified** when your agent finishes a task
- 🎤 **Send voice messages** — Ricochet transcribes them using Whisper
- ✅ **Approve dangerous commands** (`rm -rf`, `git push`) from your phone
- 💬 **Answer agent questions** without touching your keyboard
- 🔄 **Switch between projects** with a single tap

---

## 📦 Installation

### Claude Code (One command!)

```bash
claude mcp add ricochet -- npx -y ricochet-mcp
```

Then set your bot token:
```bash
claude mcp add ricochet -- npx -y ricochet-mcp --env TELEGRAM_BOT_TOKEN=your_token
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

### Alternative: Download Binary

```bash
# macOS Apple Silicon
curl -L https://github.com/igoryanba/ricochet/releases/latest/download/ricochet-darwin-arm64 -o ricochet
chmod +x ricochet

# macOS Intel
curl -L https://github.com/igoryanba/ricochet/releases/latest/download/ricochet-darwin-amd64 -o ricochet

# Linux
curl -L https://github.com/igoryanba/ricochet/releases/latest/download/ricochet-linux-amd64 -o ricochet

# Windows
curl -L https://github.com/igoryanba/ricochet/releases/latest/download/ricochet-windows-amd64.exe -o ricochet.exe
```

---

## 🚀 Quick Start

### 1. Create a Telegram Bot
1. Open [@BotFather](https://t.me/BotFather) in Telegram
2. Send `/newbot` and follow instructions
3. Copy your bot token

### 2. Add to your IDE (see above)

### 3. Restart your IDE

### 4. Start chatting! 🎉
Open your bot in Telegram and send `/start`.

---

## 🪟 Windows Users

**Important**: If `ricochet.exe` closes immediately, you need to run the installer first:

```powershell
$env:TELEGRAM_BOT_TOKEN="your_token_here"
.\ricochet.exe install
```

Then restart your IDE. See [Windows Setup Guide](docs/WINDOWS.md) for detailed instructions.

---

## 🧰 MCP Tools

| Tool | Description |
|------|-------------|
| `notify` | Send a message to Telegram |
| `ask` | Ask a question and wait for answer |
| `confirm_dangerous` | Request approval for dangerous commands |
| `send_image` | Send screenshots to chat |
| `send_code_block` | Send formatted code with syntax highlighting |
| `voice_reply` | Send voice message (TTS) |
| `update_progress` | Send progress updates (planning/execution/verification) |
| `wait_for_command` | Enter standby mode, wait for user input |
| `browser_search` | Search the web via DuckDuckGo |

---

## 🎤 Voice Control (Whisper)

Send a voice message in Telegram → Ricochet transcribes it locally using Whisper → Your agent receives the text command.

**Requirements**: FFmpeg installed (`brew install ffmpeg`)

---

## 🐳 Docker (Headless Mode)

Run Ricochet without an IDE:
```bash
docker build -t ricochet .
docker run -e TELEGRAM_BOT_TOKEN=xxx ricochet
```

---

## 🔧 Build from Source

```bash
git clone https://github.com/igoryanba/ricochet.git
cd ricochet
go build -o ricochet ./cmd/ricochet
```

---

## 📄 License

MIT © 2025 Igor Pryimak ([@igoryan34](https://t.me/igoryan34))
