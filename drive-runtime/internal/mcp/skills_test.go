package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

const skillRecipes = `{
	"skills-core": {"id": "skills-core", "type": "skills", "description": "Procedures for this drive", "tags": []},
	"js-core": {"id": "js-core", "type": "js-lib", "description": "Browser libraries", "tags": []}
}`

// skillFixture is a drive with an archive (wiki.zim), two skills and one
// library quickstart, on disk and in the index as the host writes them.
type skillFixture struct {
	t    *testing.T
	root string
	db   *sql.DB
	cap  *SkillCapability
}

func newSkillFixture(t *testing.T) *skillFixture {
	t.Helper()
	root := searchFixtureDrive(t)
	t.Setenv("PATH", t.TempDir())
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	f := &skillFixture{t: t, root: root, db: db}

	f.file("skills/skills-core/data-dashboard/SKILL.md", "---\nname: data-dashboard\ndescription: Charts and tables from CSV or JSON in one HTML file\nlicense: MIT\nrequires: [js-core, py-docs]\n---\n# Data dashboard\n")
	f.file("skills/skills-core/data-dashboard/scripts/run.py", "print('run')\n")
	f.file("skills/skills-core/data-dashboard/references/r.md", "# Axis reference\n")
	f.file("skills/skills-core/data-dashboard/templates/page.html", "<html></html>\n")
	f.file("skills/skills-core/data-dashboard/.hidden", "x")
	indexSkillFile(t, db, "skills/skills-core/data-dashboard/SKILL.md", "data-dashboard",
		[2]string{"Overview", "Charts and tables from CSV or JSON in one HTML file\n\n# Data dashboard\n\nDraw a chart from a table."},
		[2]string{"Steps", "## Steps\n\n1. Load the CSV.\n2. Draw the chart with Chart.js."},
		[2]string{"Check", "## Check\n\nOpen the page and look at the chart."})
	indexSkillFile(t, db, "skills/skills-core/data-dashboard/references/r.md", "Axis reference",
		[2]string{"Overview", "# Axis reference\n\nLogarithmic axis options for a histogram."})

	f.file("skills/skills-core/vault-research/SKILL.md", "---\nname: vault-research\ndescription: >\n  Search, read and cite\n  the archives on this drive\n---\n")
	indexSkillFile(t, db, "skills/skills-core/vault-research/SKILL.md", "vault-research",
		[2]string{"Overview", "Search, read and cite the archives on this drive"},
		[2]string{"Steps", "## Steps\n\n1. Search.\n2. Read.\n3. Cite the source, or chart what you found."})

	f.file("lib/js/js-core/docs/chartjs.md", "---\nname: Chart.js\ndescription: Canvas charts\nversion: 4.5.0\nlicense: MIT\nportability: single-file\n---\n# Chart.js\n")
	indexSkillFile(t, db, "lib/js/js-core/docs/chartjs.md", "Chart.js",
		[2]string{"Overview", "Canvas charts\n\n# Chart.js\n\nImport chart.js/auto and draw a bar chart."})

	f.file(".svalbard/recipes.json", skillRecipes)
	meta, err := LoadMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	f.cap = NewSkillCapability(root, meta)
	t.Cleanup(func() { _ = f.cap.Close() })
	return f
}

func (f *skillFixture) file(rel, body string) {
	f.t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *skillFixture) search(params map[string]any) ([]SkillResult, string) {
	f.t.Helper()
	result, err := f.cap.Handle(context.Background(), "search", params)
	if err != nil {
		f.t.Fatalf("skill_search %v: %v", params, err)
	}
	items, _ := result.Data.([]SkillResult)
	return items, result.Text
}

func (f *skillFixture) read(params map[string]any) (SkillReadResult, error) {
	f.t.Helper()
	result, err := f.cap.Handle(context.Background(), "read", params)
	if err != nil {
		return SkillReadResult{}, err
	}
	return result.Data.(SkillReadResult), nil
}

