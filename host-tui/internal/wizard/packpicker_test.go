package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func samplePackGroups() []PackGroup {
	return []PackGroup{
		{
			Name: "Core",
			Packs: []Pack{
				{
					Name: "core", Description: "Universal foundation",
					Sources: []PackSource{
						{ID: "wikiciv", Description: "Wikipedia Civilization", SizeGB: 0.1},
						{ID: "permacomputing", Description: "Permacomputing Wiki", SizeGB: 0.05},
					},
				},
			},
		},
		{
			Name: "Maps & Geodata",
			Packs: []Pack{
				{
					Name: "fi-maps", Description: "Finnish maps and geodata",
					Sources: []PackSource{
						{ID: "osm-finland", Description: "OpenStreetMap Finland", SizeGB: 3.0},
						{ID: "natural-earth", Description: "Natural Earth vectors", SizeGB: 0.3},
					},
				},
			},
		},
	}
}

// helper to send a key to the model and return the updated model.
func sendKey(m packPickerModel, key string) packPickerModel {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(packPickerModel)
}

// helper to send a key and capture the command.
func sendKeyCmd(m packPickerModel, key string) (packPickerModel, tea.Cmd) {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := m.Update(msg)
	return updated.(packPickerModel), cmd
}

func TestPackPickerShowsGroupsAndPacks(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups: samplePackGroups(),
		freeGB: 64,
	})
	m.width = 80
	m.height = 30
	out := stripAnsi(m.View())

	for _, want := range []string{"Core", "core", "Maps & Geodata", "fi-maps"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() should contain %q, got:\n%s", want, out)
		}
	}
}

func TestPackPickerTogglePack(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups: samplePackGroups(),
		freeGB: 64,
	})
	m.width = 80
	m.height = 30

	// cursor starts at first row (group "Core"), move down to "core" pack
	m = sendKey(m, "j")
	// now on pack "core", toggle with Space => check all
	m = sendKey(m, " ")

	if !containsID(m.picker.CheckedIDSlice(), "wikiciv") || !containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Error("Space on pack should check all sources; got:", m.picker.CheckedIDSlice())
	}

	// Space again => uncheck all
	m = sendKey(m, " ")
	if containsID(m.picker.CheckedIDSlice(), "wikiciv") || containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Error("Second Space on pack should uncheck all; got:", m.picker.CheckedIDSlice())
	}
}

func TestPackPickerReconcilesAutomaticDependencies(t *testing.T) {
	resolveDeps := func(selected map[string]bool) (map[string]bool, []string) {
		if selected["wikiciv"] {
			return map[string]bool{"permacomputing": true}, nil
		}
		return map[string]bool{}, nil
	}
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		freeGB:      64,
		resolveDeps: resolveDeps,
	})
	m = sendKey(m, "j")
	m = sendKey(m, "enter")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	if !containsID(m.picker.CheckedIDSlice(), "wikiciv") || !containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Fatalf("automatic dependency selection = %v", m.picker.CheckedIDSlice())
	}

	m = sendKey(m, " ")
	if containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Fatalf("stale automatic dependency remained selected: %v", m.picker.CheckedIDSlice())
	}

	m = newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: map[string]bool{"wikiciv": true, "permacomputing": true},
		freeGB:      64,
		resolveDeps: resolveDeps,
	})
	m = sendKey(m, "j")
	m = sendKey(m, "enter")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	if !containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Fatalf("explicit dependency was removed: %v", m.picker.CheckedIDSlice())
	}
}

func TestPackPickerExpandCollapse(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups: samplePackGroups(),
		freeGB: 64,
	})
	m.width = 80
	m.height = 30

	// move to pack "core"
	m = sendKey(m, "j")
	// packs start collapsed; Enter expands to show individual sources
	m = sendKey(m, "enter")

	out := stripAnsi(m.View())
	if !strings.Contains(out, "wikiciv") {
		t.Errorf("After Enter on pack, individual sources should be visible; got:\n%s", out)
	}

	// Enter again collapses
	m = sendKey(m, "enter")
	out = stripAnsi(m.View())
	if strings.Contains(out, "wikiciv") {
		t.Errorf("After second Enter on pack, individual sources should be hidden; got:\n%s", out)
	}
}

