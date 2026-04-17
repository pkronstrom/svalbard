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

func TestOpenVaultShowsInput(t *testing.T) {
	m := sizedModel()
	out := stripAnsi(m.View())

	if !strings.Contains(out, "Path:") {
		t.Errorf("View() should contain 'Path:', got:\n%s", out)
	}
}

func TestOpenVaultTypeAndBackspace(t *testing.T) {
	m := sizedModel()

	// Type "/tmp"
	for _, r := range "/tmp" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(Model)
	}

	if m.input != "/tmp" {
		t.Errorf("after typing '/tmp', expected input='/tmp', got %q", m.input)
	}

	// Backspace once
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result.(Model)

	if m.input != "/tm" {
		t.Errorf("after backspace, expected input='/tm', got %q", m.input)
	}

	// Backspace three more times to empty
	for i := 0; i < 3; i++ {
		result, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = result.(Model)
	}

	if m.input != "" {
		t.Errorf("after clearing input, expected empty string, got %q", m.input)
	}

	// Backspace on empty should not panic
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result.(Model)

	if m.input != "" {
		t.Errorf("backspace on empty input should remain empty, got %q", m.input)
	}
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

func TestOpenVaultInvalidPath(t *testing.T) {
	m := sizedModel()

	// Type a path that almost certainly does not have a manifest.yaml.
	path := "/tmp/svalbard-nonexistent-test-dir-12345"
	for _, r := range path {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(Model)
	}

	// Press Enter to validate.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	// Should not emit a command (validation failed).
	if cmd != nil {
		t.Error("enter with invalid path should not produce a command")
	}

	// Should have an error message.
	if m.errMsg == "" {
		t.Fatal("errMsg should be set after invalid path submission")
	}

	// Error should be visible in the View (may be line-wrapped, so check a
	// short unique fragment instead of the full message).
	out := stripAnsi(m.View())
	if !strings.Contains(out, "manifest.yaml") {
		t.Errorf("View() should contain 'manifest.yaml' error text, got:\n%s", out)
	}
}
