// Package zimext provides utilities for processing article text extracted from
// ZIM files, including HTML stripping and text truncation for search indexing.
package zimext

import (
	"regexp"
	"strings"
)

var (
	scriptRe  = regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
	styleRe   = regexp.MustCompile(`(?si)<style[^>]*>.*?</style>`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	wsRe      = regexp.MustCompile(`[\s]+`)
	headingRe = regexp.MustCompile(`(?i)<h[23][^>]*>(.*?)</h[23]>`)
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

// Section represents a heading-delimited section of an HTML document.
type Section struct {
	Heading string `json:"heading"` // "" for intro/lead content before first heading
	Body    string `json:"body"`
}

// ExtractSections splits raw HTML content on <h2> and <h3> heading boundaries,
// returning structured sections. Content before the first heading becomes a
// section with an empty Heading. Sections with empty bodies after stripping
// HTML are omitted.
func ExtractSections(htmlContent string) []Section {
	if strings.TrimSpace(htmlContent) == "" {
		return nil
	}

	matches := headingRe.FindAllStringSubmatchIndex(htmlContent, -1)

	// No headings found — return the whole content as a single section.
	if len(matches) == 0 {
		body := StripHTML(htmlContent)
		if body == "" {
			return nil
		}
		return []Section{{Heading: "", Body: body}}
	}

	var sections []Section

	// Content before the first heading (intro/lead).
	if matches[0][0] > 0 {
		intro := StripHTML(htmlContent[:matches[0][0]])
		if intro != "" {
			sections = append(sections, Section{Heading: "", Body: intro})
		}
	}

	for i, m := range matches {
		// m[0]:m[1] is the full <h2>...</h2> match
		// m[2]:m[3] is the capture group (heading inner HTML)
		headingHTML := htmlContent[m[2]:m[3]]
		heading := StripHTML(headingHTML)

		// Body runs from after the closing tag to the start of the next heading
		// (or end of content).
		bodyStart := m[1]
		var bodyEnd int
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		} else {
			bodyEnd = len(htmlContent)
		}

		body := StripHTML(htmlContent[bodyStart:bodyEnd])
		if body == "" {
			continue
		}
		sections = append(sections, Section{Heading: heading, Body: body})
	}

	if len(sections) == 0 {
		return nil
	}
	return sections
}
