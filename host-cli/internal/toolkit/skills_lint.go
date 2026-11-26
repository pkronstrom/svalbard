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
var lintedExtensions = map[string]bool{".md": true, ".html": true, ".py": true, ".sh": true, ".js": true}

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
	{regexp.MustCompile(`\bgit\s+clone\b`), "clones from the network"},
	{regexp.MustCompile(`(?i)\bsrc\s*=\s*["']?(https?:)?//`), "loads a resource from the network"},
	{regexp.MustCompile(`(?i)<link\b[^>]*\bhref\s*=\s*["']?(https?:)?//`), "loads a resource from the network"},
	{regexp.MustCompile(`\bimport\b[^;]*?\bfrom\s*["'](https?:)?//`), "imports a module from the network"},
	{regexp.MustCompile(`\bimport\s*\(\s*["'](https?:)?//`), "imports a module from the network"},
	{regexp.MustCompile(`\bfetch\s*\(\s*["'](https?:)?//`), "fetches from the network"},
	{regexp.MustCompile(`(?i)\burl\(\s*["']?(https?:)?//`), "loads a resource from the network"},
}

// LintSkills checks a skill tree laid out as <recipe>/<skill>/: every skill
// folder sits in a catalog recipe's directory and has a SKILL.md whose
// frontmatter names it, describes it and requires only catalog recipes or
// known types, and no file tells an agent to install or fetch anything
// outside a fence marked not-for-this-stick. Files at the top (the authoring
// README) are not skills.
func LintSkills(skills fs.FS, recipeIDs map[string]bool) []SkillProblem {
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
			problems = append(problems, lintSkill(skills, skill, folder.Name(), recipeIDs)...)
		}
	}
	return problems
}

func lintSkill(skills fs.FS, skill, name string, recipeIDs map[string]bool) []SkillProblem {
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
				if _, known := TypeDirs[t]; !known {
					add("SKILL.md", 1, fmt.Sprintf("requires %s, but no recipe type is %q", req, t))
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
		skip := false
		for i, line := range strings.Split(string(data), "\n") {
			if markdown {
				trimmed := strings.TrimLeft(line, " ")
				if fence == "" {
					if marker := fenceMarker(trimmed); marker != "" {
						fence = marker
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
