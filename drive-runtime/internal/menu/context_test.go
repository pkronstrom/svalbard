package menu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/tui"
)

func TestContextForBrowse(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "browse",
		Label:       "Browse",
		Description: "Browse packaged offline archives.",
		Items: []config.MenuItem{
			{ID: "wiki", Label: "Wikipedia"},
			{ID: "dict", Label: "Wiktionary"},
			{ID: "stack", Label: "StackOverflow"},
		},
	}

	detail := contextForGroup(group, theme)

	rendered := detail.Render()
	if detail.Title != "Browse" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Browse")
	}
	if len(detail.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(detail.Fields))
	}
	if detail.Fields[0].Label != "Archives" {
		t.Fatalf("Fields[0].Label = %q, want %q", detail.Fields[0].Label, "Archives")
	}
	if detail.Fields[0].Value != fmt.Sprintf("%d", len(group.Items)) {
		t.Fatalf("Fields[0].Value = %q, want %q", detail.Fields[0].Value, fmt.Sprintf("%d", len(group.Items)))
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
	if !strings.Contains(rendered, "Browse") {
		t.Fatalf("rendered output missing title: %q", rendered)
	}
	if !strings.Contains(rendered, "Archives") {
		t.Fatalf("rendered output missing field label: %q", rendered)
	}
	if !strings.Contains(rendered, "3") {
		t.Fatalf("rendered output missing item count: %q", rendered)
	}
}

func TestContextForLibrary(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "library",
		Label:       "Library",
		Description: "Browse packaged offline archives and documents.",
		Items: []config.MenuItem{
			{ID: "wiki", Label: "Wikipedia"},
		},
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Library" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Library")
	}
	if len(detail.Fields) != 1 || detail.Fields[0].Label != "Archives" {
		t.Fatalf("Fields = %v, want [{Archives 1}]", detail.Fields)
	}
	if detail.Fields[0].Value != "1" {
		t.Fatalf("Fields[0].Value = %q, want %q", detail.Fields[0].Value, "1")
	}
}

func TestContextForSearch(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "search",
		Label:       "Search",
		Description: "Search across indexed archives and documents.",
		Items: []config.MenuItem{
			{ID: "search-all", Label: "Search all content"},
		},
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Search" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Search")
	}
	if len(detail.Fields) != 0 {
		t.Fatalf("len(Fields) = %d, want 0", len(detail.Fields))
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
}

func TestContextForMaps(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "maps",
		Label:       "Maps",
		Description: "Offline map layers and navigation.",
		Items: []config.MenuItem{
			{ID: "world", Label: "World Map"},
			{ID: "topo", Label: "Topographic"},
		},
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Maps" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Maps")
	}
	if len(detail.Fields) != 1 || detail.Fields[0].Label != "Layers" {
		t.Fatalf("Fields = %v, want [{Layers 2}]", detail.Fields)
	}
	if detail.Fields[0].Value != "2" {
		t.Fatalf("Fields[0].Value = %q, want %q", detail.Fields[0].Value, "2")
	}
}

func TestContextForChat(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "chat",
		Label:       "Chat",
		Description: "Offline messaging and collaboration.",
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Chat" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Chat")
	}
	if len(detail.Fields) != 0 {
		t.Fatalf("len(Fields) = %d, want 0", len(detail.Fields))
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
}

func TestContextForTools(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "tools",
		Label:       "Tools",
		Description: "Inspect the drive and launch bundled utilities.",
		Items: []config.MenuItem{
			{ID: "inspect", Label: "List drive contents"},
			{ID: "verify", Label: "Verify checksums"},
			{ID: "logs", Label: "View logs"},
		},
	}

	detail := contextForGroup(group, theme)

	rendered := detail.Render()
	if detail.Title != "Tools" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Tools")
	}
	if len(detail.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1", len(detail.Fields))
	}
	if detail.Fields[0].Label != "Tools" {
		t.Fatalf("Fields[0].Label = %q, want %q", detail.Fields[0].Label, "Tools")
	}
	if detail.Fields[0].Value != "3" {
		t.Fatalf("Fields[0].Value = %q, want %q", detail.Fields[0].Value, "3")
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
	if !strings.Contains(rendered, "Tools") {
		t.Fatalf("rendered output missing title or field label: %q", rendered)
	}
	if !strings.Contains(rendered, "3") {
		t.Fatalf("rendered output missing tool count: %q", rendered)
	}
}

func TestContextForApps(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "apps",
		Label:       "Apps",
		Description: "Bundled applications.",
		Items: []config.MenuItem{
			{ID: "calc", Label: "Calculator"},
		},
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Apps" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Apps")
	}
	if len(detail.Fields) != 1 || detail.Fields[0].Label != "Tools" {
		t.Fatalf("Fields = %v, want [{Tools 1}]", detail.Fields)
	}
	if detail.Fields[0].Value != "1" {
		t.Fatalf("Fields[0].Value = %q, want %q", detail.Fields[0].Value, "1")
	}
}

func TestContextForVerify(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "verify",
		Label:       "Verify",
		Description: "Verify drive integrity and content checksums.",
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Verify" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Verify")
	}
	if len(detail.Fields) != 0 {
		t.Fatalf("len(Fields) = %d, want 0", len(detail.Fields))
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
}

func TestContextForUnknownGroup(t *testing.T) {
	theme := tui.DefaultTheme()
	group := config.MenuGroup{
		ID:          "custom",
		Label:       "Custom",
		Description: "A custom section.",
	}

	detail := contextForGroup(group, theme)

	if detail.Title != "Custom" {
		t.Fatalf("Title = %q, want %q", detail.Title, "Custom")
	}
	if len(detail.Fields) != 0 {
		t.Fatalf("len(Fields) = %d, want 0", len(detail.Fields))
	}
	if detail.Body != group.Description {
		t.Fatalf("Body = %q, want %q", detail.Body, group.Description)
	}
}
