package index

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// stripAnsi removes ANSI escape sequences for plain-text assertions.
func stripAnsi(s string) string { return tui.StripAnsi(s) }

func TestIndexShowsBothOptions(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			SemanticEnabled: true,
		},
	})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	if !strings.Contains(out, "Full index") {
		t.Errorf("View() should contain 'Full index', got:\n%s", out)
	}
	if !strings.Contains(out, "Keyword only") {
		t.Errorf("View() should contain 'Keyword only', got:\n%s", out)
	}
}

func TestIndexNavigate(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			SemanticEnabled: true,
		},
	})
	m.width = 80
	m.height = 24

	// Start at keyword (selected=0)
	if m.selected != 0 {
		t.Fatalf("expected initial selected=0, got %d", m.selected)
	}

	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Move down to semantic
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("after j, expected selected=1 (semantic), got %d", m.selected)
	}

	// Clamp at bottom
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("should clamp at 1, got %d", m.selected)
	}

	// Move back up to keyword
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("after k, expected selected=0 (keyword), got %d", m.selected)
	}

	// Clamp at top
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("should clamp at 0, got %d", m.selected)
	}
}

func TestIndexEscEmitsBack(t *testing.T) {
	m := New(Config{})
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

func TestIndexShowsStatus(t *testing.T) {
	m := New(Config{
		Status: IndexStatus{
			KeywordEnabled:  true,
			KeywordSources:  5,
			KeywordArticles: 1200,
		},
	})
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	if !strings.Contains(out, "1200") {
		t.Errorf("View() should show article count 1200, got:\n%s", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("View() should show source count 5, got:\n%s", out)
	}
}

func TestIndexFailureRemainsVisibleInShellAt80x8(t *testing.T) {
	m := New(Config{})
	m.width, m.height = 80, 8
	m.rebuilding, m.rebuildDone, m.rebuildType = true, true, "configured"
	m.terminal, m.terminalDetail = "failed", "model unavailable"
	if got := m.View(); !strings.Contains(got, "model unavailable") {
		t.Fatalf("80x8 shell hid failure: %q", got)
	}
}

func TestGlobalProgressEventDoesNotCreateStep(t *testing.T) {
	m := New(Config{})
	m.width = 80
	m.height = 24
	m.rebuilding = true
	m.rebuildType = "full"

	m.updateStep(IndexEvent{
		File:   "",
		Status: "indexing",
		Detail: "Starting embedding server...",
	})

	if len(m.steps) != 0 {
		t.Fatalf("global progress event created %d step(s), want 0", len(m.steps))
	}

	out := stripAnsi(m.View())
	if !strings.Contains(out, "Starting embedding server...") {
		t.Fatalf("View() should show global status, got:\n%s", out)
	}
}

func TestIndexFailureAfterProgressCloseUsesTerminalResult(t *testing.T) {
	m := New(Config{RunIndex: func(context.Context, string, func(IndexEvent)) error { return errors.New("model unavailable") }})
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, wait := m.Update(start())
	m = next.(Model)
	next, result := m.Update(wait()) // progress closes before the buffered result is consumed
	m = next.(Model)
	next, _ = m.Update(result())
	m = next.(Model)
	if !m.rebuildDone || m.terminal != "failed" || !strings.Contains(m.terminalDetail, "unavailable") {
		t.Fatalf("terminal result lost after close: %+v", m)
	}
}

func TestIndexCancelAfterProducerCloseKeepsSuccessfulOutcome(t *testing.T) {
	m := New(Config{RunIndex: func(context.Context, string, func(IndexEvent)) error { return nil }})
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, wait := m.Update(start())
	m = next.(Model)
	next, result := m.Update(wait())
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(Model)
	next, _ = m.Update(result())
	m = next.(Model)
	if m.terminal != "finished" {
		t.Fatalf("late Esc relabelled completed index: %q", m.terminal)
	}
}

func TestIndexFailureBeforeLateEscRemainsFailure(t *testing.T) {
	m := New(Config{RunIndex: func(context.Context, string, func(IndexEvent)) error { return errors.New("model unavailable") }})
	next, start := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, wait := m.Update(start())
	m = next.(Model)
	next, result := m.Update(wait())
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(Model)
	next, _ = m.Update(result())
	m = next.(Model)
	if m.terminal != "failed" {
		t.Fatalf("late Esc relabelled failure: %q", m.terminal)
	}
}

func TestIndexAutoStartEscBeforeStartedCancelsProducer(t *testing.T) {
	exited := make(chan struct{})
	m := New(Config{AutoStart: true, RunIndex: func(ctx context.Context, _ string, _ func(IndexEvent)) error {
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(Model)
	started := m.Init()()
	next, wait := m.Update(started)
	m = next.(Model)
	next, result := m.Update(wait())
	m = next.(Model)
	next, _ = m.Update(result())
	m = next.(Model)
	<-exited
	if m.terminal != "cancelled" {
		t.Fatalf("terminal = %q", m.terminal)
	}
}
