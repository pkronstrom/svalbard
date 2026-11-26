package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
)

// newWiki returns the wiki tools on a fresh drive, with no wiki open in a
// browser and a fixed clock, and the drive's root.
func newWiki(t *testing.T) (*WikiCapability, string) {
	t.Helper()
	root := t.TempDir()
	cap := NewWikiCapability(root)
	cap.running = func() string { return "" }
	cap.now = func() time.Time { return time.Date(2026, 11, 26, 18, 55, 3, 0, time.UTC) }
	t.Cleanup(func() { cap.Close() })
	return cap, root
}

// callWiki runs one wiki action and returns its result as the client sees
// it: the JSON data, then any text.
func callWiki(t *testing.T, cap *WikiCapability, action string, params map[string]any) (string, error) {
	t.Helper()
	result, err := cap.Handle(context.Background(), action, params)
	if err != nil {
		return "", err
	}
	if result.Data == nil {
		return result.Text, nil
	}
	data, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + result.Text, nil
}

func mustCallWiki(t *testing.T, cap *WikiCapability, action string, params map[string]any) string {
	t.Helper()
	out, err := callWiki(t, cap, action, params)
	if err != nil {
		t.Fatalf("wiki_%s(%v) error = %v", action, params, err)
	}
	return out
}

func wantWikiError(t *testing.T, cap *WikiCapability, action string, params map[string]any, wantInError ...string) {
	t.Helper()
	out, err := callWiki(t, cap, action, params)
	if err == nil {
		t.Errorf("wiki_%s(%v) = %q, want an error", action, params, out)
		return
	}
	for _, want := range wantInError {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("wiki_%s(%v) error = %q, want it to mention %q", action, params, err, want)
		}
	}
}

// splitResult separates a result's JSON list from the note after it.
func splitResult(out string) (data, note string) {
	end := strings.LastIndex(out, "]") + 1
	return out[:end], out[end:]
}