func skillIDs(items []SkillResult) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func TestSkillCapabilityToolNames(t *testing.T) {
	srv := NewServer(NewSkillCapability(t.TempDir(), DriveMetadata{}))
	defer srv.Close()
	tools := srv.inner.ListTools()
	if len(tools) != 2 || tools["skill_search"] == nil || tools["skill_read"] == nil {
		t.Fatalf("tools = %v, want skill_search and skill_read", tools)
	}
	if got := tools["skill_search"].Tool.InputSchema.Required; !slices.Equal(got, []string{"query"}) {
		t.Errorf("skill_search required = %v", got)
	}
	if got := tools["skill_read"].Tool.InputSchema.Required; !slices.Equal(got, []string{"id"}) {
		t.Errorf("skill_read required = %v", got)
	}
	for name, tool := range tools {
		if !*tool.Tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", name)
		}
	}
}

// instructionsOf asks a server what it tells a client on connecting.
func instructionsOf(t *testing.T, srv *Server) string {
	t.Helper()
	client, err := mcpclient.NewInProcessClient(srv.inner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := client.Initialize(context.Background(), gomcp.InitializeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return result.Instructions
}

func TestDriveCapabilitiesGateSkills(t *testing.T) {
	without := t.TempDir()
	writeRecipes := func(root, recipes string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeRecipes(without, `{"wikem": {"id": "wikem", "type": "zim", "description": "", "tags": []}}`)
	// The folders alone do not open the gate: the recipe does.
	if err := os.MkdirAll(filepath.Join(without, "skills", "by-hand", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if names := DriveToolNames(without); slices.Contains(names, "skill_search") || slices.Contains(names, "skill_read") {
		t.Errorf("tools without a skills recipe = %v", names)
	}
	srv := NewDriveServer(without)
	if got := instructionsOf(t, srv); got != "" {
		t.Errorf("instructions without a skills recipe = %q", got)
	}
	_ = srv.Close()
	if SkillsInstalled(without) || LibrariesInstalled(without) {
		t.Error("a drive without the recipes reports skills or libraries")
	}

	for recipes, want := range map[string][2]bool{
		`{"skills-core": {"id": "skills-core", "type": "skills"}}`: {true, false},
		`{"js-core": {"id": "js-core", "type": "js-lib"}}`:         {false, true},
		skillRecipes: {true, true},
	} {
		with := t.TempDir()
		writeRecipes(with, recipes)
		names := DriveToolNames(with)
		if !slices.Contains(names, "skill_search") || !slices.Contains(names, "skill_read") || len(names) != 10 {
			t.Errorf("%s: tools = %v, want the eight drive tools and the two skill tools", recipes, names)
		}
		// After search, before the inventory: the order a client lists them in.
		if at := slices.Index(names, "skill_search"); at != slices.Index(names, "search_read")+1 {
			t.Errorf("%s: tool order = %v", recipes, names)
		}
		srv := NewDriveServer(with)
		if got := instructionsOf(t, srv); !strings.Contains(got, "skill_search") || !strings.Contains(got, "skill_read") {
			t.Errorf("%s: instructions = %q", recipes, got)
		}
		if _, ok := srv.inner.ListTools()["skill_search"]; !ok {
			t.Errorf("%s: the drive server has no skill_search", recipes)
		}
		_ = srv.Close()
		if SkillsInstalled(with) != want[0] || LibrariesInstalled(with) != want[1] {
			t.Errorf("%s: skills %v, libraries %v; want %v", recipes, SkillsInstalled(with), LibrariesInstalled(with), want)
		}
	}
}

func TestSkillSearchGroupsBySkill(t *testing.T) {
	f := newSkillFixture(t)
	// "chart" is in three sections of data-dashboard, in vault-research and
	// in the quickstart: each comes back once.
	items, note := f.search(map[string]any{"query": "chart"})
	ids := skillIDs(items)
	slices.Sort(ids)
	if want := []string{"lib/js/js-core/docs/chartjs.md", "skills/skills-core/data-dashboard", "skills/skills-core/vault-research"}; !slices.Equal(ids, want) || note != "" {
		t.Fatalf("results = %v (%q), want %v", ids, note, want)
	}
	by := map[string]SkillResult{}
	for _, item := range items {
		by[item.ID] = item
	}
	dashboard := by["skills/skills-core/data-dashboard"]
	if dashboard.Kind != "skill" || dashboard.Title != "data-dashboard" || dashboard.Description != "Charts and tables from CSV or JSON in one HTML file" ||
		!dashboard.HasScripts || dashboard.License != "MIT" || !slices.Equal(dashboard.Requires, []string{"js-core", "py-docs"}) || dashboard.File != "" {
		t.Errorf("data-dashboard = %+v", dashboard)
	}
	research := by["skills/skills-core/vault-research"]
	if research.HasScripts || research.Description != "Search, read and cite the archives on this drive" || research.License != "" {
		t.Errorf("vault-research = %+v", research)
	}
	library := by["lib/js/js-core/docs/chartjs.md"]
	if library.Kind != "library" || library.Title != "Chart.js" || library.Version != "4.5.0" || library.Portability != "single-file" || library.License != "MIT" {
		t.Errorf("chartjs = %+v", library)
	}

	// A hit in a file beside SKILL.md is its skill's hit, and says which file.
	items, _ = f.search(map[string]any{"query": "logarithmic histogram"})
	if len(items) != 1 || items[0].ID != "skills/skills-core/data-dashboard" || items[0].File != "references/r.md" || items[0].Title != "data-dashboard" {
		t.Fatalf("reference hit = %+v", items)
	}

	// A skill whose file is gone from the drive still answers from the index.
	if err := os.RemoveAll(filepath.Join(f.root, "skills", "skills-core", "vault-research")); err != nil {
		t.Fatal(err)
	}
	items, _ = f.search(map[string]any{"query": "cite archives"})
	if len(items) != 1 || items[0].Title != "vault-research" || items[0].Description != "Search, read and cite the archives on this drive" || items[0].HasScripts {
		t.Fatalf("skill without its file = %+v", items)
	}

	// Nothing found says so, and what there is.
	items, note = f.search(map[string]any{"query": "zeppelin"})
	if len(items) != 0 || !strings.Contains(note, "data-dashboard") || !strings.Contains(note, "vault-research") || !strings.Contains(note, "Chart.js") {
		t.Fatalf("no match: %v, %q", items, note)
	}
	if _, err := f.cap.Handle(context.Background(), "search", map[string]any{}); err == nil {
		t.Error("a search without a query was accepted")
	}
}

// One long skill must not use up the results: every section of it names the
// skill in its title, which the ranking weighs, and a knowledge search keeps
// only three pages of a document for that reason. Skills are grouped instead.
func TestSkillSearchLongSkillDoesNotCrowdOutOthers(t *testing.T) {
	f := newSkillFixture(t)
	var sections [][2]string
	for i := 1; i <= 60; i++ {
		sections = append(sections, [2]string{fmt.Sprintf("Part %d", i), fmt.Sprintf("## Part %d\n\nFirmware firmware firmware flashing, part %d.", i, i)})
	}
	indexSkillFile(t, f.db, "skills/skills-core/firmware/SKILL.md", "firmware", sections...)
	for _, name := range []string{"esp32", "arduino", "pico"} {
		indexSkillFile(t, f.db, "skills/skills-core/"+name+"/SKILL.md", name, [2]string{"Overview", "Flash firmware to the board."})
	}
	items, _ := f.search(map[string]any{"query": "firmware"})
	ids := skillIDs(items)
	if len(ids) != 4 || ids[0] != "skills/skills-core/firmware" {
		t.Fatalf("results = %v, want the long skill once and first, then the three others", ids)
	}
	if items, _ = f.search(map[string]any{"query": "firmware", "limit": 2}); len(items) != 2 {
		t.Fatalf("limit 2 returned %v", skillIDs(items))
	}
	// The default is five.
	for i := 0; i < 6; i++ {
		indexSkillFile(t, f.db, fmt.Sprintf("skills/skills-core/more-%d/SKILL.md", i), fmt.Sprintf("more-%d", i), [2]string{"Overview", "More firmware."})
	}
	if items, _ = f.search(map[string]any{"query": "firmware"}); len(items) != 5 {
		t.Fatalf("default limit returned %d results", len(items))
	}
}

func TestSkillSearchKindFilter(t *testing.T) {
	f := newSkillFixture(t)
	items, _ := f.search(map[string]any{"query": "chart", "kind": "library"})
	if ids := skillIDs(items); !slices.Equal(ids, []string{"lib/js/js-core/docs/chartjs.md"}) {
		t.Fatalf("libraries = %v", ids)
	}
	items, _ = f.search(map[string]any{"query": "chart", "kind": "skill"})
	if len(items) != 2 || items[0].Kind != "skill" || items[1].Kind != "skill" {
		t.Fatalf("skills = %+v", items)
	}
	if _, err := f.cap.Handle(context.Background(), "search", map[string]any{"query": "chart", "kind": "book"}); err == nil || !strings.Contains(err.Error(), "skill or library") {
		t.Fatalf("unknown kind: err = %v", err)
	}
}

func TestSkillSearchWithoutIndex(t *testing.T) {
	root := t.TempDir()
	cap := NewSkillCapability(root, DriveMetadata{})
	defer cap.Close()
	for action, params := range map[string]map[string]any{
		"search": {"query": "chart"},
		"read":   {"id": "skills/skills-core/data-dashboard"},
	} {
		result, err := cap.Handle(context.Background(), action, params)
		if err != nil {
			t.Fatalf("%s: err = %v, want a message", action, err)
		}
		if result.Data != nil || !strings.Contains(result.Text, "no search index") || !strings.Contains(result.Text, "skills/") || !strings.Contains(result.Text, "lib/js/") {
			t.Errorf("%s: result = %+v", action, result)
		}
	}
}

var continuesOffset = regexp.MustCompile(`offset (\d+)`)

func TestSkillReadInParts(t *testing.T) { // RF5
	f := newSkillFixture(t)
	var sections [][2]string
	var whole []string
	for i := 1; i <= 5; i++ {
		text := fmt.Sprintf("## Part %d\n\n", i) + strings.Repeat(fmt.Sprintf("Line of part %d, forty characters or so.\n", i), 60)
		text = strings.TrimSpace(text)
		sections = append(sections, [2]string{fmt.Sprintf("Part %d", i), text})
		whole = append(whole, text)
	}
	sections = append(sections, [2]string{"Last", "## Last\n\nThe very end of the skill."})
	whole = append(whole, "## Last\n\nThe very end of the skill.")
	indexSkillFile(t, f.db, "skills/skills-core/long/SKILL.md", "long", sections...)

	var got []string
	params := map[string]any{"id": "skills/skills-core/long"}
	for part := 0; ; part++ {
		if part > 10 {
			t.Fatal("the read never ends")
		}
		result, err := f.read(params)
		if err != nil {
			t.Fatalf("part %d: %v", part, err)
		}
		if n := len([]rune(result.Body)); n > readChars {
			t.Errorf("part %d has %d characters, over %d", part, n, readChars)
		}
		got = append(got, result.Body)
		if part == 0 {
			if result.ID != "skills/skills-core/long" || result.Title != "long" || result.Continues == "" {
				t.Fatalf("first part = %+v, want a continuation", result)
			}
			// A file that does not fit in one part names its sections, so one
			// can be asked for.
			if !slices.Equal(result.Sections, []string{"Part 1", "Part 2", "Part 3", "Part 4", "Part 5", "Last"}) {
				t.Errorf("sections = %v", result.Sections)
			}
		}
		if result.Continues == "" {
			if !strings.HasSuffix(result.Body, "The very end of the skill.") {
				t.Errorf("last part ends with %q", result.Body[max(0, len(result.Body)-60):])
			}
			break
		}
		match := continuesOffset.FindStringSubmatch(result.Continues)
		if match == nil || !strings.Contains(result.Continues, "skill_read") {
			t.Fatalf("continues = %q", result.Continues)
		}
		offset, _ := strconv.Atoi(match[1])
		params["offset"] = float64(offset) // as JSON delivers it
	}
	if len(got) < 2 {
		t.Fatalf("read in %d part, want several", len(got))
	}
	// Nothing was dropped between the parts.
	squeeze := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	if squeeze(strings.Join(got, "\n")) != squeeze(strings.Join(whole, "\n\n")) {
		t.Error("the parts do not add up to the file")
	}
	if _, err := f.read(map[string]any{"id": "skills/skills-core/long", "offset": float64(999999)}); err == nil || !strings.Contains(err.Error(), "past the end") {
		t.Errorf("offset past the end: err = %v", err)
	}

	// One section, by heading or by number.
	for _, section := range []any{"Last", "last", "section/6", "6", float64(6)} {
		result, err := f.read(map[string]any{"id": "skills/skills-core/long", "section": section})
		if err != nil || result.Body != "## Last\n\nThe very end of the skill." || result.Continues != "" {
			t.Errorf("section %v = %+v, %v", section, result, err)
		}
	}
	if _, err := f.read(map[string]any{"id": "skills/skills-core/long", "section": "Missing"}); err == nil || !strings.Contains(err.Error(), "Part 1") {
		t.Errorf("unknown section: err = %v, want the headings named", err)
	}
}

func TestSkillReadListsFiles(t *testing.T) {
	f := newSkillFixture(t)
	result, err := f.read(map[string]any{"id": "skills/skills-core/data-dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"references/r.md", "scripts/run.py", "templates/page.html"}; !slices.Equal(result.Files, want) {
		t.Errorf("files = %v, want %v", result.Files, want)
	}
	// Where they are on this computer: a model told only a path within the
	// drive looks for it in its working folder, then all over the disk.
	if want := filepath.Join(f.root, "skills", "skills-core", "data-dashboard"); result.Folder != want {
		t.Errorf("folder = %q, want %q", result.Folder, want)
	}
	// And not where its own work goes: given the folder alone, a 4B model
	// saved its page into the skill.
	if !strings.Contains(result.Note, "working folder") {
		t.Errorf("note = %q", result.Note)
	}
	if !strings.HasPrefix(result.Body, "Charts and tables from CSV or JSON in one HTML file") || !strings.HasSuffix(result.Body, "Open the page and look at the chart.") ||
		!strings.Contains(result.Body, "## Steps\n\n1. Load the CSV.") || result.Continues != "" || result.Sections != nil {
		t.Errorf("body = %q (continues %q, sections %v)", result.Body, result.Continues, result.Sections)
	}

	// A markdown file beside SKILL.md is read by naming it.
	reference, err := f.read(map[string]any{"id": "skills/skills-core/data-dashboard", "file": "references/r.md"})
	if err != nil || reference.File != "references/r.md" || !strings.Contains(reference.Body, "Logarithmic axis options") || reference.Files != nil || reference.Folder != "" || reference.Note != "" {
		t.Fatalf("reference = %+v, %v", reference, err)
	}
	// Another kind of file is not in the index; the error says where it is.
	_, err = f.read(map[string]any{"id": "skills/skills-core/data-dashboard", "file": "scripts/run.py"})
	if err == nil || !strings.Contains(err.Error(), "skills/skills-core/data-dashboard/scripts/run.py") {
		t.Errorf("script: err = %v", err)
	}

	// The id may come back as the SKILL.md path, the skill's name, or in
	// another case.
	for _, id := range []string{"skills/skills-core/data-dashboard/SKILL.md", "skills/skills-core/data-dashboard/", "data-dashboard", "Data-Dashboard", "/skills/skills-core/data-dashboard"} {
		if result, err := f.read(map[string]any{"id": id}); err != nil || result.ID != "skills/skills-core/data-dashboard" {
			t.Errorf("id %q = %+v, %v", id, result.ID, err)
		}
	}
	// A quickstart is read by its path or its library's name; it has no folder.
	for _, id := range []string{"lib/js/js-core/docs/chartjs.md", "chartjs", "Chart.js"} {
		result, err := f.read(map[string]any{"id": id})
		if err != nil || result.ID != "lib/js/js-core/docs/chartjs.md" || !strings.Contains(result.Body, "Import chart.js/auto") || result.Files != nil || result.Folder != "" {
			t.Errorf("id %q = %+v, %v", id, result, err)
		}
	}
	// A skill whose folder is gone is still read; it lists no files.
	if err := os.RemoveAll(filepath.Join(f.root, "skills", "skills-core", "data-dashboard")); err != nil {
		t.Fatal(err)
	}
	if result, err := f.read(map[string]any{"id": "skills/skills-core/data-dashboard"}); err != nil || result.Files != nil || result.Folder != "" || result.Body == "" {
		t.Errorf("without its folder = %+v, %v", result, err)
	}
}

func TestSkillReadRejectsPathsOutsideRoots(t *testing.T) {
	f := newSkillFixture(t)
	f.file("books/x.pdf", "%PDF-1.4")
	for _, params := range []map[string]any{
		{"id": "zim/../books/x"},
		{"id": "skills/../books/x.pdf"},
		{"id": "skills/skills-core/data-dashboard/../../../manifest.yaml"},
		{"id": "books/x.pdf"},
		{"id": "wiki.zim"},
		{"id": "skills"},
		{"id": "/etc/passwd"},
		{"id": `skills\skills-core\data-dashboard`},
		{"id": ""},
		{"id": "skills/skills-core/data-dashboard", "file": "../vault-research/SKILL.md"},
		{"id": "skills/skills-core/data-dashboard", "file": "/etc/passwd"},
		{"id": "skills/skills-core/data-dashboard", "file": "../../../manifest.yaml"},
	} {
		if result, err := f.read(params); err == nil {
			t.Errorf("read %v = %+v, want an error", params, result)
		}
	}
	// What it cannot find, it does not guess at.
	if _, err := f.read(map[string]any{"id": "skills/skills-core/no-such-skill"}); err == nil || !strings.Contains(err.Error(), "skill_search") {
		t.Errorf("unknown skill: err = %v", err)
	}
}

// search_read is what the search tool's results point at, also when a search
// was scoped to the skills.
func TestSearchReadReadsASkillSection(t *testing.T) {
	f := newSkillFixture(t)
	cap := NewSearchCapability(f.root, DriveMetadata{})
	defer cap.Close()
	result, err := cap.Handle(context.Background(), "read", map[string]any{"source": "skills/skills-core/data-dashboard/SKILL.md", "path": "section/2"})
	if err != nil {
		t.Fatal(err)
	}
	read := result.Data.(readResult)
	if read.Body != "## Steps\n\n1. Load the CSV.\n2. Draw the chart with Chart.js." || read.Title != "data-dashboard, Steps" || read.Path != "section/2" || read.Note != "" {
		t.Errorf("section = %+v", read)
	}
	// Without a path it is the whole file.
	result, err = cap.Handle(context.Background(), "read", map[string]any{"source": "skills/skills-core/data-dashboard/SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	if read = result.Data.(readResult); !strings.Contains(read.Body, "## Check") || read.Title != "data-dashboard" {
		t.Errorf("whole file = %+v", read)
	}
	if _, err := cap.Handle(context.Background(), "read", map[string]any{"source": "skills/skills-core/data-dashboard/SKILL.md", "path": "section/9"}); err == nil {
		t.Error("a section that does not exist was read")
	}
}

// With a semantic index a request in other words than the skill's own still
// finds it, and only among the skills: the archive's article has the very
// same vector here.
func TestSkillSearchUsesHybridSearchWithinTheSkills(t *testing.T) {
	root, db, queryLog := hybridFixtureDrive(t)
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	ids := indexSkillFile(t, db, "skills/skills-core/data-dashboard/SKILL.md", "data-dashboard",
		[2]string{"Overview", "Charts and tables from CSV."}, [2]string{"Steps", "## Steps\n\nDraw the chart."})
	for _, id := range ids {
		if _, err := db.Exec("INSERT INTO embeddings VALUES (?, 'data-dashboard', ?)", id, mcpFloat32Blob(256, 1.0/16)); err != nil {
			t.Fatal(err)
		}
	}
	cap := NewSkillCapability(root, DriveMetadata{})
	defer cap.Close()

	result, err := cap.Handle(context.Background(), "search", map[string]any{"query": "visualise my spreadsheet numbers"})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := result.Data.([]SkillResult)
	if len(items) != 1 || items[0].ID != "skills/skills-core/data-dashboard" || items[0].Description != "Charts and tables from CSV." {
		t.Fatalf("results = %+v (%q), want the skill once", items, result.Text)
	}
	if embedded, err := os.ReadFile(queryLog); err != nil || string(embedded) != "search_query: visualise my spreadsheet numbers" {
		t.Fatalf("embedded query = %q, %v", embedded, err)
	}

	// A long skill whose every section is as near the query as the others
	// must not take all the places before the sections are grouped.
	var sections [][2]string
	for i := 1; i <= 80; i++ {
		sections = append(sections, [2]string{fmt.Sprintf("Part %d", i), fmt.Sprintf("Firmware flashing, part %d.", i)})
	}
	long := indexSkillFile(t, db, "skills/skills-core/firmware/SKILL.md", "firmware", sections...)
	others := []int64{}
	for _, name := range []string{"esp32", "arduino"} {
		others = append(others, indexSkillFile(t, db, "skills/skills-core/"+name+"/SKILL.md", name, [2]string{"Overview", "Flash firmware to the board."})...)
	}
	for _, id := range append(long, others...) {
		if _, err := db.Exec("INSERT INTO embeddings VALUES (?, '', ?)", id, mcpFloat32Blob(256, 1.0/16)); err != nil {
			t.Fatal(err)
		}
	}
	result, err = cap.Handle(context.Background(), "search", map[string]any{"query": "firmware"})
	if err != nil {
		t.Fatal(err)
	}
	found := skillIDs(result.Data.([]SkillResult))
	slices.Sort(found)
	if want := []string{"skills/skills-core/arduino", "skills/skills-core/esp32", "skills/skills-core/firmware"}; !slices.Equal(found, want) {
		t.Fatalf("hybrid results with a long skill = %v, want %v", found, want)
	}
}

func TestReadFrontmatter(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	front := readFrontmatter(write("---\r\nname: \"tdd\"\r\ndescription: Use when: writing tests first\r\nlicense: MIT\r\nrequires:\r\n  - js-core\r\n  - type:pmtiles\r\nallowed-tools:\r\n  - Bash\r\nmetadata:\r\n  version: '1.2'\r\n  author: someone\r\n---\r\n# TDD\r\nversion: 9\r\n"))
	want := frontmatter{Name: "tdd", Description: "Use when: writing tests first", License: "MIT", Version: "1.2", Requires: []string{"js-core", "type:pmtiles"}}
	if front.Name != want.Name || front.Description != want.Description || front.License != want.License || front.Version != want.Version || !slices.Equal(front.Requires, want.Requires) {
		t.Errorf("frontmatter = %+v, want %+v", front, want)
	}
	front = readFrontmatter(write("---\nname: maps\ndescription: |\n  Interactive maps\n  over PMTiles.\nportability: served\nversion: 5.1.0\nrequires: js-maps\n---\n"))
	if front.Description != "Interactive maps over PMTiles." || front.Portability != "served" || front.Version != "5.1.0" || !slices.Equal(front.Requires, []string{"js-maps"}) {
		t.Errorf("folded frontmatter = %+v", front)
	}
	// No frontmatter, one that never closes, and no file at all.
	for _, body := range []string{"# Title\nname: not-frontmatter\n", "---\nname: unclosed\n"} {
		if front := readFrontmatter(write(body)); front.Name != "" {
			t.Errorf("frontmatter of %q = %+v", body, front)
		}
	}
	if front := readFrontmatter(filepath.Join(dir, "missing.md")); front.Name != "" || front.Requires != nil {
		t.Errorf("frontmatter of a missing file = %+v", front)
	}
}

// On a keyword-only index a request shares few words with the skill it is
// for. The nearest skill is an answer; "nothing found" sends a small model
// off to improvise.
func TestSkillSearchReturnsTheNearestSkillForLooseWords(t *testing.T) {
	f := newSkillFixture(t)
	items, _ := f.search(map[string]any{"query": "make a population dashboard with a world map"})
	if len(items) == 0 || items[0].ID != "skills/skills-core/data-dashboard" {
		t.Fatalf("results = %v, want data-dashboard first", skillIDs(items))
	}
	items, _ = f.search(map[string]any{"query": "histogram zeppelin"})
	if len(items) != 1 || items[0].File != "references/r.md" {
		t.Fatalf("one word of two = %+v", items)
	}
}
