package search_test

import (
	"bytes"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func TestBuildFTSQueryPrefixesTermsForWildcardSearch(t *testing.T) {
	got := search.BuildFTSQuery(`hello world`)
	want := `"hello"* "world"*`
	if got != want {
		t.Fatalf("BuildFTSQuery() = %q, want %q", got, want)
	}
}

func TestBestModeReturnsHybridWhenCapabilitiesAreAvailable(t *testing.T) {
	got := search.BestMode(search.Capabilities{
		HasEmbeddings:    true,
		HasEmbeddingData: true,
		HasLlamaServer:   true,
		EmbeddingModel:   "/tmp/model.gguf",
	})
	if want := search.ModeHybrid; got != want {
		t.Fatalf("BestMode() = %q, want %q", got, want)
	}
}

func TestBestModeReturnsKeywordWhenCapabilitiesAreMissing(t *testing.T) {
	got := search.BestMode(search.Capabilities{
		HasEmbeddings:    true,
		HasEmbeddingData: true,
		HasLlamaServer:   false,
		EmbeddingModel:   "",
	})
	if want := search.ModeKeyword; got != want {
		t.Fatalf("BestMode() = %q, want %q", got, want)
	}
}

func TestRenderResultsIncludesNumberedEntries(t *testing.T) {
	var out bytes.Buffer
	search.RenderResults(&out, []search.Result{
		{Filename: "wiki.zim", Title: "Solar power", Snippet: "Overview"},
	})
	text := out.String()
	if text == "" || !bytes.Contains(out.Bytes(), []byte("1. [wiki] Solar power")) {
		t.Fatalf("RenderResults() output = %q", text)
	}
}
