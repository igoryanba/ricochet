package handoff

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// GenerateFunc function signature for text generation
type GenerateFunc func(ctx context.Context, prompt string) (string, error)

type Service struct {
	generator GenerateFunc
}

func NewService(generator GenerateFunc) *Service {
	return &Service{generator: generator}
}

// GenerateSpec creates a specification summary from the message history
func (s *Service) GenerateSpec(ctx context.Context, messages []protocol.Message) (string, error) {
	// 1. Format history for the prompt
	var historyBuilder strings.Builder
	for _, msg := range messages {
		// Skip large tool outputs provided they aren't critical, but for now include everything truncated
		content := msg.Content
		if len(content) > 2000 {
			content = content[:2000] + "... (truncated)"
		}
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", strings.ToUpper(msg.Role), content))
	}

	// 2. Construct Prompt
	prompt := fmt.Sprintf(`
You are the "Reflex Engine" of an advanced AI agent.
Your goal is to perform an "Intelligent Handoff" by condensing the following conversation history into a technical specification (SPEC.md).
The agent is switching modes (e.g., from Architect to Code). The new mode needs a clean state but MUST know what was decided.

CONVERSATION HISTORY:
%s

INSTRUCTIONS:
Create a Markdown specification that includes:
1. **Goal**: What is the user trying to achieve?
2. **Decisions**: Key architectural or design decisions agreed upon.
3. **Plan**: The step-by-step plan that was devised.
4. **Context**: Any specific file paths, constraints, or libraries mentioned that are crucial.

Output ONLY the Markdown content for SPEC.md. Do not include introductory text.
`, historyBuilder.String())

	// 3. Call Generator
	return s.generator(ctx, prompt)
}

// SaveSpec writes the spec to SPEC.md in the workspace root or .ricochet folder
func (s *Service) SaveSpec(cwd string, content string) error {
	// Create .ricochet dir if not exists
	ricoDir := filepath.Join(cwd, ".ricochet")
	if err := os.MkdirAll(ricoDir, 0755); err != nil {
		return err
	}

	specPath := filepath.Join(ricoDir, "SPEC.md")
	return os.WriteFile(specPath, []byte(content), 0644)
}
