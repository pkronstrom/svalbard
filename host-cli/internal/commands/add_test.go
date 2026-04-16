package commands

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

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
