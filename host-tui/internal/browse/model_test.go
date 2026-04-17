package browse

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
)

// stripAnsi removes ANSI escape sequences for plain-text assertions.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// samplePackGroups returns two groups with one pack each for testing.
func samplePackGroups() []wizard.PackGroup {
	return []wizard.PackGroup{
		{
			Name: "Maps & Geodata",
			Packs: []wizard.Pack{
				{
					Name:        "OpenStreetMap",
					Description: "OSM planet data",
					Sources: []wizard.PackSource{
						{ID: "osm-planet", Type: "pmtiles", Description: "Planet PMTiles", SizeGB: 80},
						{ID: "osm-extract", Type: "pmtiles", Description: "Regional Extract", SizeGB: 5},
					},
				},
			},
		},
		{
			Name: "Reference",
			Packs: []wizard.Pack{
				{
					Name:        "Wikipedia",
					Description: "Offline Wikipedia",
					Sources: []wizard.PackSource{
						{ID: "wiki-en", Type: "zim", Description: "English Wikipedia", SizeGB: 95},
					},
				},
			},
		},
	}
}

// sizedBrowse creates a browse model with sample data and applies a window size.
func sizedBrowse(cfg Config) Model {
	m := New(cfg)
	m.width = 100
	m.height = 40
	return m
}

func TestBrowseShowsPackGroups(t *testing.T) {
	cfg := Config{
		PackGroups: samplePackGroups(),
		FreeGB:     500,
	}
	m := sizedBrowse(cfg)
	out := stripAnsi(m.View())

	for _, want := range []string{"Maps & Geodata", "OpenStreetMap", "Reference", "Wikipedia"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() should contain %q, got:\n%s", want, out)
		}
	}
}

func TestBrowseToggleItem(t *testing.T) {
	saveCalled := false
	cfg := Config{
		PackGroups:  samplePackGroups(),
		FreeGB:      500,
		SaveDesired: func(ids []string) error { saveCalled = true; return nil },
	}
	m := sizedBrowse(cfg)

	// Cursor starts at row 0 (group header "Maps & Geodata").
	// Move down to the first pack row, then down again to the first item row.
	down := tea.KeyMsg{Type: tea.KeyDown}
	space := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}

	// Navigate to the first item (osm-planet).
	// Row 0: group "Maps & Geodata"
	// Row 1: pack "OpenStreetMap"
	// Packs start collapsed, so we need to expand first.
	result, _ := m.Update(down)
	m = result.(Model)
	// cursor=1 is the pack "OpenStreetMap" - expand it
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ = m.Update(enter)
	m = result.(Model)
	// Now items are visible. Move down to the first source.
	result, _ = m.Update(down)
	m = result.(Model)

	// Verify the item is not checked initially.
	if m.picker.CheckedIDs["osm-planet"] {
		t.Fatal("osm-planet should not be checked initially")
	}

	// Toggle it.
	result, _ = m.Update(space)
	m = result.(Model)

	if !m.picker.CheckedIDs["osm-planet"] {
		t.Error("osm-planet should be checked after space toggle")
	}

	// Toggle again to uncheck.
	result, _ = m.Update(space)
	m = result.(Model)

	if m.picker.CheckedIDs["osm-planet"] {
		t.Error("osm-planet should be unchecked after second space toggle")
	}

	if saveCalled {
		t.Error("save should not have been called by toggle alone")
	}
}

func TestBrowseEscWithNoChanges(t *testing.T) {
	cfg := Config{
		PackGroups:  samplePackGroups(),
		FreeGB:      500,
		SaveDesired: func(ids []string) error { return nil },
	}
	m := sizedBrowse(cfg)

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(esc)
	if cmd == nil {
		t.Fatal("esc with no changes should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("esc with no changes should emit BackMsg, got %T", msg)
	}
}

func TestBrowseEscWithChanges(t *testing.T) {
	cfg := Config{
		PackGroups:  samplePackGroups(),
		FreeGB:      500,
		SaveDesired: func(ids []string) error { return nil },
	}
	m := sizedBrowse(cfg)

	// Make a change: directly toggle an ID so the model becomes dirty.
	m.picker.CheckedIDs["osm-planet"] = true

	esc := tea.KeyMsg{Type: tea.KeyEscape}
	result, cmd := m.Update(esc)
	m = result.(Model)

	if cmd != nil {
		t.Fatal("esc with changes should not immediately produce a command (should show save prompt)")
	}
	if !m.showSavePrompt {
		t.Fatal("esc with changes should show save prompt")
	}

	// Press 'n' to discard.
	nKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	_, cmd = m.Update(nKey)
	if cmd == nil {
		t.Fatal("'n' on save prompt should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("'n' on save prompt should emit BackMsg, got %T", msg)
	}
}

func TestBrowseSave(t *testing.T) {
	var savedIDs []string
	cfg := Config{
		PackGroups: samplePackGroups(),
		FreeGB:     500,
		SaveDesired: func(ids []string) error {
			savedIDs = ids
			return nil
		},
	}
	m := sizedBrowse(cfg)

	// Make a change.
	m.picker.CheckedIDs["wiki-en"] = true

	// Press esc to trigger save prompt.
	esc := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.Update(esc)
	m = result.(Model)

	if !m.showSavePrompt {
		t.Fatal("should show save prompt")
	}

	// Press 'y' to save.
	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	_, cmd := m.Update(yKey)
	if cmd == nil {
		t.Fatal("'y' on save prompt should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(SavedMsg); !ok {
		t.Errorf("'y' on save prompt should emit SavedMsg, got %T", msg)
	}

	if len(savedIDs) == 0 {
		t.Error("SaveDesired should have been called with checked IDs")
	}

	found := false
	for _, id := range savedIDs {
		if id == "wiki-en" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("saved IDs should contain 'wiki-en', got %v", savedIDs)
	}
}

func TestBrowseReadOnly(t *testing.T) {
	cfg := Config{
		PackGroups:  samplePackGroups(),
		FreeGB:      500,
		SaveDesired: nil, // read-only mode
	}
	m := sizedBrowse(cfg)

	if !m.readOnly {
		t.Fatal("model should be in read-only mode when SaveDesired is nil")
	}

	// Navigate to an item and try to toggle.
	down := tea.KeyMsg{Type: tea.KeyDown}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	space := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}

	// Expand first pack.
	result, _ := m.Update(down)
	m = result.(Model)
	result, _ = m.Update(enter)
	m = result.(Model)
	result, _ = m.Update(down)
	m = result.(Model)

	// Try toggle - should be ignored in read-only mode.
	result, _ = m.Update(space)
	m = result.(Model)

	if m.picker.CheckedIDs["osm-planet"] {
		t.Error("toggle should be ignored in read-only mode")
	}

	// Esc should emit BackMsg directly (no save prompt).
	esc := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(esc)
	if cmd == nil {
		t.Fatal("esc in read-only mode should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("esc in read-only mode should emit BackMsg, got %T", msg)
	}
}
