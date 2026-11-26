package zimext

import (
	"path"
	"regexp"
	"strings"
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
	// A title that only numbers a part says nothing on its own.
	genericTitleRe = regexp.MustCompile(`(?i)^(part|chapter|vol(ume)?|section|book)\s*[\dIVX.]+$`)
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
		title := strings.TrimSpace(unescapeQuoted(titleMatch[1]))
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

func unescapeQuoted(s string) string {
	return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(s)
}
