package engine

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"testing"
)

// scopeDB holds one article of each world, all about charts and each with a
// vector: an archive, a book, a skill and a library quickstart.
func scopeDB(t *testing.T) *sql.DB {
	t.Helper()
	db := documentDB(t)
	insertEmbedding(t, db, insertArticle(t, db, insertSource(t, db, "wiki.zim"), "/Chart", "Chart", "A chart is a graphic of data."), "Chart", []float32{0.6, 0.8})
	book := insertDocument(t, db, "books/x.pdf", "Statistics", "")
	insertEmbedding(t, db, insertArticle(t, db, book, "page/1", "Statistics, p. 1", "Read a chart before the table."), "Statistics, p. 1", []float32{0.8, 0.6})
	skill := insertDocument(t, db, "skills/skills-core/a/SKILL.md", "data-dashboard", "")
	insertEmbedding(t, db, insertArticle(t, db, skill, "section/1", "data-dashboard, Steps", "Draw the chart with Chart.js."), "data-dashboard, Steps", []float32{1, 0})
	library := insertDocument(t, db, "lib/js/js-core/docs/chartjs.md", "Chart.js", "")
	insertEmbedding(t, db, insertArticle(t, db, library, "section/1", "Chart.js, Overview", "Import chart.js/auto for a chart."), "Chart.js, Overview", []float32{0.98, 0.2})
	return db
}

func filenames(results []Result) []string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Filename)
	}
	sort.Strings(names)
	return names
}

var (
	knowledgeSources = []string{"books/x.pdf", "wiki.zim"}
	skillSources     = []string{"lib/js/js-core/docs/chartjs.md", "skills/skills-core/a/SKILL.md"}
)

func TestKeywordSearchExcludesSkillsByDefault(t *testing.T) {
	db := scopeDB(t)
	if got := filenames(search(t, db, "chart", KeywordOptions{}).Results); !reflect.DeepEqual(got, knowledgeSources) {
		t.Fatalf("unscoped search found %v, want %v", got, knowledgeSources)
	}
	results, err := New(db).KeywordContext(context.Background(), "chart", 10, "")
	if err != nil || !reflect.DeepEqual(filenames(results), knowledgeSources) {
		t.Fatalf("unscoped KeywordContext found %v, %v; want %v", filenames(results), err, knowledgeSources)
	}
	// A word only the skills have is not on the drive, as far as knowledge
	// search can tell.
	if matches := search(t, db, "chart import", KeywordOptions{}); !reflect.DeepEqual(matches.Missing, []string{"import"}) {
		t.Errorf("missing = %v, want import", matches.Missing)
	}
}

func TestKeywordSearchScopedToSkills(t *testing.T) {
	db := scopeDB(t)
	for scope, want := range map[string][]string{
		SkillScope:                      skillSources,
		"skills/":                       {"skills/skills-core/a/SKILL.md"},
		"skills/skills-core/":           {"skills/skills-core/a/SKILL.md"},
		"skills/skills-core/a/SKILL.md": {"skills/skills-core/a/SKILL.md"},
		"lib/js/":                       {"lib/js/js-core/docs/chartjs.md"},
		"books/":                        {"books/x.pdf"},
	} {
		if got := filenames(search(t, db, "chart", KeywordOptions{Source: scope}).Results); !reflect.DeepEqual(got, want) {
			t.Errorf("scope %q found %v, want %v", scope, got, want)
		}
		results, err := New(db).KeywordContext(context.Background(), "chart", 10, scope)
		if err != nil || !reflect.DeepEqual(filenames(results), want) {
			t.Errorf("KeywordContext scope %q found %v, %v; want %v", scope, filenames(results), err, want)
		}
	}
	if matches := search(t, db, "chart graphic", KeywordOptions{Source: SkillScope}); !reflect.DeepEqual(matches.Missing, []string{"graphic"}) {
		t.Errorf("missing in the skills = %v, want graphic", matches.Missing)
	}
}

func TestHybridSearchExcludesSkillVectors(t *testing.T) { // RF3
	db := scopeDB(t)
	eng := New(db)
	query := []float32{1, 0} // the skill's own vector

	results, err := eng.HybridContext(context.Background(), "chart", query, 10, "")
	if err != nil || !reflect.DeepEqual(filenames(results), knowledgeSources) {
		t.Fatalf("unscoped hybrid found %v, %v; want %v", filenames(results), err, knowledgeSources)
	}
	results, err = eng.HybridContext(context.Background(), "chart", query, 10, SkillScope)
	if err != nil || !reflect.DeepEqual(filenames(results), skillSources) {
		t.Fatalf("skill-scoped hybrid found %v, %v; want %v", filenames(results), err, skillSources)
	}

	// With no keyword candidate every vector is scanned: that path keeps to
	// the scope as well.
	results, err = eng.HybridContext(context.Background(), "zzzunmatched", query, 10, "")
	if err != nil || !reflect.DeepEqual(filenames(results), knowledgeSources) {
		t.Fatalf("unscoped vector scan found %v, %v; want %v", filenames(results), err, knowledgeSources)
	}
	results, err = eng.HybridContext(context.Background(), "zzzunmatched", query, 10, "skills/")
	if err != nil || !reflect.DeepEqual(filenames(results), []string{"skills/skills-core/a/SKILL.md"}) {
		t.Fatalf("skills/ vector scan found %v, %v", filenames(results), err)
	}
}