func writePage(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(wiki.Dir(root), filepath.FromSlash(name)+".md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPageFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(wiki.Dir(root), filepath.FromSlash(name)+".md"))
	if err != nil {
		t.Fatalf("page %q: %v", name, err)
	}
	return string(data)
}

func installWiki(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipes := `{"silverbullet": {"id": "silverbullet", "type": "binary", "description": "SilverBullet wiki", "tags": []}}`
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWikiCapabilityActions(t *testing.T) {
	cap, _ := newWiki(t)
	if cap.Tool() != "wiki" {
		t.Errorf("tool name = %q, want wiki", cap.Tool())
	}
	writes := map[string]bool{}
	for _, action := range cap.Actions() {
		writes[action.Name] = action.Writes
		// Each page parameter carries an example a small model can copy.
		for _, param := range action.Params {
			if param.Name == "page" && param.Name != "content" && action.Name != "move" && !strings.Contains(param.Desc, "for example") {
				t.Errorf("%s.%s description has no example: %q", action.Name, param.Name, param.Desc)
			}
		}
	}
	want := map[string]bool{
		"list": false, "read": false, "search": false,
		"create": true, "append": true, "edit": true, "move": true, "delete": true,
	}
	if fmt.Sprint(writes) != fmt.Sprint(want) {
		t.Errorf("actions = %v, want %v", writes, want)
	}
	if _, err := cap.Handle(context.Background(), "purge", map[string]any{"page": "index"}); err == nil {
		t.Error("unknown action succeeded")
	}
}

func TestWikiToolsAreRegisteredOnlyWhenTheWikiIsInstalled(t *testing.T) {
	vault := []string{"query_describe", "query_sql", "search", "search_read", "vault_databases", "vault_maps", "vault_sources", "vault_stats"}
	wikiTools := []string{"wiki_append", "wiki_create", "wiki_delete", "wiki_edit", "wiki_list", "wiki_move", "wiki_read", "wiki_search"}
	registered := func(root string) []string {
		srv := NewServer(DriveCapabilities(root)...)
		defer srv.Close()
		var names []string
		for name := range srv.inner.ListTools() {
			names = append(names, name)
		}
		slices.Sort(names)
		return names
	}

	without := t.TempDir()
	if got := registered(without); !slices.Equal(got, vault) {
		t.Errorf("tools without the wiki = %v, want the eight vault tools", got)
	}
	if WikiInstalled(without) {
		t.Error("WikiInstalled() is true on a drive without the recipe")
	}
	// Another installed recipe is not the wiki.
	other := t.TempDir()
	installWiki(t, other)
	if err := os.WriteFile(filepath.Join(other, ".svalbard", "recipes.json"), []byte(`{"dufs": {"id": "dufs", "type": "binary"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := registered(other); !slices.Equal(got, vault) {
		t.Errorf("tools with another recipe = %v, want the eight vault tools", got)
	}

	with := t.TempDir()
	installWiki(t, with)
	want := slices.Sorted(slices.Values(slices.Concat(vault, wikiTools)))
	if got := registered(with); !slices.Equal(got, want) {
		t.Errorf("tools with the wiki = %v\nwant %v", got, want)
	}
	// Launchers that enable tools by name read this list.
	listed := DriveToolNames(with)
	slices.Sort(listed)
	if !slices.Equal(listed, want) {
		t.Errorf("DriveToolNames() = %v\nwant %v", listed, want)
	}
	if !WikiInstalled(with) {
		t.Error("WikiInstalled() is false on a drive with the recipe")
	}
	// Listing tools is not using the wiki: no folder appears.
	for _, dir := range []string{wiki.Dir(with), wiki.TrashDir(with)} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("registering the tools created %s", dir)
		}
	}

	srv := NewServer(DriveCapabilities(with)...)
	defer srv.Close()
	tools := srv.inner.ListTools()
	for _, name := range wikiTools {
		writes := !slices.Contains([]string{"wiki_list", "wiki_read", "wiki_search"}, name)
		annotations := tools[name].Tool.Annotations
		if annotations.ReadOnlyHint == nil || *annotations.ReadOnlyHint == writes {
			t.Errorf("%s read-only hint = %v, want %v", name, annotations.ReadOnlyHint, !writes)
		}
		if annotations.DestructiveHint == nil || *annotations.DestructiveHint != writes {
			t.Errorf("%s destructive hint = %v, want %v", name, annotations.DestructiveHint, writes)
		}
	}
}

func TestPageNameAcceptsAndCleans(t *testing.T) {
	for raw, want := range map[string]string{
		"index":                    "index",
		"Projects/Garden plan":     "Projects/Garden plan",
		"  Garden plan  ":          "Garden plan",
		"Garden plan.md":           "Garden plan",
		"Notes.MD":                 "Notes",
		"Lääkkeet/Särkylääkkeet":   "Lääkkeet/Särkylääkkeet",
		"Dr. Smith":                "Dr. Smith",
		"Paracetamol (adults)":     "Paracetamol (adults)",
		"A/B/C/D":                  "A/B/C/D",
		"Burns - first aid & care": "Burns - first aid & care",
	} {
		got, err := pageName("page", raw)
		if err != nil || got != want {
			t.Errorf("pageName(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestPageNameRefusesWithWhatToSendInstead(t *testing.T) {
	for raw, wants := range map[string][]string{
		"":                      {"missing required parameter: page", "Projects/Garden plan"},
		"   ":                   {"missing required parameter: page"},
		"/Garden":               {`starts with "/"`, "send it without: Garden"},
		"/":                     {"is not a page"},
		"/etc/passwd":           {`starts with "/"`},
		"~/notes":               {"path on the computer"},
		"../secret":             {`contains ".."`},
		"Projects/../../secret": {`contains ".."`},
		`Projects\Garden`:       {`uses "\"`, `separate folders with "/"`},
		".hidden":               {"starting with a dot"},
		"Projects/.hidden":      {"starting with a dot"},
		"Projects//Garden":      {"empty part"},
		"Projects/":             {"empty part"},
		"Projects / Garden":     {"space next to", "send it without: Projects/Garden"},
		"Garden.":               {"ending in a dot", "send it without: Garden"},
		"Paracetamol: dosing":   {`contains ":"`, "for example: Paracetamol dosing"},
		"What is a burn?":       {`contains "?"`, "for example: What is a burn"},
		"Garden #2":             {`contains "#"`, "for example: Garden 2"},
		"me@home":               {`contains "@"`},
		"a|b":                   {`contains "|"`},
		"Garden<br>":            {`contains "<"`},
		"Cost $5":               {`contains "$"`},
		"Line\nbreak":           {"which a page name cannot hold"},
		"[[Garden]]":            {`"[[" or "]]"`},
		"^meta":                 {`starting with "^"`},
		// SilverBullet reads these as files, not pages.
		"Version 1.2":            {`ends in ".2"`, "reads as a file type", "for example: Version 1-2"},
		"notes.txt":              {`ends in ".txt"`, "for example: notes-txt"},
		"Projects/page.htm":      {`ends in ".htm"`, "for example: Projects/page-htm"},
		"NUL\x00byte":            {"NUL byte"},
		strings.Repeat("a", 300): {"over the limit of 240"},
	} {
		got, err := pageName("page", raw)
		if err == nil {
			t.Errorf("pageName(%q) = %q, want an error", raw, got)
			continue
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("pageName(%q) error = %q, want it to mention %q", raw, err, want)
			}
		}
	}
	// The error names the parameter it is about.
	if _, err := pageName("new_name", ""); err == nil || !strings.Contains(err.Error(), "new_name") {
		t.Errorf("pageName(new_name) error = %v, want it to name new_name", err)
	}
}

func TestWikiSeedsTheStartPageOnFirstUse(t *testing.T) {
	cap, root := newWiki(t)
	out := mustCallWiki(t, cap, "list", nil)
	if !strings.Contains(out, `"page":"index"`) {
		t.Errorf("list on a new drive = %s, want the seeded start page", out)
	}
	if !strings.Contains(readPageFile(t, root, "index"), "# Wiki") {
		t.Error("start page was not seeded")
	}
	// The trash appears with the first delete, not before.
	if _, err := os.Stat(wiki.TrashDir(root)); err == nil {
		t.Error("the trash folder was created before anything was deleted")
	}
}

func TestWikiCreateWritesAPageAndRefusesAnExistingOne(t *testing.T) {
	cap, root := newWiki(t)

	out := mustCallWiki(t, cap, "create", map[string]any{"page": "Projects/Garden plan", "content": "# Garden\n\nBeans in May."})
	want := `{"page":"Projects/Garden plan","file":"wiki/Projects/Garden plan.md","bytes":24}` +
		"The wiki is not open in a browser now. The user opens it with Wiki in the drive's menu, or reads the file."
	if out != want {
		t.Errorf("create = %s\nwant     %s", out, want)
	}
	// A page ends with a line break, as an editor would leave it.
	if got := readPageFile(t, root, "Projects/Garden plan"); got != "# Garden\n\nBeans in May.\n" {
		t.Errorf("page = %q", got)
	}

	wantWikiError(t, cap, "create", map[string]any{"page": "Projects/Garden plan", "content": "other"},
		`page "Projects/Garden plan" already exists`, "wiki_append", "wiki_edit")
	// The file's own name is the same page.
	wantWikiError(t, cap, "create", map[string]any{"page": "Projects/Garden plan.md", "content": "other"}, "already exists")
	if got := readPageFile(t, root, "Projects/Garden plan"); got != "# Garden\n\nBeans in May.\n" {
		t.Errorf("a refused create changed the page: %q", got)
	}

	wantWikiError(t, cap, "create", map[string]any{"page": "Empty"}, "missing required parameter: content")
	wantWikiError(t, cap, "create", map[string]any{"page": "Empty", "content": "  \n"}, "missing required parameter: content")
	wantWikiError(t, cap, "create", map[string]any{"content": "x"}, "missing required parameter: page")
	// A page SilverBullet could not show is not written.
	wantWikiError(t, cap, "create", map[string]any{"page": "Dose: adults", "content": "x"}, `contains ":"`)
	if entries, _ := os.ReadDir(wiki.Dir(root)); len(entries) != 2 {
		t.Errorf("wiki folder = %v, want only index.md and Projects", entries)
	}

	big := strings.Repeat("x", 2<<20+1)
	wantWikiError(t, cap, "create", map[string]any{"page": "Big", "content": big}, "over the 2097152 byte limit")
}

func TestWikiWritesReturnALinkWhileTheWikiIsOpen(t *testing.T) {
	cap, _ := newWiki(t)
	cap.running = func() string { return "http://localhost:8088" }

	out := mustCallWiki(t, cap, "create", map[string]any{"page": "Projects/Garden [2026]", "content": "x"})
	// The link is the one thing to pass on: no file path beside it.
	want := `{"page":"Projects/Garden [2026]","bytes":2,` +
		`"link":"[Projects/Garden \\[2026\\]](http://localhost:8088/Projects/Garden%20%5B2026%5D)"}` +
		"Put the link in your reply exactly as returned, so the user can open the page."
	if out != want {
		t.Errorf("create = %s\nwant     %s", out, want)
	}
	for action, params := range map[string]map[string]any{
		"append": {"page": "Projects/Garden [2026]", "content": "more"},
		"edit":   {"page": "Projects/Garden [2026]", "old_text": "x", "new_text": "y"},
		"move":   {"page": "Projects/Garden [2026]", "new_name": "Garden"},
	} {
		if out := mustCallWiki(t, cap, action, params); !strings.Contains(out, `"link":"[`) || strings.Contains(out, "not open") || strings.Contains(out, `"file"`) {
			t.Errorf("%s = %s, want a link, no file path and no note about opening the wiki", action, out)
		}
	}
}

func TestWikiAppendAddsAfterABlankLineAndCreatesAMissingPage(t *testing.T) {
	cap, root := newWiki(t)

	out := mustCallWiki(t, cap, "append", map[string]any{"page": "Log", "content": "First entry."})
	if !strings.HasPrefix(out, `{"page":"Log","file":"wiki/Log.md","bytes":13}`) {
		t.Errorf("append to a missing page = %s", out)
	}
	if got := readPageFile(t, root, "Log"); got != "First entry.\n" {
		t.Errorf("created page = %q", got)
	}

	mustCallWiki(t, cap, "append", map[string]any{"page": "Log", "content": "Second entry.\n\n"})
	if got := readPageFile(t, root, "Log"); got != "First entry.\n\nSecond entry.\n" {
		t.Errorf("appended page = %q", got)
	}
	// A page the user left without a final line break, or with several.
	writePage(t, root, "Loose", "no newline")
	mustCallWiki(t, cap, "append", map[string]any{"page": "Loose", "content": "- item"})
	if got := readPageFile(t, root, "Loose"); got != "no newline\n\n- item\n" {
		t.Errorf("appended page = %q", got)
	}
	writePage(t, root, "Blank", "\n\n")
	mustCallWiki(t, cap, "append", map[string]any{"page": "Blank", "content": "text"})
	if got := readPageFile(t, root, "Blank"); got != "text\n" {
		t.Errorf("appended to a blank page = %q", got)
	}

	wantWikiError(t, cap, "append", map[string]any{"page": "Log"}, "missing required parameter: content")
	// Text is not appended to something that is not text.
	if err := os.WriteFile(filepath.Join(wiki.Dir(root), "Binary.md"), []byte{0xff, 0xfe, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	wantWikiError(t, cap, "append", map[string]any{"page": "Binary", "content": "x"}, `page "Binary" is not text`)
}

func TestWikiEditReplacesOneExactOccurrence(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Garden", "# Garden\n\n- beans\n  - runner beans\n- peas\n- beans\n")

	out := mustCallWiki(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "- peas\n", "new_text": "- sugar peas\n"})
	if !strings.HasPrefix(out, `{"page":"Garden","file":"wiki/Garden.md","bytes":56}`) {
		t.Errorf("edit = %s", out)
	}
	if got := readPageFile(t, root, "Garden"); got != "# Garden\n\n- beans\n  - runner beans\n- sugar peas\n- beans\n" {
		t.Errorf("edited page = %q", got)
	}
	before := readPageFile(t, root, "Garden")

	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "- beans\n", "new_text": "- broad beans\n"},
		"old_text occurs 2 times", `page "Garden"`, "surrounding text")
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "- carrots", "new_text": "x"},
		`old_text was not found in page "Garden"`, "wiki_read")
	// Wrong indentation is the usual miss; the real line is quoted.
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "    - runner beans", "new_text": "x"},
		"Line 4 is close", `"  - runner beans"`)
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "new_text": "x"}, "missing required parameter: old_text", "wiki_append")
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "- peas"}, "missing required parameter: new_text")
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "# Garden", "new_text": "# Garden"}, "nothing would change")
	if got := readPageFile(t, root, "Garden"); got != before {
		t.Errorf("a refused edit changed the page: %q", got)
	}

	// An empty new_text deletes.
	mustCallWiki(t, cap, "edit", map[string]any{"page": "Garden", "old_text": "  - runner beans\n", "new_text": ""})
	if got := readPageFile(t, root, "Garden"); strings.Contains(got, "runner") {
		t.Errorf("deletion left the text: %q", got)
	}
	// Editing does not create.
	wantWikiError(t, cap, "edit", map[string]any{"page": "Gardn", "old_text": "a", "new_text": "b"},
		`page "Gardn" does not exist`, "Nearest pages: Garden")
}

