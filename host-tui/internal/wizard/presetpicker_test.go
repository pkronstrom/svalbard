package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func samplePresets() []PresetOption {
	return []PresetOption{
		{
			Name:         "default-2",
			Description:  "Bugout kit",
			ContentGB:    2,
			TargetSizeGB: 4,
			Region:       "europe",
			SourceIDs:    []string{"survival-guide"},
		},
		{
			Name:         "default-32",
			Description:  "Broad reference",
			ContentGB:    25,
			TargetSizeGB: 32,
			Region:       "europe",
			SourceIDs:    []string{"survival-guide", "wikipedia"},
		},
		{
			Name:         "default-128",
			Description:  "Full reference",
			ContentGB:    100,
			TargetSizeGB: 128,
			Region:       "europe",
			SourceIDs:    []string{"survival-guide", "wikipedia", "osm"},
		},
	}
}

func TestPresetPickerShowsPresets(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	for _, p := range presets {
		if !strings.Contains(out, p.Name) {
			t.Errorf("View() should contain preset name %q, got:\n%s", p.Name, out)
		}
	}

	// Should also show "Customize"
	if !strings.Contains(out, "Customize") {
		t.Errorf("View() should contain Customize option, got:\n%s", out)
	}
}

func TestPresetPickerHighlightsRecommended(t *testing.T) {
	presets := samplePresets()
	// 50 GB free: default-2 (2 GB) fits, default-32 (25 GB) fits, default-128 (100 GB) does not
	// Recommended = largest fitting = default-32 at index 1
	m := newPresetPicker(presets, []string{"europe"}, 50)

	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 (default-32), got %d", m.cursor)
	}
}

func TestPresetPickerHighlightsRecommendedAllFit(t *testing.T) {
	presets := samplePresets()
	// 200 GB free: all presets fit; recommended = last = index 2
	m := newPresetPicker(presets, []string{"europe"}, 200)

	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 (default-128), got %d", m.cursor)
	}
}

func TestPresetPickerHighlightsRecommendedNoneFit(t *testing.T) {
	presets := samplePresets()
	// 1 GB free: no preset fits; recommended = 0 (first)
	m := newPresetPicker(presets, []string{"europe"}, 1)

	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 when none fit, got %d", m.cursor)
	}
}

func TestPresetPickerSelectsSendsMsg(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)
	m.width = 80
	m.height = 24
	// cursor starts at 1 (default-32)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected a non-nil Cmd on Enter")
	}

	result := cmd()
	done, ok := result.(presetDoneMsg)
	if !ok {
		t.Fatalf("expected presetDoneMsg, got %T", result)
	}

	if done.preset.Name != "default-32" {
		t.Errorf("expected preset name %q, got %q", "default-32", done.preset.Name)
	}
}

func TestPresetPickerSkipOption(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)
	m.width = 80
	m.height = 24
	// cursor starts at 1; move to Customize (index 3 = len(presets))

	// Navigate down twice: 1 -> 2 -> 3 (Customize)
	down := tea.KeyMsg{Type: tea.KeyDown}
	var tm tea.Model = m
	var cmd tea.Cmd
	tm, _ = tm.(presetPickerModel).Update(down)
	tm, _ = tm.(presetPickerModel).Update(down)

	// Now press Enter on Customize
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd = tm.(presetPickerModel).Update(enter)

	if cmd == nil {
		t.Fatal("expected a non-nil Cmd on Enter for Customize")
	}

	result := cmd()
	done, ok := result.(presetDoneMsg)
	if !ok {
		t.Fatalf("expected presetDoneMsg, got %T", result)
	}

	if done.preset.Name != "" {
		t.Errorf("expected empty preset name for Customize, got %q", done.preset.Name)
	}
}

func TestPresetPickerShowsNeedsMoreSpace(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	// default-128 needs 100 GB but only 50 GB free => "needs ~50 GB more"
	if !strings.Contains(out, "needs") {
		t.Errorf("View() should show 'needs' for presets exceeding free space, got:\n%s", out)
	}
}

func TestPresetPickerNavigationBounds(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)

	// Move up past the top
	up := tea.KeyMsg{Type: tea.KeyUp}
	tm, _ := m.Update(up)
	m2 := tm.(presetPickerModel)
	tm, _ = m2.Update(up)
	m3 := tm.(presetPickerModel)

	if m3.cursor < 0 {
		t.Errorf("cursor should not go below 0, got %d", m3.cursor)
	}

	// Move down past the bottom (3 presets + 1 customize = 4 items, max index 3)
	down := tea.KeyMsg{Type: tea.KeyDown}
	tm = m3
	for i := 0; i < 10; i++ {
		tm, _ = tm.(presetPickerModel).Update(down)
	}
	mFinal := tm.(presetPickerModel)

	if mFinal.cursor > 3 {
		t.Errorf("cursor should not exceed 3, got %d", mFinal.cursor)
	}
}

func TestFormatSizeGB(t *testing.T) {
	tests := []struct {
		gb   float64
		want string
	}{
		{2, "~2 GB"},
		{25, "~25 GB"},
		{100, "~100 GB"},
		{0.5, "~512 MB"},
		{0.1, "~102 MB"},
		{1, "~1 GB"},
		{1.5, "~2 GB"},
	}

	for _, tt := range tests {
		got := formatSizeGB(tt.gb)
		if got != tt.want {
			t.Errorf("formatSizeGB(%v) = %q, want %q", tt.gb, got, tt.want)
		}
	}
}

func TestPresetPickerShowsFreeSpace(t *testing.T) {
	presets := samplePresets()
	m := newPresetPicker(presets, []string{"europe"}, 50)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	if !strings.Contains(out, "50 GB free") {
		t.Errorf("View() should show free space, got:\n%s", out)
	}
}
