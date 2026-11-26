package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newSkillsDrive is a drive without actions.json whose index holds two
// skills and a library quickstart, as the host writes them.
func newSkillsDrive(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL, title TEXT NOT NULL DEFAULT '');
		CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER NOT NULL, path TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '', sections TEXT);
		CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id');
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	`); err != nil {
		t.Fatal(err)
	}
	add := func(filename, title string, sections ...string) {
		t.Helper()
		res, err := db.Exec(`INSERT INTO sources (filename, title) VALUES (?, ?)`, filename, title)
		if err != nil {
			t.Fatal(err)
		}
		source, _ := res.LastInsertId()
		for i, lines := range sections {
			body := strings.Join(strings.Fields(lines), " ")
			heading := fmt.Sprintf("%s, Part %d", title, i+1)
			res, err := db.Exec(`INSERT INTO articles (source_id, path, title, body, sections) VALUES (?, ?, ?, ?, json_array(json_object('heading', '', 'body', ?)))`,
				source, fmt.Sprintf("section/%d", i+1), heading, body, lines)
			if err != nil {
				t.Fatal(err)
			}
			id, _ := res.LastInsertId()
			if _, err := db.Exec(`INSERT INTO articles_fts(rowid, title, body) VALUES (?, ?, ?)`, id, heading, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("skills/skills-core/data-dashboard/SKILL.md", "data-dashboard", "Charts from CSV in one HTML file", "## Steps\n\nDraw the chart.", "## Check\n\nLook at the chart.")
	add("skills/skills-core/long/SKILL.md", "long",
		"## One\n\n"+strings.TrimSpace(strings.Repeat("A chart line of forty characters or so.\n", 200)),
		"## Two\n\nThe very end.")
	add("lib/js/js-core/docs/chartjs.md", "Chart.js", "Canvas charts. Import chart.js/auto.")

	skill := filepath.Join(root, "skills", "skills-core", "data-dashboard")
	if err := os.MkdirAll(filepath.Join(skill, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"SKILL.md":       "---\nname: data-dashboard\ndescription: Charts from CSV in one HTML file\n---\n",
		"scripts/run.py": "print('run')\n",
	} {
		if err := os.WriteFile(filepath.Join(skill, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func skills(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Setenv("PATH", t.TempDir()) // no llama-server: keyword search
	var out, errOut bytes.Buffer
	err = runSkills(&out, &errOut, root, args)
	return out.String(), errOut.String(), err
}

func TestSkillsFindPrintsOneLinePerSkill(t *testing.T) {
	root := newSkillsDrive(t)
	out, errOut, err := skills(t, root, "find", "chart")
	if err != nil || errOut != "" {
		t.Fatalf("find: %v, stderr %q", err, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("output = %q, want three lines", out)
	}
	for _, want := range []string{
		"skills/skills-core/data-dashboard  data-dashboard — Charts from CSV in one HTML file",
		"lib/js/js-core/docs/chartjs.md  Chart.js — Canvas charts. Import chart.js/auto.",
		"skills/skills-core/long  long — ",
	} {
		found := false
		for _, line := range lines {
			found = found || strings.HasPrefix(line, want)
		}
		if !found {
			t.Errorf("output %q has no line starting %q", out, want)
		}
	}

	// Flags may come before or after the words.
	for _, args := range [][]string{
		{"find", "chart", "--kind", "library"}, {"find", "--kind=library", "chart"}, {"find", "chart", "csv", "--limit", "1", "--kind", "library"},
	} {
		out, _, err := skills(t, root, args...)
		if err != nil || !strings.HasPrefix(out, "lib/js/js-core/docs/chartjs.md  Chart.js") || strings.Count(out, "\n") != 1 {
			t.Errorf("%v: %q, %v", args, out, err)
		}
	}
	// Nothing found is said on stdout and is not a failure.
	if out, _, err := skills(t, root, "find", "zeppelin"); err != nil || !strings.HasPrefix(out, "Nothing found.") || !strings.Contains(out, "data-dashboard") {
		t.Errorf("no match: %q, %v", out, err)
	}
	// Neither is a drive without an index.
	if out, _, err := skills(t, t.TempDir(), "find", "chart"); err != nil || !strings.Contains(out, "no search index") {
		t.Errorf("no index: %q, %v", out, err)
	}
	if _, errOut, err := skills(t, root, "find"); err == nil || !strings.Contains(errOut, "skills find <words...>") {
		t.Errorf("find without words: stderr %q, %v", errOut, err)
	}
}

func TestSkillsReadPrintsContinuation(t *testing.T) {
	root := newSkillsDrive(t)
	out, errOut, err := skills(t, root, "read", "skills/skills-core/long")
	if err != nil || errOut != "" {
		t.Fatalf("read: %v, stderr %q", err, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	last := lines[len(lines)-1]
	offset, ok := strings.CutPrefix(last, "continues: svalbard-drive skills read skills/skills-core/long --offset ")
	if !ok || !strings.HasPrefix(out, "## One\n\nA chart line") {
		t.Fatalf("first part starts %q and ends %q", out[:min(len(out), 40)], last)
	}
	out, _, err = skills(t, root, "read", "skills/skills-core/long", "--offset", offset)
	if err != nil || !strings.HasSuffix(out, "## Two\n\nThe very end.\n") || strings.Contains(out, "continues:") {
		t.Fatalf("second part ends %q, %v", out[max(0, len(out)-60):], err)
	}

	// A skill that fits is printed whole, with the files it brings.
	out, _, err = skills(t, root, "read", "data-dashboard")
	want := "Charts from CSV in one HTML file\n\n## Steps\n\nDraw the chart.\n\n## Check\n\nLook at the chart.\n\nfiles in " + filepath.Join(root, "skills", "skills-core", "data-dashboard") + ":\n  scripts/run.py\nCopy from the folder; save what you make in your working folder, not there.\n"
	if err != nil || out != want {
		t.Fatalf("whole skill = %q, %v; want %q", out, err, want)
	}
	if out, _, err = skills(t, root, "read", "skills/skills-core/data-dashboard", "--section", "Part 2"); err != nil || out != "## Steps\n\nDraw the chart.\n" {
		t.Errorf("one section = %q, %v", out, err)
	}
	if out, _, err = skills(t, root, "read", "skills/skills-core/long", "--section=2"); err != nil || out != "## Two\n\nThe very end.\n" {
		t.Errorf("section by number = %q, %v", out, err)
	}
	if out, _, err := skills(t, root, "read", "skills/../data/search.db"); err == nil || out != "" {
		t.Errorf("a path outside the skills was read: %q", out)
	}
	if _, _, err := skills(t, root, "read", "skills/skills-core/none"); err == nil || !strings.Contains(err.Error(), "skills find") {
		t.Errorf("unknown skill: err = %v, want the command to find one", err)
	}
}

func TestSkillsUnknownSubcommandShowsUsage(t *testing.T) {
	root := newSkillsDrive(t)
	for _, args := range [][]string{{"list"}, {"find", "chart", "--colour", "red"}, {"read"}, {"read", "a", "b"}, {"find", "chart", "--limit", "many"}, {"read", "a", "--offset"}} {
		out, errOut, err := skills(t, root, args...)
		if err == nil || out != "" {
			t.Errorf("%v: stdout %q, err %v; want an error and nothing on stdout", args, out, err)
		}
		if !strings.Contains(errOut, "svalbard-drive skills find <words...>") || !strings.Contains(errOut, "svalbard-drive skills read <id>") {
			t.Errorf("%v: stderr = %q, want the usage", args, errOut)
		}
	}
	// Alone it says what the drive is and how to find a procedure: for an
	// agent that was not started from the menu this is the orientation.
	out, errOut, err := skills(t, root)
	if err != nil || errOut != "" || !strings.Contains(out, "Svalbard offline drive") || !strings.Contains(out, "svalbard-drive skills find <words...>") || strings.Count(out, "\n") > 12 {
		t.Errorf("skills alone: %q, stderr %q, %v", out, errOut, err)
	}
	// It is a fast path: no actions.json is needed, as for settings.
	t.Setenv("DRIVE_ROOT", root)
	if out, err := runSubcommand(t, "skills", "find", "chart", "--limit", "1"); err != nil || strings.Count(out, "\n") != 1 {
		t.Fatalf("through run(): %q, %v", out, err)
	}
}

// An agent on a laptop runs the binary by its path on the mounted stick, from
// its own project folder and without DRIVE_ROOT: the drive is where the
// binary is, not where the agent stands.
func TestDriveRootIsFoundFromTheExecutable(t *testing.T) {
	drive := t.TempDir()
	exe := filepath.Join(drive, ".svalbard", "runtime", "macos-arm64", "svalbard-drive")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if got := driveRootFor("", project, exe); got != drive {
		t.Errorf("from a project folder: %q, want the executable's drive %q", got, drive)
	}
	// DRIVE_ROOT and a working folder that is a drive keep their say.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := driveRootFor("/set/by/run", project, exe); got != "/set/by/run" {
		t.Errorf("with DRIVE_ROOT: %q", got)
	}
	if got := driveRootFor("", other, exe); got != other {
		t.Errorf("in a drive's folder: %q, want %q", got, other)
	}
	// A binary that is not on a drive (a development build) leaves the
	// working folder, as before.
	if got := driveRootFor("", project, filepath.Join(t.TempDir(), "bin", "svalbard-drive")); got != project {
		t.Errorf("binary outside a drive: %q, want %q", got, project)
	}
}

func TestSkillsContinuationNamesTheProgramAsItWasRun(t *testing.T) {
	root := newSkillsDrive(t)
	old := skillsProgram
	skillsProgram = "/Volumes/My Stick/run"
	t.Cleanup(func() { skillsProgram = old })
	out, _, err := skills(t, root, "read", "skills/skills-core/long")
	if err != nil || !strings.Contains(out, "\ncontinues: '/Volumes/My Stick/run' skills read skills/skills-core/long --offset ") {
		t.Fatalf("continuation = %q, %v", out[max(0, len(out)-160):], err)
	}
	if out, _, _ = skills(t, root); !strings.Contains(out, "  '/Volumes/My Stick/run' skills find <words...>") {
		t.Errorf("usage = %q", out)
	}
}

// Run by its path, as from a mounted stick, the command names the drive's run
// script; found on PATH, as in a harness's shell, its own name.
func TestSkillsProgramFollowsHowTheBinaryWasRun(t *testing.T) {
	root := newSkillsDrive(t)
	t.Setenv("DRIVE_ROOT", root)
	old := skillsProgram
	t.Cleanup(func() { skillsProgram = old })
	for argv0, want := range map[string]string{
		"svalbard-drive": "\n  svalbard-drive skills find <words...>",
		filepath.Join(root, ".svalbard", "runtime", "macos-arm64", "svalbard-drive"): "\n  " + shellWord(filepath.Join(root, "run")) + " skills find <words...>",
	} {
		skillsProgram = old
		if out, err := runAs(t, argv0, "skills"); err != nil || !strings.Contains(out, want) {
			t.Errorf("run as %s: %q, %v; want %q", argv0, out, err, want)
		}
	}
}
