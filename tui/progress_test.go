package tui_test

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

// TestProgressViewShowsPhases creates a view with Done, Active, and Pending
// steps and verifies all labels appear in the rendered output.
func TestProgressViewShowsPhases(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Title: "Applying host config",
		Steps: []tui.ProgressStep{
			{Label: "Validate config", Status: tui.StepDone},
			{Label: "Apply changes", Status: tui.StepActive},
			{Label: "Restart services", Status: tui.StepPending},
		},
	}

	out := view.Render()

	// Title must appear
	if !strings.Contains(out, "Applying host config") {
		t.Errorf("expected title in output, got:\n%s", out)
	}

	// All step labels must appear
	for _, label := range []string{"Validate config", "Apply changes", "Restart services"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected label %q in output, got:\n%s", label, out)
		}
	}

	// Status icons must appear
	if !strings.Contains(out, "[done]") {
		t.Errorf("expected [done] icon in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[....]") {
		t.Errorf("expected [....] icon in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[    ]") {
		t.Errorf("expected [    ] icon in output, got:\n%s", out)
	}
}

// TestProgressViewShowsError creates a view with a Failed step that has an
// error message and verifies the error text appears in the rendered output.
func TestProgressViewShowsError(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{Label: "Download artifact", Status: tui.StepDone},
			{Label: "Verify checksum", Status: tui.StepFailed, Error: "sha256 mismatch: expected abc123"},
		},
	}

	out := view.Render()

	// FAIL icon must appear
	if !strings.Contains(out, "[FAIL]") {
		t.Errorf("expected [FAIL] icon in output, got:\n%s", out)
	}

	// Error message must appear
	if !strings.Contains(out, "sha256 mismatch: expected abc123") {
		t.Errorf("expected error message in output, got:\n%s", out)
	}
}

// TestProgressViewShowsLog verifies that optional log content is rendered.
func TestProgressViewShowsLog(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{Label: "Indexing files", Status: tui.StepActive},
		},
		Log: "Processed 142 of 500 files...",
	}

	out := view.Render()

	if !strings.Contains(out, "Processed 142 of 500 files...") {
		t.Errorf("expected log content in output, got:\n%s", out)
	}
}

// TestProgressViewEmptyTitle verifies no title line when Title is empty.
func TestProgressViewEmptyTitle(t *testing.T) {
	view := tui.ProgressView{
		Theme: tui.DefaultTheme(),
		Steps: []tui.ProgressStep{
			{Label: "Do something", Status: tui.StepPending},
		},
	}

	out := view.Render()

	// First non-empty content should be the step, not a blank line at start
	lines := strings.Split(out, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		t.Errorf("expected no leading blank line when title is empty, got:\n%s", out)
	}
}