func TestWikiMoveRenamesAndNeverReplaces(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Garden", "beans\n")
	writePage(t, root, "Kitchen", "See [[Garden]] and [[Garden|the plan]].\n")
	writePage(t, root, "Tools", "Spade, for the [[Garden#Beds]].\n")
	writePage(t, root, "Other", "A garden is not a link, nor is [[Gardening]].\n")

	out := mustCallWiki(t, cap, "move", map[string]any{"page": "Garden", "new_name": "Archive/2026/Garden"})
	if !strings.HasPrefix(out, `{"page":"Archive/2026/Garden","file":"wiki/Archive/2026/Garden.md","from":"Garden"}`) {
		t.Errorf("move = %s", out)
	}
	// Links are not rewritten; the pages that hold them are named.
	if !strings.Contains(out, "These pages still link to [[Garden]]: Kitchen, Tools.") || strings.Contains(out, "Other") {
		t.Errorf("move = %s, want Kitchen and Tools named as linking pages", out)
	}
	if got := readPageFile(t, root, "Archive/2026/Garden"); got != "beans\n" {
		t.Errorf("moved page = %q", got)
	}
	if _, err := os.Stat(filepath.Join(wiki.Dir(root), "Garden.md")); err == nil {
		t.Error("the old page is still there")
	}
	if got := readPageFile(t, root, "Kitchen"); got != "See [[Garden]] and [[Garden|the plan]].\n" {
		t.Errorf("a linking page was changed: %q", got)
	}

	wantWikiError(t, cap, "move", map[string]any{"page": "Kitchen", "new_name": "Tools"},
		`page "Tools" already exists`, "does not replace", "wiki_delete")
	if got := readPageFile(t, root, "Tools"); got != "Spade, for the [[Garden#Beds]].\n" {
		t.Errorf("a refused move replaced the page: %q", got)
	}
	if got := readPageFile(t, root, "Kitchen"); got == "" {
		t.Error("a refused move took the page away")
	}
	wantWikiError(t, cap, "move", map[string]any{"page": "Kitchn", "new_name": "Cooking"},
		`page "Kitchn" does not exist`, "Nearest pages: Kitchen")
	wantWikiError(t, cap, "move", map[string]any{"page": "Kitchen", "new_name": "Kitchen"}, "nothing would move")
	wantWikiError(t, cap, "move", map[string]any{"page": "Kitchen"}, "missing required parameter: new_name")
	wantWikiError(t, cap, "move", map[string]any{"page": "Kitchen", "new_name": "Cooking: basics"}, `contains ":"`)

	// A move with no page linking to the old name says nothing about links.
	if out := mustCallWiki(t, cap, "move", map[string]any{"page": "Other", "new_name": "Misc"}); strings.Contains(out, "still link") {
		t.Errorf("move = %s, want no note about links", out)
	}
}

