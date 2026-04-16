package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

func TestKeyMatchMoveDown(t *testing.T) {
	km := tui.DefaultKeyMap()

	// "j" should match MoveDown
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	if !km.MoveDown.Matches(jMsg) {
		t.Error("expected 'j' to match MoveDown")
	}

	// down arrow should match MoveDown
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	if !km.MoveDown.Matches(downMsg) {
		t.Error("expected down arrow to match MoveDown")
	}

	// "x" should not match MoveDown
	xMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	if km.MoveDown.Matches(xMsg) {
		t.Error("expected 'x' NOT to match MoveDown")
	}
}

func TestKeyMatchMoveUp(t *testing.T) {
	km := tui.DefaultKeyMap()

	// "k" should match MoveUp
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	if !km.MoveUp.Matches(kMsg) {
		t.Error("expected 'k' to match MoveUp")
	}

	// up arrow should match MoveUp
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	if !km.MoveUp.Matches(upMsg) {
		t.Error("expected up arrow to match MoveUp")
	}

	// "x" should not match MoveUp
	xMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	if km.MoveUp.Matches(xMsg) {
		t.Error("expected 'x' NOT to match MoveUp")
	}
}

func TestFooterHintsSkipsEmpty(t *testing.T) {
	km := tui.DefaultKeyMap()

	// MoveDown has empty label, should be skipped
	result := tui.FooterHints(km.MoveUp, km.MoveDown, km.Enter)
	expected := "j/k: move | Enter: open"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFooterHintsJoinsWithPipe(t *testing.T) {
	km := tui.DefaultKeyMap()

	result := tui.FooterHints(km.MoveUp, km.Enter, km.Back)
	expected := "j/k: move | Enter: open | Esc: back"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
