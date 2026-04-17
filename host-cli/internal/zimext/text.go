// Package zimext provides utilities for processing article text extracted from
// ZIM files, including HTML stripping and text truncation for search indexing.
package zimext

import (
	"regexp"
	"strings"
)

var (
	scriptRe = regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	wsRe     = regexp.MustCompile(`[\s]+`)
)

var entityReplacer = strings.NewReplacer(
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
	"&nbsp;", " ",
)

// StripHTML removes HTML tags, decodes common HTML entities, and normalizes
// whitespace. It returns clean plain text suitable for indexing.
func StripHTML(raw string) string {
	if raw == "" {
		return ""
	}

	// Remove script and style blocks before stripping tags.
	s := scriptRe.ReplaceAllString(raw, " ")
	s = styleRe.ReplaceAllString(s, " ")

	// Remove all HTML tags (including multi-line).
	s = tagRe.ReplaceAllString(s, " ")

	// Decode common HTML entities.
	s = entityReplacer.Replace(s)

	// Collapse multiple whitespace characters into a single space.
	s = wsRe.ReplaceAllString(s, " ")

	// Trim leading and trailing whitespace.
	return strings.TrimSpace(s)
}

// TruncateText truncates text at a natural boundary (sentence or word) without
// exceeding maxChars runes. If the text is already short enough it is returned
// as-is. This correctly handles multi-byte characters (e.g. CJK, emoji).
func TruncateText(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}

	window := string(runes[:maxChars])

	// Try to find last sentence boundary (". ") within the window.
	if idx := strings.LastIndex(window, ". "); idx > 0 && idx > len(window)/2 {
		return window[:idx+1] // include the period
	}

	// Try to find last word boundary (space) within the window.
	if idx := strings.LastIndex(window, " "); idx > 0 {
		return window[:idx]
	}

	// Hard truncate.
	return window
}
