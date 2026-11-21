package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// stripAnsi removes ANSI escape sequences for plain-text assertions.
func stripAnsi(s string) string { return tui.StripAnsi(s) }

func sampleItems() []PlanItem {
	return []PlanItem{
		{ID: "wiki-physics", Type: "zim", SizeGB: 1.2, Description: "Wikipedia Physics", Action: "download"},
		{ID: "wiki-math", Type: "zim", SizeGB: 0.8, Description: "Wikipedia Math", Action: "download"},
		{ID: "wiki-old", Type: "zim", SizeGB: 0.5, Description: "Wikipedia Old", Action: "remove"},
	}
}

func TestPlanShowsItems(t *testing.T) {
	items := sampleItems()
	m := New(Config{
		Items:      items,
		DownloadGB: 2.0,
		RemoveGB:   0.5,
	})
	m.width = 120
	m.height = 24

	out := stripAnsi(m.View())

	for _, it := range items {
		if !strings.Contains(out, it.ID) {
			t.Errorf("View() should contain item ID %q, got:\n%s", it.ID, out)
		}
	}
}

func TestPlanEmptyShowsNoRecipeChanges(t *testing.T) {
	m := New(Config{})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	if !strings.Contains(out, "No recipe downloads or removals pending") {
		t.Errorf("empty plan should describe recipe work without claiming runtime health: %s", out)
	}
}

