package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleReviewItems() []ReviewItem {
	return []ReviewItem{
		{ID: "kiwix-serve", Type: "binary", SizeGB: 0.003, Description: "Kiwix web server"},
		{ID: "osm-finland", Type: "pmtiles", SizeGB: 3.0, Description: "OpenStreetMap Finland"},
		{ID: "wikipedia-en-100", Type: "zim", SizeGB: 1.0, Description: "Wikipedia top 100 articles"},
		{ID: "survival-guide", Type: "zim", SizeGB: 0.5, Description: "Survival reference guide"},
	}
}

func TestReviewShowsSelectedItems(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)
	m.width = 120
	m.height = 30

	out := stripAnsi(m.View())

	if !strings.Contains(out, "/mnt/vault") {
		t.Errorf("expected vault path in view, got:\n%s", out)
	}

	for _, item := range items {
		if !strings.Contains(out, item.ID) {
			t.Errorf("expected item ID %q in view, got:\n%s", item.ID, out)
		}
	}
}

func TestReviewShowsTotal(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)
	m.width = 120
	m.height = 30

	out := stripAnsi(m.View())

	if !strings.Contains(out, "4 sources") {
		t.Errorf("expected '4 sources' in view, got:\n%s", out)
	}
	if !strings.Contains(out, "64") {
		t.Errorf("expected free space in view, got:\n%s", out)
	}
}

func TestReviewShowsTypeSymbols(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)
	m.width = 120
	m.height = 30

	out := m.View()

	// Should show type symbols
	if !strings.Contains(out, "⚙") {
		t.Errorf("expected tool symbol in view")
	}
	if !strings.Contains(out, "⊞") {
		t.Errorf("expected map symbol in view")
	}
	if !strings.Contains(out, "✦") {
		t.Errorf("expected reference symbol in view")
	}
}

func TestReviewConfirm(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd on Enter")
	}
	result := cmd()
	if _, ok := result.(reviewConfirmMsg); !ok {
		t.Fatalf("expected reviewConfirmMsg, got %T", result)
	}
}

func TestReviewCancel(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd on Esc")
	}
	result := cmd()
	if _, ok := result.(reviewBackMsg); !ok {
		t.Fatalf("expected reviewBackMsg, got %T", result)
	}
}

func TestReviewNavigates(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)
	m.width = 120
	m.height = 30

	// Move down
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ := m.Update(down)
	m = updated.(reviewModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	// Move up
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	updated, _ = m.Update(up)
	m = updated.(reviewModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", m.cursor)
	}
}

func TestReviewDetailChanges(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, nil, 64)
	m.width = 120
	m.height = 30

	// At cursor 0, detail should show first item's description
	out0 := stripAnsi(m.View())

	// Move to cursor 1
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ := m.Update(down)
	m = updated.(reviewModel)
	out1 := stripAnsi(m.View())

	if out0 == out1 {
		t.Error("detail pane should change when cursor moves")
	}
}

func TestReviewShowsUnavailableMissingItemsWithUnknownSize(t *testing.T) {
	m := newReviewModel("/mnt/vault", []ReviewItem{{ID: "known", Type: "zim", SizeGB: 1}, {ID: "missing", Unavailable: true}}, []string{"missing"}, 64)
	m.width = 80
	m.height = 8
	m.cursor = 1
	m.ensureVisible()

	out := stripAnsi(m.View())
	for _, want := range []string{"known-size subtotal", "missing", "unknown size", "Missing catalog recipes: missing"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() should contain %q, got:\n%s", want, out)
		}
	}
	if got := m.totalSizeGB(); got != 1 {
		t.Fatalf("totalSizeGB() = %v, want 1", got)
	}
}

func TestReviewReservesRowsForWrappedValidation(t *testing.T) {
	items := sampleReviewItems()
	plain := newReviewModel("/vault", items, nil, 0)
	plain.width, plain.height = 80, 30
	wrapped := newReviewModel("/vault", items, []string{"a-very-long-missing-recipe-id-that-wraps-in-the-right-pane"}, 0)
	wrapped.width, wrapped.height = 80, 30
	if wrapped.maxVisible() >= plain.maxVisible() {
		t.Fatalf("wrapped validation did not reserve rows: wrapped=%d plain=%d", wrapped.maxVisible(), plain.maxVisible())
	}
}

func TestValidationLinesCountsRenderedBreaks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		width int
		text  string
		want  int
	}{
		{"explicit newline", 80, "first\nsecond", 2},
		{"wide characters", 80, strings.Repeat("界", 30), 2},
		{"unknown width", 0, "first\nsecond", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validationLines(tc.width, tc.text); got != tc.want {
				t.Fatalf("rendered validation lines = %d, want %d", got, tc.want)
			}
		})
	}
}
