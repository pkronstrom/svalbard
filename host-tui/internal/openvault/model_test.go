package openvault

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stripAnsi removes ANSI escape sequences for plain-text assertions.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// sizedModel returns an openvault Model after processing a WindowSizeMsg.
func sizedModel() Model {
	m := New()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return result.(Model)
}

func TestOpenVaultEscEmitsBack(t *testing.T) {
	m := sizedModel()

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(esc)
	if cmd == nil {
		t.Fatal("pressing Esc should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("Esc should produce BackMsg, got %T", msg)
	}
}

func TestOpenVaultQEmitsBack(t *testing.T) {
	m := sizedModel()

	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(q)
	if cmd == nil {
		t.Fatal("pressing q should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("q should produce BackMsg, got %T", msg)
	}
}

func TestOpenVaultShowsCurrentDirectory(t *testing.T) {
	m := sizedModel()
	out := stripAnsi(m.View())

	// The view should show the current directory path.
	if !strings.Contains(out, m.picker.CurrentDirectory) {
		t.Errorf("View() should contain current directory %q, got:\n%s", m.picker.CurrentDirectory, out)
	}
}

func TestOpenVaultShowsOpenVaultStatus(t *testing.T) {
	m := sizedModel()
	out := stripAnsi(m.View())

	if !strings.Contains(out, "open vault") {
		t.Errorf("View() should contain 'open vault' status, got:\n%s", out)
	}
}

func TestOpenVaultDirOnlyMode(t *testing.T) {
	m := New()

	if !m.picker.DirAllowed {
		t.Error("filepicker should allow directory selection")
	}
	if m.picker.FileAllowed {
		t.Error("filepicker should not allow file selection")
	}
}

func TestOpenVaultValidateNoManifest(t *testing.T) {
	m := sizedModel()

	// Validate a path that certainly has no manifest.yaml.
	cmd := m.validate("/tmp")
	if cmd != nil {
		t.Error("validate should return nil cmd for path without manifest.yaml")
	}
}
