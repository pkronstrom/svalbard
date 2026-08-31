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
	resolveDeps := func(selected map[string]bool) map[string]bool {
		if selected["wikiciv"] {
			return map[string]bool{"permacomputing": true}
		}
		return map[string]bool{}
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
		checkedIDs:  map[string]bool{"wikiciv": true, "permacomputing": true},
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
		groups:     samplePackGroups(),
		checkedIDs: checked,
		freeGB:     64,
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
		groups:     samplePackGroups(),
		checkedIDs: checked,
		freeGB:     64,
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
		groups:     samplePackGroups(),
		checkedIDs: checked,
		freeGB:     64,
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
		groups:     samplePackGroups(),
		checkedIDs: checked,
		freeGB:     1.0,
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
		groups:     samplePackGroups(),
		checkedIDs: checked,
		freeGB:     64,
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
}

func TestPackPickerCancel(t *testing.T) {
	m := newPackPicker(packPickerConfig{
		groups: samplePackGroups(),
		freeGB: 64,
	})
	m.width = 80
	m.height = 30

	_, cmd := sendKeyCmd(m, "q")
	if cmd == nil {
		t.Fatal("'q' key should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(packCancelMsg); !ok {
		t.Fatalf("expected packCancelMsg, got %T", msg)
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
