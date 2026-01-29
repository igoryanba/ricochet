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

## 🚀 Features

*   **Swarm Mode (Parallel Execution)**: Uses a DAG-based planner to spawn multiple workers (up to 5+) for handling independent tasks simultaneously.
*   **Plan Mode**: A dedicated planning engine that tracks task lifecycle (pending, active, verification) and persists plans across sessions.
*   **Ether Mode (Remote Bridge)**: Control your agent remotely via Telegram (voice/text). Start IDE sessions and approve sensitive actions while away from your keyboard.
*   **4-Level Context Management**: Optimizes memory via de-duplication, eviction, and summarization to prevent "hallucinations" on long tasks.
*   **Shadow Git**: Every task has a hidden git checkpoint. Instantly undo/redo AI-generated code without polluting your main project history.
*   **CLI & VS Code**: Full feature parity between the Visual Studio Code extension and the standalone Terminal User Interface (TUI).

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

## License

Apache 2.0 © 2025 Igor Pryimak
