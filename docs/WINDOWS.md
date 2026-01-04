# Windows Setup Guide

## Quick Start

1. **Get a Telegram Bot Token**
   - Open [@BotFather](https://t.me/BotFather) in Telegram
   - Send `/newbot` and follow instructions
   - Copy your bot token

2. **Set Environment Variable**
   ```powershell
   # PowerShell
   $env:TELEGRAM_BOT_TOKEN="your_token_here"
   
   # Or set permanently
   [System.Environment]::SetEnvironmentVariable('TELEGRAM_BOT_TOKEN', 'your_token_here', 'User')
   ```

3. **Run Installer**
   ```powershell
   .\ricochet.exe install
   ```

4. **Restart your IDE** (Cursor, Claude Desktop, etc.)

---

## Troubleshooting

### "Failed to load config: TELEGRAM_BOT_TOKEN is required"

**Cause**: Environment variable not set

**Solution**:
```powershell
# Check if token is set
echo $env:TELEGRAM_BOT_TOKEN

# Set it
$env:TELEGRAM_BOT_TOKEN="your_token_from_botfather"

# Or set permanently in System Properties > Environment Variables
```

### Voice Commands Not Working

**Cause**: FFmpeg or Whisper not installed (optional features)

**Solution**:

Voice transcription is **optional**. Ricochet works fine without it.

To enable voice commands:

1. **Install FFmpeg**:
   ```powershell
   # Using Chocolatey
   choco install ffmpeg
   
   # Or download from: https://ffmpeg.org/download.html
   ```

2. **Install Whisper** (optional):
   - Download whisper.cpp from: https://github.com/ggerganov/whisper.cpp
   - Build or download pre-built binary
   - Set environment variables:
     ```powershell
     $env:WHISPER_PATH="C:\path\to\whisper-cli.exe"
     $env:WHISPER_MODEL_PATH="C:\path\to\ggml-base.bin"
     ```

### Double-Clicking .exe Shows Help and Exits

**This is normal!** Ricochet is an MCP server that runs inside your IDE.

- ✅ **Correct**: Run `ricochet.exe install` then restart your IDE
- ❌ **Incorrect**: Double-click `ricochet.exe` (it will just show help)

---

## Manual Configuration

If the installer doesn't work, manually edit your IDE config:

### Cursor

Edit `%USERPROFILE%\.cursor\mcp.json`:

```json
{
  "mcpServers": {
    "ricochet": {
      "command": "C:\\path\\to\\ricochet.exe",
      "env": {
        "TELEGRAM_BOT_TOKEN": "your_token_here"
      }
    }
  }
}
```

### Claude Desktop

Edit `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ricochet": {
      "command": "C:\\path\\to\\ricochet.exe",
      "env": {
        "TELEGRAM_BOT_TOKEN": "your_token_here"
      }
    }
  }
}
```

---

## FAQ

**Q: Do I need Whisper to use Ricochet?**  
A: No! Whisper is optional. Without it, you can still send text commands via Telegram.

**Q: Why does the .exe close immediately?**  
A: Ricochet needs to be configured in your IDE first. Run `ricochet.exe install` instead of double-clicking.

**Q: Can I use it without an IDE?**  
A: Yes! Run `ricochet.exe --standalone` for Telegram-only mode (no MCP).
