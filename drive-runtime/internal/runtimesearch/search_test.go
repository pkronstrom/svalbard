package runtimesearch_test

import (
	"bytes"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimesearch"
)

func TestBuildFTSQueryPrefixesTermsForWildcardSearch(t *testing.T) {
	got := runtimesearch.BuildFTSQuery(`hello world`)
	want := `"hello"* "world"*`
	if got != want {
		t.Fatalf("BuildFTSQuery() = %q, want %q", got, want)
	}
}

func TestDecodeVectorHexParsesLittleEndianFloat32(t *testing.T) {
	got, err := runtimesearch.DecodeVectorHex("0000803F00000040")
	if err != nil {
		t.Fatalf("DecodeVectorHex() error = %v", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("DecodeVectorHex() = %#v, want [1 2]", got)
	}
}

func TestBestModePrefersSemanticWhenCapabilitiesAreAvailable(t *testing.T) {
	got := runtimesearch.BestMode(runtimesearch.Capabilities{
		HasEmbeddings:    true,
		HasEmbeddingData: true,
		HasLlamaServer:   true,
		EmbeddingModel:   "/tmp/model.gguf",
	})
	if want := runtimesearch.ModeSemantic; got != want {
		t.Fatalf("BestMode() = %q, want %q", got, want)
	}
}

func TestRenderResultsIncludesNumberedEntries(t *testing.T) {
	var out bytes.Buffer
	runtimesearch.RenderResults(&out, []runtimesearch.Result{
		{Filename: "wiki.zim", Title: "Solar power", Snippet: "Overview"},
	})
	text := out.String()
	if text == "" || !bytes.Contains(out.Bytes(), []byte("1. [wiki] Solar power")) {
		t.Fatalf("RenderResults() output = %q", text)
	}
}
