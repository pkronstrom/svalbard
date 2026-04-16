package mcp

import (
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func TestNewSearchResultItemIncludesReadHint(t *testing.T) {
	result := search.Result{
		Filename: "alpinelinux_en_all_maxi_2026-01.zim",
		Path:     "Linux_iSCSI_Target_(tgt)/en",
		Title:    "Linux iSCSI Target (tgt)",
		Snippet:  "Snippet",
	}

	item := newSearchResultItem(result)

	if item.Source != "alpinelinux_en_all_maxi_2026-01" {
		t.Fatalf("item.Source = %q", item.Source)
	}
	if item.Path != result.Path {
		t.Fatalf("item.Path = %q, want %q", item.Path, result.Path)
	}
	if item.ReadHint == "" {
		t.Fatal("item.ReadHint is empty")
	}
	want := "Use search_read with this exact source and path"
	if item.ReadHint != want {
		t.Fatalf("item.ReadHint = %q, want %q", item.ReadHint, want)
	}
}
