package menu

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimesearch"
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

func TestQReturnsFromGroupScreen(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.inGroup = true
	m.activeGroup = "library"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
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

type fakeSearchSession struct {
	info      runtimesearch.SessionInfo
	response  runtimesearch.SearchResponse
	searchErr error
	openErr   error
	searches  []string
	opened    []runtimesearch.Result
	closed    bool
}

func (f *fakeSearchSession) Info() runtimesearch.SessionInfo {
	return f.info
}

func (f *fakeSearchSession) Search(_ context.Context, mode runtimesearch.Mode, query string) (runtimesearch.SearchResponse, error) {
	f.searches = append(f.searches, string(mode)+":"+query)
	return f.response, f.searchErr
}

func (f *fakeSearchSession) OpenResult(result runtimesearch.Result) error {
	f.opened = append(f.opened, result)
	return f.openErr
}

func (f *fakeSearchSession) Close() error {
	f.closed = true
	return nil
}

func TestEnterOnSearchItemOpensSearchSession(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{
		info: runtimesearch.SessionInfo{
			SourceCount:  2,
			ArticleCount: 10,
			BestMode:     runtimesearch.ModeKeyword,
		},
	}
	m.searchFactory = func(string) (searchSession, error) { return fake, nil }
	m.inGroup = true
	m.activeGroup = "search"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if !got.searchActive {
		t.Fatal("searchActive = false, want true")
	}
	if got.inGroup {
		t.Fatal("inGroup = true, want false")
	}
	if got.searchMode != runtimesearch.ModeKeyword {
		t.Fatalf("searchMode = %q", got.searchMode)
	}
}

func TestSearchEscClearsQueryThenLeavesSession(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{info: runtimesearch.SessionInfo{BestMode: runtimesearch.ModeKeyword}}
	m.searchFactory = func(string) (searchSession, error) { return fake, nil }
	if err := m.openSearchSession(); err != nil {
		t.Fatalf("openSearchSession() error = %v", err)
	}
	m.searchQuery = "linux"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty", got.searchQuery)
	}
	if !got.searchActive {
		t.Fatal("searchActive = false after first esc, want true")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.searchActive {
		t.Fatal("searchActive = true after second esc, want false")
	}
	if !fake.closed {
		t.Fatal("search session was not closed")
	}
}

func TestSearchEnterRunsQueryAndShowsResults(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{
		info: runtimesearch.SessionInfo{BestMode: runtimesearch.ModeKeyword},
		response: runtimesearch.SearchResponse{
			EffectiveMode: runtimesearch.ModeKeyword,
			Results: []runtimesearch.Result{
				{Filename: "wiki.zim", Path: "A/Linux", Title: "Linux", Snippet: "kernel"},
			},
		},
	}
	m.searchFactory = func(string) (searchSession, error) { return fake, nil }
	if err := m.openSearchSession(); err != nil {
		t.Fatalf("openSearchSession() error = %v", err)
	}
	m.searchQuery = "linux"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if !got.searchLoading {
		t.Fatal("searchLoading = false, want true")
	}
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)
	if len(got.searchResults) != 1 {
		t.Fatalf("len(searchResults) = %d, want 1", len(got.searchResults))
	}
	if !got.searchResultsFocus {
		t.Fatal("searchResultsFocus = false, want true")
	}
	if len(fake.searches) != 1 || fake.searches[0] != "keyword:linux" {
		t.Fatalf("searches = %v", fake.searches)
	}
}

func TestSearchQTypesIntoQueryInput(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{info: runtimesearch.SessionInfo{BestMode: runtimesearch.ModeKeyword}}
	m.searchFactory = func(string) (searchSession, error) { return fake, nil }
	if err := m.openSearchSession(); err != nil {
		t.Fatalf("openSearchSession() error = %v", err)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	got := updated.(Model)
	if got.searchQuery != "q" {
		t.Fatalf("searchQuery = %q, want q", got.searchQuery)
	}
	if !got.searchActive {
		t.Fatal("searchActive = false, want true")
	}
}

func TestSearchViewShowsTitleFirstAndSourceOnlyInMetadata(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{
		info: runtimesearch.SessionInfo{BestMode: runtimesearch.ModeKeyword},
		response: runtimesearch.SearchResponse{
			EffectiveMode: runtimesearch.ModeKeyword,
			Results: []runtimesearch.Result{
				{Filename: "wiki.zim", Path: "A/Linux", Title: "Linux", Snippet: "kernel and userspace"},
			},
		},
	}
	m.searchFactory = func(string) (searchSession, error) { return fake, nil }
	if err := m.openSearchSession(); err != nil {
		t.Fatalf("openSearchSession() error = %v", err)
	}
	m.searchQuery = "linux"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ = updated.(Model).Update(msg)
	got := updated.(Model)

	view := got.View()
	if !strings.Contains(view, "01") || !strings.Contains(view, "Linux") {
		t.Fatalf("View() missing stable numbered result row: %q", view)
	}
	if strings.Contains(view, "[wiki]") {
		t.Fatalf("View() still shows source name in row prefix: %q", view)
	}
	if !strings.Contains(view, "Source: wiki.zim") {
		t.Fatalf("View() missing selected result metadata: %q", view)
	}
}