func TestInScopeEmptyExcludesSkills(t *testing.T) {
	for _, tc := range []struct {
		filename, scope string
		want            bool
	}{
		{"wiki.zim", "", true},
		{"books/x.pdf", "", true},
		{"skills/skills-core/a/SKILL.md", "", false},
		{"lib/js/js-core/docs/chartjs.md", "", false},
		{"skills/skills-core/a/SKILL.md", SkillScope, true},
		{"lib/js/js-core/docs/chartjs.md", SkillScope, true},
		{"wiki.zim", SkillScope, false},
		{"books/x.pdf", SkillScope, false},
		{"skills/skills-core/a/SKILL.md", "skills/", true},
		{"skills/skills-core/a/SKILL.md", "skills/skills-core/a/", true},
		{"skills/skills-core/a/SKILL.md", "skills/skills-core/a/SKILL.md", true},
		{"skills/skills-core/a/SKILL.md", "lib/js/", false},
		{"skills/skills-core/a/SKILL.md", "books/", false},
		{"skillset.zim", "", true}, // an archive whose name merely starts the same
	} {
		if got := InScope(tc.filename, tc.scope); got != tc.want {
			t.Errorf("InScope(%q, %q) = %v, want %v", tc.filename, tc.scope, got, tc.want)
		}
	}
	for filename, want := range map[string]bool{
		"skills/skills-core/a/SKILL.md": true, "lib/js/js-core/docs/chartjs.md": true,
		"books/x.pdf": false, "wiki.zim": false, "lib.zim": false,
	} {
		if got := IsSkillSource(filename); got != want {
			t.Errorf("IsSkillSource(%q) = %v", filename, got)
		}
	}
}

func TestListSourcesLeavesSkillsOut(t *testing.T) {
	db := scopeDB(t)
	sources, err := New(db).ListSources()
	if err != nil || len(sources) != 2 || sources[0].Filename != "books/x.pdf" || sources[1].Filename != "wiki.zim" {
		t.Fatalf("ListSources = %+v, %v", sources, err)
	}
	// SourceFilenames is what an explicit source is checked against.
	all, err := New(db).SourceFilenames(context.Background())
	if err != nil || len(all) != 4 {
		t.Fatalf("SourceFilenames = %v, %v; want all four", all, err)
	}
}

func TestSectionsComeBackInFileOrderWithTheirLines(t *testing.T) {
	db := documentDB(t)
	skill := insertDocument(t, db, "skills/skills-core/a/SKILL.md", "data-dashboard", "")
	// Section 10 sorts after 9, not after 1.
	for _, number := range []int{10, 2, 9, 1} {
		id := insertArticle(t, db, skill, SectionPath(number), "data-dashboard, Part", "one line of section "+SectionPath(number))
		if number == 2 {
			if _, err := db.Exec(`UPDATE articles SET sections = json_array(json_object('heading', '', 'body', ?)) WHERE id = ?`, "## Steps\n\n1. First.\n2. Second.", id); err != nil {
				t.Fatal(err)
			}
		}
	}
	insertDocument(t, db, "lib/js/js-core/docs/chartjs.md", "Chart.js", "")
	insertArticle(t, db, insertDocument(t, db, "books/x.pdf", "Statistics", ""), "page/1", "Statistics, p. 1", "a page")
	eng := New(db)

	sections, err := eng.Sections(context.Background(), "skills/skills-core/a/SKILL.md")
	if err != nil || len(sections) != 4 {
		t.Fatalf("Sections = %+v, %v", sections, err)
	}
	var order []int
	for _, section := range sections {
		order = append(order, section.Number)
	}
	if !reflect.DeepEqual(order, []int{1, 2, 9, 10}) {
		t.Errorf("order = %v", order)
	}
	if sections[1].Text != "## Steps\n\n1. First.\n2. Second." || sections[0].Text != "one line of section section/1" {
		t.Errorf("texts = %q, %q", sections[1].Text, sections[0].Text)
	}
	if none, err := eng.Sections(context.Background(), "skills/none/SKILL.md"); err != nil || len(none) != 0 {
		t.Errorf("sections of a file that is not indexed = %+v, %v", none, err)
	}

	sources, err := eng.SkillSources(context.Background())
	want := []SkillSource{{"lib/js/js-core/docs/chartjs.md", "Chart.js"}, {"skills/skills-core/a/SKILL.md", "data-dashboard"}}
	if err != nil || !reflect.DeepEqual(sources, want) {
		t.Fatalf("SkillSources = %+v, %v; want %+v", sources, err, want)
	}
}