func TestPackPickerTriState(t *testing.T) {
	// Pre-check only one source in the core pack
	checked := map[string]bool{"wikiciv": true}
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: checked,
		freeGB:      64,
	})
	m.width = 80
	m.height = 30

	out := m.View()
	// The core pack should show partial indicator ~
	if !strings.Contains(out, "~") {
		t.Errorf("Partial check should show ~; got:\n%s", stripAnsi(out))
	}
}

func TestPackPickerPreChecked(t *testing.T) {
	checked := map[string]bool{
		"wikiciv":        true,
		"permacomputing": true,
	}
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: checked,
		freeGB:      64,
	})

	if !containsID(m.picker.CheckedIDSlice(), "wikiciv") || !containsID(m.picker.CheckedIDSlice(), "permacomputing") {
		t.Error("Constructor should pre-select items from checked map; got:", m.picker.CheckedIDSlice())
	}

	out := m.View()
	// Full check should show ✓
	if !strings.Contains(out, "✓") {
		t.Errorf("Fully checked pack should show ✓; got:\n%s", stripAnsi(out))
	}
}

func TestPackPickerSizeTotal(t *testing.T) {
	checked := map[string]bool{
		"wikiciv":        true,
		"permacomputing": true,
	}
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: checked,
		freeGB:      64,
	})
	m.width = 80
	m.height = 30

	out := stripAnsi(m.View())
	// Total should be 0.15 GB => "0.2" (rounded to 1 decimal) or shown as MB
	// 0.1 + 0.05 = 0.15 GB => "150 MB" or "0.2 GB"
	if !strings.Contains(out, "Total:") {
		t.Errorf("View() should contain 'Total:'; got:\n%s", out)
	}
	if !strings.Contains(out, "64") {
		t.Errorf("View() should show freeGB '64'; got:\n%s", out)
	}
	if !strings.Contains(out, "fits") {
		t.Errorf("View() should show 'fits' when under budget; got:\n%s", out)
	}
}

func TestPackPickerOverBudget(t *testing.T) {
	checked := map[string]bool{
		"osm-finland":   true,
		"natural-earth": true,
	}
	// Only 1 GB free but selected 3.3 GB
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: checked,
		freeGB:      1.0,
	})
	m.width = 80
	m.height = 30

	out := stripAnsi(m.View())
	if !strings.Contains(out, "over") {
		t.Errorf("View() should show 'over' when over budget; got:\n%s", out)
	}
}

func TestPackPickerApply(t *testing.T) {
	checked := map[string]bool{
		"wikiciv": true,
	}
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: checked,
		freeGB:      64,
	})
	m.width = 80
	m.height = 30

	m, cmd := sendKeyCmd(m, "a")
	if cmd == nil {
		t.Fatal("'a' key should produce a command")
	}

	msg := cmd()
	done, ok := msg.(packDoneMsg)
	if !ok {
		t.Fatalf("expected packDoneMsg, got %T", msg)
	}
	if !done.selectedIDs["wikiciv"] {
		t.Error("packDoneMsg should contain checked IDs")
	}
	if !done.explicitIDs["wikiciv"] {
		t.Error("packDoneMsg should contain explicit IDs")
	}
}

func TestPackPickerCancel(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: map[string]bool{"wikiciv": true},
		freeGB:      64,
	})
	m.width = 80
	m.height = 30

	_, cmd := sendKeyCmd(m, "q")
	if cmd == nil {
		t.Fatal("'q' key should produce a command")
	}

	msg := cmd()
	cancel, ok := msg.(packCancelMsg)
	if !ok {
		t.Fatalf("expected packCancelMsg, got %T", msg)
	}
	if !cancel.explicitIDs["wikiciv"] || !cancel.selectedIDs["wikiciv"] || len(cancel.missingIDs) != 0 {
		t.Fatalf("packCancelMsg = %+v, want wikiciv selection snapshot", cancel)
	}
	cancel.explicitIDs["changed"] = true
	cancel.selectedIDs["changed"] = true
	if m.picker.UserSelection()["changed"] || containsID(m.picker.CheckedIDSlice(), "changed") {
		t.Fatal("cancel snapshot aliases picker state")
	}
}

