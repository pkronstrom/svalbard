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
