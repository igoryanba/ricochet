# Ricochet 🚀

**Control your AI coding agents from Telegram**

Ricochet is an MCP server that bridges your IDE (Antigravity, Cursor, Claude, Windsurf) with Telegram and Discord. Get notifications, answer questions, and send commands — all from your phone.

![Demo](https://img.shields.io/badge/demo-coming_soon-blue)
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

### Option 1: Download Binary (Recommended)
```bash
# Download latest release for your OS
curl -L https://github.com/igoryanba/ricochet/releases/latest/download/ricochet-darwin-arm64 -o ricochet
chmod +x ricochet
```

### Option 2: Build from Source
```bash
git clone https://github.com/igoryanba/ricochet.git
cd ricochet
go build -o ricochet ./cmd/ricochet
```

---

## 🚀 Quick Start

### 1. Create a Telegram Bot
1. Open [@BotFather](https://t.me/BotFather) in Telegram
2. Send `/newbot` and follow instructions
3. Copy your bot token

### 2. Configure
```bash
export TELEGRAM_BOT_TOKEN="your_token_here"
```

### 3. Add to your IDE
Run the auto-installer:
```bash
./ricochet install
```

Or manually add to your MCP config (`~/.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "ricochet": {
      "command": "/path/to/ricochet",
      "env": {
        "TELEGRAM_BOT_TOKEN": "your_token_here"
      }
    }
  }
}
```

### 4. Start chatting! 🎉
Open your bot in Telegram and send `/start`.

---

## 🧰 MCP Tools

| Tool | Description |
|------|-------------|
| `notify` | Send a message to Telegram |
| `ask` | Ask a question and wait for answer |
| `confirm_dangerous` | Request approval for dangerous commands |
| `send_image` | Send screenshots to chat |
| `voice_reply` | Send voice message (TTS) |
| `wait_for_command` | Enter standby mode, wait for user input |

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

## 📄 License

MIT © 2025 Igor Pryimak ([@igoryan34](https://t.me/igoryan34))