func TestPackPickerReturnsDistinctSelectionSnapshotsAndMissingIDs(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: map[string]bool{"wikiciv": true},
		resolveDeps: func(selected map[string]bool) (map[string]bool, []string) {
			if !selected["wikiciv"] {
				t.Fatalf("resolver selection = %v, want wikiciv", selected)
			}
			return map[string]bool{"permacomputing": true}, []string{"missing-z", "missing-a"}
		},
	})

	_, cmd := sendKeyCmd(m, "a")
	done := cmd().(packDoneMsg)
	if !done.explicitIDs["wikiciv"] || len(done.explicitIDs) != 1 {
		t.Fatalf("explicit IDs = %v, want wikiciv", done.explicitIDs)
	}
	if !done.selectedIDs["wikiciv"] || !done.selectedIDs["permacomputing"] || len(done.selectedIDs) != 2 {
		t.Fatalf("effective IDs = %v, want wikiciv and permacomputing", done.selectedIDs)
	}
	if got, want := strings.Join(done.missingIDs, ","), "missing-a,missing-z"; got != want {
		t.Fatalf("missing IDs = %v, want %s", done.missingIDs, want)
	}

	done.explicitIDs["changed"] = true
	done.selectedIDs["changed"] = true
	if m.picker.UserSelection()["changed"] || containsID(m.picker.CheckedIDSlice(), "changed") {
		t.Fatal("completion snapshot aliases picker state")
	}
}

func TestPackPickerRendersMissingAndUnavailableExplicitSources(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: map[string]bool{"stale-source": true},
		resolveDeps: func(selected map[string]bool) (map[string]bool, []string) {
			if selected["stale-source"] {
				return nil, []string{"missing-dependency", "stale-source"}
			}
			return nil, []string{"missing-dependency"}
		},
	})
	m.width = 80
	m.height = 30
	m.picker.Width = 80
	m.picker.Height = 30
	for i, row := range m.picker.Rows {
		if row.GroupName == "Unavailable" {
			m.picker.Cursor = i
			m.picker.ExpandAtCursor()
			break
		}
	}
	for i, row := range m.picker.Rows {
		if row.Pack != nil && row.Pack.Name == "unavailable-items" {
			m.picker.Cursor = i
			m.picker.ExpandAtCursor()
			break
		}
	}

	out := stripAnsi(m.View())
	for _, want := range []string{"unavailable-items", "stale-source", "unknown size", "Missing catalog recipes:", "Known-size subtotal:"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() should contain %q, got:\n%s", want, out)
		}
	}
	if joined := strings.Join(strings.Fields(out), ""); !strings.Contains(joined, "Missingcatalogrecipes:missing-dependency,stale-source") {
		t.Fatalf("wrapped warning lost a missing ID:\n%s", out)
	}
	for i, row := range m.picker.Rows {
		if row.Source != nil && row.Source.ID == "stale-source" {
			m.picker.Cursor = i
			break
		}
	}
	m = sendKey(m, " ")
	if containsID(m.picker.CheckedIDSlice(), "stale-source") || m.picker.UserSelection()["stale-source"] {
		t.Fatal("ordinary toggle did not remove unavailable explicit source")
	}
	if got, want := strings.Join(m.missingIDs, ","), "missing-dependency"; got != want {
		t.Fatalf("missing IDs after removal = %v, want %s", m.missingIDs, want)
	}
}

func TestPackPickerReconcilesSharedAndExplicitDependencies(t *testing.T) {
	groups := []PackGroup{{Name: "group", Packs: []Pack{{Name: "pack", Sources: []PackSource{{ID: "a"}, {ID: "b"}, {ID: "d"}}}}}}
	resolveDeps := func(selected map[string]bool) (map[string]bool, []string) {
		if selected["a"] || selected["b"] {
			return map[string]bool{"d": true}, nil
		}
		return nil, nil
	}
	m := newPackPicker(packPickerConfig{groups: groups, resolveDeps: resolveDeps})
	m = sendKey(m, "j")
	m = sendKey(m, "enter")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	if got, want := m.picker.CheckedIDSlice(), []string{"a", "b", "d"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("shared dependencies = %v, want %v", got, want)
	}
	m.picker.Cursor = 2
	m = sendKey(m, " ")
	if got, want := m.picker.CheckedIDSlice(), []string{"b", "d"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dependency after removing a = %v, want %v", got, want)
	}
	m.picker.Cursor = 3
	m = sendKey(m, " ")
	if len(m.picker.CheckedIDSlice()) != 0 {
		t.Fatalf("obsolete dependency remained selected: %v", m.picker.CheckedIDSlice())
	}

	m = newPackPicker(packPickerConfig{groups: groups, explicitIDs: map[string]bool{"d": true}, resolveDeps: resolveDeps})
	m = sendKey(m, "j")
	m = sendKey(m, "enter")
	m = sendKey(m, "j")
	m = sendKey(m, " ")
	m = sendKey(m, " ")
	if got, want := m.picker.CheckedIDSlice(), []string{"d"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("explicit dependency after consumer removal = %v, want %v", got, want)
	}
	if !m.picker.UserSelection()["d"] {
		t.Fatal("explicit dependency was not retained")
	}
}

