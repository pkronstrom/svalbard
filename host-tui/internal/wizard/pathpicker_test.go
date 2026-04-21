package wizard

import (
	"os"
	"path/filepath"
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
	// Should show size info
	if !strings.Contains(out, "50") && !strings.Contains(out, "64") {
		t.Errorf("expected size info for USB volume, got:\n%s", out)
	}
	// Should show text input
	if !strings.Contains(out, "Path:") {
		t.Errorf("expected 'Path:' input label in view, got:\n%s", out)
	}
}

func TestPathPickerDefaultsToSwalbardVault(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, "svalbard-vault")
	if m.input != expected {
		t.Errorf("expected default input %q, got %q", expected, m.input)
	}
}

func TestPathPickerSelectsVolumeViaQuickSelect(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	// Move down to first option (from text input at cursor -1)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	m = updated.(pathPickerModel)

	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	// Input should be set to volume path + /svalbard-vault
	expected := filepath.Join("/Volumes/USB/svalbard", "svalbard-vault")
	if m.input != expected {
		t.Errorf("expected input %q, got %q", expected, m.input)
	}
}

func TestPathPickerNavigates(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	if m.cursor != -1 {
		t.Fatalf("expected initial cursor -1 (text input), got %d", m.cursor)
	}

	// Press down to first option
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	m = updated.(pathPickerModel)

	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after down, got %d", m.cursor)
	}

	// Press down again
	updated, _ = m.Update(msg)
	m = updated.(pathPickerModel)

	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after two downs, got %d", m.cursor)
	}

	// Press up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(upMsg)
	m = updated.(pathPickerModel)

	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", m.cursor)
	}

	// Press up again to go back to text input
	updated, _ = m.Update(upMsg)
	m = updated.(pathPickerModel)

	if m.cursor != -1 {
		t.Errorf("expected cursor -1 (text input) after up, got %d", m.cursor)
	}
}

func TestPathPickerValidatesParentDir(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")
	m.input = "/nonexistent/parent/svalbard-vault"

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(enterMsg)
	m = updated.(pathPickerModel)

	if cmd != nil {
		t.Error("expected no command when parent dir doesn't exist")
	}
	if m.errMsg == "" {
		t.Error("expected error message for nonexistent parent")
	}
	if !strings.Contains(m.errMsg, "parent directory does not exist") {
		t.Errorf("expected parent directory error, got: %s", m.errMsg)
	}
}

func TestPathPickerAcceptsValidPath(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")
	m.input = filepath.Join(os.TempDir(), "svalbard-vault")

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)

	if cmd == nil {
		t.Fatal("expected a command after confirming valid path")
	}

	result := cmd()
	done, ok := result.(pathDoneMsg)
	if !ok {
		t.Fatalf("expected pathDoneMsg, got %T", result)
	}

	expected := filepath.Join(os.TempDir(), "svalbard-vault")
	if done.path != expected {
		t.Errorf("expected path %q, got %q", expected, done.path)
	}
}

func TestPathPickerPrefill(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "/prefilled/path")
	m.width = 80
	m.height = 24

	if m.input != "/prefilled/path" {
		t.Errorf("expected input to be prefilled path, got %q", m.input)
	}

	out := stripAnsi(m.View())
	if !strings.Contains(out, "/prefilled/path") {
		t.Errorf("expected prefilled path in view, got:\n%s", out)
	}
}

func TestPathPickerTypingResetsToFreeForm(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")

	// Navigate to a volume option
	down := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(down)
	m = updated.(pathPickerModel)

	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	// Type a character — should reset to free-form input
	typeMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	updated, _ = m.Update(typeMsg)
	m = updated.(pathPickerModel)

	if m.cursor != -1 {
		t.Errorf("expected cursor -1 after typing, got %d", m.cursor)
	}
}

func TestPathPickerRejectsEmptyPath(t *testing.T) {
	m := newPathPicker(sampleVolumes(), sampleHome(), "")
	m.input = ""

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, cmd := m.Update(enterMsg)
	m = updated.(pathPickerModel)

	if cmd != nil {
		t.Error("expected no command for empty path")
	}
	if m.errMsg == "" {
		t.Error("expected error for empty path")
	}
}
