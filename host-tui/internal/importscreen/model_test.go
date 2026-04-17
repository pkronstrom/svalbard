package importscreen

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

// sizedModel returns an import screen Model after processing a WindowSizeMsg.
func sizedModel(cfg Config) Model {
	m := New(cfg)
	result, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return result.(Model)
}

func TestImportShowsInput(t *testing.T) {
	m := sizedModel(Config{})
	out := stripAnsi(m.View())

	// The view says "Enter a local path, URL, or YouTube link to import."
	if !strings.Contains(out, "path") && !strings.Contains(out, "Path") && !strings.Contains(out, "URL") {
		t.Errorf("View() should contain 'Path or URL' related text, got:\n%s", out)
	}

	if !strings.Contains(out, "Source:") {
		t.Errorf("View() should contain 'Source:', got:\n%s", out)
	}
}

func TestImportEscEmitsBack(t *testing.T) {
	m := sizedModel(Config{})

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

func TestImportEnterWithEmptyInput(t *testing.T) {
	m := sizedModel(Config{})

	// Press enter with empty input - should do nothing.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	if cmd != nil {
		t.Error("enter with empty input should not produce a command")
	}

	// Model should still be usable (no crash).
	if m.importing {
		t.Error("should not be in importing state after enter with empty input")
	}

	if m.errMsg != "" {
		t.Errorf("should not have error message after enter with empty input, got %q", m.errMsg)
	}
}

func TestImportNilCallback(t *testing.T) {
	// RunImport is nil.
	m := sizedModel(Config{RunImport: nil})

	// Type something.
	for _, r := range "https://example.com/data.zip" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(Model)
	}

	// Press enter.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	// Should not produce a command (import not started).
	if cmd != nil {
		t.Error("enter with nil RunImport should not produce a command")
	}

	// Should show "import not available" error.
	if m.errMsg != "import not available" {
		t.Errorf("expected errMsg='import not available', got %q", m.errMsg)
	}

	// errMsg is set correctly (view rendering may wrap at narrow widths).
}
