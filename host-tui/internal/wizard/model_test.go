package wizard

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stripAnsi removes ANSI escape sequences for assertions on rendered output.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func testConfig() WizardConfig {
	return WizardConfig{
		Volumes: []Volume{
			{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50},
		},
		HomeVolume: Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100},
		Presets: []PresetOption{
			{Name: "default-2", Description: "Bugout kit", ContentGB: 1.5, TargetSizeGB: 2, SourceIDs: []string{"kiwix-serve"}},
		},
		Regions:    []string{"default"},
		PackGroups: samplePackGroups(),
	}
}

func TestWizardShowsAllSteps(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	for _, step := range wizardSteps {
		if !strings.Contains(out, step.label) {
			t.Errorf("View() should contain step label %q", step.label)
		}
	}
}

func TestWizardStartsAtPathStep(t *testing.T) {
	m := New(testConfig())
	if m.stage != stagePath {
		t.Errorf("expected stagePath, got %d", m.stage)
	}
}

func TestWizardPathToPresetTransition(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	// Select first volume (cursor starts at 0, Enter selects)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected path done command")
	}
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.stage != stagePreset {
		t.Errorf("expected stagePreset after path selection, got %d", m.stage)
	}
	if m.vaultPath == "" {
		t.Error("vault path should be set after path selection")
	}
}

func TestWizardPresetToPacksTransition(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 40

	// Step 1: Select path
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Step 2: Select preset
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.stage != stagePacks {
		t.Errorf("expected stagePacks after preset selection, got %d", m.stage)
	}
}

func TestWizardFullFlow(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 40

	// Step 1: Select path
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePreset {
		t.Fatalf("expected stagePreset, got %d", m.stage)
	}

	// Step 2: Select preset
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePacks {
		t.Fatalf("expected stagePacks, got %d", m.stage)
	}

	// Step 3: Apply pack selection (press 'a')
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.stage != stageReview {
		t.Fatalf("expected stageReview, got %d", m.stage)
	}

	// Step 4: Confirm review (Enter → reviewConfirmMsg → wizard converts to DoneMsg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	// reviewConfirmMsg goes back into wizard shell which produces DoneMsg
	updated, cmd = m.Update(msg)
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected DoneMsg command after review confirm")
	}
	doneMsg := cmd()
	done, ok := doneMsg.(DoneMsg)
	if !ok {
		t.Fatalf("expected DoneMsg, got %T", doneMsg)
	}
	if done.Result.VaultPath == "" {
		t.Error("DoneMsg should have vault path")
	}
}

func TestWizardEscAtPathGoesBack(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected BackMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestWizardPackCancelGoesBackToPreset(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 40

	// Get to packs stage
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if m.stage != stagePacks {
		t.Fatalf("expected stagePacks, got %d", m.stage)
	}

	// Cancel from packs (press 'q')
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.stage != stagePreset {
		t.Errorf("expected stagePreset after pack cancel, got %d", m.stage)
	}
}

func TestWizardReviewBackGoesToPacks(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 40

	// Get to review stage
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if m.stage != stageReview {
		t.Fatalf("expected stageReview, got %d", m.stage)
	}

	// Esc from review
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.stage != stagePacks {
		t.Errorf("expected stagePacks after review back, got %d", m.stage)
	}
}

func TestWizardStartAtStep(t *testing.T) {
	config := testConfig()
	config.StartAtStep = 1
	m := New(config)
	if m.stage != stagePreset {
		t.Errorf("expected stagePreset with StartAtStep=1, got %d", m.stage)
	}
}