func TestWikiDeleteMovesThePageToTheTrash(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Projects/Garden", "beans\n")

	out := mustCallWiki(t, cap, "delete", map[string]any{"page": "Projects/Garden"})
	want := `{"page":"Projects/Garden","trash":".svalbard/state/wiki-trash/Projects/Garden.20261126-185503.md"}` +
		`Page "Projects/Garden" was moved to the trash, not erased. To restore it the user moves ` +
		`.svalbard/state/wiki-trash/Projects/Garden.20261126-185503.md on the drive back into the wiki folder as Projects/Garden.md.`
	if out != want {
		t.Errorf("delete = %s\nwant     %s", out, want)
	}
	trashed := filepath.Join(wiki.TrashDir(root), "Projects", "Garden.20261126-185503.md")
	if data, err := os.ReadFile(trashed); err != nil || string(data) != "beans\n" {
		t.Errorf("trashed page = %q (%v), want the page's text", data, err)
	}
	if _, err := os.Stat(filepath.Join(wiki.Dir(root), "Projects", "Garden.md")); err == nil {
		t.Error("the deleted page is still in the wiki")
	}
	// The trash is outside the page folder, so the wiki does not show it.
	if out := mustCallWiki(t, cap, "list", map[string]any{"depth": 5}); strings.Contains(out, "Garden") {
		t.Errorf("list after delete = %s", out)
	}

	// The same page deleted again in the same second is kept apart.
	for i, wantName := range []string{"Garden.20261126-185503-2.md", "Garden.20261126-185503-3.md"} {
		writePage(t, root, "Projects/Garden", fmt.Sprintf("version %d\n", i+2))
		out := mustCallWiki(t, cap, "delete", map[string]any{"page": "Projects/Garden"})
		if !strings.Contains(out, `"trash":".svalbard/state/wiki-trash/Projects/`+wantName+`"`) {
			t.Errorf("delete %d = %s, want it trashed as %s", i+2, out, wantName)
		}
	}
	if data, _ := os.ReadFile(trashed); string(data) != "beans\n" {
		t.Errorf("a later delete replaced the first trashed page: %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Join(wiki.TrashDir(root), "Projects")); len(entries) != 3 {
		t.Errorf("trash holds %d files, want 3", len(entries))
	}

	wantWikiError(t, cap, "delete", map[string]any{"page": "Projects/Garden"}, `page "Projects/Garden" does not exist`)
	wantWikiError(t, cap, "delete", map[string]any{}, "missing required parameter: page")
	// A folder is not a page.
	writePage(t, root, "Projects/Shed", "x")
	wantWikiError(t, cap, "delete", map[string]any{"page": "Projects"}, `page "Projects" does not exist`)
	if _, err := os.Stat(filepath.Join(wiki.Dir(root), "Projects", "Shed.md")); err != nil {
		t.Errorf("deleting a folder's name took a page with it: %v", err)
	}
}

func TestWikiSearchNeedsEveryWordAndRanksByMatches(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Burns", "# Burns\n\nCool the burn under running water for 20 minutes.\nDo not use ice.\n")
	writePage(t, root, "First aid/Kit", "Plasters, burn gel, tape.\nWater purification tablets.\n")
	writePage(t, root, "Water", "Store water in clean containers. Water, water, water.\n")
	writePage(t, root, "Lääkkeet", "Särkylääke: parasetamoli 500 mg.\n")

	// Every word must be in the page's name or text.
	out := mustCallWiki(t, cap, "search", map[string]any{"query": "burn water"})
	var hits []wikiHit
	if err := json.Unmarshal([]byte(out), &hits); err != nil {
		t.Fatalf("search = %s (%v)", out, err)
	}
	if len(hits) != 2 || hits[0].Page != "Burns" || hits[1].Page != "First aid/Kit" {
		t.Fatalf("search = %s, want Burns then First aid/Kit", out)
	}
	// A word in the name outweighs a page that only mentions it.
	if hits[0].Matches != 4 || hits[1].Matches != 2 {
		t.Errorf("matches = %d and %d, want 4 and 2", hits[0].Matches, hits[1].Matches)
	}
	if !slices.Equal(hits[0].Excerpts, []string{"# Burns", "Cool the burn under running water for 20 minutes."}) {
		t.Errorf("excerpts = %q", hits[0].Excerpts)
	}

	// Case and punctuation in the query do not matter, and a word may be in
	// the name alone.
	out = mustCallWiki(t, cap, "search", map[string]any{"query": "KIT, plasters!"})
	if !strings.Contains(out, `"page":"First aid/Kit"`) || strings.Contains(out, `"page":"Burns"`) {
		t.Errorf("search = %s, want only First aid/Kit", out)
	}
	out = mustCallWiki(t, cap, "search", map[string]any{"query": "SÄRKYLÄÄKE"})
	if !strings.Contains(out, `"page":"Lääkkeet"`) || !strings.Contains(out, "Särkylääke: parasetamoli 500 mg.") {
		t.Errorf("search = %s, want the Finnish page with its line", out)
	}

	wantWikiError(t, cap, "search", map[string]any{}, "missing required parameter: query")
	wantWikiError(t, cap, "search", map[string]any{"query": " ?! "}, "missing required parameter: query")
}

func TestWikiSearchThatFindsNothingSaysWhatTheWikiHolds(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Burns", "Cool the burn under running water.\n")
	writePage(t, root, "Water", "Store water in clean containers.\n")

	// Words that are nowhere are named, so the model stops rewording.
	out := mustCallWiki(t, cap, "search", map[string]any{"query": "toyota tyre water"})
	want := "No wiki page mentions: toyota, tyre. The wiki holds 2 pages: Burns, Water. " +
		"A subject outside these is not in the wiki; the drive's reference archives are searched with search."
	if out != want {
		t.Errorf("search = %s\nwant     %s", out, want)
	}
	// Each word is somewhere, but no page has both.
	out = mustCallWiki(t, cap, "search", map[string]any{"query": "burn containers"})
	if !strings.HasPrefix(out, "No wiki page has all of these words: burn, containers. The wiki holds 2 pages:") {
		t.Errorf("search = %s", out)
	}

	// A long wiki is not listed whole.
	for i := range 20 {
		writePage(t, root, fmt.Sprintf("Notes/Day %02d", i), "nothing\n")
	}
	out = mustCallWiki(t, cap, "search", map[string]any{"query": "toyota"})
	if !strings.Contains(out, "The wiki holds 22 pages: Burns, Notes/Day 00,") || !strings.Contains(out, ", and 10 more, listed by wiki_list.") {
		t.Errorf("search = %s, want twelve names and the count of the rest", out)
	}
}

func TestWikiSearchOnAnEmptyWikiSaysSo(t *testing.T) {
	cap, root := newWiki(t)
	mustCallWiki(t, cap, "list", nil)
	if err := os.Remove(filepath.Join(wiki.Dir(root), "index.md")); err != nil {
		t.Fatal(err)
	}
	out := mustCallWiki(t, cap, "search", map[string]any{"query": "anything"})
	if want := "The wiki has no pages yet, so there is nothing to find. Create a page with wiki_create."; out != want {
		t.Errorf("search = %s\nwant     %s", out, want)
	}
	if out := mustCallWiki(t, cap, "list", nil); out != "The wiki has no pages yet. Create one with wiki_create." {
		t.Errorf("list = %s", out)
	}
}

func TestWikiSearchCapsResultsAndExcerpts(t *testing.T) {
	cap, root := newWiki(t)
	for i := range 25 {
		writePage(t, root, fmt.Sprintf("Log/Entry %02d", i), strings.Repeat("rain ", i+1)+"\n")
	}
	long := strings.Repeat("word ", 60) + "needle in a long line " + strings.Repeat("tail ", 60)
	writePage(t, root, "Long", "needle one\nneedle two\nneedle three\nneedle four\n"+long+"\n")

	out := mustCallWiki(t, cap, "search", map[string]any{"query": "rain"})
	data, note := splitResult(out)
	var hits []wikiHit
	if err := json.Unmarshal([]byte(data), &hits); err != nil {
		t.Fatalf("search = %s (%v)", out, err)
	}
	if len(hits) != 8 || note != "These are the best 8 of 25 matching pages. Add a word to narrow the search." {
		t.Errorf("search returned %d pages and the note %q, want 8 and the count of 25", len(hits), note)
	}
	// One page full of a word does not outrank the rest without bound, and
	// equal pages keep their name order.
	if hits[0].Page != "Log/Entry 09" || hits[1].Page != "Log/Entry 10" {
		t.Errorf("first hits = %s, %s; want Log/Entry 09 then 10", hits[0].Page, hits[1].Page)
	}
	if out := mustCallWiki(t, cap, "search", map[string]any{"query": "rain", "limit": 99}); strings.Count(out, `"page"`) != 20 {
		t.Errorf("limit 99 returned %d pages, want the cap of 20", strings.Count(out, `"page"`))
	}
	if out := mustCallWiki(t, cap, "search", map[string]any{"query": "rain", "limit": "2"}); strings.Count(out, `"page"`) != 2 {
		t.Errorf("limit \"2\" returned %d pages, want 2", strings.Count(out, `"page"`))
	}

	out = mustCallWiki(t, cap, "search", map[string]any{"query": "needle"})
	if err := json.Unmarshal([]byte(out), &hits); err != nil || len(hits) != 1 {
		t.Fatalf("search = %s (%v)", out, err)
	}
	if !slices.Equal(hits[0].Excerpts, []string{"needle one", "needle two", "needle three"}) {
		t.Errorf("excerpts = %q, want the first three matching lines", hits[0].Excerpts)
	}
	// A long line is cut around the word.
	writePage(t, root, "Long", long+"\n")
	out = mustCallWiki(t, cap, "search", map[string]any{"query": "needle"})
	if err := json.Unmarshal([]byte(out), &hits); err != nil || len(hits) != 1 || len(hits[0].Excerpts) != 1 {
		t.Fatalf("search = %s (%v)", out, err)
	}
	excerpt := hits[0].Excerpts[0]
	if len([]rune(excerpt)) != wikiExcerptRunes+2 || !strings.HasPrefix(excerpt, "…") || !strings.HasSuffix(excerpt, "…") || !strings.Contains(excerpt, "needle in a long line") {
		t.Errorf("excerpt = %q, want %d characters around the word", excerpt, wikiExcerptRunes)
	}
}

func TestWikiReadReturnsAPageInParts(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Garden", "# Garden\n\nBeans in May.\n")

	out := mustCallWiki(t, cap, "read", map[string]any{"page": "Garden"})
	if want := `{"page":"Garden","body":"# Garden\n\nBeans in May."}`; out != want {
		t.Errorf("read = %s\nwant   %s", out, want)
	}

	var lines []string
	for i := range 400 {
		lines = append(lines, fmt.Sprintf("Line %03d of the long page, with some words to fill it.", i))
	}
	long := strings.Join(lines, "\n")
	writePage(t, root, "Long", long+"\n")

	// Following the continues line rebuilds the page.
	var rebuilt []string
	offset, calls := 0, 0
	for {
		calls++
		var part wikiPage
		out := mustCallWiki(t, cap, "read", map[string]any{"page": "Long", "offset": offset})
		if err := json.Unmarshal([]byte(out), &part); err != nil {
			t.Fatalf("read = %s (%v)", out, err)
		}
		rebuilt = append(rebuilt, part.Body)
		if part.Continues == "" {
			break
		}
		var from, to, total int
		if _, err := fmt.Sscanf(part.Continues, "Characters %d to %d of %d. Call wiki_read with the same page and offset %d for the next part.", &from, &to, &total, &offset); err != nil {
			t.Fatalf("continues = %q (%v)", part.Continues, err)
		}
		if from >= to || to != offset || calls > 20 {
			t.Fatalf("continues = %q after %d calls", part.Continues, calls)
		}
	}
	if calls < 3 || strings.Join(rebuilt, "\n") != long {
		t.Errorf("the page read in %d parts does not match the page", calls)
	}
	wantWikiError(t, cap, "read", map[string]any{"page": "Long", "offset": 999999}, "offset 999999 is past the end", `page "Long"`)

	writePage(t, root, "Empty", "\n")
	if out := mustCallWiki(t, cap, "read", map[string]any{"page": "Empty"}); out != `[page "Empty" is empty]` {
		t.Errorf("read of an empty page = %s", out)
	}
	wantWikiError(t, cap, "read", map[string]any{}, "missing required parameter: page")
}

func TestWikiReadCorrectsANearMissAndNamesNearestPages(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Projects/Garden plan", "beans\n")
	writePage(t, root, "Projects/Garden tools", "spade\n")
	writePage(t, root, "Water storage", "barrels\n")

	// Other case is the same page on a stick, and is named as the wiki
	// names it. Other separators, or the folder left out: the page that was
	// meant is read, and the result says which it is.
	for _, asked := range []string{"projects/garden plan", "Projects/Garden_plan", "Projects/garden-plan", "Garden plan", "garden_plan.md"} {
		out := mustCallWiki(t, cap, "read", map[string]any{"page": asked})
		if !strings.Contains(out, `"page":"Projects/Garden plan"`) || !strings.Contains(out, `"body":"beans"`) {
			t.Errorf("read(%q) = %s, want the page Projects/Garden plan", asked, out)
		}
	}
	out := mustCallWiki(t, cap, "read", map[string]any{"page": "Garden plan"})
	if !strings.Contains(out, `There is no page \"Garden plan\"; this is \"Projects/Garden plan\".`) {
		t.Errorf("read = %s, want a note naming the page that was read", out)
	}

	// Nothing unambiguous: the nearest names, most alike first.
	wantWikiError(t, cap, "read", map[string]any{"page": "Garden"},
		`page "Garden" does not exist. Nearest pages: Projects/Garden plan, Projects/Garden tools`)
	wantWikiError(t, cap, "read", map[string]any{"page": "Watr storage"}, "Nearest pages: Water storage")
	wantWikiError(t, cap, "read", map[string]any{"page": "Zebra"},
		`page "Zebra" does not exist. Call wiki_list to see the pages, or create it with wiki_create.`)

	// The writing tools do not guess which page was meant. Letter case is
	// no guess: on a stick the two names are one file.
	wantWikiError(t, cap, "edit", map[string]any{"page": "Garden plan", "old_text": "beans", "new_text": "peas"},
		`page "Garden plan" does not exist`, "Nearest pages: Projects/Garden plan")
	wantWikiError(t, cap, "delete", map[string]any{"page": "projects/garden_plan"}, "does not exist")
	if got := readPageFile(t, root, "Projects/Garden plan"); got != "beans\n" {
		t.Errorf("a near miss changed the page: %q", got)
	}
	wantWikiError(t, cap, "create", map[string]any{"page": "PROJECTS/garden PLAN", "content": "x"}, `page "Projects/Garden plan" already exists`)
	out = mustCallWiki(t, cap, "append", map[string]any{"page": "projects/GARDEN plan", "content": "peas"})
	if !strings.HasPrefix(out, `{"page":"Projects/Garden plan","file":"wiki/Projects/Garden plan.md"`) {
		t.Errorf("append = %s, want the page under the name it has", out)
	}
	// A new page in a folder that exists goes into that folder as spelled.
	out = mustCallWiki(t, cap, "create", map[string]any{"page": "projects/Compost", "content": "x"})
	if !strings.HasPrefix(out, `{"page":"Projects/Compost"`) {
		t.Errorf("create = %s, want the page in the existing Projects folder", out)
	}
	if entries, _ := os.ReadDir(wiki.Dir(root)); len(entries) != 2 {
		t.Errorf("wiki folder = %v, want only Projects and Water storage.md", entries)
	}
}

func TestWikiListShowsPagesToADepth(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Garden", "beans\n")
	writePage(t, root, "Projects/Shed", "x")
	writePage(t, root, "Projects/2026/Roof", "x")
	writePage(t, root, "Projects/2026/Q1/Plan", "x")
	// Not pages: other files, hidden files, and the writer's temporary files.
	for _, name := range []string{"photo.jpg", ".DS_Store", ".Garden.md.tmp-1a2b", "Projects/._Shed.md"} {
		if err := os.WriteFile(filepath.Join(wiki.Dir(root), filepath.FromSlash(name)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	names := func(out string) []string {
		t.Helper()
		data, _ := splitResult(out)
		var entries []wikiEntry
		if err := json.Unmarshal([]byte(data), &entries); err != nil {
			t.Fatalf("list = %s (%v)", out, err)
		}
		var listed []string
		for _, entry := range entries {
			if entry.Folder != "" {
				listed = append(listed, entry.Folder+"/")
				continue
			}
			if entry.Modified == "" || entry.Size == 0 {
				t.Errorf("page entry without size or time: %+v", entry)
			}
			listed = append(listed, entry.Page)
		}
		return listed
	}

	// Two levels by default; a folder at the limit is named, not opened.
	out := mustCallWiki(t, cap, "list", nil)
	if got, want := names(out), []string{"Garden", "Projects/2026/", "Projects/Shed"}; !slices.Equal(got, want) {
		t.Errorf("list = %v, want %v", got, want)
	}
	if !strings.HasSuffix(out, "Call wiki_list with folder=Projects/2026 to see inside.") {
		t.Errorf("list = %s, want the next call for the folder", out)
	}
	if got, want := names(mustCallWiki(t, cap, "list", map[string]any{"depth": 1})), []string{"Garden", "Projects/"}; !slices.Equal(got, want) {
		t.Errorf("list depth 1 = %v, want %v", got, want)
	}
	all := []string{"Garden", "Projects/2026/Q1/Plan", "Projects/2026/Roof", "Projects/Shed"}
	if got := names(mustCallWiki(t, cap, "list", map[string]any{"depth": 99})); !slices.Equal(got, all) {
		t.Errorf("list depth 99 = %v, want %v", got, all)
	}
	for _, folder := range []string{"Projects/2026", "/Projects/2026/"} {
		if got, want := names(mustCallWiki(t, cap, "list", map[string]any{"folder": folder})), []string{"Projects/2026/Q1/Plan", "Projects/2026/Roof"}; !slices.Equal(got, want) {
			t.Errorf("list folder %q = %v, want %v", folder, got, want)
		}
	}

	wantWikiError(t, cap, "list", map[string]any{"folder": "Nowhere"}, `the wiki has no folder "Nowhere"`, "wiki_list without folder")
	// A page is not a folder.
	wantWikiError(t, cap, "list", map[string]any{"folder": "Garden"}, `the wiki has no folder "Garden"`)
	if err := os.Mkdir(filepath.Join(wiki.Dir(root), "Bare"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := mustCallWiki(t, cap, "list", map[string]any{"folder": "Bare"}); out != "The folder Bare has no pages." {
		t.Errorf("list of an empty folder = %s", out)
	}
}

func TestWikiListCapsItsEntries(t *testing.T) {
	cap, root := newWiki(t)
	for i := range 250 {
		writePage(t, root, fmt.Sprintf("Log/Entry %03d", i), "x")
	}
	out := mustCallWiki(t, cap, "list", nil)
	if strings.Count(out, `"page"`) != 200 || !strings.HasSuffix(out, "Only the first 200 entries are shown. List one folder to see the rest, or find a page with wiki_search.") {
		t.Errorf("list returned %d pages and ends %q", strings.Count(out, `"page"`), out[max(0, len(out)-120):])
	}
}

func TestWikiToolsCannotLeaveTheWiki(t *testing.T) {
	cap, root := newWiki(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "manifest.yaml")
	if err := os.WriteFile(manifest, []byte("drive: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePage(t, root, "Garden", "beans\n")
	// Links out of the page folder: to a folder, to a file, and one whose
	// target does not exist yet.
	for link, target := range map[string]string{"out": outside, "leak.md": secret, "dangling.md": filepath.Join(outside, "new.md")} {
		if err := os.Symlink(target, filepath.Join(wiki.Dir(root), link)); err != nil {
			t.Fatal(err)
		}
	}

	escapes := []string{
		"../manifest", "../manifest.yaml", "Garden/../../manifest", "/etc/passwd", "~/secret",
		filepath.Join(outside, "secret"), `..\manifest`, "out/secret", "leak", "dangling", ".svalbard/recipes",
		"../.svalbard/state/wiki-trash/x",
	}
	for _, name := range escapes {
		for action, params := range map[string]map[string]any{
			"read":   {"page": name},
			"create": {"page": name, "content": "owned"},
			"append": {"page": name, "content": "owned"},
			"edit":   {"page": name, "old_text": "secret", "new_text": "owned"},
			"delete": {"page": name},
			"move":   {"page": name, "new_name": "Stolen"},
		} {
			if out, err := callWiki(t, cap, action, params); err == nil {
				t.Errorf("wiki_%s(%q) = %q, want a refusal", action, name, out)
			}
		}
		// A move out is as bad as a move in.
		if out, err := callWiki(t, cap, "move", map[string]any{"page": "Garden", "new_name": name}); err == nil {
			t.Errorf("wiki_move to %q = %q, want a refusal", name, out)
		}
		if out, err := callWiki(t, cap, "list", map[string]any{"folder": name}); err == nil && strings.Contains(out, "secret") {
			t.Errorf("wiki_list(%q) = %q, which shows a file outside the wiki", name, out)
		}
	}

	if data, _ := os.ReadFile(secret); string(data) != "secret" {
		t.Errorf("the file outside the drive changed: %q", data)
	}
	if data, _ := os.ReadFile(manifest); string(data) != "drive: true\n" {
		t.Errorf("the drive's manifest changed: %q", data)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 1 {
		t.Errorf("the folder outside the drive gained files: %v", entries)
	}
	if got := readPageFile(t, root, "Garden"); got != "beans\n" {
		t.Errorf("the page changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(wiki.Dir(root), "Stolen.md")); err == nil {
		t.Error("a file from outside was moved into the wiki")
	}
	// Linked files are neither listed nor searched.
	if out := mustCallWiki(t, cap, "list", map[string]any{"depth": 5}); strings.Contains(out, "leak") || strings.Contains(out, "secret") {
		t.Errorf("list = %s, which shows a link out of the wiki", out)
	}
	if out := mustCallWiki(t, cap, "search", map[string]any{"query": "secret"}); strings.Contains(out, `"page"`) {
		t.Errorf("search = %s, which found text outside the wiki", out)
	}
}

func TestWikiDeleteCannotBeSentOutsideTheTrash(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Projects/Garden", "beans\n")
	outside := t.TempDir()
	// The trash folder for this page's folder is a link out of the drive.
	if err := os.MkdirAll(wiki.TrashDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(wiki.TrashDir(root), "Projects")); err != nil {
		t.Fatal(err)
	}

	wantWikiError(t, cap, "delete", map[string]any{"page": "Projects/Garden"}, "leaves the wiki trash through a link")
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("a deleted page left the drive: %v", entries)
	}
	if got := readPageFile(t, root, "Projects/Garden"); got != "beans\n" {
		t.Errorf("the page is gone or changed after a refused delete: %q", got)
	}
}

func TestWikiReportsADriveThatCannotHoldTheWiki(t *testing.T) {
	root := t.TempDir()
	// A file where the page folder would be.
	if err := os.WriteFile(wiki.Dir(root), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cap := NewWikiCapability(root)
	defer cap.Close()
	_, err := cap.Handle(context.Background(), "list", nil)
	if err == nil || !strings.Contains(err.Error(), "the wiki folder cannot be opened") || strings.Contains(err.Error(), root) {
		t.Errorf("list error = %v, want a sentence without the host path", err)
	}
}

// SilverBullet runs space-lua blocks on pages under Library/ for the whole
// wiki, with the browser's reach. Text a model read in the vault must not
// get there.
func TestWikiToolsDoNotWriteTheWikisOwnScripts(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Library/Config", "```space-lua\nconfig.set(\"x\", 1)\n```\n")
	writePage(t, root, "Garden", "beans\n")
	script := "```space-lua\njs.window.fetch(\"/.fs/index.md\", {method = \"DELETE\"})\n```"

	for name, call := range map[string]struct {
		action string
		params map[string]any
	}{
		"create":            {"create", map[string]any{"page": "Library/Helper", "content": script}},
		"create, lower":     {"create", map[string]any{"page": "library/Helper", "content": script}},
		"create, nested":    {"create", map[string]any{"page": "Library/Std/Override", "content": "x"}},
		"append":            {"append", map[string]any{"page": "Library/Config", "content": script}},
		"append, new":       {"append", map[string]any{"page": "LIBRARY/New", "content": script}},
		"edit":              {"edit", map[string]any{"page": "Library/Config", "old_text": "1", "new_text": "2"}},
		"delete":            {"delete", map[string]any{"page": "Library/Config"}},
		"move in":           {"move", map[string]any{"page": "Garden", "new_name": "Library/Garden"}},
		"move out":          {"move", map[string]any{"page": "Library/Config", "new_name": "Config"}},
		"move in, by case":  {"move", map[string]any{"page": "Garden", "new_name": "library/Garden"}},
		"file name of page": {"create", map[string]any{"page": "Library/Helper.md", "content": script}},
	} {
		out, err := callWiki(t, cap, call.action, call.params)
		if err == nil || !strings.Contains(err.Error(), "under Library/") || !strings.Contains(err.Error(), "do not change those pages") {
			t.Errorf("%s: wiki_%s = %q, %v; want the Library refusal", name, call.action, out, err)
		}
	}
	if got := readPageFile(t, root, "Library/Config"); got != "```space-lua\nconfig.set(\"x\", 1)\n```\n" {
		t.Errorf("the library page changed: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(wiki.Dir(root), "Library")); len(entries) != 1 {
		t.Errorf("Library holds %v, want only Config.md", entries)
	}
	if got := readPageFile(t, root, "Garden"); got != "beans\n" {
		t.Errorf("the page changed: %q", got)
	}

	// Reading them is harmless, and a page that only has the word is a page.
	if out := mustCallWiki(t, cap, "read", map[string]any{"page": "Library/Config"}); !strings.Contains(out, "config.set") {
		t.Errorf("read = %s", out)
	}
	mustCallWiki(t, cap, "create", map[string]any{"page": "Library", "content": "Books I own."})
	mustCallWiki(t, cap, "create", map[string]any{"page": "Books/Library", "content": "x"})
	mustCallWiki(t, cap, "create", map[string]any{"page": "Librarything/Notes", "content": "x"})
}

// SilverBullet evaluates ${...} as Lua when a page is shown, and Lua there
// reaches the browser's JavaScript.
func TestWikiToolsAddNoLuaExpressions(t *testing.T) {
	cap, root := newWiki(t)
	attack := `Dose: ${js.window.fetch("/.fs/index.md", {method = "DELETE"})}`

	wantWikiError(t, cap, "create", map[string]any{"page": "Dosing", "content": attack}, `"${" outside a code block`, "inside backticks")
	if _, err := os.Stat(filepath.Join(wiki.Dir(root), "Dosing.md")); err == nil {
		t.Error("the refused page was written")
	}
	writePage(t, root, "Dosing", "Paracetamol.\n\n$\n")
	wantWikiError(t, cap, "append", map[string]any{"page": "Dosing", "content": attack}, `"${" outside a code block`)
	// An edit that completes one from pieces already on the page.
	wantWikiError(t, cap, "edit", map[string]any{"page": "Dosing", "old_text": "$\n", "new_text": "${os.exit()}\n"}, `"${" outside a code block`)
	writePage(t, root, "Split", "Total: $\n{1 + 1}\n")
	wantWikiError(t, cap, "edit", map[string]any{"page": "Split", "old_text": "$\n{", "new_text": "${"}, `"${" outside a code block`)
	if got := readPageFile(t, root, "Split"); got != "Total: $\n{1 + 1}\n" {
		t.Errorf("a refused edit changed the page: %q", got)
	}

	// Code that is shown, not run: a fenced block and a code span.
	fenced := "Set the variable:\n\n```sh\necho ${HOME}\n```\n\nor `${HOME}` inline, or ``a ` ${b}``.\n\n~~~\n${also.shown}\n~~~\n"
	mustCallWiki(t, cap, "create", map[string]any{"page": "Shell", "content": fenced})
	mustCallWiki(t, cap, "append", map[string]any{"page": "Shell", "content": "Costs $5 {roughly} and $ {this}."})
	// A span that never closes, or a fence that closed, hides nothing.
	for _, content := range []string{"`open ${x}", "```\ncode\n```\n${x}", "    ${indented}", "text ``two` ${x}"} {
		wantWikiError(t, cap, "create", map[string]any{"page": "Tricks", "content": content}, `"${" outside a code block`)
	}

	// What the user wrote stays, and a tool may edit around it.
	writePage(t, root, "Queries", "Pages: ${query[[from index.tag \"page\"]]}\n\nNotes: none\n")
	mustCallWiki(t, cap, "edit", map[string]any{"page": "Queries", "old_text": "Notes: none", "new_text": "Notes: checked"})
	mustCallWiki(t, cap, "append", map[string]any{"page": "Queries", "content": "More notes."})
	if got := readPageFile(t, root, "Queries"); !strings.Contains(got, `${query[[from index.tag "page"]]}`) || !strings.Contains(got, "Notes: checked") {
		t.Errorf("page = %q, want the user's expression kept and the edit made", got)
	}
	wantWikiError(t, cap, "append", map[string]any{"page": "Queries", "content": "${1}"}, `"${" outside a code block`)
}

func TestWikiAppendsSentTogetherAllArrive(t *testing.T) {
	cap, root := newWiki(t)
	const writers = 20
	errs := make(chan error, writers)
	for i := range writers {
		go func() {
			_, err := cap.Handle(context.Background(), "append", map[string]any{"page": "Log", "content": fmt.Sprintf("entry %02d", i)})
			errs <- err
		}()
	}
	for range writers {
		if err := <-errs; err != nil {
			t.Errorf("append error = %v", err)
		}
	}
	page := readPageFile(t, root, "Log")
	for i := range writers {
		if !strings.Contains(page, fmt.Sprintf("entry %02d\n", i)) {
			t.Errorf("entry %02d was lost:\n%s", i, page)
		}
	}

	// Of several creates of one page, one wins and the rest are told.
	created := make(chan error, writers)
	for i := range writers {
		go func() {
			_, err := cap.Handle(context.Background(), "create", map[string]any{"page": "Once", "content": fmt.Sprintf("version %02d", i)})
			created <- err
		}()
	}
	won := 0
	for range writers {
		if err := <-created; err == nil {
			won++
		} else if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("create error = %v", err)
		}
	}
	if won != 1 {
		t.Errorf("%d creates of one page succeeded, want 1", won)
	}
}

// A page the user named in a way the tools would not: found by search, so it
// must be readable and changeable under that name.
func TestWikiToolsWorkOnPagesTheUserNamedFreely(t *testing.T) {
	cap, root := newWiki(t)
	for _, name := range []string{"Recipe: bread", "Version 1.2", "notes.txt", "Q&A #1"} {
		writePage(t, root, name, "flour and water\n")
	}

	out := mustCallWiki(t, cap, "search", map[string]any{"query": "flour"})
	if !strings.Contains(out, `"page":"Recipe: bread"`) || !strings.Contains(out, `"page":"Version 1.2"`) {
		t.Fatalf("search = %s, want the pages under their own names", out)
	}
	for _, name := range []string{"Recipe: bread", "Version 1.2", "notes.txt", "Q&A #1", "recipe: BREAD"} {
		if out := mustCallWiki(t, cap, "read", map[string]any{"page": name}); !strings.Contains(out, "flour and water") {
			t.Errorf("read(%q) = %s", name, out)
		}
	}
	mustCallWiki(t, cap, "append", map[string]any{"page": "Recipe: bread", "content": "and salt"})
	mustCallWiki(t, cap, "edit", map[string]any{"page": "Recipe: bread", "old_text": "flour", "new_text": "rye flour"})
	if got := readPageFile(t, root, "Recipe: bread"); got != "rye flour and water\n\nand salt\n" {
		t.Errorf("page = %q", got)
	}
	// Moving it gives it a name the tools would give.
	wantWikiError(t, cap, "move", map[string]any{"page": "Version 1.2", "new_name": "Version 1.3"}, "reads as a file type")
	if out := mustCallWiki(t, cap, "move", map[string]any{"page": "Version 1.2", "new_name": "Version 1-2"}); !strings.Contains(out, `"from":"Version 1.2"`) {
		t.Errorf("move = %s", out)
	}
	if out := mustCallWiki(t, cap, "delete", map[string]any{"page": "Q&A #1"}); !strings.Contains(out, `wiki-trash/Q&A #1.20261126-185503.md`) {
		t.Errorf("delete = %s", out)
	}

	// The rules still hold for a page that is not there, and for names
	// that would leave the wiki.
	wantWikiError(t, cap, "read", map[string]any{"page": "Recipe: cake"}, `contains ":"`)
	wantWikiError(t, cap, "append", map[string]any{"page": "Recipe: cake", "content": "x"}, `contains ":"`)
	wantWikiError(t, cap, "create", map[string]any{"page": "Recipe: bread", "content": "x"}, `page "Recipe: bread" already exists`)
	wantWikiError(t, cap, "read", map[string]any{"page": "../Recipe: bread"}, `contains ".."`)
}

func TestWikiMoveCanChangeOnlyTheLetterCase(t *testing.T) {
	cap, root := newWiki(t)
	writePage(t, root, "Projects/garden plan", "beans\n")

	out := mustCallWiki(t, cap, "move", map[string]any{"page": "Projects/garden plan", "new_name": "projects/Garden Plan"})
	if !strings.HasPrefix(out, `{"page":"Projects/Garden Plan"`) || !strings.Contains(out, `"from":"Projects/garden plan"`) {
		t.Errorf("move = %s, want the page renamed in its existing folder", out)
	}
	entries, err := os.ReadDir(filepath.Join(wiki.Dir(root), "Projects"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "Garden Plan.md" {
		t.Errorf("folder = %v (%v), want only Garden Plan.md", entries, err)
	}
	if entries, _ := os.ReadDir(wiki.Dir(root)); len(entries) != 1 {
		t.Errorf("wiki = %v, want only the Projects folder", entries)
	}
	// The same name again is still nothing to do.
	wantWikiError(t, cap, "move", map[string]any{"page": "Projects/Garden Plan", "new_name": "projects/Garden Plan"}, "nothing would move")
}
