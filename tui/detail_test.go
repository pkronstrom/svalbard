package tui_test

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

// TestDetailPaneRendersFields verifies that title, fields (Label+Value),
// and body all appear in the rendered output.
func TestDetailPaneRendersFields(t *testing.T) {
	theme := tui.DefaultTheme()

	pane := tui.DetailPane{
		Theme: theme,
		Title: "Overview",
		Fields: []tui.DetailField{
			{Label: "Name", Value: "glacier-alpha"},
			{Label: "Status", Value: "running"},
		},
		Body: "This drive is performing well.",
	}

	output := pane.Render()

	// Title must appear
	if !strings.Contains(output, "Overview") {
		t.Error("expected output to contain title 'Overview'")
	}

	// Field labels and values must appear
	if !strings.Contains(output, "Name") {
		t.Error("expected output to contain label 'Name'")
	}
	if !strings.Contains(output, "glacier-alpha") {
		t.Error("expected output to contain value 'glacier-alpha'")
	}
	if !strings.Contains(output, "Status") {
		t.Error("expected output to contain label 'Status'")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected output to contain value 'running'")
	}

	// Body must appear
	if !strings.Contains(output, "This drive is performing well.") {
		t.Error("expected output to contain body text")
	}
}

// TestDetailPaneEmptyGraceful verifies that an empty DetailPane renders
// the "No details available." fallback message.
func TestDetailPaneEmptyGraceful(t *testing.T) {
	theme := tui.DefaultTheme()

	pane := tui.DetailPane{
		Theme: theme,
	}

	output := pane.Render()

	if !strings.Contains(output, "No details available.") {
		t.Errorf("expected 'No details available.' in output, got: %q", output)
	}
}
