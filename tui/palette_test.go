package tui_test

import (
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

func TestPaletteMatchesLabel(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "overview", Label: "Overview"},
			{ID: "add-content", Label: "Add Content"},
			{ID: "plan", Label: "Plan"},
		},
	}

	results := p.Match("plan")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "plan" {
		t.Errorf("expected ID=plan, got %q", results[0].ID)
	}
}

func TestPaletteMatchesAlias(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "wikipedia", Label: "Wikipedia", Aliases: []string{"wiki"}},
		},
	}

	results := p.Match("wiki")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "wikipedia" {
		t.Errorf("expected ID=wikipedia, got %q", results[0].ID)
	}
}

func TestPaletteMatchesFuzzy(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "add-content", Label: "Add Content"},
			{ID: "remove-content", Label: "Remove Content"},
		},
	}

	results := p.Match("add")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "add-content" {
		t.Errorf("expected ID=add-content, got %q", results[0].ID)
	}
}

func TestPaletteMatchesVerbPrefix(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "wikipedia", Label: "Wikipedia", Verbs: []string{"browse", "open"}},
		},
	}

	results := p.Match("browse wikipedia")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "wikipedia" {
		t.Errorf("expected ID=wikipedia, got %q", results[0].ID)
	}
	if results[0].FreeformArg != "" {
		t.Errorf("expected empty FreeformArg, got %q", results[0].FreeformArg)
	}
}

func TestPaletteImportPrefill(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "import", Label: "Import", Verbs: []string{"import"}, AcceptsFreeform: true},
		},
	}

	results := p.Match("import /path/to/file.pdf")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "import" {
		t.Errorf("expected ID=import, got %q", results[0].ID)
	}
	if results[0].FreeformArg != "/path/to/file.pdf" {
		t.Errorf("expected FreeformArg=/path/to/file.pdf, got %q", results[0].FreeformArg)
	}
}

func TestPaletteEmptyQuery(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
	}

	results := p.Match("")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestPaletteNoMatch(t *testing.T) {
	p := tui.Palette{
		Entries: []tui.PaletteEntry{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
	}

	results := p.Match("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}
