package tui_test

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

func TestNavListRendersCaretOnSelected(t *testing.T) {
	nl := tui.NavList{
		Items: []tui.NavItem{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
		Selected: 1,
		Theme:    tui.DefaultTheme(),
	}

	out := nl.Render()
	lines := strings.Split(out, "\n")

	// Filter out empty lines
	var nonEmpty []string
	for _, l := range lines {
		if l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}

	if len(nonEmpty) != 3 {
		t.Fatalf("expected 3 non-empty lines, got %d:\n%s", len(nonEmpty), out)
	}

	// Line 0 (Alpha) should NOT have caret
	if strings.Contains(nonEmpty[0], "> ") {
		t.Errorf("line 0 should not have caret, got: %q", nonEmpty[0])
	}
	// Line 0 should have caret space
	if !strings.Contains(nonEmpty[0], "  ") {
		t.Errorf("line 0 should have caret space, got: %q", nonEmpty[0])
	}

	// Line 1 (Beta) should have caret
	if !strings.Contains(nonEmpty[1], "> ") {
		t.Errorf("line 1 should have caret, got: %q", nonEmpty[1])
	}

	// Line 2 (Gamma) should NOT have caret
	if strings.HasPrefix(stripAnsi(nonEmpty[2]), "> ") {
		t.Errorf("line 2 should not have caret, got: %q", nonEmpty[2])
	}
}

func TestNavListSubheaderGrouping(t *testing.T) {
	nl := tui.NavList{
		Items: []tui.NavItem{
			{ID: "a", Label: "Alpha", Subheader: "Group A"},
			{ID: "b", Label: "Beta", Subheader: "Group A"},
			{ID: "c", Label: "Gamma", Subheader: "Group B"},
		},
		Selected: 0,
		Theme:    tui.DefaultTheme(),
	}

	out := nl.Render()

	// "Group A" should appear exactly once
	if count := strings.Count(out, "Group A"); count != 1 {
		t.Errorf("expected 'Group A' to appear once, appeared %d times in:\n%s", count, out)
	}

	// "Group B" should appear exactly once
	if count := strings.Count(out, "Group B"); count != 1 {
		t.Errorf("expected 'Group B' to appear once, appeared %d times in:\n%s", count, out)
	}

	// Items under subheaders should be indented with subIndent ("  ")
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		stripped := stripAnsi(line)
		// Lines containing item labels (not subheader lines, not blank) should start with subIndent
		if strings.Contains(stripped, "Alpha") || strings.Contains(stripped, "Beta") || strings.Contains(stripped, "Gamma") {
			if !strings.HasPrefix(stripped, "  ") {
				t.Errorf("item line should be indented with subIndent, got: %q", stripped)
			}
		}
	}

	// There should be a blank line between Group A and Group B sections
	if !strings.Contains(out, "\n\n") {
		t.Errorf("expected blank line between groups, output:\n%s", out)
	}
}

func TestNavListMoveUpDown(t *testing.T) {
	nl := tui.NavList{
		Items: []tui.NavItem{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
			{ID: "c", Label: "Gamma"},
		},
		Selected: 0,
		Theme:    tui.DefaultTheme(),
	}

	// Move down
	nl.MoveDown()
	if nl.Selected != 1 {
		t.Errorf("after MoveDown, expected Selected=1, got %d", nl.Selected)
	}

	nl.MoveDown()
	if nl.Selected != 2 {
		t.Errorf("after second MoveDown, expected Selected=2, got %d", nl.Selected)
	}

	// Clamp at end
	nl.MoveDown()
	if nl.Selected != 2 {
		t.Errorf("after third MoveDown (clamp), expected Selected=2, got %d", nl.Selected)
	}

	// Move up
	nl.MoveUp()
	if nl.Selected != 1 {
		t.Errorf("after MoveUp, expected Selected=1, got %d", nl.Selected)
	}

	// Move up to 0
	nl.MoveUp()
	if nl.Selected != 0 {
		t.Errorf("after second MoveUp, expected Selected=0, got %d", nl.Selected)
	}

	// Clamp at start
	nl.MoveUp()
	if nl.Selected != 0 {
		t.Errorf("after third MoveUp (clamp), expected Selected=0, got %d", nl.Selected)
	}
}

func TestNavListSelectedItem(t *testing.T) {
	nl := tui.NavList{
		Items: []tui.NavItem{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
		Selected: 1,
		Theme:    tui.DefaultTheme(),
	}

	item, ok := nl.SelectedItem()
	if !ok {
		t.Fatal("SelectedItem should return ok=true")
	}
	if item.ID != "b" {
		t.Errorf("expected selected item ID 'b', got %q", item.ID)
	}

	// Empty list
	empty := tui.NavList{Theme: tui.DefaultTheme()}
	_, ok = empty.SelectedItem()
	if ok {
		t.Error("SelectedItem on empty list should return ok=false")
	}
}

func TestNavListClamp(t *testing.T) {
	nl := tui.NavList{
		Items: []tui.NavItem{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
		Selected: 5,
		Theme:    tui.DefaultTheme(),
	}

	nl.Clamp()
	if nl.Selected != 1 {
		t.Errorf("Clamp should bring Selected to last index, got %d", nl.Selected)
	}

	nl.Selected = -3
	nl.Clamp()
	if nl.Selected != 0 {
		t.Errorf("Clamp should bring negative Selected to 0, got %d", nl.Selected)
	}
}
