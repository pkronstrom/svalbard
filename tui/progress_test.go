package tui_test

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

func TestProgressViewShowsPhases(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{ID: "a", Label: "Validate config", Status: tui.StatusDone},
			{ID: "b", Label: "Apply changes", Status: tui.StatusActive},
			{ID: "c", Label: "Restart services", Status: tui.StatusQueued},
		},
	}

	out := stripAnsi(view.Render())

	for _, label := range []string{"Validate config", "Apply changes", "Restart services"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, out)
		}
	}
}

func TestProgressViewShowsError(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{ID: "a", Label: "Download artifact", Status: tui.StatusDone},
			{ID: "b", Label: "Verify checksum", Status: tui.StatusFailed, Error: "sha256 mismatch"},
		},
	}

	out := stripAnsi(view.Render())

	if !strings.Contains(out, "sha256 mismatch") {
		t.Errorf("expected error message in output, got:\n%s", out)
	}
}

func TestProgressViewSummary(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{ID: "a", Status: tui.StatusDone},
			{ID: "b", Status: tui.StatusDone},
			{ID: "c", Status: tui.StatusActive},
			{ID: "d", Status: tui.StatusFailed},
		},
	}

	out := stripAnsi(view.RenderSummary())

	if !strings.Contains(out, "2/4 done") {
		t.Errorf("expected '2/4 done' in summary, got: %s", out)
	}
	if !strings.Contains(out, "1 active") {
		t.Errorf("expected '1 active' in summary, got: %s", out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("expected '1 failed' in summary, got: %s", out)
	}
}

func TestProgressViewScrollToTail(t *testing.T) {
	steps := make([]tui.ProgressStep, 20)
	for i := range steps {
		steps[i] = tui.ProgressStep{ID: "s", Label: "step", Status: tui.StatusDone}
	}
	steps[19].Label = "last-step"

	view := tui.ProgressView{
		Theme:        tui.DefaultTheme(),
		Steps:        steps,
		MaxVisible:   5,
		ScrollToTail: true,
	}

	out := stripAnsi(view.Render())

	if !strings.Contains(out, "last-step") {
		t.Errorf("expected 'last-step' visible with ScrollToTail, got:\n%s", out)
	}
}

func TestStatusIconQueued(t *testing.T) {
	out := stripAnsi(tui.StatusIcon(tui.StatusQueued, tui.DefaultTheme()))
	if out != "○" {
		t.Errorf("expected ○ for StatusQueued, got %q", out)
	}
}
