package commands

import (
	"context"
	"strings"
	"testing"
)

func readMarkdown(t *testing.T, rel, body string) documentText {
	t.Helper()
	reader, err := openMarkdownReader()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	text, err := reader.Read(context.Background(), writeVaultFile(t, t.TempDir(), rel, body))
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func partPaths(text documentText) []string {
	var paths []string
	for _, part := range text.Parts {
		paths = append(paths, part.Path+" "+part.Label)
	}
	return paths
}

func TestMarkdownReaderSplitsOnSecondLevelHeadings(t *testing.T) {
	text := readMarkdown(t, "skills/skills-core/data-dashboard/SKILL.md", `---
name: data-dashboard
description: Charts from CSV
requires: [js-core]
---
# Data dashboard

Use this for charts.

## Steps

1. Read the file.

### A detail

Still part of the steps.

## Check

Open the page.
`)
	if text.Title != "data-dashboard" || text.Note != "" {
		t.Fatalf("title = %q, note = %q", text.Title, text.Note)
	}
	want := []string{"section/1 Overview", "section/2 Steps", "section/3 Check"}
	if got := partPaths(text); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parts = %q, want %q", got, want)
	}
	if !strings.HasPrefix(text.Parts[0].Text, "Charts from CSV") || !strings.Contains(text.Parts[0].Text, "Use this for charts.") {
		t.Errorf("overview = %q", text.Parts[0].Text)
	}
	if strings.Contains(text.Parts[0].Text, "requires:") {
		t.Errorf("the frontmatter block is in the overview: %q", text.Parts[0].Text)
	}
	if steps := text.Parts[1].Text; !strings.HasPrefix(steps, "## Steps") || !strings.Contains(steps, "Still part of the steps.") || strings.Contains(steps, "Open the page") {
		t.Errorf("steps = %q", steps)
	}
}

func TestMarkdownReaderWithoutFrontmatterOrHeadings(t *testing.T) { // RF1
	text := readMarkdown(t, "skills/x/notes.md", "Plain notes.\nNo headings here.\n")
	if text.Title != "notes" || text.Note != "" {
		t.Fatalf("title = %q, note = %q", text.Title, text.Note)
	}
	if got := partPaths(text); len(got) != 1 || got[0] != "section/1 Overview" {
		t.Fatalf("parts = %q", got)
	}
	if text.Parts[0].Text != "Plain notes.\nNo headings here." {
		t.Errorf("text = %q", text.Parts[0].Text)
	}

	// Frontmatter but no headings is one section too, and a SKILL.md that
	// names nothing is called by its folder: every skill's file is SKILL.md.
	text = readMarkdown(t, "skills/x/gear-maker/SKILL.md", "---\ndescription: Makes gears\n---\nDo the thing.\n")
	if text.Title != "gear-maker" || len(text.Parts) != 1 || !strings.HasPrefix(text.Parts[0].Text, "Makes gears") {
		t.Fatalf("frontmatter only: %+v", text)
	}
	// The first top-level heading names a file that has no frontmatter name.
	if text = readMarkdown(t, "lib/js/js-core/docs/chartjs.md", "# Chart.js quickstart\n\nImport it.\n"); text.Title != "Chart.js quickstart" {
		t.Fatalf("title from heading = %q", text.Title)
	}
	// A description with a colon is not valid YAML; skills in the wild have them.
	text = readMarkdown(t, "skills/x/y/SKILL.md", "---\nname: tdd\ndescription: Use when: writing tests\n---\nBody.\n")
	if text.Title != "tdd" || !strings.HasPrefix(text.Parts[0].Text, "Use when: writing tests") {
		t.Fatalf("loose frontmatter: %+v", text)
	}
	// Nothing to index is no part and no note.
	if text = readMarkdown(t, "skills/x/empty.md", "\n\n"); len(text.Parts) != 0 || text.Note != "" {
		t.Fatalf("empty file: %+v", text)
	}
}

func TestMarkdownReaderKeepsCodeFencesIntact(t *testing.T) {
	text := readMarkdown(t, "skills/x/a/SKILL.md", "# A\n\n## Steps\n\n```md\n## not a heading\n```\n\n~~~~\n```\n## nor this\n~~~~\n\n## Check\n\nDone.\n")
	want := []string{"section/1 Overview", "section/2 Steps", "section/3 Check"}
	if got := partPaths(text); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parts = %q, want %q", got, want)
	}
	if !strings.Contains(text.Parts[1].Text, "## not a heading") || !strings.Contains(text.Parts[1].Text, "## nor this") {
		t.Errorf("steps lost its fenced lines: %q", text.Parts[1].Text)
	}
}

func TestMarkdownReaderSplitsLongSections(t *testing.T) {
	long := strings.Repeat("A line of forty characters, give or take\n", 1000) // about 41,000 characters
	text := readMarkdown(t, "skills/x/a/SKILL.md", "## Steps\n\n"+long+"\n## Check\n\nDone.\n")
	want := []string{"section/1 Steps", "section/2 Steps (cont.)", "section/3 Steps (cont.)", "section/4 Check"}
	if got := partPaths(text); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("parts = %q, want %q", got, want)
	}
	lines := 0
	for _, part := range text.Parts[:3] {
		if n := len([]rune(part.Text)); n > maxDocumentPartChars {
			t.Errorf("part %s has %d characters, over the cap", part.Path, n)
		}
		lines += strings.Count(part.Text, "A line of forty characters, give or take")
	}
	if lines != 1000 {
		t.Errorf("the parts hold %d of the 1000 lines", lines)
	}
}
