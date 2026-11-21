package wizard

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

func executeWizardApplyCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
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

func drainWizardApply(t *testing.T, m wizardApplyModel, cmd tea.Cmd) (wizardApplyModel, tea.Cmd) {
	t.Helper()
	for !m.finished {
		next, nextCmd := m.Update(executeWizardApplyCmd(t, cmd))
		m, cmd = next.(wizardApplyModel), nextCmd
	}
	return m, cmd
}

func TestWizardCancelBeforeStartWaitsForTerminalQuit(t *testing.T) {
	observed := make(chan error, 1)
	m := newWizardApply("vault", []string{"pending"}, func(ctx context.Context, _ string, _ func(ApplyEvent)) error { observed <- ctx.Err(); return ctx.Err() })
	start := m.Init()
	next, action := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(wizardApplyModel)
	if action != nil || !m.stopping {
		t.Fatal("wizard quit before saving")
	}
	m, action = drainWizardApply(t, m, start)
	if !errors.Is(<-observed, context.Canceled) {
		t.Fatal("cancel-before-start lost")
	}
	if _, ok := executeWizardApplyCmd(t, action).(tea.QuitMsg); !ok {
		t.Fatal("saved wizard did not quit")
	}
}

func TestWizardCancelWaitsForSaveAndRetainsPeerOutcome(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyCtrlC, tea.KeyEsc} {
		t.Run(fmt.Sprint(key), func(t *testing.T) {
			started, saveRelease := make(chan struct{}), make(chan struct{})
			m := newWizardApply("vault", []string{"peer", "pending"}, func(ctx context.Context, _ string, progress func(ApplyEvent)) error {
				close(started)
				<-ctx.Done()
				<-saveRelease
				progress(ApplyEvent{ID: "peer", Status: tui.StatusDone})
				return ctx.Err()
			})
			next, wait := m.Update(executeWizardApplyCmd(t, m.Init()))
			m = next.(wizardApplyModel)
			<-started
			calls := 0
			cancel := m.cancel
			m.cancel = func() { calls++; cancel() }
			for range 2 {
				next, action := m.Update(tea.KeyMsg{Type: key})
				m = next.(wizardApplyModel)
				if action != nil {
					t.Fatal("navigation before save")
				}
			}
			if calls != 1 || !strings.Contains(stripAnsi(m.View()), "Stopping") {
				t.Fatal("cancel was not idempotent/visible")
			}
			close(saveRelease)
			m, action := drainWizardApply(t, m, wait)
			if m.steps[0].status != tui.StatusDone || m.steps[1].status != "" {
				t.Fatalf("post-cancel peer checkpoint lost: %+v", m.steps)
			}
			msg := executeWizardApplyCmd(t, action)
			if key == tea.KeyCtrlC {
				if _, ok := msg.(tea.QuitMsg); !ok {
					t.Fatal("Ctrl+C did not quit after save")
				}
			} else {
				done, ok := msg.(wizardApplyDoneMsg)
				if !ok || done.success {
					t.Fatal("Esc did not return to dashboard after saving")
				}
			}
		})
	}
}

func TestWizardIncompleteEnterSkipsIndexAndShowsPending(t *testing.T) {
	for _, terminalErr := range []error{nil, errors.New("global save failure")} {
		t.Run(fmt.Sprint(terminalErr), func(t *testing.T) {
			m := newWizardApply("vault", []string{"complete", "pending"}, func(ctx context.Context, _ string, progress func(ApplyEvent)) error {
				progress(ApplyEvent{ID: "complete", Status: tui.StatusDone})
				return terminalErr
			})
			m.height = 30
			m, _ = drainWizardApply(t, m, m.Init())
			view := stripAnsi(m.View())
			if !strings.Contains(strings.ToLower(view), "pending") || strings.Contains(view, "2 items applied") {
				t.Fatalf("incomplete result rendered success: %s", view)
			}
			if terminalErr != nil && !strings.Contains(view, terminalErr.Error()) {
				t.Fatal("empty-ID/global error lost")
			}
			_, action := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			msg := executeWizardApplyCmd(t, action).(wizardApplyDoneMsg)
			if msg.success {
				t.Fatal("pending apply marked successful")
			}
			indexed := false
			cfg := testConfig()
			cfg.RunIndex = func(context.Context, string, func(string, string, string)) error { indexed = true; return nil }
			parent := New(cfg)
			parent.stage = stageApply
			parent.applyModel = m
			next, done := parent.Update(msg)
			parent = next.(Model)
			if parent.stage == stageIndex || indexed || done == nil {
				t.Fatal("incomplete wizard entered indexing")
			}
			if _, ok := executeWizardApplyCmd(t, done).(DoneMsg); !ok {
				t.Fatal("incomplete wizard did not return vault result")
			}
		})
	}
}

func TestWizardParentForwardsActiveApplyCtrlC(t *testing.T) {
	parent := New(testConfig())
	parent.stage = stageApply
	parent.applyModel = newWizardApply("vault", []string{"pending"}, func(ctx context.Context, _ string, _ func(ApplyEvent)) error { return ctx.Err() })
	next, action := parent.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	parent = next.(Model)
	if action != nil || !parent.applyModel.stopping || parent.applyModel.ctx.Err() != context.Canceled {
		t.Fatal("parent bypassed apply cleanup")
	}
}

func TestWizardConcurrentProgressSnapshots(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	m := newWizardApply("vault", ids, func(ctx context.Context, _ string, progress func(ApplyEvent)) error {
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				for range 12 {
					progress(ApplyEvent{ID: id, Status: tui.StatusActive})
				}
				progress(ApplyEvent{ID: id, Status: tui.StatusDone})
			}(id)
		}
		wg.Wait()
		return nil
	})
	m, _ = drainWizardApply(t, m, m.Init())
	for _, step := range m.steps {
		if step.status != tui.StatusDone {
			t.Fatalf("completion lost: %+v", step)
		}
	}
	_, action := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if done := executeWizardApplyCmd(t, action).(wizardApplyDoneMsg); !done.success {
		t.Fatal("completed wizard did not permit indexing")
	}
}
