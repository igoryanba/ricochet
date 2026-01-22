package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// MemoryManager handles project-specific long-term memory
type MemoryManager struct {
	workspacePath string
	memoryFile    string
}

// NewMemoryManager creates a manager for .ricochet/MEMORY.md
func NewMemoryManager(workspacePath string) *MemoryManager {
	return &MemoryManager{
		workspacePath: workspacePath,
		memoryFile:    filepath.Join(workspacePath, ".ricochet", "MEMORY.md"),
	}
}

// Load reads the memory content
func (m *MemoryManager) Load() (string, error) {
	if _, err := os.Stat(m.memoryFile); os.IsNotExist(err) {
		return "", nil
	}
	content, err := os.ReadFile(m.memoryFile)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Save writes content to memory
func (m *MemoryManager) Save(content string) error {
	dir := filepath.Dir(m.memoryFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(m.memoryFile, []byte(content), 0644)
}

// Add appends content to memory
func (m *MemoryManager) Add(content string) error {
	existing, err := m.Load()
	if err != nil {
		return err
	}
	newContent := existing + "\n" + content
	return m.Save(newContent)
}

// Clear deletes memory
func (m *MemoryManager) Clear() error {
	if _, err := os.Stat(m.memoryFile); err == nil {
		return os.Remove(m.memoryFile)
	}
	return nil
}

// GetSystemPromptPart returns the memory formatted for a system prompt
func (m *MemoryManager) GetSystemPromptPart() string {
	content, _ := m.Load()
	if content == "" {
		return ""
	}
	return fmt.Sprintf("\n\n# Project Memory\nThe following information is recorded in your long-term memory for this project:\n\n%s\n", content)
}
