package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleVolumes() []Volume {
	return []Volume{
		{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50, Network: false},
		{Path: "/Volumes/NAS/svalbard", Name: "NAS", TotalGB: 1000, FreeGB: 500, Network: true},
	}
}

func sampleHome() Volume {
	return Volume{Path: "~/svalbard/", Name: "Home", TotalGB: 500, FreeGB: 100, Network: false}
}

func TestPathPickerShowsVolumes(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	// Should show volume paths
	if !strings.Contains(out, "/Volumes/USB/svalbard") {
		t.Errorf("expected USB volume in view, got:\n%s", out)
	}
	if !strings.Contains(out, "/Volumes/NAS/svalbard") {
		t.Errorf("expected NAS volume in view, got:\n%s", out)
	}
	// Should show network annotation
	if !strings.Contains(out, "[network]") {
		t.Errorf("expected [network] annotation, got:\n%s", out)
	}
	// Should show home
	if !strings.Contains(out, "~/svalbard/") {
		t.Errorf("expected home path in view, got:\n%s", out)
	}
	// Should show custom option
	if !strings.Contains(out, "Custom path") {
		t.Errorf("expected 'Custom path' option in view, got:\n%s", out)
	}
	// Should show size info
	if !strings.Contains(out, "50") && !strings.Contains(out, "64") {
		t.Errorf("expected size info for USB volume, got:\n%s", out)
	}
}

func TestPathPickerSelectsVolume(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	// Press Enter on the first option (cursor starts at 0)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(msg)
	_ = updated

	if cmd == nil {
		t.Fatal("expected a command after selecting a volume")
	}

	result := cmd()
	done, ok := result.(pathDoneMsg)
	if !ok {
		t.Fatalf("expected pathDoneMsg, got %T", result)
	}

	if done.path != "/Volumes/USB/svalbard" {
		t.Errorf("expected path %q, got %q", "/Volumes/USB/svalbard", done.path)
	}
	if done.freeGB != 50 {
		t.Errorf("expected freeGB 50, got %f", done.freeGB)
	}
}

func TestPathPickerNavigates(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	if m.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.cursor)
	}

	// Press down arrow
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	m = updated.(pathPickerModel)

	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.cursor)
	}

	// Press down again
	updated, _ = m.Update(msg)
	m = updated.(pathPickerModel)

	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after two downs, got %d", m.cursor)
	}

	// Press up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(upMsg)
	m = updated.(pathPickerModel)

	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.cursor)
	}
}

func TestPathPickerCustomInputActivatesFilePicker(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	// Navigate to the custom option (last item)
	lastIdx := len(m.options) - 1
	for i := 0; i < lastIdx; i++ {
		msg := tea.KeyMsg{Type: tea.KeyDown}
		updated, _ := m.Update(msg)
		m = updated.(pathPickerModel)
	}

	if m.cursor != lastIdx {
		t.Fatalf("expected cursor at custom option (%d), got %d", lastIdx, m.cursor)
	}

	// Press Enter to activate file picker
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(enterMsg)
	m = updated.(pathPickerModel)

	if !m.customInput {
		t.Fatal("expected customInput mode to be active")
	}

	// Should have produced an Init cmd (to read the initial directory)
	if cmd == nil {
		t.Error("expected a command from filepicker Init")
	}

	// Verify file picker view shows directory-related content
	m.width = 80
	m.height = 24
	// Send a size so the picker has dimensions
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(pathPickerModel)

	out := stripAnsi(m.View())
	if !strings.Contains(out, "Select a directory") {
		t.Errorf("expected file picker header in view, got:\n%s", out)
	}
}

func TestPathPickerPrefill(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "/prefilled/path")
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	if !strings.Contains(out, "/prefilled/path") {
		t.Errorf("expected prefilled path in view, got:\n%s", out)
	}

	// Prefill should be the first option
	if m.options[0].path != "/prefilled/path" {
		t.Errorf("expected first option to be prefilled path, got %q", m.options[0].path)
	}
}

func TestPathPickerCustomInputEscCancels(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	// Navigate to custom option and activate it
	lastIdx := len(m.options) - 1
	for i := 0; i < lastIdx; i++ {
		msg := tea.KeyMsg{Type: tea.KeyDown}
		updated, _ := m.Update(msg)
		m = updated.(pathPickerModel)
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(enterMsg)
	m = updated.(pathPickerModel)

	if !m.customInput {
		t.Fatal("expected customInput mode to be active")
	}

	// Press Esc to cancel
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ = m.Update(escMsg)
	m = updated.(pathPickerModel)

	if m.customInput {
		t.Error("expected customInput mode to be deactivated after Esc")
	}
}
