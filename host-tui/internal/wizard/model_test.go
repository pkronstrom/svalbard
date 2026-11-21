package wizard

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// stripAnsi removes ANSI escape sequences for assertions on rendered output.
func stripAnsi(s string) string { return tui.StripAnsi(s) }

func testConfig() WizardConfig {
	return WizardConfig{
		Volumes: []Volume{
			{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50},
		},
		HomeVolume: Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100},
		Presets: []PresetOption{
			{Name: "default-2", Description: "Bugout kit", ContentGB: 1.5, TargetSizeGB: 2, SourceIDs: []string{"wikiciv"}},
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

func TestWizardReestimatesSizesForChosenPlatforms(t *testing.T) {
	cfg := testConfig()
	var asked []string
	cfg.ForPlatforms = func(platforms []string) ([]PresetOption, []PackGroup) {
		asked = platforms
		presets := append([]PresetOption(nil), cfg.Presets...)
		presets[0].ContentGB = 7
		groups := []PackGroup{{Name: "Tools", Packs: []Pack{{Name: "bin", Sources: []PackSource{{ID: "wikiciv", SizeGB: 7}}}}}}
		return presets, groups
	}
	m := sizedModel(cfg)
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	if strings.Join(asked, ",") != strings.Join(m.hostPlatforms, ",") || len(asked) == 0 {
		t.Fatalf("ForPlatforms got %v, want the chosen platforms %v", asked, m.hostPlatforms)
	}
	if got := m.presetPicker.presets[0].ContentGB; got != 7 {
		t.Errorf("preset picker ContentGB = %v, want the re-estimated 7", got)
	}
	m = advancePastPreset(t, m)
	if got := m.packPicker.picker.TotalCheckedGB(); got != 7 {
		t.Errorf("pack picker total = %v, want the re-estimated 7", got)
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

func TestWizardIndexCtrlCCancelsBeforeProducerStarts(t *testing.T) {
	w := New(testConfig())
	w.stage = stageIndex
	w.indexModel = newWizardIndex("/vault", func(ctx context.Context, _ string, _ func(string, string, string)) error { return ctx.Err() })
	next, cmd := w.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	w = next.(Model)
	if cmd != nil || w.indexModel.ctx.Err() == nil || w.indexModel.exitIntent != "quit" {
		t.Fatalf("Ctrl-C bypassed index cancellation: intent=%q err=%v cmd=%v", w.indexModel.exitIntent, w.indexModel.ctx.Err(), cmd)
	}
}

func TestWizardSuccessfulApplySkipsDisabledIndex(t *testing.T) {
	indexed := false
	w := New(WizardConfig{
		ShouldIndex: func(string) bool { return false },
		RunIndex:    func(context.Context, string, func(string, string, string)) error { indexed = true; return nil },
	})
	w.vaultPath = "/vault"
	next, cmd := w.Update(wizardApplyDoneMsg{success: true})
	w = next.(Model)
	if w.stage == stageIndex || cmd == nil || indexed {
		t.Fatalf("disabled wizard index state = stage %d indexed %v", w.stage, indexed)
	}
	if _, ok := cmd().(DoneMsg); !ok {
		t.Fatal("disabled indexing did not complete wizard")
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

func TestWizardPreservesExplicitSelectionAcrossReviewBackAndReselect(t *testing.T) {
	config := testConfig()
	config.ResolveDeps = func(selected map[string]bool) (map[string]bool, []string) {
		if selected["wikiciv"] {
			return map[string]bool{"permacomputing": true}, nil
		}
		return nil, nil
	}
	m := sizedModel(config)
	m = advancePastPath(t, m)
	m = advancePastPlatforms(t, m)
	preset := config.Presets[0]
	updated, _ := m.Update(presetDoneMsg{preset: preset})
	m = updated.(Model)
	updated, _ = m.Update(packDoneMsg{
		explicitIDs: map[string]bool{"wikiciv": true},
		selectedIDs: map[string]bool{"wikiciv": true, "permacomputing": true},
	})
	m = updated.(Model)
	if m.stage != stageReview || !m.explicitIDs["wikiciv"] || !m.effectiveIDs["permacomputing"] {
		t.Fatalf("review state = explicit %v effective %v", m.explicitIDs, m.effectiveIDs)
	}
	updated, _ = m.Update(reviewBackMsg{})
	m = updated.(Model)
	if got := m.packPicker.picker.UserSelection(); len(got) != 1 || !got["wikiciv"] || !m.packPicker.picker.IsAutoDep("permacomputing") {
		t.Fatalf("picker after review back = explicit %v, auto permacomputing %v", got, m.packPicker.picker.IsAutoDep("permacomputing"))
	}
	updated, _ = m.Update(packCancelMsg{explicitIDs: m.explicitIDs, selectedIDs: m.effectiveIDs})
	m = updated.(Model)
	updated, _ = m.Update(presetDoneMsg{preset: preset})
	m = updated.(Model)
	if got := m.packPicker.picker.UserSelection(); len(got) != 1 || !got["wikiciv"] {
		t.Fatalf("same preset reset explicit selection: %v", got)
	}

	updated, _ = m.Update(presetDoneMsg{preset: PresetOption{Name: "other", Region: "default", SourceIDs: []string{"permacomputing"}}})
	m = updated.(Model)
	if got := m.packPicker.picker.UserSelection(); len(got) != 1 || !got["permacomputing"] {
		t.Fatalf("new preset did not reset selection: %v", got)
	}
}

func TestWizardBlocksMissingReviewConfirmationAndKeepsInitRetryable(t *testing.T) {
	initCalls, applyCalls := 0, 0
	m := sizedModel(testConfig())
	m.stage = stageReview
	m.vaultPath = "/vault"
	m.effectiveIDs = map[string]bool{"wikiciv": true}
	m.missingIDs = []string{"missing"}
	m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), m.missingIDs, m.freeGB)
	m.config.InitVault = func(string, []string, string, string, []string) error { initCalls++; return nil }
	m.config.RunApply = func(_ context.Context, _ string, _ func(ApplyEvent)) error { applyCalls++; return nil }
	updated, _ := m.Update(reviewConfirmMsg{})
	m = updated.(Model)
	if initCalls != 0 || applyCalls != 0 || m.stage != stageReview || !strings.Contains(m.review.errMsg, "missing") {
		t.Fatalf("invalid confirmation: init=%d apply=%d stage=%d error=%q", initCalls, applyCalls, m.stage, m.review.errMsg)
	}

	m.missingIDs = nil
	m.config.InitVault = func(string, []string, string, string, []string) error { initCalls++; return fmt.Errorf("write failed") }
	updated, _ = m.Update(reviewConfirmMsg{})
	m = updated.(Model)
	if initCalls != 1 || applyCalls != 0 || m.stage != stageReview || !strings.Contains(m.review.errMsg, "write failed") || !m.effectiveIDs["wikiciv"] {
		t.Fatalf("init failure state: init=%d apply=%d stage=%d error=%q effective=%v", initCalls, applyCalls, m.stage, m.review.errMsg, m.effectiveIDs)
	}
}

func TestWizardConfirmationUsesSortedEffectiveIDs(t *testing.T) {
	var got []string
	m := sizedModel(testConfig())
	m.stage = stageReview
	m.vaultPath = "/vault"
	m.effectiveIDs = map[string]bool{"z": true, "a": true}
	m.presetName = "preset"
	m.region = "default"
	m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), nil, m.freeGB)
	m.config.InitVault = func(_ string, ids []string, _ string, _ string, _ []string) error {
		got = append([]string(nil), ids...)
		return nil
	}
	_, cmd := m.Update(reviewConfirmMsg{})
	if want := []string{"a", "z"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("InitVault IDs = %v, want %v", got, want)
	}
	if done := cmd().(DoneMsg); strings.Join(done.Result.SelectedIDs, ",") != "a,z" {
		t.Fatalf("WizardResult IDs = %v, want a,z", done.Result.SelectedIDs)
	}
}

// wizardKey feeds actual key-command messages back through the orchestrator.
func wizardKey(t *testing.T, m Model, key tea.KeyMsg) (Model, tea.Msg) {
	t.Helper()
	updated, cmd := m.Update(key)
	m = updated.(Model)
	var msg tea.Msg
	for cmd != nil {
		msg = cmd()
		if _, done := msg.(DoneMsg); done {
			return m, msg
		}
		updated, cmd = m.Update(msg)
		m = updated.(Model)
	}
	return m, msg
}

func wizardToggleSource(t *testing.T, m Model, id string) Model {
	t.Helper()
	for name := range m.packPicker.picker.CollapsedPacks {
		m.packPicker.picker.CollapsedPacks[name] = false
	}
	m.packPicker.picker.RebuildRows()
	for i, row := range m.packPicker.picker.Rows {
		if row.Kind == tui.RowItem && row.Source.ID == id {
			m.packPicker.picker.Cursor = i
			m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			return m
		}
	}
	t.Fatalf("no editable row for %q", id)
	return m
}

func TestWizardKeyPathConfirmsCustomizedPresetAndRetainsAutomaticDependencies(t *testing.T) {
	config := testConfig()
	config.Presets = []PresetOption{{Name: "phase3-snapshot", Region: "custom", SourceIDs: []string{"dufs", "wikiciv"}}}
	config.PackGroups = []PackGroup{{Name: "All", Packs: []Pack{{Name: "all", Sources: []PackSource{
		{ID: "dufs", Type: "binary"}, {ID: "wikiciv", Type: "zim"}, {ID: "permacomputing", Type: "zim"}, {ID: "kiwix-serve", Type: "binary"},
	}}}}}
	config.ResolveDeps = func(selected map[string]bool) (map[string]bool, []string) {
		if (selected["wikiciv"] || selected["permacomputing"]) && !selected["kiwix-serve"] {
			return map[string]bool{"kiwix-serve": true}, nil
		}
		return nil, nil
	}
	var confirmed []string
	initCalls := 0
	config.InitVault = func(_ string, ids []string, preset, region string, _ []string) error {
		initCalls++
		confirmed = append([]string(nil), ids...)
		if preset != "phase3-snapshot" || region != "custom" {
			t.Errorf("provenance changed: %q / %q", preset, region)
		}
		return nil
	}
	m := advancePastPlatforms(t, advancePastPath(t, sizedModel(config)))
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = wizardToggleSource(t, m, "wikiciv")
	m = wizardToggleSource(t, m, "permacomputing")
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.stage != stageReview || initCalls != 0 {
		t.Fatalf("continue bypassed review: stage=%d init=%d", m.stage, initCalls)
	}
	want := "dufs,kiwix-serve,permacomputing"
	var reviewed []string
	for _, item := range m.review.items {
		reviewed = append(reviewed, item.ID)
	}
	if strings.Join(reviewed, ",") != want {
		t.Fatalf("review IDs = %v, want %s", reviewed, want)
	}
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if !m.packPicker.picker.IsAutoDep("kiwix-serve") || m.packPicker.picker.UserSelection()["wikiciv"] {
		t.Fatal("review/back promoted dependency or restored removed item")
	}
	m = wizardToggleSource(t, m, "permacomputing")
	if m.packPicker.picker.CheckedCount() != 1 {
		t.Fatalf("obsolete auto dependency retained: %v", m.packPicker.picker.CheckedIDSlice())
	}
	m = wizardToggleSource(t, m, "permacomputing")
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.packPicker.picker.UserSelection()["wikiciv"] || !m.packPicker.picker.IsAutoDep("kiwix-serve") {
		t.Fatal("cancel/reselect lost edits or promoted dependency")
	}
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, result := wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	done, ok := result.(DoneMsg)
	if !ok || initCalls != 1 || strings.Join(confirmed, ",") != want || strings.Join(done.Result.SelectedIDs, ",") != want {
		t.Fatalf("review/init/result mismatch: init=%d confirmed=%v result=%+v", initCalls, confirmed, result)
	}
	if strings.Join(config.Presets[0].SourceIDs, ",") != "dufs,wikiciv" {
		t.Fatal("base preset mutated")
	}
}

func TestWizardKeyPathRepairsUnavailablePresetBeforeConfirmation(t *testing.T) {
	config := testConfig()
	config.Presets = []PresetOption{{Name: "stale", SourceIDs: []string{"missing", "wikiciv"}}}
	var confirmed []string
	calls := 0
	config.InitVault = func(_ string, ids []string, _, _ string, _ []string) error {
		calls++
		confirmed = append([]string(nil), ids...)
		return nil
	}
	m := advancePastPlatforms(t, advancePastPath(t, sizedModel(config)))
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if calls != 0 || m.stage != stageReview || !strings.Contains(stripAnsi(m.View()), "missing") {
		t.Fatalf("unavailable escaped validation: calls=%d stage=%d", calls, m.stage)
	}
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	m = wizardToggleSource(t, m, "missing")
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, result := wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := result.(DoneMsg); !ok || calls != 1 || strings.Join(confirmed, ",") != "wikiciv" {
		t.Fatalf("repaired selection did not confirm: calls=%d IDs=%v result=%+v", calls, confirmed, result)
	}
}

func TestWizardKeyPathPreservesCustomizeEditsAndAllowsEmptyConfirmation(t *testing.T) {
	config := testConfig()
	calls := 0
	config.InitVault = func(_ string, ids []string, preset, _ string, _ []string) error {
		calls++
		if len(ids) != 0 || preset != "" {
			t.Errorf("empty Customize changed: IDs=%v preset=%q", ids, preset)
		}
		return nil
	}
	m := advancePastPlatforms(t, advancePastPath(t, sizedModel(config)))
	m.presetPicker.cursor = len(config.Presets)
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = wizardToggleSource(t, m, "wikiciv")
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m.presetPicker.cursor = len(config.Presets)
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.packPicker.picker.UserSelection()["wikiciv"] {
		t.Fatal("reselecting Customize lost edits")
	}
	m = wizardToggleSource(t, m, "wikiciv")
	m, _ = wizardKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, result := wizardKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if done, ok := result.(DoneMsg); !ok || calls != 1 || len(done.Result.SelectedIDs) != 0 {
		t.Fatalf("empty Customize did not confirm: calls=%d result=%+v", calls, result)
	}
}

func TestWizardShortViewportShowsPickerAndReviewValidation(t *testing.T) {
	m := sizedModel(testConfig())
	m.width = 80
	m.height = 8
	m.stage = stagePacks
	m.packPicker = newPackPicker(packPickerConfig{
		groups:      m.config.PackGroups,
		explicitIDs: map[string]bool{"missing": true},
	})
	if out := stripAnsi(m.View()); !strings.Contains(out, "Missing catalog recipes: missing") {
		t.Fatalf("short picker viewport hid validation:\n%s", out)
	}
	m.stage = stageReview
	m.review = newReviewModel("/vault", []ReviewItem{{ID: "missing", Unavailable: true}}, []string{"missing"}, 0)
	m.review.errMsg = "Init failed: disk error"
	if out := stripAnsi(m.View()); !strings.Contains(out, "Missing catalog recipes: missing") || !strings.Contains(out, "Init failed: disk error") {
		t.Fatalf("short review viewport hid validation:\n%s", out)
	}
}
