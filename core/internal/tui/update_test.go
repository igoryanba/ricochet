package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_TabToggle(t *testing.T) {
	// Initialize minimal model
	m := Model{
		Textarea:        textarea.New(),
		Viewport:        viewport.New(80, 20),
		IsShellFocused:  false, // Start focused on Input
		ShowSuggestions: false,
	}

	// 1. Send Tab -> Should toggle to Shell Focus
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newM, _ := m.Update(msg)
	newModel := newM.(Model)

	if !newModel.IsShellFocused {
		t.Error("Expected IsShellFocused to be true after Tab")
	}
	if newModel.Textarea.Focused() {
		t.Error("Expected Textarea to be blurred (not Focused) when Shell is focused")
	}

	// 2. Send Tab again -> Should toggle back to Input Focus
	newM2, _ := newModel.Update(msg)
	newModel2 := newM2.(Model)

	if newModel2.IsShellFocused {
		t.Error("Expected IsShellFocused to be false after second Tab")
	}
	if !newModel2.Textarea.Focused() {
		t.Error("Expected Textarea to be Focused when Input is focused")
	}
}

func TestUpdate_TabSelectsSuggestion(t *testing.T) {
	// Initialize model with suggestions open
	ta := textarea.New()
	ta.SetValue("/") // Vital: Set value so updateSuggestions doesn't clear suggestions

	m := Model{
		Textarea:           ta,
		Viewport:           viewport.New(80, 20),
		IsShellFocused:     false,
		ShowSuggestions:    true,
		AllCommands:        []string{"/help", "/exit"}, // Vital for updateSuggestions
		Suggestions:        []string{"/help", "/exit"},
		SelectedSuggestion: 0,
	}

	// 1. Send Tab -> Should NOT toggle focus, should select suggestion
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newM, _ := m.Update(msg)
	newModel := newM.(Model)

	// Focus should remain unchanged
	if newModel.IsShellFocused {
		t.Error("Expected IsShellFocused to be false (unchanged) when suggestions handle Tab")
	}

	// Textarea should contain suggestion
	if newModel.Textarea.Value() != "/help" { // /help is auto-exec, no space
		t.Errorf("Expected Textarea value to be '/help', got '%s'", newModel.Textarea.Value())
	}
}