func TestPlanEscEmitsBack(t *testing.T) {
	m := New(Config{Items: sampleItems()})
	m.width = 80
	m.height = 24

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(escMsg)
	if cmd == nil {
		t.Fatal("pressing Esc should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestPlanBKeyEmitsBrowse(t *testing.T) {
	m := New(Config{Items: sampleItems()})
	m.width = 80
	m.height = 24

	bMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	_, cmd := m.Update(bMsg)
	if cmd == nil {
		t.Fatal("pressing 'b' should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(BrowseMsg); !ok {
		t.Errorf("expected BrowseMsg, got %T", msg)
	}
}

func TestPlanScrolling(t *testing.T) {
	items := sampleItems()
	m := New(Config{Items: items})
	m.width = 80
	m.height = 24

	// Start at cursor 0
	if m.cursor != 0 {
		t.Fatalf("expected initial cursor=0, got %d", m.cursor)
	}

	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Move down
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 1 {
		t.Errorf("after j, expected cursor=1, got %d", m.cursor)
	}

	// Move down again
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 2 {
		t.Errorf("after second j, expected cursor=2, got %d", m.cursor)
	}

	// Clamp at bottom
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.cursor != 2 {
		t.Errorf("should clamp at %d, got %d", len(items)-1, m.cursor)
	}

	// Move up
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.cursor != 1 {
		t.Errorf("after k, expected cursor=1, got %d", m.cursor)
	}

	// Back to top and clamp
	result, _ = m.Update(upMsg)
	m = result.(Model)
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.cursor != 0 {
		t.Errorf("should clamp at 0, got %d", m.cursor)
	}
}

func TestPlanEnterStartsApply(t *testing.T) {
	items := sampleItems()
	m := New(Config{
		Items:      items,
		DownloadGB: 2.0,
		RemoveGB:   0.5,
		RunApply: func(ctx context.Context, onProgress func(ApplyEvent)) error {
			return nil
		},
	})
	m.width = 80
	m.height = 24

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if !m.applying {
		t.Error("after Enter with RunApply set, expected applying=true")
	}
	if cmd == nil {
		t.Error("after Enter with RunApply set, expected a non-nil command")
	}
	if len(m.applyItems) != len(items) {
		t.Errorf("expected %d applyItems, got %d", len(items), len(m.applyItems))
	}
}

func executePlanCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing apply command")
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	select {
	case msg := <-result:
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("apply command stranded")
		return nil
	}
}

func drainPlanApply(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	for !m.applyDone {
		next, nextCmd := m.Update(executePlanCmd(t, cmd))
		m, cmd = next.(Model), nextCmd
	}
	return m, cmd
}

func TestApplyGlobalErrorPreservesPendingRows(t *testing.T) {
	m := New(Config{Items: sampleItems(), RunApply: func(context.Context, func(ApplyEvent)) error { return errors.New("recipe not found in catalog") }})
	m.width, m.height = 120, 30
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = drainPlanApply(t, next.(Model), cmd)
	for _, step := range m.applyItems {
		if step.status != "" {
			t.Fatalf("untouched row marked attempted: %+v", step)
		}
	}
	out := stripAnsi(m.View())
	for _, want := range []string{"not found", "Pending: 3", "remaining plan"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestApplyPerItemFailureDoesNotFailUntouchedRows(t *testing.T) {
	m := New(Config{Items: sampleItems(), RunApply: func(ctx context.Context, progress func(ApplyEvent)) error {
		progress(ApplyEvent{ID: "wiki-physics", Status: tui.StatusFailed, Error: "download failed"})
		return errors.New("download failed")
	}})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = drainPlanApply(t, next.(Model), cmd)
	if m.applyItems[0].status != tui.StatusFailed {
		t.Fatal("attempted failure missing")
	}
	for _, step := range m.applyItems[1:] {
		if step.status != "" {
			t.Fatalf("untouched row marked failed: %+v", step)
		}
	}
	if !strings.Contains(m.applyErr, "download failed") {
		t.Fatal("terminal error lost")
	}
}

func TestPlanCancelBeforeStartWaitsForTerminalQuit(t *testing.T) {
	observed := make(chan error, 1)
	m := New(Config{Items: sampleItems(), RunApply: func(ctx context.Context, _ func(ApplyEvent)) error { observed <- ctx.Err(); return ctx.Err() }})
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, action := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(Model)
	if action != nil || !m.stopping {
		t.Fatal("quit before terminal save")
	}
	m, action = drainPlanApply(t, m, start)
	if !errors.Is(<-observed, context.Canceled) {
		t.Fatal("early cancel was discarded")
	}
	if _, ok := executePlanCmd(t, action).(tea.QuitMsg); !ok {
		t.Fatal("terminal result did not quit")
	}
}

func TestPlanBackWaitsForSaveAndReconcilesPostCancelPeer(t *testing.T) {
	started, saveRelease := make(chan struct{}), make(chan struct{})
	m := New(Config{Items: []PlanItem{{ID: "peer"}, {ID: "pending"}}, RunApply: func(ctx context.Context, progress func(ApplyEvent)) error {
		close(started)
		<-ctx.Done()
		<-saveRelease
		progress(ApplyEvent{ID: "peer", Status: tui.StatusDone})
		return ctx.Err()
	}})
	m.width, m.height = 120, 30
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, wait := m.Update(executePlanCmd(t, start))
	m = next.(Model)
	<-started
	calls := 0
	originalCancel := m.applyCancel
	m.applyCancel = func() { calls++; originalCancel() }
	for range 2 {
		next, action := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = next.(Model)
		if action != nil {
			t.Fatal("back before save")
		}
	}
	if calls != 1 || !strings.Contains(stripAnsi(m.View()), "Stopping") {
		t.Fatal("cancellation is not idempotent/visible")
	}
	close(saveRelease)
	m, action := drainPlanApply(t, m, wait)
	if _, ok := executePlanCmd(t, action).(BackMsg); !ok {
		t.Fatal("saved cancellation did not go back")
	}
	if m.applyItems[0].status != tui.StatusDone || m.applyItems[1].status != "" {
		t.Fatalf("post-cancel checkpoint lost: %+v", m.applyItems)
	}
}

func TestPlanSuccessfulTerminalHonorsRecordedBackAndQuitIntent(t *testing.T) {
	for _, intent := range []string{"back", "quit"} {
		m := New(Config{})
		m.applying, m.exitIntent = true, intent
		_, cmd := m.Update(applyResultMsg{}) // acquisition completed before late cancellation reached its worker
		if cmd == nil {
			t.Fatalf("%s intent was ignored by nil-error result", intent)
		}
		msg := executePlanCmd(t, cmd)
		if intent == "back" {
			if _, ok := msg.(BackMsg); !ok {
				t.Fatalf("back result = %T", msg)
			}
		} else if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("quit result = %T", msg)
		}
	}
}

func TestEmptyPlanEnterFinalizesAndBStillBrowses(t *testing.T) {
	calls := 0
	cfg := Config{RunApply: func(context.Context, func(ApplyEvent)) error { calls++; return errors.New("finalization failed") }}
	m := New(cfg)
	m.width, m.height = 120, 30
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.applying || !strings.Contains(stripAnsi(m.View()), "Applying") {
		t.Fatal("empty plan cannot enter visible finalization")
	}
	m, _ = drainPlanApply(t, m, cmd)
	if calls != 1 || !strings.Contains(stripAnsi(m.View()), "finalization failed") {
		t.Fatal("empty finalization outcome lost")
	}
	m = New(cfg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if _, ok := executePlanCmd(t, cmd).(BrowseMsg); !ok {
		t.Fatal("existing browse path lost")
	}
}

func TestPlanConcurrentProgressSnapshots(t *testing.T) {
	items := make([]PlanItem, 8)
	for i := range items {
		items[i] = PlanItem{ID: fmt.Sprint(i)}
	}
	m := New(Config{Items: items, RunApply: func(ctx context.Context, progress func(ApplyEvent)) error {
		var wg sync.WaitGroup
		for _, item := range items {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				for range 12 {
					progress(ApplyEvent{ID: id, Status: tui.StatusActive})
				}
				progress(ApplyEvent{ID: id, Status: tui.StatusDone})
			}(item.ID)
		}
		wg.Wait()
		return nil
	}})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = drainPlanApply(t, next.(Model), cmd)
	for _, step := range m.applyItems {
		if step.status != tui.StatusDone {
			t.Fatalf("completion lost: %+v", step)
		}
	}
}

func TestPlanShowsBuildResourceRequirements(t *testing.T) {
	m := New(Config{
		Items: []PlanItem{{
			ID:              "compact-zim",
			Type:            "zim",
			SizeGB:          1.2,
			Action:          "download",
			Network:         true,
			BuildDownloadGB: 2.4,
			BuildWorkGB:     4,
			Requires:        []string{"zimwriterfs"},
		}},
		BuildDownloadGB: 2.4,
		BuildWorkGB:     4,
		ToolsRequired:   true,
	})
	m.width = 100
	m.height = 40
	view := stripAnsi(m.View())
	for _, want := range []string{"network", "~2.4 GB source", "~4.0 GB staging", "tools: zimwriterfs", "tools image required"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q:\\n%s", want, view)
		}
	}
}

func TestPlanPendingWithoutReturnedErrorStillShowsIncompleteOutcome(t *testing.T) {
	m := New(Config{Items: sampleItems(), RunApply: func(context.Context, func(ApplyEvent)) error { return nil }})
	m.width, m.height = 120, 30
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = drainPlanApply(t, next.(Model), cmd)
	view := stripAnsi(m.View())
	if !strings.Contains(view, "Pending: 3") || !strings.Contains(view, "remaining plan") {
		t.Fatalf("pending result omitted incomplete/resume state: %s", view)
	}
}

func TestCompletedPlanCtrlCQuitsWithoutWaitingForAnotherResult(t *testing.T) {
	m := New(Config{RunApply: func(context.Context, func(ApplyEvent)) error { return nil }})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = drainPlanApply(t, next.(Model), cmd)
	_, quit := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quit == nil {
		t.Fatal("completed apply waits for a terminal result that already arrived")
	}
	if _, ok := executePlanCmd(t, quit).(tea.QuitMsg); !ok {
		t.Fatal("completed apply did not quit")
	}
}

func TestEmptyPlanHintsMatchEnterAndBrowseActions(t *testing.T) {
	for _, available := range []bool{false, true} {
		cfg := Config{}
		if available {
			cfg.RunApply = func(context.Context, func(ApplyEvent)) error { return nil }
		}
		m := New(cfg)
		m.width, m.height = 120, 30
		view := stripAnsi(m.View())
		if strings.Contains(view, "Everything in sync") || !strings.Contains(view, "No recipe downloads or removals pending") || !strings.Contains(view, "b: browse") {
			t.Fatalf("misleading empty state: %s", view)
		}
		expected := "Enter: browse"
		if available {
			expected = "Enter: apply/finalize"
		}
		if !strings.Contains(view, expected) {
			t.Fatalf("Enter hint does not match action: %s", view)
		}
	}
}
