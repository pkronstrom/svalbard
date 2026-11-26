package toolkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func skillFile(name, requires string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("---\nname: " + name + "\ndescription: Does " + name + ".\nrequires: " + requires + "\n---\n\n## Steps\n\n1. Do it.\n")}
}

// fixtureSkills: a needs js-core, b needs nothing, c needs any map.
func fixtureSkills() fstest.MapFS {
	return fstest.MapFS{
		"README.md":                         {Data: []byte("authoring rules\n")},
		"skills-core/a/SKILL.md":            skillFile("a", "[js-core]"),
		"skills-core/b/SKILL.md":            skillFile("b", "[]"),
		"skills-core/b/references/notes.md": {Data: []byte("## Notes\n")},
		"skills-core/b/examples/check.sh":   {Data: []byte("#!/bin/sh\nexit 0\n")},
		"skills-core/c/SKILL.md":            skillFile("c", "\n  - type:pmtiles"),
	}
}

var skillsCoreEntry = manifest.RealizedEntry{ID: "skills-core", Type: "skills", Filename: "skills-core", RelativePath: "skills/skills-core"}

func skillDirs(t *testing.T, root string) []string {
	t.Helper()
	children, err := os.ReadDir(filepath.Join(root, "skills", "skills-core"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range children {
		if c.IsDir() {
			names = append(names, c.Name())
		}
	}
	return names
}

func TestParseSkillMeta(t *testing.T) {
	meta, err := ParseSkillMeta([]byte("---\nname: x\ndescription: \"Make x.\"\nrequires:\n  - py-docs\n  - type:pmtiles\n---\n# body\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := SkillMeta{Name: "x", Description: "Make x.", Requires: []string{"py-docs", "type:pmtiles"}}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("meta = %+v, want %+v", meta, want)
	}
	if _, err := ParseSkillMeta([]byte("# no frontmatter\n")); err == nil {
		t.Fatal("parsed a file without frontmatter")
	}
	if _, err := ParseSkillMeta([]byte("---\nname: x\n")); err == nil {
		t.Fatal("parsed unterminated frontmatter")
	}
}

func TestRequirementsMet(t *testing.T) {
	entries := []manifest.RealizedEntry{{ID: "py-docs", Type: "python-package"}, {ID: "osm-fi", Type: "pmtiles"}}
	for _, tc := range []struct {
		requires []string
		want     bool
	}{
		{nil, true},
		{[]string{"py-docs"}, true},
		{[]string{"py-docs", "type:pmtiles"}, true},
		{[]string{"js-core"}, false},
		{[]string{"type:gguf"}, false},
		{[]string{"type:pmtiles", "js-core"}, false},
	} {
		if got := RequirementsMet(tc.requires, entries); got != tc.want {
			t.Errorf("RequirementsMet(%v) = %v, want %v", tc.requires, got, tc.want)
		}
	}
}

func TestWriteSkillsFiltersByRequires(t *testing.T) {
	root := t.TempDir()
	report, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry}, fixtureSkills())
	if err != nil {
		t.Fatal(err)
	}
	if got := skillDirs(t, root); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("skills = %v, want [b]", got)
	}
	if !reflect.DeepEqual(report.Written, []string{"skills/skills-core/b"}) {
		t.Fatalf("written = %v", report.Written)
	}
	for _, rel := range []string{"SKILL.md", "references/notes.md", "examples/check.sh"} {
		if _, err := os.Stat(filepath.Join(root, "skills", "skills-core", "b", rel)); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(filepath.Join(root, "skills", "skills-core", "b", "examples", "check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("check.sh mode = %v, want executable", info.Mode())
	}

	maps := manifest.RealizedEntry{ID: "osm-fi", Type: "pmtiles", RelativePath: "maps/osm-fi.pmtiles"}
	if _, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry, maps}, fixtureSkills()); err != nil {
		t.Fatal(err)
	}
	if got := skillDirs(t, root); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("skills = %v, want [b c]", got)
	}
}

func TestWriteSkillsCreatesFolderWhenNoSkillQualifies(t *testing.T) {
	root := t.TempDir()
	only := fstest.MapFS{"skills-core/a/SKILL.md": skillFile("a", "[js-core]")}
	if _, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry}, only); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "skills", "skills-core")); err != nil || !info.IsDir() {
		t.Fatalf("skills folder missing: %v", err)
	}
}

func TestWriteSkillsLeavesUnchangedFilesAlone(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{skillsCoreEntry}
	if _, err := writeSkills(root, entries, fixtureSkills()); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	mtimes := map[string]time.Time{}
	base := filepath.Join(root, "skills", "skills-core")
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if err := os.Chtimes(p, old, old); err != nil {
			return err
		}
		mtimes[p] = old
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := writeSkills(root, entries, fixtureSkills())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Restored) != 0 || len(report.Removed) != 0 {
		t.Fatalf("report = %+v", report)
	}
	for p, want := range mtimes {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(want) {
			t.Errorf("%s rewritten (mtime %v)", p, info.ModTime())
		}
	}
}

