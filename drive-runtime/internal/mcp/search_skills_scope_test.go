package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

// indexSkillFile adds a skill file or a library quickstart to an index as the
// host writes it: a source named by its path, one article per section.
func indexSkillFile(t *testing.T, db *sql.DB, filename, title string, sections ...[2]string) []int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO sources (filename, title) VALUES (?, ?)`, filename, title)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := res.LastInsertId()
	var ids []int64
	for i, section := range sections {
		heading, lines := title+", "+section[0], section[1]
		body := strings.Join(strings.Fields(lines), " ")
		res, err := db.Exec(`INSERT INTO articles (source_id, path, title, body, sections) VALUES (?, ?, ?, ?, json_array(json_object('heading', '', 'body', ?)))`,
			source, fmt.Sprintf("section/%d", i+1), heading, body, lines)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := db.Exec(`INSERT INTO articles_fts(rowid, title, body) VALUES (?, ?, ?)`, id, heading, body); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func skillScopeFixture(t *testing.T) *documentFixture {
	t.Helper()
	f := newDocumentFixture(t)
	indexSkillFile(t, f.db, "skills/skills-core/engine-care/SKILL.md", "engine-care",
		[2]string{"Overview", "Keep an engine cool.\n\n# Engine care"},
		[2]string{"Steps", "## Steps\n\n1. Check the coolant level."})
	indexSkillFile(t, f.db, "lib/js/js-core/docs/chartjs.md", "Chart.js",
		[2]string{"Overview", "Chart the coolant temperature with chart.js/auto."})
	f.file("skills/skills-core/engine-care/SKILL.md", "---\nname: engine-care\n---\n")
	f.file("lib/js/js-core/docs/chartjs.md", "# Chart.js\n")
	return f
}

func TestSearchToolScopedToSkillsFolder(t *testing.T) {
	f := skillScopeFixture(t)

	// Knowledge search does not see the skills, whatever they say.
	items, _ := f.search(map[string]any{"query": "coolant", "limit": 50})
	if len(items) == 0 {
		t.Fatal("no knowledge results")
	}
	for _, item := range items {
		if strings.HasPrefix(item.Source, "skills/") || strings.HasPrefix(item.Source, "lib/") {
			t.Errorf("unscoped search returned %s %s", item.Source, item.Path)
		}
	}
	// Nor does what it says the drive holds.
	if _, note := f.search(map[string]any{"query": "zeppelin"}); strings.Contains(note, "skills") || strings.Contains(note, "lib/js") || strings.Contains(note, "chartjs") {
		t.Errorf("holdings name the skills: %q", note)
	}

	// Named as the scope, they are found, under the names the tools use.
	for source, want := range map[string][]string{
		"skills/":                        {"skills/skills-core/engine-care/SKILL.md section/2"},
		"skills":                         {"skills/skills-core/engine-care/SKILL.md section/2"},
		"skills/skills-core":             {"skills/skills-core/engine-care/SKILL.md section/2"},
		"Skills/Skills-Core/":            {"skills/skills-core/engine-care/SKILL.md section/2"},
		"lib/js":                         {"lib/js/js-core/docs/chartjs.md section/1"},
		"lib/js/js-core/docs/":           {"lib/js/js-core/docs/chartjs.md section/1"},
		"lib/js/js-core/docs/chartjs.md": {"lib/js/js-core/docs/chartjs.md section/1"},
		"skills/skills-core/engine-care/SKILL.md": {"skills/skills-core/engine-care/SKILL.md section/2"},
	} {
		items, _ := f.search(map[string]any{"query": "coolant", "source": source})
		if got := sources(items); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("source %q found %v, want %v", source, got, want)
		}
	}

	// A skill is never what a loosely named source resolves to, and a failed
	// name lists the knowledge sources only.
	for _, source := range []string{"engine-care", "chartjs", "skill", "SKILL.md", "skills/no-such-folder"} {
		_, err := f.cap.Handle(context.Background(), "search", map[string]any{"query": "coolant", "source": source})
		if err == nil || !strings.Contains(err.Error(), "unknown source") {
			t.Errorf("source %q: err = %v, want unknown source", source, err)
			continue
		}
		if _, listed, _ := strings.Cut(err.Error(), "Sources on this drive:"); !strings.Contains(listed, "wiki") || strings.Contains(listed, "skills/") || strings.Contains(listed, "lib/js") {
			t.Errorf("source %q: the error lists %q, want the knowledge sources only", source, listed)
		}
	}
}

func TestHybridSearchToolKeepsSkillsOut(t *testing.T) { // RF3, through the tool
	root, db, _ := hybridFixtureDrive(t)
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	// The skill is about water too, and its vector is the one every query
	// embeds to.
	ids := indexSkillFile(t, db, "skills/skills-core/water-filter/SKILL.md", "water-filter", [2]string{"Steps", "Filter the water through sand."})
	if _, err := db.Exec("INSERT INTO embeddings VALUES (?, 'water-filter, Steps', ?)", ids[0], mcpFloat32Blob(256, 1.0/16)); err != nil {
		t.Fatal(err)
	}
	cap := NewSearchCapability(root, DriveMetadata{})
	defer cap.Close()

	for _, detail := range []string{"snippet", "full"} {
		result, err := cap.Handle(context.Background(), "search", map[string]any{"query": "water", "detail": detail})
		if err != nil || result.Text != "" {
			t.Fatalf("%s: err = %v, advisory = %q; want a hybrid search", detail, err, result.Text)
		}
		if got := sources(result.Data.([]searchResultItem)); strings.Join(got, "|") != "wiki /Water" {
			t.Errorf("%s: unscoped hybrid search found %v, want the archive only", detail, got)
		}
		result, err = cap.Handle(context.Background(), "search", map[string]any{"query": "water", "detail": detail, "source": "skills/"})
		if err != nil || result.Text != "" {
			t.Fatalf("%s, skills: err = %v, advisory = %q", detail, err, result.Text)
		}
		if got := sources(result.Data.([]searchResultItem)); strings.Join(got, "|") != "skills/skills-core/water-filter/SKILL.md section/1" {
			t.Errorf("%s: skills-scoped hybrid search found %v", detail, got)
		}
	}
}

func TestVaultSourcesHidesSkills(t *testing.T) {
	f := skillScopeFixture(t)
	meta, err := LoadMetadata(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, filter := range []string{"", "book"} {
		result, err := NewVaultCapability(f.root, meta).Handle(context.Background(), "sources", map[string]any{"type": filter})
		if err != nil {
			t.Fatal(err)
		}
		listed := result.Data.([]inspect.SourceInfo)
		if len(listed) == 0 {
			t.Fatalf("type %q: no sources", filter)
		}
		for _, source := range listed {
			if strings.HasPrefix(source.Path, "skills/") || strings.HasPrefix(source.Path, "lib/") || strings.Contains(source.Name, "SKILL") {
				t.Errorf("type %q: vault_sources lists %+v", filter, source)
			}
		}
	}
}

// A document collection may be called "skills". Its short name keeps meaning
// the collection on a drive that has no skill files.
func TestCollectionNamedSkillsStillResolves(t *testing.T) {
	f := newDocumentFixture(t)
	guide := f.document("books/skills/knots.pdf", "Knots", "")
	f.page(guide, 1, "Knots, p. 1", "Tie a bowline around the coolant hose")
	items, _ := f.search(map[string]any{"query": "bowline", "source": "skills"})
	if got := sources(items); len(got) != 1 || got[0] != "books/skills/knots.pdf page/1" {
		t.Fatalf("source skills on a drive without skill files = %v", got)
	}
	// With skill files on the drive the bare name is the skills; the
	// collection is named with its folder.
	indexSkillFile(t, f.db, "skills/skills-core/ropes/SKILL.md", "ropes", [2]string{"Overview", "Splice a rope; a bowline is a knot."})
	items, _ = f.search(map[string]any{"query": "bowline", "source": "skills"})
	if got := sources(items); len(got) != 1 || got[0] != "skills/skills-core/ropes/SKILL.md section/1" {
		t.Fatalf("source skills with skill files = %v", got)
	}
	items, _ = f.search(map[string]any{"query": "bowline", "source": "books/skills"})
	if got := sources(items); len(got) != 1 || got[0] != "books/skills/knots.pdf page/1" {
		t.Fatalf("source books/skills = %v", got)
	}
}
