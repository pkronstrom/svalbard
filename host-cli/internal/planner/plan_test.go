package planner

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestBuildPlanFindsDownloadsAndRemovals(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia", "ifixit"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia"},
		{ID: "old-source"},
	}

	p := Build(m)

	if len(p.ToDownload) != 1 || p.ToDownload[0] != "ifixit" {
		t.Fatalf("expected ToDownload=[ifixit], got %v", p.ToDownload)
	}
	if len(p.ToRemove) != 1 || p.ToRemove[0] != "old-source" {
		t.Fatalf("expected ToRemove=[old-source], got %v", p.ToRemove)
	}
}

func TestBuildPlanEmptyWhenSynced(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia"},
	}

	p := Build(m)

	if len(p.ToDownload) != 0 {
		t.Fatalf("expected empty ToDownload, got %v", p.ToDownload)
	}
	if len(p.ToRemove) != 0 {
		t.Fatalf("expected empty ToRemove, got %v", p.ToRemove)
	}
}

func TestBuildPlanAllNewWhenNoRealized(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia", "ifixit"}

	p := Build(m)

	if len(p.ToDownload) != 2 {
		t.Fatalf("expected 2 downloads, got %v", p.ToDownload)
	}
	if p.ToDownload[0] != "wikipedia" || p.ToDownload[1] != "ifixit" {
		t.Fatalf("expected ToDownload=[wikipedia, ifixit], got %v", p.ToDownload)
	}
	if len(p.ToRemove) != 0 {
		t.Fatalf("expected empty ToRemove, got %v", p.ToRemove)
	}
}

func TestBuildWithDiskReportsUnmanagedFiles(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia", RelativePath: "zim/wikipedia.zim"},
	}

	p := BuildWithDisk(m, []string{"zim/wikipedia.zim", "zim/manual-drop.zim"})

	if len(p.Unmanaged) != 1 || p.Unmanaged[0] != "zim/manual-drop.zim" {
		t.Fatalf("expected Unmanaged=[zim/manual-drop.zim], got %v", p.Unmanaged)
	}
}

func TestBuildWithDiskNoUnmanagedWhenClean(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia", RelativePath: "zim/wikipedia.zim"},
	}

	p := BuildWithDisk(m, []string{"zim/wikipedia.zim"})

	if len(p.Unmanaged) != 0 {
		t.Fatalf("expected empty Unmanaged, got %v", p.Unmanaged)
	}
}

func TestBuildWithDiskCombinesWithPlanDiffs(t *testing.T) {
	m := manifest.New("test")
	m.Desired.Items = []string{"wikipedia", "ifixit"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia", RelativePath: "zim/wikipedia.zim"},
	}

	p := BuildWithDisk(m, []string{"zim/wikipedia.zim", "zim/stale.zim"})

	if len(p.ToDownload) != 1 || p.ToDownload[0] != "ifixit" {
		t.Fatalf("expected ToDownload=[ifixit], got %v", p.ToDownload)
	}
	if len(p.Unmanaged) != 1 || p.Unmanaged[0] != "zim/stale.zim" {
		t.Fatalf("expected Unmanaged=[zim/stale.zim], got %v", p.Unmanaged)
	}
}