func TestSearchAnyWordKeepsTheNearestWhenNothingHasMostWords(t *testing.T) {
	db := scopeDB(t)
	// Neither skill file has "histogram"; knowledge search rules would return
	// nothing for two words of which one is missing.
	if matches := search(t, db, "chart histogram", KeywordOptions{Source: SkillScope}); len(matches.Results) != 0 {
		t.Fatalf("without AnyWord: %v", filenames(matches.Results))
	}
	matches := search(t, db, "chart histogram", KeywordOptions{Source: SkillScope, AnyWord: true})
	if !reflect.DeepEqual(filenames(matches.Results), skillSources) || matches.Exact != 0 || !reflect.DeepEqual(matches.Missing, []string{"histogram"}) {
		t.Fatalf("with AnyWord: %v, exact %d, missing %v", filenames(matches.Results), matches.Exact, matches.Missing)
	}
	for _, result := range matches.Results {
		if !result.Partial || !reflect.DeepEqual(result.Matched, []string{"chart"}) {
			t.Errorf("%s: partial %v, matched %v", result.Filename, result.Partial, result.Matched)
		}
	}
	// More of the words still comes first, and all of them before that.
	matches = search(t, db, "import chart histogram zeppelin", KeywordOptions{Source: SkillScope, AnyWord: true})
	if len(matches.Results) != 2 || matches.Results[0].Filename != "lib/js/js-core/docs/chartjs.md" || len(matches.Results[0].Matched) != 2 {
		t.Fatalf("ordering: %+v", matches.Results)
	}
	// Words that are nowhere find nothing, with or without it.
	if matches := search(t, db, "zeppelin airship", KeywordOptions{Source: SkillScope, AnyWord: true}); len(matches.Results) != 0 {
		t.Fatalf("no word matches: %v", filenames(matches.Results))
	}
	// The knowledge scope is unchanged by the option being off.
	if matches := search(t, db, "chart histogram", KeywordOptions{}); len(matches.Results) != 0 {
		t.Fatalf("knowledge search returned a one-word match: %v", filenames(matches.Results))
	}
}

// A long skill file matches with every section: each names the skill in its
// title. Like a document's pages, only a few of one file's sections are
// returned, so the files behind it are reached, in keyword search and in both
// halves of hybrid search.
func TestSkillSearchReturnsFewSectionsPerFile(t *testing.T) {
	db := documentDB(t)
	long := insertDocument(t, db, "skills/skills-core/firmware/SKILL.md", "firmware", "")
	for number := 1; number <= 300; number++ {
		id := insertArticle(t, db, long, SectionPath(number), "firmware, Part", "Firmware firmware firmware flashing.")
		insertEmbedding(t, db, id, "firmware", []float32{1, 0})
	}
	for _, name := range []string{"esp32", "arduino", "pico"} {
		id := insertArticle(t, db, insertDocument(t, db, "skills/skills-core/"+name+"/SKILL.md", name, ""), SectionPath(1), name+", Overview", "Flash firmware to the board.")
		insertEmbedding(t, db, id, name, []float32{0.6, 0.8})
	}
	eng := New(db)
	perFile := func(results []Result) map[string]int {
		counts := map[string]int{}
		for _, result := range results {
			counts[result.Filename]++
		}
		return counts
	}
	want := map[string]int{
		"skills/skills-core/firmware/SKILL.md": sectionsPerSkillFile,
		"skills/skills-core/esp32/SKILL.md":    1, "skills/skills-core/arduino/SKILL.md": 1, "skills/skills-core/pico/SKILL.md": 1,
	}

	matches := search(t, db, "firmware", KeywordOptions{Source: SkillScope, Limit: 200})
	if got := perFile(matches.Results); !reflect.DeepEqual(got, want) {
		t.Errorf("keyword search: %v, want %v", got, want)
	}
	if !reflect.DeepEqual(matches.MorePages, []string{"skills/skills-core/firmware/SKILL.md"}) {
		t.Errorf("more sections in %v", matches.MorePages)
	}
	// Limited to the file itself, every section is available.
	if one := search(t, db, "firmware", KeywordOptions{Source: "skills/skills-core/firmware/SKILL.md", Limit: 50}); len(one.Results) != 50 {
		t.Errorf("file scope returned %d results, want 50", len(one.Results))
	}

	for name, query := range map[string]string{"with keyword candidates": "firmware", "vector scan alone": "zzzunmatched"} {
		results, err := eng.HybridContext(context.Background(), query, []float32{1, 0}, 200, SkillScope)
		if err != nil {
			t.Fatal(err)
		}
		if got := perFile(results); !reflect.DeepEqual(got, want) {
			t.Errorf("hybrid search, %s: %v, want %v", name, got, want)
		}
	}
	// Knowledge sources are not capped here: a document's pages have their
	// own rule and an archive's articles none.
	wiki := insertSource(t, db, "wiki.zim")
	for number := 1; number <= 10; number++ {
		insertArticle(t, db, wiki, "/Firmware_"+SectionPath(number), "Firmware", "Firmware of a device.")
	}
	if results, err := eng.KeywordContext(context.Background(), "firmware", 50, ""); err != nil || len(results) != 10 {
		t.Errorf("archive articles: %d, %v; want all ten", len(results), err)
	}
}
