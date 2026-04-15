package menu

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func sampleConfig() config.RuntimeConfig {
	return config.RuntimeConfig{
		Version: 1,
		Preset:  "default-32",
		Actions: []config.MenuAction{
			{Section: "browse", Label: "Browse encyclopedias", Action: "browse", Args: map[string]string{}},
			{Section: "maps", Label: "View maps", Action: "maps", Args: map[string]string{}},
			{Section: "info", Label: "List drive contents", Action: "inspect", Args: map[string]string{}},
		},
	}
}

func TestFilterMatchesLabelAndSection(t *testing.T) {
	m := NewModel(sampleConfig(), "/tmp/drive")
	m.SetFilter("map")

	items := m.VisibleActions()
	if len(items) != 1 {
		t.Fatalf("len(VisibleActions()) = %d, want 1", len(items))
	}
	if got, want := items[0].Action, "maps"; got != want {
		t.Fatalf("VisibleActions()[0].Action = %q, want %q", got, want)
	}
}

func TestMoveSelectionStaysAtLastItem(t *testing.T) {
	m := NewModel(sampleConfig(), "/tmp/drive")
	m.SetSelected(len(m.VisibleActions()) - 1)

	m.MoveDown()

	if got, want := m.SelectedIndex(), len(m.VisibleActions())-1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d", got, want)
	}
}

func TestCapturedOutputReplacesMenuUntilDismissed(t *testing.T) {
	m := NewModel(sampleConfig(), "/tmp/drive")

	updated, _ := m.Update(actionOutputMsg{output: "Drive contents\nzim/\n", err: nil})
	got := updated.(Model)

	if !got.showingOutput {
		t.Fatal("showingOutput = false, want true")
	}
	if got.output != "Drive contents\nzim/\n" {
		t.Fatalf("output = %q", got.output)
	}
	if view := got.View(); view == "" || view == renderView(m) {
		t.Fatalf("View() did not switch to output rendering: %q", view)
	}
}

func TestEnterDismissesCapturedOutput(t *testing.T) {
	m := NewModel(sampleConfig(), "/tmp/drive")
	m.showingOutput = true
	m.output = "Drive contents"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.showingOutput {
		t.Fatal("showingOutput = true, want false")
	}
	if got.output != "" {
		t.Fatalf("output = %q, want empty", got.output)
	}
}