func TestWriteSkillsUpdatesChangedSkillQuietly(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{skillsCoreEntry}
	if _, err := writeSkills(root, entries, fixtureSkills()); err != nil {
		t.Fatal(err)
	}
	// A newer CLI ships a new SKILL.md for b: not a local edit.
	next := fixtureSkills()
	next["skills-core/b/SKILL.md"] = &fstest.MapFile{Data: append(slices.Clone(next["skills-core/b/SKILL.md"].Data), "2. Again.\n"...)}
	report, err := writeSkills(root, entries, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Restored) != 0 {
		t.Fatalf("an update counted as a local edit: %v", report.Restored)
	}
	got, _ := os.ReadFile(filepath.Join(root, "skills", "skills-core", "b", "SKILL.md"))
	if string(got) != string(next["skills-core/b/SKILL.md"].Data) {
		t.Fatalf("SKILL.md = %q", got)
	}
}

func TestWriteSkillsRemovesUnsatisfied(t *testing.T) {
	root := t.TempDir()
	js := manifest.RealizedEntry{ID: "js-core", Type: "js-lib", RelativePath: "lib/js/js-core"}
	if _, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry, js}, fixtureSkills()); err != nil {
		t.Fatal(err)
	}
	if got := skillDirs(t, root); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("skills = %v, want [a b]", got)
	}
	report, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry}, fixtureSkills())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Removed, []string{"skills/skills-core/a"}) {
		t.Fatalf("removed = %v", report.Removed)
	}
	if got := skillDirs(t, root); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("skills = %v, want [b]", got)
	}
	// The recipe's own marker stays.
	if err := os.WriteFile(filepath.Join(root, "skills", "skills-core", ".svalbard-embedded"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSkills(root, []manifest.RealizedEntry{skillsCoreEntry}, fixtureSkills()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core", ".svalbard-embedded")); err != nil {
		t.Fatal(err)
	}
}

func TestWriteSkillsReportsOverwrittenEdits(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	previous := skillTree
	skillTree = func() fs.FS { return fixtureSkills() }
	t.Cleanup(func() { skillTree = previous })

	entries := []manifest.RealizedEntry{skillsCoreEntry}
	var notices []string
	opts := GenerateOpts{Notice: func(s string) { notices = append(notices, s) }}
	if err := Generate(root, entries, "", opts); err != nil {
		t.Fatal(err)
	}
	if len(notices) != 0 {
		t.Fatalf("first write noticed %v", notices)
	}

	skill := filepath.Join(root, "skills", "skills-core", "b")
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("my own version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(skill, "references", "notes.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "extra.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := writeSkills(root, entries, fixtureSkills())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"skills/skills-core/b/SKILL.md", "skills/skills-core/b/extra.md", "skills/skills-core/b/references/notes.md"}
	if !reflect.DeepEqual(report.Restored, want) {
		t.Fatalf("restored = %v, want %v", report.Restored, want)
	}
	got, _ := os.ReadFile(filepath.Join(skill, "SKILL.md"))
	if string(got) != string(fixtureSkills()["skills-core/b/SKILL.md"].Data) {
		t.Fatalf("SKILL.md not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(skill, "extra.md")); !os.IsNotExist(err) {
		t.Fatalf("extra file kept: %v", err)
	}

	// Through Generate: one notice for the skill, however many files.
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("edited again\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "references", "notes.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(root, entries, "", opts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(notices, []string{"restored svalbard skill b (local edits replaced)"}) {
		t.Fatalf("notices = %q", notices)
	}
	if err := Generate(root, entries, "", opts); err != nil {
		t.Fatal(err)
	}
	if len(notices) != 1 {
		t.Fatalf("an unchanged apply noticed again: %q", notices)
	}
}

func TestGenerateWithoutSkillsRecipe(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	previous := skillTree
	skillTree = func() fs.FS { return fixtureSkills() }
	t.Cleanup(func() { skillTree = previous })

	if err := Generate(root, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core")); !os.IsNotExist(err) {
		t.Fatalf("skills/skills-core without a skills recipe: %v", err)
	}

	if err := Generate(root, []manifest.RealizedEntry{skillsCoreEntry}, ""); err != nil {
		t.Fatal(err)
	}
	if got := skillDirs(t, root); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("skills = %v", got)
	}
	// A user's own skill recipe beside it is not Svalbard's to touch.
	own := filepath.Join(root, "skills", "my-skills", "x")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Generate(root, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core")); !os.IsNotExist(err) {
		t.Fatalf("skills/skills-core kept after the recipe went: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "skills")); err != nil || !info.IsDir() {
		t.Fatalf("skills/ root removed: %v", err)
	}
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("another recipe's skill removed: %v", err)
	}
}
