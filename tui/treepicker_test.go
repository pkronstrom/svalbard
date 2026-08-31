package tui

import "testing"

func TestTreePickerReplaceUserSelectionClearsAutomaticDependencies(t *testing.T) {
	picker := newSelectionPicker()
	picker.ReplaceUserSelection([]string{"manual"})
	picker.SetAutoDependencies(map[string]bool{"dependency": true})

	picker.ReplaceUserSelection([]string{"replacement"})
	if got, want := picker.CheckedIDSlice(), []string{"replacement"}; !sameIDs(got, want) {
		t.Fatalf("CheckedIDSlice() = %v, want %v", got, want)
	}
	if picker.IsAutoDep("dependency") {
		t.Fatal("ReplaceUserSelection() retained a stale automatic dependency")
	}
}

func TestTreePickerSetAutoDependenciesPreservesExplicitSelection(t *testing.T) {
	picker := newSelectionPicker()
	picker.ReplaceUserSelection([]string{"manual"})
	picker.SetAutoDependencies(map[string]bool{"manual": true, "stale": true})
	picker.SetAutoDependencies(map[string]bool{"current": true})

	if got, want := picker.CheckedIDSlice(), []string{"manual", "current"}; !sameIDs(got, want) {
		t.Fatalf("CheckedIDSlice() = %v, want %v", got, want)
	}
	if !picker.IsAutoDep("current") {
		t.Fatal("current automatic dependency is not marked automatic")
	}
	if picker.IsAutoDep("manual") {
		t.Fatal("explicit selection is marked automatic")
	}

	selection := picker.UserSelection()
	delete(selection, "manual")
	if got, want := picker.CheckedIDSlice(), []string{"manual", "current"}; !sameIDs(got, want) {
		t.Fatalf("UserSelection() exposed internal state: %v, want %v", got, want)
	}
}

func TestTreePickerTogglePreservesAutomaticDependency(t *testing.T) {
	picker := newSelectionPicker()
	picker.ReplaceUserSelection([]string{"manual"})
	picker.SetAutoDependencies(map[string]bool{"manual": true})
	picker.Cursor = 1
	picker.ExpandAtCursor()
	picker.Cursor = 2

	picker.ToggleAtCursor()
	if got, want := picker.CheckedIDSlice(), []string{"manual"}; !sameIDs(got, want) {
		t.Fatalf("CheckedIDSlice() after toggle = %v, want %v", got, want)
	}
	if picker.IsAutoDep("manual") != true {
		t.Fatal("toggle did not retain automatic dependency state")
	}
	if _, explicitlySelected := picker.UserSelection()["manual"]; explicitlySelected {
		t.Fatal("toggle did not remove explicit selection")
	}
}

func newSelectionPicker() TreePicker {
	return NewTreePicker(TreePickerConfig{Groups: []PackGroup{{
		Name: "group",
		Packs: []Pack{{
			Name: "pack",
			Sources: []PackSource{
				{ID: "manual"},
				{ID: "dependency"},
				{ID: "stale"},
				{ID: "current"},
				{ID: "replacement"},
			},
		}},
	}}})
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	ids := make(map[string]bool, len(got))
	for _, id := range got {
		ids[id] = true
	}
	for _, id := range want {
		if !ids[id] {
			return false
		}
	}
	return true
}
