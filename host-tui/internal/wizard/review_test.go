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
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	// Vault path should appear
	if !strings.Contains(out, "/mnt/vault") {
		t.Errorf("expected vault path in view, got:\n%s", out)
	}

	// All item IDs should appear
	for _, item := range items {
		if !strings.Contains(out, item.ID) {
			t.Errorf("expected item ID %q in view, got:\n%s", item.ID, out)
		}
	}
}

func TestReviewShowsTotal(t *testing.T) {
	items := sampleReviewItems()
	// Total: 0.003 + 3.0 + 1.0 + 0.5 = 4.503 GB
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	// Should show total size (4.5 GB)
	if !strings.Contains(out, "4.5 GB") {
		t.Errorf("expected total '4.5 GB' in view, got:\n%s", out)
	}

	// Should show item count
	if !strings.Contains(out, "4 sources") {
		t.Errorf("expected '4 sources' in view, got:\n%s", out)
	}

	// Should show free space
	if !strings.Contains(out, "64") {
		t.Errorf("expected free space '64' in view, got:\n%s", out)
	}
}

func TestReviewConfirm(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, 64)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

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
	m := newReviewModel("/mnt/vault", items, 64)

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected a non-nil Cmd on Esc")
	}

	result := cmd()
	if _, ok := result.(reviewBackMsg); !ok {
		t.Fatalf("expected reviewBackMsg, got %T", result)
	}
}

func TestReviewGroupsByType(t *testing.T) {
	items := sampleReviewItems()
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 40

	out := stripAnsi(m.View())

	// Types should appear uppercased
	if !strings.Contains(out, "BINARY") {
		t.Errorf("expected 'BINARY' type header in view, got:\n%s", out)
	}
	if !strings.Contains(out, "PMTILES") {
		t.Errorf("expected 'PMTILES' type header in view, got:\n%s", out)
	}
	if !strings.Contains(out, "ZIM") {
		t.Errorf("expected 'ZIM' type header in view, got:\n%s", out)
	}
}

func TestReviewScrolls(t *testing.T) {
	// Create many items to force scrolling
	var items []ReviewItem
	for i := 0; i < 30; i++ {
		items = append(items, ReviewItem{
			ID:          "item-" + string(rune('a'+i%26)),
			Type:        "zim",
			SizeGB:      1.0,
			Description: "Test item",
		})
	}

	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 10 // small height to force scrolling

	// Press j to scroll down
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	var tm tea.Model = m
	for i := 0; i < 5; i++ {
		tm, _ = tm.(reviewModel).Update(down)
	}

	rm := tm.(reviewModel)
	if rm.scrollOffset <= 0 {
		t.Errorf("expected scrollOffset > 0 after scrolling down, got %d", rm.scrollOffset)
	}

	// Press k to scroll back up
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	tm, _ = rm.Update(up)
	rm = tm.(reviewModel)

	if rm.scrollOffset >= 5 {
		t.Errorf("expected scrollOffset < 5 after scrolling up, got %d", rm.scrollOffset)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string][]ReviewItem{
		"zim":     {{ID: "a"}},
		"binary":  {{ID: "b"}},
		"pmtiles": {{ID: "c"}},
	}

	keys := sortedKeys(m)

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "binary" || keys[1] != "pmtiles" || keys[2] != "zim" {
		t.Errorf("expected sorted keys [binary pmtiles zim], got %v", keys)
	}
}
