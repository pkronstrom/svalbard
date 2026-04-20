package zimext

import (
	"strings"
	"testing"
)

func TestStripHTMLRemovesTags(t *testing.T) {
	got := StripHTML("<p>Hello <b>world</b></p>")
	if got != "Hello world" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLDecodesEntities(t *testing.T) {
	got := StripHTML("Tom &amp; Jerry &lt;3&gt;")
	if got != "Tom & Jerry <3>" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLCollapsesWhitespace(t *testing.T) {
	got := StripHTML("<p>hello</p>\n\n<p>world</p>")
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesEmptyInput(t *testing.T) {
	got := StripHTML("")
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesNbsp(t *testing.T) {
	got := StripHTML("hello&nbsp;world")
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesQuotEntity(t *testing.T) {
	got := StripHTML("&quot;quoted&quot;")
	if got != `"quoted"` {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesApostropheEntity(t *testing.T) {
	got := StripHTML("it&#39;s")
	if got != "it's" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLMultilineTag(t *testing.T) {
	got := StripHTML("<div\nclass=\"foo\"\n>content</div>")
	if got != "content" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateTextShortInput(t *testing.T) {
	got := TruncateText("short", 100)
	if got != "short" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateTextAtSentenceBoundary(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence is longer."
	got := TruncateText(text, 35)
	// Should truncate at "Second sentence." boundary
	if !strings.HasSuffix(got, ".") {
		t.Errorf("should end at sentence: %q", got)
	}
	if len(got) > 35 {
		t.Errorf("too long: %d chars", len(got))
	}
}

func TestTruncateTextAtWordBoundary(t *testing.T) {
	text := "one two three four five six seven eight nine ten"
	got := TruncateText(text, 20)
	if len(got) > 20 {
		t.Errorf("too long: %d chars", len(got))
	}
	// Should not cut mid-word
	if strings.HasSuffix(got, "th") || strings.HasSuffix(got, "hre") {
		t.Errorf("cut mid-word: %q", got)
	}
}

func TestTruncateTextHardCutWhenNoBreak(t *testing.T) {
	text := "abcdefghijklmnopqrstuvwxyz"
	got := TruncateText(text, 10)
	if len([]rune(got)) != 10 {
		t.Errorf("expected 10 runes, got %d: %q", len([]rune(got)), got)
	}
}

func TestTruncateTextHardCutMultiByte(t *testing.T) {
	// Each character is a 3-byte CJK rune; truncation should count runes, not bytes.
	text := "日本語テスト文字列です"
	got := TruncateText(text, 5)
	runes := []rune(got)
	if len(runes) != 5 {
		t.Errorf("expected 5 runes, got %d: %q", len(runes), got)
	}
	if got != "日本語テス" {
		t.Errorf("expected %q, got %q", "日本語テス", got)
	}
}

func TestTruncateTextExactLength(t *testing.T) {
	text := "exact"
	got := TruncateText(text, 5)
	if got != "exact" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateTextSentenceBoundaryTooEarly(t *testing.T) {
	// Sentence boundary is at position 2 ("A."), which is < maxChars/2 (10)
	// Should fall through to word boundary instead
	text := "A. this is a longer piece of text here"
	got := TruncateText(text, 20)
	if len(got) > 20 {
		t.Errorf("too long: %d chars", len(got))
	}
}

// --- ExtractSections tests ---

func TestExtractSectionsMultipleHeadings(t *testing.T) {
	html := `<p>Intro paragraph.</p>
<h2>History</h2><p>Some history content here.</p>
<h3>Early years</h3><p>Early years detail.</p>
<h2>Geography</h2><p>Geography content.</p>`

	sections := ExtractSections(html)
	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d: %+v", len(sections), sections)
	}

	if sections[0].Heading != "" {
		t.Errorf("intro heading should be empty, got %q", sections[0].Heading)
	}
	if !strings.Contains(sections[0].Body, "Intro paragraph") {
		t.Errorf("intro body missing expected text: %q", sections[0].Body)
	}

	if sections[1].Heading != "History" {
		t.Errorf("expected heading %q, got %q", "History", sections[1].Heading)
	}
	if !strings.Contains(sections[1].Body, "history content") {
		t.Errorf("History body missing expected text: %q", sections[1].Body)
	}

	if sections[2].Heading != "Early years" {
		t.Errorf("expected heading %q, got %q", "Early years", sections[2].Heading)
	}

	if sections[3].Heading != "Geography" {
		t.Errorf("expected heading %q, got %q", "Geography", sections[3].Heading)
	}
}

func TestExtractSectionsNoHeadings(t *testing.T) {
	html := `<p>Just a paragraph with no headings at all.</p>`
	sections := ExtractSections(html)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Heading != "" {
		t.Errorf("heading should be empty, got %q", sections[0].Heading)
	}
	if !strings.Contains(sections[0].Body, "no headings") {
		t.Errorf("body missing expected text: %q", sections[0].Body)
	}
}

func TestExtractSectionsEmptyHTML(t *testing.T) {
	if sections := ExtractSections(""); sections != nil {
		t.Errorf("expected nil, got %+v", sections)
	}
	if sections := ExtractSections("   "); sections != nil {
		t.Errorf("expected nil for whitespace, got %+v", sections)
	}
}

func TestExtractSectionsSkipsEmptyBodies(t *testing.T) {
	html := `<h2>First</h2><h2>Second</h2><p>Some content.</p>`
	sections := ExtractSections(html)
	// "First" has no body content (immediately followed by another heading), so it should be skipped.
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (empty body skipped), got %d: %+v", len(sections), sections)
	}
	if sections[0].Heading != "Second" {
		t.Errorf("expected heading %q, got %q", "Second", sections[0].Heading)
	}
}

func TestExtractSectionsNestedTagsInHeading(t *testing.T) {
	html := `<h2><span class="mw-headline">Important Title</span></h2><p>Body text.</p>`
	sections := ExtractSections(html)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Heading != "Important Title" {
		t.Errorf("expected heading %q, got %q", "Important Title", sections[0].Heading)
	}
}

func TestExtractSectionsIgnoresH1AndH4(t *testing.T) {
	html := `<h1>Title</h1><p>Intro.</p><h4>Minor heading</h4><p>Detail.</p><h2>Real Section</h2><p>Content.</p>`
	sections := ExtractSections(html)
	// h1 and h4 should NOT cause splits. Only the h2 should.
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d: %+v", len(sections), sections)
	}
	// First section is everything before the h2 (intro + h4 stuff treated as body text).
	if sections[0].Heading != "" {
		t.Errorf("first section heading should be empty, got %q", sections[0].Heading)
	}
	if !strings.Contains(sections[0].Body, "Intro") {
		t.Errorf("first section should contain intro: %q", sections[0].Body)
	}
	if !strings.Contains(sections[0].Body, "Minor heading") {
		t.Errorf("first section should contain h4 text as body: %q", sections[0].Body)
	}
	if sections[1].Heading != "Real Section" {
		t.Errorf("expected heading %q, got %q", "Real Section", sections[1].Heading)
	}
}

func TestExtractSectionsHeadingWithAttributes(t *testing.T) {
	html := `<h2 id="foo" class="bar">Styled Heading</h2><p>Content here.</p>`
	sections := ExtractSections(html)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Heading != "Styled Heading" {
		t.Errorf("expected heading %q, got %q", "Styled Heading", sections[0].Heading)
	}
}

func TestExtractSectionsPreservesParagraphBreaks(t *testing.T) {
	html := `<h2>Section</h2>
<p>First paragraph with content.</p>
<p>Second paragraph with more content.</p>
<p>Third paragraph ending the section.</p>`

	sections := ExtractSections(html)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if !strings.Contains(sections[0].Body, "\n\n") {
		t.Errorf("section body should preserve paragraph breaks, got: %q", sections[0].Body)
	}
	paragraphs := strings.Split(sections[0].Body, "\n\n")
	if len(paragraphs) < 3 {
		t.Errorf("expected at least 3 paragraphs, got %d: %q", len(paragraphs), sections[0].Body)
	}
}

func TestExtractSectionsOnlyTagsNoText(t *testing.T) {
	html := `<div><span></span></div>`
	sections := ExtractSections(html)
	if sections != nil {
		t.Errorf("expected nil for HTML with no text content, got %+v", sections)
	}
}
