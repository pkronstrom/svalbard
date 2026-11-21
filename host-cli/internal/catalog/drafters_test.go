package catalog

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func drafterCatalog(t *testing.T, drafter string) (*Catalog, error) {
	t.Helper()
	recipes := fstest.MapFS{
		"models/qwen-27b.yaml":    {Data: []byte("id: qwen-27b\ntype: gguf\n")},
		"models/qwen-27b-hq.yaml": {Data: []byte("id: qwen-27b-hq\ntype: gguf\n")},
		"models/drafter.yaml":     {Data: []byte(drafter)},
	}
	return LoadFromFS(recipes, fstest.MapFS{})
}

func TestItemParsesDraftFor(t *testing.T) {
	cat, err := drafterCatalog(t, "id: qwen-27b-mtp\ntype: gguf-mtp\ndraft_for: [qwen-27b, qwen-27b-hq]\n")
	if err != nil {
		t.Fatal(err)
	}
	item, _ := cat.RecipeByID("qwen-27b-mtp")
	if !reflect.DeepEqual(item.DraftFor, []string{"qwen-27b", "qwen-27b-hq"}) {
		t.Fatalf("DraftFor = %v", item.DraftFor)
	}
}

func TestLoadRejectsUnknownDraftTarget(t *testing.T) {
	_, err := drafterCatalog(t, "id: qwen-27b-mtp\ntype: gguf-mtp\ndraft_for: [qwen-27b, missing-model]\n")
	if err == nil || !strings.Contains(err.Error(), "missing-model") {
		t.Fatalf("LoadFromFS() error = %v, want unknown draft target", err)
	}
}

func TestUnpairedDraftersReportsDrafterWithoutSelectedTarget(t *testing.T) {
	cat, err := drafterCatalog(t, "id: qwen-27b-mtp\ntype: gguf-mtp\ndraft_for: [qwen-27b, qwen-27b-hq]\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := cat.UnpairedDrafters([]string{"qwen-27b-mtp"}); !reflect.DeepEqual(got, []string{"qwen-27b-mtp"}) {
		t.Fatalf("UnpairedDrafters(drafter only) = %v", got)
	}
	if got := cat.UnpairedDrafters([]string{"qwen-27b-mtp", "qwen-27b-hq"}); len(got) != 0 {
		t.Fatalf("UnpairedDrafters(with target) = %v, want none", got)
	}
	if got := cat.UnpairedDrafters([]string{"qwen-27b"}); len(got) != 0 {
		t.Fatalf("UnpairedDrafters(no drafter) = %v, want none", got)
	}
}

func TestRealCatalogPairsQwenDrafter(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := cat.RecipeByID("qwen-27b-mtp")
	if !ok {
		t.Fatal("qwen-27b-mtp recipe missing")
	}
	if !reflect.DeepEqual(item.DraftFor, []string{"qwen-27b", "qwen-27b-hq"}) {
		t.Fatalf("qwen-27b-mtp draft_for = %v", item.DraftFor)
	}
}
