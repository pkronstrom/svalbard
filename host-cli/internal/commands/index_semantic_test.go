package commands

import (
	"strings"
	"testing"
)

func TestPrepareEmbeddingTextTruncatesLongBody(t *testing.T) {
	body := strings.Repeat("token ", 800)

	// With prefix (nomic-style).
	text := prepareEmbeddingText("search_document: ", "Permacomputing", body)
	if !strings.HasPrefix(text, "search_document: ") {
		t.Fatalf("missing search_document prefix: %q", text[:min(len(text), 32)])
	}
	if !strings.Contains(text, "Permacomputing") {
		t.Fatalf("title missing from prepared text: %q", text)
	}
	if got := len([]rune(text)); got > embeddingTextMaxRunes {
		t.Fatalf("prepared text length = %d, want <= %d", got, embeddingTextMaxRunes)
	}
	if !strings.HasSuffix(text, "...") {
		t.Fatalf("expected truncated text to end with ellipsis: %q", text[len(text)-16:])
	}

	// Without prefix (MiniLM-style).
	textNoPrefix := prepareEmbeddingText("", "Permacomputing", body)
	if strings.HasPrefix(textNoPrefix, "search_document: ") {
		t.Fatalf("should not have prefix when empty: %q", textNoPrefix[:min(len(textNoPrefix), 32)])
	}
	if !strings.HasPrefix(textNoPrefix, "Permacomputing") {
		t.Fatalf("title should be first: %q", textNoPrefix[:min(len(textNoPrefix), 32)])
	}
}

func TestPrepareEmbeddingTextCapsWordCount(t *testing.T) {
	body := strings.Repeat("a ", 1200)

	// With prefix.
	text := prepareEmbeddingText("search_document: ", "Short Tokens", body)
	maxWords := embeddingTextMaxWords + len(strings.Fields("search_document: ")) + len(strings.Fields("Short Tokens"))
	if got := len(strings.Fields(text)); got > maxWords {
		t.Fatalf("prepared text word count = %d, want <= %d", got, maxWords)
	}

	// Without prefix.
	textNoPrefix := prepareEmbeddingText("", "Short Tokens", body)
	maxWordsNoPrefix := embeddingTextMaxWords + len(strings.Fields("Short Tokens"))
	if got := len(strings.Fields(textNoPrefix)); got > maxWordsNoPrefix {
		t.Fatalf("no-prefix word count = %d, want <= %d", got, maxWordsNoPrefix)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
