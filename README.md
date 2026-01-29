# Ricochet

<p align="center">
  <strong>The Hybrid AI Coding Agent</strong><br>
  Pair-program in VS Code or control your environment remotely via messengers.
</p>

<p align="center">
  <a href="https://marketplace.visualstudio.com/items?itemName=grik.ricochet"><img src="https://img.shields.io/visual-studio-marketplace/v/grik.ricochet?style=flat-square&label=VS%20Code" alt="VS Code Extension"></a>
   <a href="https://github.com/Grik-ai/ricochet/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Grik-ai/ricochet?style=flat-square" alt="License"></a>
</p>

Ricochet is an open-source autonomous agent designed for complex coding tasks. Unlike standard autocomplete tools, it manages its own context, learns from project patterns, and plans actions using a DAG-based architecture.

## 🚀 Capabilities

### 1. Autonomous Agent
Ricochet uses a powerful local core (written in Go) to orchestrate complex coding tasks.
*   **Swarm Mode (Parallel Execution)**: Uses a DAG-based planner to spawn multiple workers (up to 5+) for handling independent tasks simultaneously.
*   **Plan Mode (Task Workspaces)**: A dedicated planning engine that tracks task lifecycle (pending, active, verification) and persists plans (`PLAN.md`, `CONTEXT.md`) across sessions to prevent agent amnesia.
*   **Reflex Engine (4-Level Context)**: Automatically manages context windows, condensing conversation history to maintain long-term memory during deep coding sessions.
*   **Shadow Git**: Every task has a hidden git checkpoint. Instantly undo/redo AI-generated code without polluting your main project history.
*   **Skill Injector**: Detects your current task (e.g., "working on backend controllers") and automatically injects relevant project guidelines and best practices.
*   **Auto-QC**: Automatically runs build and lint checks after editing code. If the build fails, Ricochet catches the error and attempts to fix it before returning control to you.

### 2. Live Mode (Ether)
Don't be tied to your desk. Toggle "Live Mode" to connect Ricochet to a Telegram or Discord bot.
*   **Remote Control**: Ask your agent to "fix the bug" or "deploy to staging" while you are away.
*   **Notifications**: Receive real-time updates when tasks are completed or if the agent needs clarification.
*   **Voice Support**: Send voice messages to your agent for natural language prompting.

### 3. Tooling & Integration
*   **CLI & VS Code**: Full feature parity between the Visual Studio Code extension and the standalone Terminal User Interface (TUI).
*   **MCP Support**: Fully compatible with the **Model Context Protocol**. Connect any MCP server (GitHub, Postgres, Filesystem) to extend Ricochet's capabilities.
*   **Cross-Platform**: Runs natively on macOS, Linux, and Windows.
*   **Multi-Provider**: Bring Your Own Key (BYOK). Supports Anthropic (Claude), OpenAI (GPT-4), Google (Gemini), DeepSeek, and OpenRouter.

## 📦 Installation

### VS Code Extension (Recommended)
Install **Ricochet** directly from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=grik.ricochet).

### CLI Tool
The standalone CLI is bundled with the extension (or can be built from source). To launch the terminal interface:

```bash
# Launch Ricochet CLI
ricochet
```

## 🔑 Setup (DeepSeek)

Currently, Ricochet interaction is handled through **DeepSeek**.

1.  **Get an API Key**: Obtain your key from the [DeepSeek Platform](https://platform.deepseek.com).
2.  **Configure**:
    *   **VS Code**: Enter the key in the extension settings.
    *   **CLI**: Run `ricochet /init` or set the `DEEPSEEK_API_KEY` environment variable.

*Note: BYOK (Bring Your Own Key) support for other providers (Anthropic, OpenAI, Gemini) is in progress.*

## 🛠 Support the Project

Ricochet is developed solo and is completely open-source. If it helps you build faster, please consider supporting its development.

*   **Star the Repo**: [github.com/Grik-ai/ricochet](https://github.com/Grik-ai/ricochet)
*   **Ko-fi**: [ko-fi.com/igoryan34](https://ko-fi.com/igoryan34)
*   **PayPal**: [Donate via PayPal](https://www.paypal.com/ncp/payment/PPMFBMFVAB8QN)
*   **Crypto**:
    *   **TON**: `UQB93GTsF6ZI7ljBViLr-IHIf93HpqwolC51jR5Und7GAwm4`
    *   **USDT (TRC20)**: `TRykhiHNeXxcdmhn5DBP2GGLnKQDBVzjhB`
    *   **USDT (SOL)**: `DAaY5J6tg6PY7M77mxpMxGFMqLsCtPRKBmgmmCS9zM5b`
    *   **USDT (ERC20)**: `0x5eB4C13497e10849724714D351C139Fc6Ab00Adc`

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Grik-ai/ricochet&type=date&legend=top-left)](https://www.star-history.com/#Grik-ai/ricochet&type=date&legend=top-left)

## License

Apache 2.0 © 2025 Igor Pryimak
