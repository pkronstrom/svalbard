package commands

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestExpandItemRefsExpandsPresetsAndKeepsIDs(t *testing.T) {
	cat := catalog.NewTestCatalog(t)

	got, err := ExpandItemRefs(cat, []string{"local:manual", "default-32", "ifixit", "unknown-id"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"local:manual", "ifixit", "wikipedia-en-nopic", "ifixit", "unknown-id"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandItemRefs = %v, want %v", got, want)
	}

	m := manifest.New("test")
	if err := AddItems(&m, got); err != nil {
		t.Fatal(err)
	}
	if len(m.Desired.Items) != 4 {
		t.Errorf("expected duplicates collapsed to 4 items, got %v", m.Desired.Items)
	}
}

func TestAddItemsDeduplicatesAndPreservesExistingEntries(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia"}

	if err := AddItems(&m, []string{"ifixit", "wikipedia"}); err != nil {
		t.Fatalf("AddItems returned error: %v", err)
	}

	if len(m.Desired.Items) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(m.Desired.Items), m.Desired.Items)
	}
	if m.Desired.Items[0] != "wikipedia" {
		t.Errorf("expected first item to be %q, got %q", "wikipedia", m.Desired.Items[0])
	}
	if m.Desired.Items[1] != "ifixit" {
		t.Errorf("expected second item to be %q, got %q", "ifixit", m.Desired.Items[1])
	}
}

func TestAddItemsToEmptyManifest(t *testing.T) {
	m := manifest.New("test")

	if err := AddItems(&m, []string{"ifixit"}); err != nil {
		t.Fatalf("AddItems returned error: %v", err)
	}

	if len(m.Desired.Items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(m.Desired.Items), m.Desired.Items)
	}
	if m.Desired.Items[0] != "ifixit" {
		t.Errorf("expected item to be %q, got %q", "ifixit", m.Desired.Items[0])
	}
}

func TestDrafterWarningsNameUnpairedDrafterAndTargets(t *testing.T) {
	cat, err := catalog.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	warnings := DrafterWarnings(cat, []string{"qwen-27b-mtp"})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "qwen-27b-mtp") || !strings.Contains(warnings[0], "qwen-27b-hq") {
		t.Fatalf("DrafterWarnings(drafter only) = %q", warnings)
	}
	if warnings := DrafterWarnings(cat, []string{"qwen-27b-mtp", "qwen-27b"}); len(warnings) != 0 {
		t.Fatalf("DrafterWarnings(with target) = %q, want none", warnings)
	}
}
