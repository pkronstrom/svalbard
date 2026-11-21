package wizard

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardIndexEscCancelsAndWaitsForProducer(t *testing.T) {
	exited := make(chan struct{})
	m := newWizardIndex("/vault", func(ctx context.Context, _ string, _ func(string, string, string)) error {
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	})
	started := m.startIndex()()
	updated, wait := m.Update(started)
	m = updated.(wizardIndexModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(wizardIndexModel)
	if cmd != nil || m.finished {
		t.Fatal("Esc left progress before producer closure")
	}
	_ = wait()
	<-exited
}

func TestWizardIndexFailureVisibleAt80x8(t *testing.T) {
	m := newWizardIndex("/vault", nil)
	m.width, m.height = 80, 8
	m.started, m.finished = true, true
	m.terminal, m.terminalDetail = "failed", "model unavailable"
	if got := m.View(); !strings.Contains(got, "Indexing failed") {
		t.Fatalf("80x8 wizard index view hid failure: %q", got)
	}
}

func TestWizardIndexFailureAfterProgressCloseUsesTerminalResult(t *testing.T) {
	m := newWizardIndex("/vault", func(context.Context, string, func(string, string, string)) error {
		return errors.New("model unavailable")
	})
	next, wait := m.Update(m.Init()())
	m = next.(wizardIndexModel)
	next, result := m.Update(wait())
	m = next.(wizardIndexModel)
	next, _ = m.Update(result())
	m = next.(wizardIndexModel)
	if m.terminal != "failed" || !strings.Contains(m.terminalDetail, "unavailable") {
		t.Fatalf("terminal result lost after close: %+v", m)
	}
}

func TestWizardIndexCancelAfterProducerCloseKeepsSuccessfulOutcome(t *testing.T) {
	m := newWizardIndex("/vault", func(context.Context, string, func(string, string, string)) error { return nil })
	next, wait := m.Update(m.Init()())
	m = next.(wizardIndexModel)
	next, result := m.Update(wait())
	m = next.(wizardIndexModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(wizardIndexModel)
	next, _ = m.Update(result())
	m = next.(wizardIndexModel)
	if m.terminal != "finished" {
		t.Fatalf("late Esc relabelled completed index: %q", m.terminal)
	}
}

func TestWizardIndexFailureBeforeLateEscRemainsFailure(t *testing.T) {
	m := newWizardIndex("/vault", func(context.Context, string, func(string, string, string)) error {
		return errors.New("model unavailable")
	})
	next, wait := m.Update(m.Init()())
	m = next.(wizardIndexModel)
	next, result := m.Update(wait())
	m = next.(wizardIndexModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(wizardIndexModel)
	next, _ = m.Update(result())
	m = next.(wizardIndexModel)
	if m.terminal != "failed" {
		t.Fatalf("late Esc relabelled failure: %q", m.terminal)
	}
}

func TestWizardIndexTerminalVisibleInShellAt80x8(t *testing.T) {
	w := New(testConfig())
	w.width, w.height, w.stage = 80, 8, stageIndex
	w.indexModel = newWizardIndex("/vault", nil)
	w.indexModel.started, w.indexModel.finished = true, true
	w.indexModel.terminal, w.indexModel.terminalDetail = "cancelled", ""
	if got := stripAnsi(w.View()); !strings.Contains(got, "Indexing cancelled") {
		t.Fatalf("wrapped 80x8 wizard hid cancellation: %q", got)
	}
	w.indexModel.terminal, w.indexModel.terminalDetail = "failed", "model unavailable"
	if got := stripAnsi(w.View()); !strings.Contains(got, "Indexing failed") {
		t.Fatalf("wrapped 80x8 wizard hid failure: %q", got)
	}
}
