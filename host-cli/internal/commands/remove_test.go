package commands

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestRemoveItemsOnlyEditsDesiredItems(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia", "ifixit"}

	if err := RemoveItems(&m, []string{"ifixit"}); err != nil {
		t.Fatalf("RemoveItems returned error: %v", err)
	}

	if len(m.Desired.Items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(m.Desired.Items), m.Desired.Items)
	}
	if m.Desired.Items[0] != "wikipedia" {
		t.Errorf("expected item to be %q, got %q", "wikipedia", m.Desired.Items[0])
	}
}

func TestRemoveNonexistentItemIsNoOp(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia"}

	if err := RemoveItems(&m, []string{"ifixit"}); err != nil {
		t.Fatalf("RemoveItems returned error: %v", err)
	}

	if len(m.Desired.Items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(m.Desired.Items), m.Desired.Items)
	}
	if m.Desired.Items[0] != "wikipedia" {
		t.Errorf("expected item to be %q, got %q", "wikipedia", m.Desired.Items[0])
	}
}
