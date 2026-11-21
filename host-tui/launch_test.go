package hosttui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/host-tui/internal/plan"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
)

func TestWizardAdapterForwardsExactContextAndProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rebuilt := false
	target := &DashboardDeps{RunApply: func(got context.Context, progress func(ApplyEvent)) error {
		if got != ctx {
			t.Fatal("wizard context replaced")
		}
		progress(ApplyEvent{ID: "item", Status: "active", Step: "unpack", Downloaded: 5, Total: 10})
		return got.Err()
	}}
	deps := &DashboardDeps{RunApply: target.RunApply, RebuildForVault: func(path string) *DashboardDeps { rebuilt = path == "chosen-vault"; return target }}
	app := newAppModel(nil, nil, deps)
	cfg := app.defaultWizardConfig()
	var event wizard.ApplyEvent
	err := cfg.RunApply(ctx, "chosen-vault", func(ev wizard.ApplyEvent) { event = ev })
	if !rebuilt || !errors.Is(err, context.Canceled) {
		t.Fatalf("adapter did not retain cancellation: %v", err)
	}
	if event.Step != "unpack" || event.Downloaded != 5 || event.Total != 10 {
		t.Fatalf("wizard progress dropped: %+v", event)
	}
}

func TestWizardAdapterUsesRebuiltShouldIndex(t *testing.T) {
	target := &DashboardDeps{ShouldIndex: func(path string) bool { return path != "chosen-vault" }}
	deps := &DashboardDeps{RebuildForVault: func(path string) *DashboardDeps {
		if path != "chosen-vault" {
			t.Fatalf("rebuild path = %q", path)
		}
		return target
	}}
	app := newAppModel(nil, nil, deps)
	if cfg := app.defaultWizardConfig(); cfg.ShouldIndex == nil || cfg.ShouldIndex("chosen-vault") {
		t.Fatal("wizard did not receive rebuilt disabled-index query")
	}
}

func TestApplySuccessTransitionsToConfiguredIndex(t *testing.T) {
	calls := 0
	stopped := make(chan struct{})
	deps := &DashboardDeps{RunConfiguredIndex: func(ctx context.Context, path string, _ func(IndexEvent)) error {
		calls++
		if path != "vault" {
			t.Fatalf("configured index path = %q", path)
		}
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}}
	app := newAppModel(nil, nil, deps)
	app.screen, app.vaultPath = screenPlan, "vault"
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = updated.(*appModel)
	updated, cmd := app.Update(plan.ApplySucceededMsg{})
	app = updated.(*appModel)
	if app.screen != screenIndex || cmd == nil {
		t.Fatal("apply success did not start configured index screen")
	}
	updated, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = updated.(*appModel)
	updated, wait := app.Update(app.indexScr.Init()())
	app = updated.(*appModel)
	if !strings.Contains(app.indexScr.View(), "Rebuilding keyword index") {
		t.Fatal("configured index did not render progress state")
	}
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = updated.(*appModel)
	updated, result := app.Update(wait())
	app = updated.(*appModel)
	updated, _ = app.Update(result())
	app = updated.(*appModel)
	<-stopped
	if calls != 1 {
		t.Fatalf("configured runner calls = %d", calls)
	}
}

func TestConfiguredIndexEnterReturnsToDashboard(t *testing.T) {
	calls := 0
	deps := &DashboardDeps{RunConfiguredIndex: func(context.Context, string, func(IndexEvent)) error { calls++; return nil }}
	app := newAppModel(nil, nil, deps)
	app.screen, app.vaultPath = screenPlan, "vault"
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = updated.(*appModel)
	updated, _ = app.Update(plan.ApplySucceededMsg{})
	app = updated.(*appModel)
	updated, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = updated.(*appModel)
	updated, wait := app.Update(app.indexScr.Init()())
	app = updated.(*appModel)
	updated, result := app.Update(wait())
	app = updated.(*appModel)
	updated, _ = app.Update(result())
	app = updated.(*appModel)
	updated, back := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(*appModel)
	updated, _ = app.Update(back())
	app = updated.(*appModel)
	if app.screen != screenDashboard || calls != 1 {
		t.Fatalf("configured acknowledgement = screen %d calls %d", app.screen, calls)
	}
}

func TestApplySuccessSkipsDisabledConfiguredIndex(t *testing.T) {
	calls := 0
	deps := &DashboardDeps{
		ShouldIndex: func(path string) bool { return path != "vault" },
		RunConfiguredIndex: func(context.Context, string, func(IndexEvent)) error {
			calls++
			return nil
		},
	}
	app := newAppModel(nil, nil, deps)
	app.screen, app.vaultPath = screenPlan, "vault"
	updated, _ := app.Update(plan.ApplySucceededMsg{})
	app = updated.(*appModel)
	if app.screen != screenDashboard || calls != 0 {
		t.Fatalf("disabled indexing state = screen %d calls %d", app.screen, calls)
	}
}
