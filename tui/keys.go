package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// KeyBinding represents a single keyboard binding with an optional alternative
// key and a human-readable label for help/footer display.
type KeyBinding struct {
	Key   string // primary key string (e.g., "j", "enter", "ctrl+k")
	Alt   string // alternative key string (e.g., "down" for arrow key)
	Label string // display label for help text (e.g., "j/k: move")
}

// Matches returns true if the given key message matches either the primary
// Key or the Alt key of this binding.
func (kb KeyBinding) Matches(msg tea.KeyMsg) bool {
	s := msg.String()
	if s == kb.Key {
		return true
	}
	if kb.Alt != "" && s == kb.Alt {
		return true
	}
	return false
}

// KeyMap holds the complete set of keyboard bindings for Svalbard TUI apps.
type KeyMap struct {
	MoveUp     KeyBinding
	MoveDown   KeyBinding
	Enter      KeyBinding
	Back       KeyBinding
	Filter     KeyBinding
	Palette    KeyBinding
	Toggle     KeyBinding
	SwitchPane KeyBinding
	Quit       KeyBinding
	ForceQuit  KeyBinding
}

// DefaultKeyMap returns the standard Svalbard keyboard bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		MoveUp:     KeyBinding{Key: "k", Alt: "up", Label: "j/k: move"},
		MoveDown:   KeyBinding{Key: "j", Alt: "down", Label: ""},
		Enter:      KeyBinding{Key: "enter", Label: "Enter: open"},
		Back:       KeyBinding{Key: "esc", Label: "Esc: back"},
		Filter:     KeyBinding{Key: "/", Label: "/: filter"},
		Palette:    KeyBinding{Key: "ctrl+k", Label: "Ctrl+K: palette"},
		Toggle:     KeyBinding{Key: " ", Label: "Space: toggle"},
		SwitchPane: KeyBinding{Key: "tab", Label: "Tab: switch pane"},
		Quit:       KeyBinding{Key: "q", Label: "q: quit"},
		ForceQuit:  KeyBinding{Key: "ctrl+c", Label: ""},
	}
}

// FooterHints joins the Labels of the given bindings with " | " as a separator.
// Bindings with an empty Label are skipped.
func FooterHints(bindings ...KeyBinding) string {
	var parts []string
	for _, b := range bindings {
		if b.Label != "" {
			parts = append(parts, b.Label)
		}
	}
	return strings.Join(parts, " | ")
}
