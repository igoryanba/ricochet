Перепроверь

# Ricochet


Этот маркетплейс находится на бета-тесте, версия v0.0.1, план выпуска - конец января.

It creates a hybrid development environment where you can pair-program with an autonomous agent directly in VS Code, or switch to **Live Mode** and control your development environment remotely via Telegram or Discord.

Unlike standard autocomplete tools, Ricochet is designed for **autonomous task execution**. It manages its own context, learns from your project patterns, and runs quality checks to ensure code integrity.

---

## Capabilities

### 1. Autonomous Agent
Ricochet uses a powerful local core (written in Go) to orchestrate complex coding tasks.
*   **Reflex Engine**: Automatically manages context windows, condensing conversation history to maintain long-term memory during deep coding sessions.
*   **Skill Injector**: Detects your current task (e.g., "working on backend controllers") and automatically injects relevant project guidelines and best practices.
*   **Task Workspaces**: Creates structured plans (`PLAN.md`, `CONTEXT.md`) for complex features to prevent agent amnesia.
*   **Auto-QC**: Automatically runs build and lint checks after editing code. If the build fails, Ricochet catches the error and attempts to fix it before returning control to you.

### 2. Live Mode (Ether)
Don't be tied to your desk. Toggle "Live Mode" to connect Ricochet to a Telegram or Discord bot.
*   **Remote Control**: Ask your agent to "fix the bug" or "deploy to staging" while you are away.
*   **Notifications**: Receive real-time updates when tasks are completed or if the agent needs clarification.
*   **Voice Support**: Send voice messages to your agent for natural language prompting.

### 3. Tooling & Integration
*   **MCP Support**: Fully compatible with the **Model Context Protocol**. Connect any MCP server (GitHub, Postgres, Filesystem) to extend Ricochet's capabilities.
*   **Cross-Platform**: Runs natively on macOS, Linux, and Windows.
*   **Multi-Provider**: Bring Your Own Key (BYOK). Supports Anthropic (Claude), OpenAI (GPT-4), Google (Gemini), DeepSeek, and OpenRouter.

---

## Installation

### VS Code Marketplace
Install **Ricochet** directly from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=ricochet.ricochet).

### Build from Source
If you prefer to build the latest version yourself:

```bash
git clone https://github.com/Grik-ai/ricochet.git
cd ricochet
./scripts/build-all.sh
```

---

## Architecture

Ricochet uses a **Sidecar Architecture** for performance and reliability.
1.  **Extension (Host)**: A lightweight VS Code extension handles the UI and Editor interactions.
2.  **Core (Sidecar)**: A standalone binary written in Go handles the heavy lifting—AI orchestration, context parsing (Tree-sitter), tool execution, and the Live Mode server.

This separation ensuring that the AI agent continues running even if the editor window is reloaded, and enables high-performance processing without slowing down VS Code.

---

## Support the Project

Ricochet is an independent open-source project. If it helps you build faster, please consider supporting its development.

*   **Star the Repo**: [github.com/Grik-ai/ricochet](https://github.com/Grik-ai/ricochet)
*   **Ko-fi**: [ko-fi.com/igoryan34](https://ko-fi.com/igoryan34)
*   **PayPal**: [Donate via PayPal](https://www.paypal.com/ncp/payment/PPMFBMFVAB8QN)

### License
Apache 2.0 © 2025 Igor Pryimak
