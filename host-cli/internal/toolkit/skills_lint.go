package toolkit

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// SkillProblem is one reason a shipped skill would not work on a stick
// without a network, or would not be found.
type SkillProblem struct {
	Skill  string // <recipe>/<skill>
	File   string // relative to the skill folder
	Line   int    // 1-based; 0 when the problem is not on one line
	Reason string
}

func (p SkillProblem) String() string {
	where := p.Skill
	if p.File != "" {
		where += "/" + p.File
	}
	if p.Line > 0 {
		where += fmt.Sprintf(":%d", p.Line)
	}
	return where + ": " + p.Reason
}

// offlineFence is the info string of a markdown fence whose lines are for a
// computer with a network and are not checked.
const offlineFence = "not-for-this-stick"

// lintedExtensions are the files the offline check reads.
var lintedExtensions = map[string]bool{
	".md": true, ".txt": true, ".html": true, ".css": true, ".js": true, ".mjs": true, ".ts": true,
	".py": true, ".sh": true, ".json": true, ".ini": true, ".c": true, ".h": true,
}

// fetchRules flag lines that install software or load something from the
// network. Plain links in prose are allowed: they attribute, nothing loads
// them.
var fetchRules = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	{regexp.MustCompile(`(^|[^\w.-])(npm|pnpm|npx)\b`), "runs a Node package manager"},
	{regexp.MustCompile(`\bpip3?\s+install\b`), "installs Python packages"},
	{regexp.MustCompile(`\buv\s+(add|pip\s+install)\b`), "installs Python packages"},
	{regexp.MustCompile(`(^|[^\w.-])(curl|wget)\b`), "downloads from the network"},
	{regexp.MustCompile(`\b(pipx|uvx)\b`), "installs Python packages"},
	{regexp.MustCompile(`["']pip3?["']\s*,\s*["']install["']`), "installs Python packages"},
	{regexp.MustCompile(`\b(conda|mamba)\s+install\b`), "installs packages"},
	{regexp.MustCompile(`\b(yarn|bun)\s+(add|install|x|dlx)\b`), "runs a Node package manager"},
	{regexp.MustCompile(`\bbrew\s+install\b`), "installs packages"},
	{regexp.MustCompile(`\bapt(-get)?\s+install\b`), "installs packages"},
	{regexp.MustCompile(`\bgo\s+(install|get)\s+(-\S+\s+)*[\w-]+(\.[\w-]+)+/`), "installs Go modules"},
	{regexp.MustCompile(`\bcargo\s+install\b`), "installs Rust crates"},
	{regexp.MustCompile(`\bpio\s+(pkg|lib)\s+install\b|\bplatformio\s+(pkg|lib)\s+install\b`), "installs PlatformIO packages"},
	{regexp.MustCompile(`\blib_deps\b|\bplatform_packages\b`), "makes PlatformIO download libraries"},
	{regexp.MustCompile(`\bgit\s+(clone|fetch|pull)\b`), "fetches from the network"},
	{regexp.MustCompile(`\burlopen\s*\(|\burllib\.request\b`), "opens a network address from Python"},
	{regexp.MustCompile(`\brequests\.(get|post|put|patch|delete|head|request|Session)\b|\bhttpx\.`), "opens a network address from Python"},
	{regexp.MustCompile(`\bhttp\.client\b|\bsocket\.create_connection\b`), "opens a network connection from Python"},
	{regexp.MustCompile(`(?i)\bsrcset\s*=\s*["']?[^"'>]*(https?:)?//`), "loads a resource from the network"},
	{regexp.MustCompile(`(?i)@import\s+(url\(\s*)?["']?(https?:)?//`), "loads a stylesheet from the network"},
	{regexp.MustCompile(`\bWebSocket\s*\(\s*["'` + "`" + `]wss?://`), "opens a network connection"},
	{regexp.MustCompile(`\bEventSource\s*\(\s*["'` + "`" + `](https?:)?//`), "opens a network connection"},
	{regexp.MustCompile(`(?i)\bsrc\s*=\s*["']?(https?:)?//`), "loads a resource from the network"},
	{regexp.MustCompile(`(?i)<link\b[^>]*\bhref\s*=\s*["']?(https?:)?//`), "loads a resource from the network"},
	{regexp.MustCompile(`\bimport\b[^;]*?\bfrom\s*["'](https?:)?//`), "imports a module from the network"},
	{regexp.MustCompile(`\bimport\s*\(\s*["'](https?:)?//`), "imports a module from the network"},
	{regexp.MustCompile(`\bfetch\s*\(\s*["'` + "`" + `](https?:)?//`), "fetches from the network"},
	{regexp.MustCompile(`(?i)\burl\(\s*["']?(https?:)?//`), "loads a resource from the network"},
}

