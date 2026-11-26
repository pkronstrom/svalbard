package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
