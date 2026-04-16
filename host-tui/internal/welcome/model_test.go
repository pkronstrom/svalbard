package welcome_test

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/host-tui/internal/welcome"
)

// stripAnsi removes ANSI escape sequences so tests can assert on plain text.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// sizedModel returns a welcome.Model after processing a WindowSizeMsg.
func sizedModel(w, h int) tea.Model {
	m := welcome.New()
	sized, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return sized
}

func TestWelcomeShowsInitAndPreset(t *testing.T) {
	m := sizedModel(80, 24)
	out := stripAnsi(m.View())

	for _, want := range []string{"Init Vault", "Choose Preset"} {
		if !strings.Contains(out, want) {
			t.Errorf("view should contain %q, got:\n%s", want, out)
		}
	}
}

func TestWelcomeShowsNoVaultStatus(t *testing.T) {
	m := sizedModel(80, 24)
	out := stripAnsi(m.View())

	if !strings.Contains(out, "no vault") {
		t.Errorf("view should contain 'no vault', got:\n%s", out)
	}
}

func TestWelcomeNavigateDownUp(t *testing.T) {
	m := welcome.New()

	// Initial selection should be 0
	if m.Selected() != 0 {
		t.Fatalf("initial selection should be 0, got %d", m.Selected())
	}

	// Move down: 0 -> 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m1 := updated.(welcome.Model)
	if m1.Selected() != 1 {
		t.Errorf("after move down, expected selected=1, got %d", m1.Selected())
	}

	// Move down again: should clamp at 1
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := updated.(welcome.Model)
	if m2.Selected() != 1 {
		t.Errorf("after second move down (clamp), expected selected=1, got %d", m2.Selected())
	}

	// Move up: 1 -> 0
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m3 := updated.(welcome.Model)
	if m3.Selected() != 0 {
		t.Errorf("after move up, expected selected=0, got %d", m3.Selected())
	}

	// Move up again: should clamp at 0
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m4 := updated.(welcome.Model)
	if m4.Selected() != 0 {
		t.Errorf("after second move up (clamp), expected selected=0, got %d", m4.Selected())
	}
}

func TestWelcomeQuitOnEsc(t *testing.T) {
	m := welcome.New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("pressing Esc should produce a command")
	}

	// Execute the command and verify it produces a tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Esc should produce tea.QuitMsg, got %T", msg)
	}
}