// LintSkills checks a skill tree laid out as <recipe>/<skill>/: every skill
// folder sits in a catalog recipe's directory and has a SKILL.md whose
// frontmatter names it, describes it and requires only catalog recipes or
// types some catalog recipe has, and no file tells an agent to install or fetch anything
// outside a fence marked not-for-this-stick. Files at the top (the authoring
// README) are not skills.
func LintSkills(skills fs.FS, recipeIDs, recipeTypes map[string]bool) []SkillProblem {
	var problems []SkillProblem
	recipes, err := fs.ReadDir(skills, ".")
	if err != nil {
		return []SkillProblem{{Reason: err.Error()}}
	}
	for _, recipe := range recipes {
		if !recipe.IsDir() {
			continue
		}
		folders, err := fs.ReadDir(skills, recipe.Name())
		if err != nil {
			problems = append(problems, SkillProblem{Skill: recipe.Name(), Reason: err.Error()})
			continue
		}
		for _, folder := range folders {
			if !folder.IsDir() {
				continue
			}
			skill := path.Join(recipe.Name(), folder.Name())
			if !recipeIDs[recipe.Name()] {
				problems = append(problems, SkillProblem{Skill: skill, Reason: fmt.Sprintf("folder %s is not a recipe in the catalog", recipe.Name())})
			}
			problems = append(problems, lintSkill(skills, skill, folder.Name(), recipeIDs, recipeTypes)...)
		}
	}
	return problems
}

func lintSkill(skills fs.FS, skill, name string, recipeIDs, recipeTypes map[string]bool) []SkillProblem {
	var problems []SkillProblem
	add := func(file string, line int, reason string) {
		problems = append(problems, SkillProblem{Skill: skill, File: file, Line: line, Reason: reason})
	}
	data, err := fs.ReadFile(skills, path.Join(skill, "SKILL.md"))
	if err != nil {
		add("", 0, "has no SKILL.md")
	} else if meta, err := ParseSkillMeta(data); err != nil {
		add("SKILL.md", 1, err.Error())
	} else {
		switch {
		case meta.Name == "":
			add("SKILL.md", 1, "frontmatter has no name")
		case meta.Name != name:
			add("SKILL.md", 1, fmt.Sprintf("frontmatter name %q is not the folder's name %q", meta.Name, name))
		}
		if strings.TrimSpace(meta.Description) == "" {
			add("SKILL.md", 1, "frontmatter has no description")
		}
		for _, req := range meta.Requires {
			if t, ok := strings.CutPrefix(req, "type:"); ok {
				if !recipeTypes[t] {
					add("SKILL.md", 1, fmt.Sprintf("requires %s, but no catalog recipe has type %q", req, t))
				}
			} else if !recipeIDs[req] {
				add("SKILL.md", 1, fmt.Sprintf("requires %s, which is not in the catalog", req))
			}
		}
	}

	var files []string
	_ = fs.WalkDir(skills, skill, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && lintedExtensions[path.Ext(p)] {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	for _, p := range files {
		data, err := fs.ReadFile(skills, p)
		rel := strings.TrimPrefix(p, skill+"/")
		if err != nil {
			add(rel, 0, err.Error())
			continue
		}
		markdown := path.Ext(p) == ".md"
		fence := "" // the open fence's marker, "" outside one
		fenceLine := 0
		skip := false
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if markdown {
				trimmed := strings.TrimLeft(line, " ")
				if fence == "" {
					if marker := fenceMarker(trimmed); marker != "" {
						fence, fenceLine = marker, i+1
						info := strings.Fields(strings.TrimLeft(trimmed, marker[:1]))
						skip = len(info) > 0 && info[0] == offlineFence
						continue
					}
				} else if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
					fence, skip = "", false
					continue
				}
				if skip {
					continue
				}
			}
			for _, rule := range fetchRules {
				if rule.pattern.MatchString(line) {
					add(rel, i+1, rule.reason+": "+strings.TrimSpace(line))
					break
				}
			}
		}
		if fence != "" {
			add(rel, fenceLine, "code fence is never closed")
		}
	}
	return problems
}

// fenceMarker returns the run of backticks or tildes that opens a fenced code
// block on this line, or "".
func fenceMarker(line string) string {
	for _, c := range []string{"`", "~"} {
		n := len(line) - len(strings.TrimLeft(line, c))
		if n >= 3 {
			return strings.Repeat(c, n)
		}
	}
	return ""
}
