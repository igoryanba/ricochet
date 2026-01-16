# Backend Development Guidelines

## Go Coding Standards
*   **Error Handling**: Always wrap errors with `fmt.Errorf("action: %w", err)`.
*   **Logging**: Use `log.Printf` for important events.
*   **Context**: Pass `ctx context.Context` as the first argument to functions that involve I/O.
*   **Testing**: Write table-driven tests for all logic.

## Architecture
*   **Controller**: Handles HTTP/RPC requests.
*   **Service**: Contains business logic.
*   **Repository**: database access (if any).

## Ricochet Specifics
*   **Tools**: Defined in `core/internal/tools`.
*   **Protocol**: Types in `core/internal/protocol`.