func TestPackPickerConfirmationCommandsSnapshotBeforeLaterToggle(t *testing.T) {
	for _, key := range []string{"a", "q"} {
		t.Run(key, func(t *testing.T) {
			m := newPackPicker(packPickerConfig{
				groups:      samplePackGroups(),
				explicitIDs: map[string]bool{"wikiciv": true},
			})
			updated, cmd := sendKeyCmd(m, key)
			updated.picker.Cursor = 1
			updated = sendKey(updated, " ")

			switch msg := cmd().(type) {
			case packDoneMsg:
				if len(msg.selectedIDs) != 1 || !msg.selectedIDs["wikiciv"] {
					t.Fatalf("done snapshot = %v, want wikiciv only", msg.selectedIDs)
				}
			case packCancelMsg:
				if len(msg.selectedIDs) != 1 || !msg.selectedIDs["wikiciv"] {
					t.Fatalf("cancel snapshot = %v, want wikiciv only", msg.selectedIDs)
				}
			default:
				t.Fatalf("command returned %T", msg)
			}
		})
	}
}

func TestPackPickerNilResolverValidatesUnavailableExplicitSource(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups:      samplePackGroups(),
		explicitIDs: map[string]bool{"stale-source": true},
	})
	if got, want := strings.Join(m.missingIDs, ","), "stale-source"; got != want {
		t.Fatalf("missing IDs = %v, want %s", m.missingIDs, want)
	}

	_, cmd := sendKeyCmd(m, "a")
	if done := cmd().(packDoneMsg); strings.Join(done.missingIDs, ",") != "stale-source" {
		t.Fatalf("done missing IDs = %v, want stale-source", done.missingIDs)
	}

	for i, row := range m.picker.Rows {
		if row.GroupName == "Unavailable" {
			m.picker.Cursor = i
			m.picker.ExpandAtCursor()
			break
		}
	}
	for i, row := range m.picker.Rows {
		if row.Pack != nil && row.Pack.Name == "unavailable-items" {
			m.picker.Cursor = i
			m.picker.ExpandAtCursor()
			break
		}
	}
	for i, row := range m.picker.Rows {
		if row.Source != nil && row.Source.ID == "stale-source" {
			m.picker.Cursor = i
			break
		}
	}
	m = sendKey(m, " ")
	if len(m.missingIDs) != 0 {
		t.Fatalf("missing IDs after removal = %v, want none", m.missingIDs)
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestPackPickerLiveViewportReservesWrappedValidation(t *testing.T) {
	plain := newPackPicker(packPickerConfig{groups: samplePackGroups(), explicitIDs: map[string]bool{"wikiciv": true}})
	bad := newPackPicker(packPickerConfig{
		groups: samplePackGroups(), explicitIDs: map[string]bool{"wikiciv": true},
		resolveDeps: func(selected map[string]bool) (map[string]bool, []string) {
			if selected["wikiciv"] {
				return nil, []string{"a-very-long-missing-recipe-id-that-wraps-in-the-right-pane"}
			}
			return nil, nil
		},
	})
	resized, _ := plain.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	plain = resized.(packPickerModel)
	resized, _ = bad.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	bad = resized.(packPickerModel)
	if bad.picker.MaxVisible() >= plain.picker.MaxVisible() {
		t.Fatalf("live warning budget did not reduce visible rows: bad=%d plain=%d", bad.picker.MaxVisible(), plain.picker.MaxVisible())
	}
	bad.picker.ReplaceUserSelection(nil)
	bad.recalcDeps()
	if bad.picker.MaxVisible() != plain.picker.MaxVisible() {
		t.Fatal("repair did not restore ordinary visible-row budget")
	}
}
