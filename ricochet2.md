# Ricochet Project Memory

> [!IMPORTANT]
> This file contains critical project context, architectural decisions, and "Do's and Don'ts" for the AI Agent. Always consult this file when planning major changes or if you are unsure about the project structure.

## Core Architecture
- **Host Abstraction**: All OS interactions must go through `host.Host` interface. Do not use `os` package directly for file/exec unless inside a Host implementation.
- **RPC Protocol**: Communication between Core and Extension uses JSON-RPC over Stdio. Use `protocol.RPCMessage`.
- **Agent Mode**: The agent operates in modes (Code, Architect, Ask). Mode logic is in `core/internal/modes`.

## Coding Standards
- **Go**: Use standard formatting. Error handling must wrap errors: `fmt.Errorf("failed to x: %w", err)`.
- **React**: Use functional components and hooks. TailwindCSS for styling.
- **LSP**: Use `get_diagnostics` and `get_definitions` for code intelligence.

## Do's and Don'ts
- **DO** use `task_boundary` frequently to keep the user informed.
- **DO NOT** commit direct changes to `main` without verification.
- **DO** maintain the `task.md` checklist.

## Workflow Automation
- Custom workflows are defined in `.agent/workflows/*.md`.
- Use `/` command in Chat to trigger them.
