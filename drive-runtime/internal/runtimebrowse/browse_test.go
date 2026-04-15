package runtimebrowse_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebrowse"
)

func TestResolveTargetsReturnsSpecificArchiveWhenRequested(t *testing.T) {
	driveRoot := t.TempDir()
	mustTouch(t, filepath.Join(driveRoot, "zim", "a.zim"))
	mustTouch(t, filepath.Join(driveRoot, "zim", "b.zim"))

	targets, err := runtimebrowse.ResolveTargets(driveRoot, "b.zim")
	if err != nil {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(targets))
	}
	if got, want := filepath.Base(targets[0]), "b.zim"; got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
}

func TestResolveTargetsReturnsAllArchivesSortedWhenNoSelection(t *testing.T) {
	driveRoot := t.TempDir()
	mustTouch(t, filepath.Join(driveRoot, "zim", "b.zim"))
	mustTouch(t, filepath.Join(driveRoot, "zim", "a.zim"))

	targets, err := runtimebrowse.ResolveTargets(driveRoot, "")
	if err != nil {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
	if got, want := len(targets), 2; got != want {
		t.Fatalf("len(targets) = %d, want %d", got, want)
	}
	if got, want := filepath.Base(targets[0]), "a.zim"; got != want {
		t.Fatalf("targets[0] = %q, want %q", got, want)
	}
	if got, want := filepath.Base(targets[1]), "b.zim"; got != want {
		t.Fatalf("targets[1] = %q, want %q", got, want)
	}
}

func mustTouch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
