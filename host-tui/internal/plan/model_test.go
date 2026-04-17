package plan

import (
	"context"
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

func sampleItems() []PlanItem {
	return []PlanItem{
		{ID: "wiki-physics", Type: "zim", SizeGB: 1.2, Description: "Wikipedia Physics", Action: "download"},
		{ID: "wiki-math", Type: "zim", SizeGB: 0.8, Description: "Wikipedia Math", Action: "download"},
		{ID: "wiki-old", Type: "zim", SizeGB: 0.5, Description: "Wikipedia Old", Action: "remove"},
	}
}

func TestPlanShowsItems(t *testing.T) {
	items := sampleItems()
	m := New(Config{
		Items:      items,
		DownloadGB: 2.0,
		RemoveGB:   0.5,
	})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	for _, it := range items {
		if !strings.Contains(out, it.Description) {
			t.Errorf("View() should contain item description %q, got:\n%s", it.Description, out)
		}
	}
}

func TestPlanEmptyShowsInSync(t *testing.T) {
	m := New(Config{})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	lower := strings.ToLower(out)

	if !strings.Contains(lower, "sync") && !strings.Contains(lower, "in sync") {
		t.Errorf("View() with no items should contain 'sync' or 'in sync', got:\n%s", out)
	}
}

func TestPlanEscEmitsBack(t *testing.T) {
	m := New(Config{Items: sampleItems()})
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

func TestPlanBKeyEmitsBrowse(t *testing.T) {
	m := New(Config{Items: sampleItems()})
	m.width = 80
	m.height = 24

	bMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	_, cmd := m.Update(bMsg)
	if cmd == nil {
		t.Fatal("pressing 'b' should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BrowseMsg); !ok {
		t.Errorf("expected BrowseMsg, got %T", msg)
	}
}

func TestPlanScrolling(t *testing.T) {
	items := sampleItems()
	m := New(Config{Items: items})
	m.width = 80
	m.height = 24

	// Start at cursor 0
	if m.cursor != 0 {
		t.Fatalf("expected initial cursor=0, got %d", m.cursor)
	}

	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Move down
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 1 {
		t.Errorf("after j, expected cursor=1, got %d", m.cursor)
	}

	// Move down again
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 2 {
		t.Errorf("after second j, expected cursor=2, got %d", m.cursor)
	}

	// Clamp at bottom
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 2 {
		t.Errorf("should clamp at %d, got %d", len(items)-1, m.cursor)
	}

	// Move up
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.cursor != 1 {
		t.Errorf("after k, expected cursor=1, got %d", m.cursor)
	}

	// Back to top and clamp
	result, _ = m.Update(upMsg)
	m = result.(Model)
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.cursor != 0 {
		t.Errorf("should clamp at 0, got %d", m.cursor)
	}
}

func TestPlanEnterStartsApply(t *testing.T) {
	items := sampleItems()
	m := New(Config{
		Items:      items,
		DownloadGB: 2.0,
		RemoveGB:   0.5,
		RunApply: func(ctx context.Context, onProgress func(ApplyEvent)) error {
			return nil
		},
	})
	m.width = 80
	m.height = 24

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if !m.applying {
		t.Error("after Enter with RunApply set, expected applying=true")
	}
	if cmd == nil {
		t.Error("after Enter with RunApply set, expected a non-nil command")
	}
	if len(m.applyItems) != len(items) {
		t.Errorf("expected %d applyItems, got %d", len(items), len(m.applyItems))
	}
}
