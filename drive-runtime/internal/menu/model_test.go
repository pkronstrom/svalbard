package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func sampleGroupedConfig() config.RuntimeConfig {
	return config.RuntimeConfig{
		Version: 2,
		Preset:  "default-32",
		Groups: []config.MenuGroup{
			{
				ID:          "search",
				Label:       "Search",
				Description: "Search across indexed archives and documents.",
				Items: []config.MenuItem{
					{ID: "search-all-content", Label: "Search all content", Description: "Query the on-drive search index.", Action: config.BuiltinAction("search", nil)},
				},
			},
			{
				ID:          "library",
				Label:       "Library",
				Description: "Browse packaged offline archives and documents.",
				Items: []config.MenuItem{
					{ID: "wikipedia-en-nopic", Label: "Wikipedia (text only)", Description: "Browse the image-free English Wikipedia archive.", Subheader: "Archives", Action: config.BuiltinAction("browse", map[string]string{"zim": "wikipedia-en-nopic.zim"})},
					{ID: "wiktionary-en", Label: "Wiktionary", Description: "Open the English Wiktionary archive.", Subheader: "Archives", Action: config.BuiltinAction("browse", map[string]string{"zim": "wiktionary-en.zim"})},
				},
			},
			{
				ID:          "tools",
				Label:       "Tools",
				Description: "Inspect the drive and launch bundled utilities.",
				Items: []config.MenuItem{
					{ID: "inspect-drive", Label: "List drive contents", Description: "Show a terminal summary of the drive contents.", Subheader: "Drive", Action: config.BuiltinAction("inspect", nil)},
				},
			},
		},
	}
}

func TestFilterMatchesGroupLabelAndDescription(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.SetFilter("packaged")

	items := m.VisibleGroups()
	if len(items) != 1 {
		t.Fatalf("len(VisibleGroups()) = %d, want 1", len(items))
	}
	if got, want := items[0].ID, "library"; got != want {
		t.Fatalf("VisibleGroups()[0].ID = %q, want %q", got, want)
	}
}

func TestEnterOpensGroupScreen(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.SetSelected(1)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if !got.inGroup {
		t.Fatal("inGroup = false, want true")
	}
	if got.activeGroup != "library" {
		t.Fatalf("activeGroup = %q, want library", got.activeGroup)
	}
	view := got.View()
	if view == "" || !strings.Contains(view, "Wikipedia (text only)") {
		t.Fatalf("View() did not render group items: %q", view)
	}
	if !strings.Contains(view, "Archives") || !strings.Contains(view, "Browse the image-free English Wikipedia archive.") {
		t.Fatalf("View() did not render subheaders and descriptions: %q", view)
	}
}

func TestEscReturnsFromGroupScreen(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.inGroup = true
	m.activeGroup = "library"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.inGroup {
		t.Fatal("inGroup = true, want false")
	}
	if got.activeGroup != "" {
		t.Fatalf("activeGroup = %q, want empty", got.activeGroup)
	}
}

func TestFilterMatchesGroupItemsInsideGroup(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.inGroup = true
	m.activeGroup = "library"
	m.SetFilter("wiktionary")

	items := m.VisibleItems()
	if len(items) != 1 {
		t.Fatalf("len(VisibleItems()) = %d, want 1", len(items))
	}
	if got, want := items[0].ID, "wiktionary-en"; got != want {
		t.Fatalf("VisibleItems()[0].ID = %q, want %q", got, want)
	}
}

func TestCapturedOutputReplacesMenuUntilDismissed(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")

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
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
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
