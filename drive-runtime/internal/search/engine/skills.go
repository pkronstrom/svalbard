package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// SectionPath is the article path of a markdown source's section.
func SectionPath(number int) string { return "section/" + strconv.Itoa(number) }

// SkillSource is an indexed skill file or library quickstart.
type SkillSource struct {
	Filename string
	// Title is the name the file gives itself: a skill's frontmatter name.
	Title string
}

// SkillSources lists the indexed skill files and library quickstarts.
func (e *Engine) SkillSources(ctx context.Context) ([]SkillSource, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT src.filename, src.title FROM sources src WHERE `+skillRoots+` ORDER BY src.filename`)
	if err != nil {
		return nil, fmt.Errorf("engine: skill sources: %w", err)
	}
	defer rows.Close()
	var sources []SkillSource
	for rows.Next() {
		var source SkillSource
		if err := rows.Scan(&source.Filename, &source.Title); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

// Section is one indexed section of a markdown source.
type Section struct {
	Number int
	// Title is the section's article title: the file's title and the heading.
	Title string
	// Text keeps the section's lines, its heading among them.
	Text string
}

// sectionOrder sorts a markdown source's articles by section number.
const sectionOrder = `CAST(substr(a.path, 9) AS INTEGER)`

// Sections returns the sections of a skill file or library quickstart in the
// order of the file. Joined, they are the file without its frontmatter.
func (e *Engine) Sections(ctx context.Context, filename string) ([]Section, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT `+sectionOrder+`, a.title, a.body, a.sections FROM articles a
         JOIN sources src ON src.id = a.source_id
         WHERE src.filename = ? AND a.path LIKE 'section/%'
         ORDER BY `+sectionOrder, filename)
	if err != nil {
		return nil, fmt.Errorf("engine: sections: %w", err)
	}
	defer rows.Close()
	var sections []Section
	for rows.Next() {
		var section Section
		var body, lines sql.NullString
		if err := rows.Scan(&section.Number, &section.Title, &body, &lines); err != nil {
			return nil, err
		}
		section.Text = body.String
		// The indexed body is one line for keyword search; the section's own
		// lines are kept beside it.
		var parts []struct {
			Body string `json:"body"`
		}
		if lines.Valid && json.Unmarshal([]byte(lines.String), &parts) == nil && len(parts) > 0 && strings.TrimSpace(parts[0].Body) != "" {
			section.Text = parts[0].Body
		}
		sections = append(sections, section)
	}
	return sections, rows.Err()
}

// SkillFolders returns the folders that hold a SKILL.md: the skills.
func (e *Engine) SkillFolders(ctx context.Context) (map[string]bool, error) {
	sources, err := e.SkillSources(ctx)
	if err != nil {
		return nil, err
	}
	folders := map[string]bool{}
	for _, source := range sources {
		if strings.HasPrefix(source.Filename, SkillsDir+"/") && strings.EqualFold(path.Base(source.Filename), "SKILL.md") {
			folders[path.Dir(source.Filename)] = true
		}
	}
	return folders, nil
}

// SkillGroup names what an indexed skill file belongs to: the nearest folder
// above it that holds a SKILL.md, which is the skill. A library quickstart,
// and markdown under skills/ that no SKILL.md is above, stand for themselves.
func SkillGroup(filename string, folders map[string]bool) string {
	if strings.HasPrefix(filename, SkillsDir+"/") {
		for folder := path.Dir(filename); folder != SkillsDir && folder != "." && folder != "/"; folder = path.Dir(folder) {
			if folders[folder] {
				return folder
			}
		}
	}
	return filename
}

// skillBound keeps a search that covers several skills to sectionsPerSkill
// sections of each, as a search over documents keeps to a few pages of each.
// Every section of a skill's files carries the skill's name in its title, so
// a long skill matches with all of them, and skill search groups what it is
// given into one result per skill: without the bound the skills behind a long
// one are never reached. A nil bound bounds nothing.
type skillBound struct {
	scope   string
	folders map[string]bool
	seen    map[string]int
}

// newSkillBound returns the bound for a search in scope, nil when the scope
// cannot hold more than one skill's files.
func (e *Engine) newSkillBound(ctx context.Context, scope string) *skillBound {
	if scope != SkillScope && !(strings.HasSuffix(scope, "/") && IsSkillSource(scope)) {
		return nil
	}
	// Loaded before the search's own rows are open: both may have to share
	// one connection.
	folders, _ := e.SkillFolders(ctx)
	return &skillBound{scope: scope, folders: folders, seen: map[string]int{}}
}

// over counts a result row and reports whether it is beyond the bound. more
// is set for the first such row of a skill: the scope that returns the rest.
func (b *skillBound) over(filename string) (skip bool, more string) {
	if b == nil || !IsSkillSource(filename) {
		return false, ""
	}
	group := SkillGroup(filename, b.folders)
	if strings.HasPrefix(b.scope, group+"/") {
		return false, "" // the search is limited to this one skill
	}
	if b.seen[group]++; b.seen[group] <= sectionsPerSkill {
		return false, ""
	}
	if b.seen[group] == sectionsPerSkill+1 {
		if more = group; b.folders[group] {
			more += "/"
		}
	}
	return true, more
}
