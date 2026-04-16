package search

import (
	"testing"
)

func TestExtractTextReturnsPlainTextFromHTML(t *testing.T) {
	html := `<html><head><title>Test Page</title></head><body>
		<h1>Hello World</h1>
		<p>This is a paragraph.</p>
		<p>Another paragraph with <b>bold</b> text.</p>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if page.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", page.Title)
	}
	if page.Body == "" {
		t.Fatal("expected non-empty body")
	}
	// Should contain plain text, not HTML tags.
	if contains(page.Body, "<p>") || contains(page.Body, "<b>") {
		t.Errorf("body should not contain HTML tags: %q", page.Body)
	}
	if !contains(page.Body, "This is a paragraph.") {
		t.Errorf("body should contain paragraph text, got: %q", page.Body)
	}
	if !contains(page.Body, "bold") {
		t.Errorf("body should contain bold text content, got: %q", page.Body)
	}
}

func TestExtractTextExtractsLinks(t *testing.T) {
	html := `<html><body>
		<a href="/content/wikipedia/Article_One">Article One</a>
		<a href="/content/wiktionary/Word">Some Word</a>
		<a href="Relative_Link">Relative</a>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if len(page.Links) != 3 {
		t.Fatalf("expected 3 links, got %d: %+v", len(page.Links), page.Links)
	}
	// First link: /content/wikipedia/Article_One -> Article_One
	if page.Links[0].Path != "Article_One" {
		t.Errorf("expected path 'Article_One', got %q", page.Links[0].Path)
	}
	if page.Links[0].Label != "Article One" {
		t.Errorf("expected label 'Article One', got %q", page.Links[0].Label)
	}
	// Second link: /content/wiktionary/Word -> Word
	if page.Links[1].Path != "Word" {
		t.Errorf("expected path 'Word', got %q", page.Links[1].Path)
	}
	// Third link: relative path kept as-is
	if page.Links[2].Path != "Relative_Link" {
		t.Errorf("expected path 'Relative_Link', got %q", page.Links[2].Path)
	}
}

func TestExtractTextSkipsExternalLinks(t *testing.T) {
	html := `<html><body>
		<a href="http://example.com">External HTTP</a>
		<a href="https://example.com">External HTTPS</a>
		<a href="//cdn.example.com/file">Protocol-relative</a>
		<a href="/content/wiki/Internal">Internal</a>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if len(page.Links) != 1 {
		t.Fatalf("expected 1 internal link, got %d: %+v", len(page.Links), page.Links)
	}
	if page.Links[0].Path != "Internal" {
		t.Errorf("expected path 'Internal', got %q", page.Links[0].Path)
	}
}

func TestExtractTextStripsKiwixChrome(t *testing.T) {
	html := `<html><body>
		<div id="kiwix_serve_taskbar">Taskbar content</div>
		<div id="kiwix-header">Header content</div>
		<div id="kiwix_searchbar">Search bar</div>
		<div id="content">
			<p>Actual article content.</p>
		</div>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if contains(page.Body, "Taskbar content") {
		t.Error("body should not contain kiwix taskbar content")
	}
	if contains(page.Body, "Header content") {
		t.Error("body should not contain kiwix header content")
	}
	if contains(page.Body, "Search bar") {
		t.Error("body should not contain kiwix searchbar content")
	}
	if !contains(page.Body, "Actual article content.") {
		t.Errorf("body should contain article content, got: %q", page.Body)
	}
}

func TestExtractTextHandlesEmptyHTML(t *testing.T) {
	page, err := ExtractText("")
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if page.Title != "" {
		t.Errorf("expected empty title, got %q", page.Title)
	}
	if page.Body != "" {
		t.Errorf("expected empty body, got %q", page.Body)
	}
	if len(page.Links) != 0 {
		t.Errorf("expected no links, got %d", len(page.Links))
	}
}

func TestExtractTextUsesH1AsFallbackTitle(t *testing.T) {
	html := `<html><body><h1>Main Heading</h1><p>Content here.</p></body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if page.Title != "Main Heading" {
		t.Errorf("expected title 'Main Heading', got %q", page.Title)
	}
}

func TestExtractTextURLDecodesLinks(t *testing.T) {
	html := `<html><body>
		<a href="/content/wiki/Caf%C3%A9_Culture">Café Culture</a>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if len(page.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(page.Links))
	}
	if page.Links[0].Path != "Café_Culture" {
		t.Errorf("expected decoded path 'Café_Culture', got %q", page.Links[0].Path)
	}
}

func TestExtractTextSkipsScriptAndStyleContent(t *testing.T) {
	html := `<html><head>
		<style>body { color: red; }</style>
		<script>alert("hello");</script>
	</head><body>
		<p>Visible content.</p>
		<script>var x = 1;</script>
	</body></html>`

	page, err := ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText error = %v", err)
	}
	if contains(page.Body, "alert") {
		t.Error("body should not contain script content")
	}
	if contains(page.Body, "color: red") {
		t.Error("body should not contain style content")
	}
	if !contains(page.Body, "Visible content.") {
		t.Errorf("body should contain visible content, got: %q", page.Body)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
