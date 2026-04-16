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

func TestDecodeVectorHexParsesLittleEndianFloat32(t *testing.T) {
	got, err := search.DecodeVectorHex("0000803F00000040")
	if err != nil {
		t.Fatalf("DecodeVectorHex() error = %v", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("DecodeVectorHex() = %#v, want [1 2]", got)
	}
}

func TestBestModePrefersSemanticWhenCapabilitiesAreAvailable(t *testing.T) {
	got := search.BestMode(search.Capabilities{
		HasEmbeddings:    true,
		HasEmbeddingData: true,
		HasLlamaServer:   true,
		EmbeddingModel:   "/tmp/model.gguf",
	})
	if want := search.ModeSemantic; got != want {
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
