package index

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

func TestIndexShowsBothTypes(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			SemanticEnabled: true,
		},
	})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	if !strings.Contains(out, "Keyword") {
		t.Errorf("View() should contain 'Keyword', got:\n%s", out)
	}
	if !strings.Contains(out, "Semantic") {
		t.Errorf("View() should contain 'Semantic', got:\n%s", out)
	}
}

func TestIndexNavigate(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			SemanticEnabled: true,
		},
	})
	m.width = 80
	m.height = 24

	// Start at keyword (selected=0)
	if m.selected != 0 {
		t.Fatalf("expected initial selected=0, got %d", m.selected)
	}

	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Move down to semantic
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("after j, expected selected=1 (semantic), got %d", m.selected)
	}

	// Clamp at bottom
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("should clamp at 1, got %d", m.selected)
	}

	// Move back up to keyword
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("after k, expected selected=0 (keyword), got %d", m.selected)
	}

	// Clamp at top
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("should clamp at 0, got %d", m.selected)
	}
}

func TestIndexEscEmitsBack(t *testing.T) {
	m := New(Config{})
	m.width = 80
	m.height = 24

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(escMsg)
	if cmd == nil {
		t.Fatal("pressing Esc should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestIndexShowsStatus(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			KeywordSources:  5,
			KeywordArticles: 1200,
		},
	})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	if !strings.Contains(out, "yes") {
		t.Errorf("View() with KeywordEnabled=true should contain 'yes', got:\n%s", out)
	}
}
