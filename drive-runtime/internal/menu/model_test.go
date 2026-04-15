package menu_test

import (
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/menu"
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
	m := menu.NewModel(sampleConfig(), "/tmp/drive")
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
	m := menu.NewModel(sampleConfig(), "/tmp/drive")
	m.SetSelected(len(m.VisibleActions()) - 1)

	m.MoveDown()

	if got, want := m.SelectedIndex(), len(m.VisibleActions())-1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d", got, want)
	}
}
