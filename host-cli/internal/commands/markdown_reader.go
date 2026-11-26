package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// overviewLabel names the part of a markdown file that comes before its first
// "##" heading.
const overviewLabel = "Overview"

// byteOrderMark starts some files written on Windows.
const byteOrderMark = string(rune(0xFEFF))

// markdownReader reads markdown: a skill's SKILL.md and the files beside it,
// and the quickstart of a JS library. A file becomes one part per "##"
// section, so a search hit is a section and a reader can ask for one.
type markdownReader struct{}

func openMarkdownReader() (documentReader, error) { return markdownReader{}, nil }

func (markdownReader) Close() error { return nil }

func (markdownReader) Read(ctx context.Context, file string) (documentText, error) {
	if err := ctx.Err(); err != nil {
		return documentText{}, err
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return documentText{Note: "unreadable: " + unwrapPathError(err)}, nil
	}
	content := strings.ToValidUTF8(string(raw), string(utf8.RuneError))
	content = strings.ReplaceAll(strings.TrimPrefix(content, byteOrderMark), "\r\n", "\n")
	front, body := splitFrontmatter(content)
	name, description := frontmatterNameAndDescription(front)

	// The text before the first "##" is the overview; each "##" starts a
	// section that runs to the next one, deeper headings included.
	type section struct {
		label string
		lines []string
	}
	sections := []section{{label: overviewLabel}}
	heading := ""
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		if fence != "" {
			if closesFence(line, fence) {
				fence = ""
			}
		} else if fence = opensFence(line); fence == "" {
			if rest, ok := strings.CutPrefix(line, "## "); ok {
				label := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rest), "#"))
				if label == "" {
					label = "Section"
				}
				sections = append(sections, section{label: label})
			} else if rest, ok := strings.CutPrefix(line, "# "); ok && heading == "" {
				heading = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rest), "#"))
			}
		}
		last := &sections[len(sections)-1]
		last.lines = append(last.lines, line)
	}

	text := documentText{Title: firstNonEmpty(name, heading, fileTitle(file))}
	for i, section := range sections {
		body := strings.TrimSpace(strings.Join(section.lines, "\n"))
		if i == 0 && description != "" {
			body = strings.TrimSpace(description + "\n\n" + body)
		}
		for j, piece := range splitAtLines(body, maxDocumentPartChars) {
			label := section.label
			if j > 0 {
				label += " (cont.)"
			}
			text.Parts = append(text.Parts, documentPart{
				Path:  fmt.Sprintf("section/%d", len(text.Parts)+1),
				Label: label,
				Text:  piece,
			})
		}
	}
	return text, nil
}

// splitFrontmatter separates a leading "---" block from the text after it. A
// block that is never closed is not one.
func splitFrontmatter(content string) (front, body string) {
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		return "", content
	}
	for offset := 0; offset < len(rest); {
		end := strings.IndexByte(rest[offset:], '\n')
		line := rest[offset:]
		next := len(rest)
		if end >= 0 {
			line, next = rest[offset:offset+end], offset+end+1
		}
		if marker := strings.TrimRight(line, " \t"); marker == "---" || marker == "..." {
			return rest[:offset], rest[next:]
		}
		offset = next
	}
	return "", content
}

// frontmatterNameAndDescription reads the two fields the index uses. Skills
// in the wild often write a description with a colon in it, unquoted, which
// YAML refuses; those are read line by line.
func frontmatterNameAndDescription(front string) (name, description string) {
	var fields struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(front), &fields); err == nil {
		return strings.TrimSpace(fields.Name), strings.TrimSpace(fields.Description)
	}
	for _, line := range strings.Split(front, "\n") {
		if value, ok := strings.CutPrefix(line, "name:"); ok && name == "" {
			name = strings.Trim(strings.TrimSpace(value), `"'`)
		} else if value, ok := strings.CutPrefix(line, "description:"); ok && description == "" {
			description = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return name, description
}

// opensFence returns the marker of the code fence a line opens, or "".
func opensFence(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	for _, char := range []string{"`", "~"} {
		if run := len(trimmed) - len(strings.TrimLeft(trimmed, char)); run >= 3 {
			return trimmed[:run]
		}
	}
	return ""
}

// closesFence reports whether a line ends the fence opened with marker: the
// same character, at least as many of them, and nothing after.
func closesFence(line, marker string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) >= len(marker) && strings.Trim(trimmed, marker[:1]) == ""
}

// fileTitle names a file that states no title: its name without extension, or
// for a SKILL.md, which every skill has, the skill's folder.
func fileTitle(file string) string {
	base := filepath.Base(file)
	if strings.EqualFold(base, "SKILL.md") {
		return filepath.Base(filepath.Dir(file))
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// splitAtLines cuts text into pieces of at most limit characters, at a line
// break when one falls in the second half of a piece and at a space
// otherwise, so that no piece is cut again when it is stored. Empty text has
// no pieces.
func splitAtLines(text string, limit int) []string {
	var pieces []string
	runes := []rune(text)
	for len(runes) > limit {
		cut := limit
		for _, separator := range []rune{'\n', ' '} {
			at := limit
			for at > limit/2 && runes[at-1] != separator {
				at--
			}
			if at > limit/2 {
				cut = at
				break
			}
		}
		if piece := strings.TrimSpace(string(runes[:cut])); piece != "" {
			pieces = append(pieces, piece)
		}
		runes = runes[cut:]
	}
	if piece := strings.TrimSpace(string(runes)); piece != "" {
		pieces = append(pieces, piece)
	}
	return pieces
}
