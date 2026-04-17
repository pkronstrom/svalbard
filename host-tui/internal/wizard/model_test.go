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

// advancePastPath selects the first volume and returns the model at stagePlatforms.
func advancePastPath(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected path done command")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.stage != stagePlatforms {
		t.Fatalf("expected stagePlatforms after path, got %d", m.stage)
	}
	return m
}

// advancePastPlatforms accepts default platform selection and returns the model at stagePreset.
func advancePastPlatforms(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected platform done command")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.stage != stagePreset {
		t.Fatalf("expected stagePreset after platforms, got %d", m.stage)
	}
	return m
}

// advancePastPreset selects the first preset and returns the model at stagePacks.
func advancePastPreset(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected preset done command")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.stage != stagePacks {
		t.Fatalf("expected stagePacks after preset, got %d", m.stage)
	}
	return m
}

// advancePastPacks applies pack selection and returns the model at stageReview.
func advancePastPacks(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected pack done command")
	}
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.stage != stageReview {
		t.Fatalf("expected stageReview after packs, got %d", m.stage)
	}
	return m
}

func sizedModel(cfg WizardConfig) Model {
	m := New(cfg)
	m.width = 80
	m.height = 40
	return m
}

func TestWizardShowsAllSteps(t *testing.T) {
	m := sizedModel(testConfig())
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

func TestWizardPathToPlatformsTransition(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	if m.vaultPath == "" {
		t.Error("vault path should be set after path selection")
	}
}

func TestWizardPlatformsToPresetTransition(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	if len(m.hostPlatforms) == 0 {
		t.Error("hostPlatforms should be set after platform selection")
	}
}

func TestWizardPresetToPacksTransition(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	m = advancePastPreset(t, m)
}

func TestWizardFullFlow(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	m = advancePastPreset(t, m)
	m = advancePastPacks(t, m)

	// Confirm review
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	_, cmd = m.Update(msg)
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
	if len(done.Result.HostPlatforms) == 0 {
		t.Error("DoneMsg should have host platforms")
	}
}

func TestWizardEscAtPathGoesBack(t *testing.T) {
	m := sizedModel(testConfig())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected BackMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestWizardEscAtPlatformsGoesBackToPath(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.stage != stagePath {
		t.Errorf("expected stagePath after esc from platforms, got %d", m.stage)
	}
}

func TestWizardPackCancelGoesBackToPreset(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	m = advancePastPreset(t, m)

	// Cancel from packs (press 'q')
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePreset {
		t.Errorf("expected stagePreset after pack cancel, got %d", m.stage)
	}
}

func TestWizardReviewBackGoesToPacks(t *testing.T) {
	m := sizedModel(testConfig())
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	m = advancePastPreset(t, m)
	m = advancePastPacks(t, m)

	// Esc from review
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePacks {
		t.Errorf("expected stagePacks after review back, got %d", m.stage)
	}
}

func TestWizardAlwaysStartsAtPath(t *testing.T) {
	config := testConfig()
	config.StartAtStep = 1
	m := New(config)
	if m.stage != stagePath {
		t.Errorf("expected stagePath regardless of StartAtStep, got %d", m.stage)
	}
}
