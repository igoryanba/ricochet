# 🚀 Ricochet v2 Deployment & Verification Guide

This guide explains how to set up and verify the **Cloud Bridge** architecture.

## 1. Prerequisites
- **TELEGRAM_BOT_TOKEN**: Your primary bot token (for the Cloud Bridge).
- **RICOCHET_BRIDGE_SECRET**: A long random string shared between Cloud and Agent.
- **RICOCHET_CLOUD_URL**: The address of your Cloud Bridge (e.g., `ws://your-vps:8080/ws`).

---

## 2. Server Side (Cloud Bridge)

The Cloud Bridge acts as a proxy between Telegram and your local Agents.

### Step 1: Compile the binary
```bash
go build -o cloud-bridge ./cmd/cloud-bridge
```

### Step 2: Run the server
```bash
export TELEGRAM_BOT_TOKEN="your_bot_token"
export RICOCHET_BRIDGE_SECRET="your_shared_secret"
export PORT=8080
./cloud-bridge
```
*The server will start listening for WebSockets on `/ws` and Telegram updates on `/webhook/telegram` (or long polling if configured).*

---

## 3. Client Side (Local Agent)

The Local Agent runs on your machine and connects to the Cloud Bridge.

### Step 1: Compile Ricochet
```bash
go build -o ricochet ./cmd/ricochet
```

### Step 2: Run Ricochet in Bridge Mode
```bash
export RICOCHET_CLOUD_URL="ws://localhost:8080/ws"
export RICOCHET_BRIDGE_SECRET="your_shared_secret"
./ricochet
```
*Notice: You don't need `TELEGRAM_BOT_TOKEN` locally anymore if using the Bridge.*

---

## 4. Verification Flow

1. **Check Connectivity**: Look at the `cloud-bridge` logs. You should see:  
   `🔌 Local Agent connected: session_xxx`
2. **Link Telegram**: Open your bot in Telegram and send:  
   `/link session_xxx` (Replace `session_xxx` with the ID from the logs).
3. **Test Messaging**: Send any message to the bot. 
   - Cloud Bridge receives it.
   - Forwards it via gRPC to your Local Agent.
   - Local Agent processes it (AI response).
   - Agent sends response back through the tunnel.
   - Cloud Bridge delivers it to you in Telegram.

---

## 🛠️ Debugging
- **Lint Errors**: Fixed (unused handlers removed).
- **Offline Agent**: If you see "Agent offline", check if the туннель is active.
- **Secret Mismatch**: If the secret doesn't match, the Cloud Bridge will log `⛔ Unauthorized connection attempt!`.
