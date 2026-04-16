package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

func TestPaletteModelShowsEntries(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "plan", Label: "Plan"},
		{ID: "apply", Label: "Apply"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())
	out := m.View()

	if !strings.Contains(out, "Plan") {
		t.Errorf("expected View to contain 'Plan', got:\n%s", out)
	}
	if !strings.Contains(out, "Apply") {
		t.Errorf("expected View to contain 'Apply', got:\n%s", out)
	}
}

func TestPaletteModelFiltersOnType(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "plan", Label: "Plan"},
		{ID: "apply", Label: "Apply"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())

	// Type "p" — should filter to entries containing "p"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	out := updated.(tui.PaletteModel).View()

	// "Plan" should match (contains "p" case-insensitive)
	if !strings.Contains(out, "Plan") {
		t.Errorf("expected 'Plan' in filtered view, got:\n%s", out)
	}

	// "Apply" also contains "p", so let's type a more specific query
	// to verify filtering actually works: type "lan"
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated3, _ := updated2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated4, _ := updated3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	out2 := updated4.(tui.PaletteModel).View()

	if !strings.Contains(out2, "Plan") {
		t.Errorf("expected 'Plan' in filtered view for 'plan', got:\n%s", out2)
	}
	if strings.Contains(stripANSI(out2), "Apply") {
		t.Errorf("expected 'Apply' to be filtered out for query 'plan', got:\n%s", out2)
	}
}

func TestPaletteModelEscCloses(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "plan", Label: "Plan"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a command from Esc, got nil")
	}

	msg := cmd()
	if _, ok := msg.(tui.PaletteCloseMsg); !ok {
		t.Errorf("expected PaletteCloseMsg, got %T", msg)
	}
}

func TestPaletteModelEnterSelects(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "plan", Label: "Plan"},
		{ID: "apply", Label: "Apply"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from Enter, got nil")
	}

	msg := cmd()
	selectMsg, ok := msg.(tui.PaletteSelectMsg)
	if !ok {
		t.Fatalf("expected PaletteSelectMsg, got %T", msg)
	}
	if selectMsg.Entry.ID != "plan" {
		t.Errorf("expected selected entry ID 'plan', got %q", selectMsg.Entry.ID)
	}
}

func TestPaletteModelNavigates(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
		{ID: "c", Label: "Gamma"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())

	// Move down with arrow key
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify we moved down: press Enter and check selection
	_, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter after down")
	}
	msg := cmd()
	selectMsg := msg.(tui.PaletteSelectMsg)
	if selectMsg.Entry.ID != "b" {
		t.Errorf("after down, expected selected 'b', got %q", selectMsg.Entry.ID)
	}

	// Move down again
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Move up with arrow key
	updated3, _ := updated2.Update(tea.KeyMsg{Type: tea.KeyUp})

	_, cmd2 := updated3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 == nil {
		t.Fatal("expected command from Enter after up")
	}
	msg2 := cmd2()
	selectMsg2 := msg2.(tui.PaletteSelectMsg)
	if selectMsg2.Entry.ID != "b" {
		t.Errorf("after down,down,up expected selected 'b', got %q", selectMsg2.Entry.ID)
	}

	// Test clamping at top: move up from 0
	m2 := tui.NewPaletteModel(entries, tui.DefaultTheme())
	updated4, _ := m2.Update(tea.KeyMsg{Type: tea.KeyUp})
	_, cmd3 := updated4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg3 := cmd3()
	selectMsg3 := msg3.(tui.PaletteSelectMsg)
	if selectMsg3.Entry.ID != "a" {
		t.Errorf("clamped at top, expected 'a', got %q", selectMsg3.Entry.ID)
	}

	// Test clamping at bottom
	m3 := tui.NewPaletteModel(entries, tui.DefaultTheme())
	down1, _ := m3.Update(tea.KeyMsg{Type: tea.KeyDown})
	down2, _ := down1.Update(tea.KeyMsg{Type: tea.KeyDown})
	down3, _ := down2.Update(tea.KeyMsg{Type: tea.KeyDown}) // should clamp
	_, cmd4 := down3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg4 := cmd4()
	selectMsg4 := msg4.(tui.PaletteSelectMsg)
	if selectMsg4.Entry.ID != "c" {
		t.Errorf("clamped at bottom, expected 'c', got %q", selectMsg4.Entry.ID)
	}
}

func TestPaletteModelJKTypesCharacters(t *testing.T) {
	entries := []tui.PaletteEntry{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
	}
	m := tui.NewPaletteModel(entries, tui.DefaultTheme())

	// Typing 'j' should add it to query, not navigate
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	out := updated.(tui.PaletteModel).View()
	if !strings.Contains(out, "j") {
		t.Errorf("typing 'j' should appear in query input, got:\n%s", out)
	}

	// Typing 'k' should add it to query, not navigate
	updated2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	out2 := updated2.(tui.PaletteModel).View()
	if !strings.Contains(out2, "jk") {
		t.Errorf("typing 'k' after 'j' should show 'jk' in query, got:\n%s", out2)
	}
}
