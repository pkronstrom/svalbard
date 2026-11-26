package zimext

import (
	"path"
	"regexp"
	"strings"
	"unicode"
)

// nautilusCatalogue is the entry where an openZIM nautilus ZIM lists its
// files with their titles and descriptions.
const nautilusCatalogue = "database.js"

var (
	// The catalogue is a Python repr, not JSON: one record per line, values
	// in single quotes. Each field is matched on its own.
	nautilusRecordRe = regexp.MustCompile(`\{[^{}]*\}`)
	nautilusTitleRe  = regexp.MustCompile(`'ti':\s*'((?:[^'\\]|\\.)*)'`)
	nautilusDescRe   = regexp.MustCompile(`'dsc':\s*'((?:[^'\\]|\\.)*)'`)
	nautilusFilesRe  = regexp.MustCompile(`'fp':\s*\[([^\]]*)\]`)
	nautilusFileRe   = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)
	// A title that only numbers a part, or names a section any book has,
	// says nothing on its own.
	genericTitleRe = regexp.MustCompile(`(?i)^((part|chapter|vol(ume)?|section|book)\s*[\dIVX.]+|appendix(\s+[A-Z\d]+)?|introduction|preface|foreword|contents|index|glossary)$`)
)

// parseNautilusTitles reads a nautilus catalogue into titles by file name. A
// title that only numbers a part ("Part 2") is completed with the archive's
// title and the description: "USDA Guide to Home Canning, Part 2: Fruit and
// Fruit Products". A catalogue that cannot be read gives no titles.
func parseNautilusTitles(js, archiveTitle string) map[string]string {
	titles := map[string]string{}
	if !strings.Contains(js, "DATABASE") {
		return titles
	}
	for _, record := range nautilusRecordRe.FindAllString(js, -1) {
		titleMatch := nautilusTitleRe.FindStringSubmatch(record)
		filesMatch := nautilusFilesRe.FindStringSubmatch(record)
		if titleMatch == nil || filesMatch == nil {
			continue
		}
		title := readableTitle(unescapeQuoted(titleMatch[1]))
		if title == "" {
			continue
		}
		if genericTitleRe.MatchString(title) {
			if m := nautilusDescRe.FindStringSubmatch(record); m != nil && strings.TrimSpace(m[1]) != "" {
				title += ": " + strings.TrimSpace(unescapeQuoted(m[1]))
			}
			if archiveTitle != "" {
				title = archiveTitle + ", " + title
			}
		}
		for _, file := range nautilusFileRe.FindAllStringSubmatch(filesMatch[1], -1) {
			titles[path.Base(unescapeQuoted(file[1]))] = title
		}
	}
	return titles
}

// readableTitle turns a catalogue title that is a file-name slug
// ("Wilderness-Survival-FM-2176") into words, and drops one without a letter
// in it ("_173165"), which names nothing.
func readableTitle(title string) string {
	title = strings.TrimSpace(title)
	if !strings.ContainsFunc(title, unicode.IsLetter) {
		return ""
	}
	if !strings.Contains(title, " ") {
		title = strings.Join(strings.FieldsFunc(title, func(r rune) bool { return r == '-' || r == '_' }), " ")
	}
	return title
}

func unescapeQuoted(s string) string {
	return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(s)
}
