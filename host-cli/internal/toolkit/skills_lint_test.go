package toolkit

import (
	"strings"
	"testing"
	"testing/fstest"
)

var lintRecipeIDs = map[string]bool{"skills-core": true, "py-docs": true, "js-core": true}

func fixtureSkill(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys["skills-core/s/"+name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func skillMD(requires, body string) string {
	return "---\nname: s\ndescription: Does s.\nrequires: " + requires + "\n---\n\n## Steps\n\n" + body
}

func problemsText(problems []SkillProblem) string {
	var lines []string
	for _, p := range problems {
		lines = append(lines, p.String())
	}
	return strings.Join(lines, "\n")
}

func TestLintTypeRequirement(t *testing.T) {
	if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[type:pmtiles]", "1. Go.\n")}), lintRecipeIDs); len(problems) != 0 {
		t.Fatalf("type:pmtiles flagged:\n%s", problemsText(problems))
	}
	problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[type:nonsense]", "1. Go.\n")}), lintRecipeIDs)
	if len(problems) != 1 || !strings.Contains(problems[0].Reason, "type:nonsense") {
		t.Fatalf("problems = %s", problemsText(problems))
	}
}

func TestLintFlagsInstallCommands(t *testing.T) {
	// Line 10 of SKILL.md: five lines of frontmatter, a blank, "## Steps",
	// a blank, then the body.
	body := "1. Run this:\nnpm install x\n"
	md := skillMD("[]", body)
	if got := strings.Split(md, "\n")[9]; got != "npm install x" {
		t.Fatalf("fixture line 10 = %q", got)
	}
	problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": md}), lintRecipeIDs)
	if len(problems) != 1 || problems[0].Line != 10 || problems[0].File != "SKILL.md" || problems[0].Skill != "skills-core/s" {
		t.Fatalf("problems = %s", problemsText(problems))
	}

	for _, line := range []string{
		"pip install openpyxl", "python3 -m pip install x", "uv add requests", "curl -O https://x/y", "wget http://x",
		"git clone https://github.com/a/b", "npx serve", "pnpm add x",
	} {
		if problems := LintSkills(fixtureSkill(map[string]string{"scripts/run.sh": line + "\n", "SKILL.md": skillMD("[]", "1. Go.\n")}), lintRecipeIDs); len(problems) != 1 {
			t.Errorf("%q: problems = %s", line, problemsText(problems))
		}
	}
	// Words that merely contain the names are fine.
	if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[]", "1. Use the curly braces and the snpm-like output.\n")}), lintRecipeIDs); len(problems) != 0 {
		t.Fatalf("problems = %s", problemsText(problems))
	}
}

func TestLintAllowsMarkedBlocks(t *testing.T) {
	body := "1. Go.\n\nOn a computer with a network:\n\n```not-for-this-stick\nnpm install x\ncurl -O https://example.com/a.js\n```\n"
	if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[]", body)}), lintRecipeIDs); len(problems) != 0 {
		t.Fatalf("problems = %s", problemsText(problems))
	}
	// Any other fence is checked.
	body = "1. Go.\n\n```sh\nnpm install x\n```\n"
	if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[]", body)}), lintRecipeIDs); len(problems) != 1 {
		t.Fatalf("problems = %s", problemsText(problems))
	}
}

func TestLintFlagsRemoteResources(t *testing.T) {
	files := map[string]string{
		"SKILL.md":         skillMD("[]", "1. Go. Data from https://www.naturalearthdata.com (public domain).\n"),
		"templates/a.html": "<html>\n<script src=\"https://cdn.example.com/x.js\"></script>\n</html>\n",
	}
	problems := LintSkills(fixtureSkill(files), lintRecipeIDs)
	if len(problems) != 1 || problems[0].File != "templates/a.html" || problems[0].Line != 2 {
		t.Fatalf("problems = %s", problemsText(problems))
	}
	for _, line := range []string{
		`<link rel="stylesheet" href="https://fonts.example.com/a.css">`,
		`<img src='//cdn.example.com/a.png'>`,
		`import * as d3 from "https://cdn.example.com/d3.js";`,
		`const r = await fetch("https://api.example.com/x");`,
		`body { background: url(https://example.com/a.png); }`,
	} {
		if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[]", "1. Go.\n"), "templates/b.html": line + "\n"}), lintRecipeIDs); len(problems) != 1 {
			t.Errorf("%q: problems = %s", line, problemsText(problems))
		}
	}
	// A link the reader may follow is not a load.
	if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[]", "1. Go.\n"), "templates/c.html": `<a href="https://example.com">source</a>` + "\n"}), lintRecipeIDs); len(problems) != 0 {
		t.Fatalf("problems = %s", problemsText(problems))
	}
}

func TestLintUnknownRequires(t *testing.T) {
	problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": skillMD("[py-docs, py-dcos]", "1. Go.\n")}), lintRecipeIDs)
	if len(problems) != 1 || problems[0].Skill != "skills-core/s" || !strings.Contains(problems[0].Reason, "py-dcos") {
		t.Fatalf("problems = %s", problemsText(problems))
	}
}

func TestLintRequiresFrontmatter(t *testing.T) {
	for name, md := range map[string]string{
		"no name":        "---\ndescription: Does s.\nrequires: []\n---\n## Steps\n",
		"no description": "---\nname: s\nrequires: []\n---\n## Steps\n",
		"wrong name":     "---\nname: t\ndescription: Does s.\nrequires: []\n---\n## Steps\n",
		"no frontmatter": "## Steps\n",
	} {
		if problems := LintSkills(fixtureSkill(map[string]string{"SKILL.md": md}), lintRecipeIDs); len(problems) != 1 || problems[0].File != "SKILL.md" {
			t.Errorf("%s: problems = %s", name, problemsText(problems))
		}
	}
	if problems := LintSkills(fixtureSkill(map[string]string{"references/a.md": "## A\n"}), lintRecipeIDs); len(problems) != 1 || !strings.Contains(problems[0].Reason, "SKILL.md") {
		t.Errorf("missing SKILL.md: problems = %s", problemsText(problems))
	}
	// A skill folder in a directory that names no recipe.
	fsys := fstest.MapFS{"skils-core/s/SKILL.md": {Data: []byte(skillMD("[]", "1. Go.\n"))}}
	if problems := LintSkills(fsys, lintRecipeIDs); len(problems) != 1 || !strings.Contains(problems[0].Reason, "skils-core") {
		t.Errorf("unknown recipe folder: problems = %s", problemsText(problems))
	}
}

func TestRealSkillsPassLint(t *testing.T) {
	ids := make(map[string]bool)
	for _, recipe := range embeddedRecipes(t) {
		ids[recipe.ID] = true
	}
	if problems := LintSkills(EmbeddedSkills(), ids); len(problems) != 0 {
		t.Fatalf("shipped skills fail the offline check:\n%s", problemsText(problems))
	}
}
