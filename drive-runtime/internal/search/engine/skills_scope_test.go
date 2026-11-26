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
