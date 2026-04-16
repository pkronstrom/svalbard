package menu

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
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
		t.Fatalf("View() did not render subheaders and selected footer description: %q", view)
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

func TestEscAtRootQuits(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("cmd = nil, want quit command")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("cmd() = %#v, want tea.QuitMsg", msg)
	}
}

func TestRootViewShowsSelectedDescriptionInFooter(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")

	view := m.View()
	if !strings.Contains(view, "Selected") || !strings.Contains(view, "Search across indexed archives and documents.") {
		t.Fatalf("View() missing selected group footer: %q", view)
	}
}

func TestSubmenuViewShowsSelectedDescriptionOnlyInFooter(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.inGroup = true
	m.activeGroup = "library"

	view := m.View()
	if !strings.Contains(view, "Selected") || !strings.Contains(view, "Browse the image-free English Wikipedia archive.") {
		t.Fatalf("View() missing selected item footer: %q", view)
	}
	if strings.Count(view, "Browse the image-free English Wikipedia archive.") != 1 {
		t.Fatalf("View() still renders per-row descriptions: %q", view)
	}
}

func TestRootViewShowsFooterLegend(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")

	view := m.View()
	if !strings.Contains(view, "j/k or arrows: move • Enter: open • Esc/q: quit") {
		t.Fatalf("View() missing footer legend: %q", view)
	}
}

func TestSubmenuItemsUnderSubheaderAreIndented(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.inGroup = true
	m.activeGroup = "library"

	view := m.View()
	if !strings.Contains(view, "\nArchives\n  > Wikipedia (text only)") {
		t.Fatalf("View() missing expected indented selected item under subheader: %q", view)
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
	info      search.SessionInfo
	response  search.SearchResponse
	searchErr error
	openErr   error
	searches  []string
	opened    []search.Result
	closed    bool
}

func (f *fakeSearchSession) Info() search.SessionInfo {
	return f.info
}

func (f *fakeSearchSession) Search(_ context.Context, mode search.Mode, query string) (search.SearchResponse, error) {
	f.searches = append(f.searches, string(mode)+":"+query)
	return f.response, f.searchErr
}

func (f *fakeSearchSession) OpenResult(result search.Result) error {
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
		info: search.SessionInfo{
			SourceCount:  2,
			ArticleCount: 10,
			BestMode:     search.ModeKeyword,
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
	if got.searchMode != search.ModeKeyword {
		t.Fatalf("searchMode = %q", got.searchMode)
	}
}

func TestSearchEscClearsQueryThenLeavesSession(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{info: search.SessionInfo{BestMode: search.ModeKeyword}}
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
		info: search.SessionInfo{BestMode: search.ModeKeyword},
		response: search.SearchResponse{
			EffectiveMode: search.ModeKeyword,
			Results: []search.Result{
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
	fake := &fakeSearchSession{info: search.SessionInfo{BestMode: search.ModeKeyword}}
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
		info: search.SessionInfo{BestMode: search.ModeKeyword},
		response: search.SearchResponse{
			EffectiveMode: search.ModeKeyword,
			Results: []search.Result{
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
	if strings.Contains(view, "●") {
		t.Fatalf("View() still shows source dot in result rows: %q", view)
	}
	if !strings.Contains(view, "Source: wiki.zim") {
		t.Fatalf("View() missing selected result metadata: %q", view)
	}
	if strings.Contains(view, "Mode:   keyword") {
		t.Fatalf("View() still shows redundant mode metadata: %q", view)
	}
}

func TestSearchViewHighlightsSelectedTitleText(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	fake := &fakeSearchSession{
		info: search.SessionInfo{BestMode: search.ModeKeyword},
		response: search.SearchResponse{
			EffectiveMode: search.ModeKeyword,
			Results: []search.Result{
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
	selectedTitle := selectedRowStyle.Render("Linux")
	if !strings.Contains(view, selectedTitle) {
		t.Fatalf("View() missing selected-row background on title text: %q", view)
	}
}
