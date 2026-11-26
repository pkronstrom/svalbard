package apply

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// zimCatalog has one ZIM recipe "item" with the given url and, if non-empty,
// a fixed filename.
func zimCatalog(t *testing.T, url, filename string) *catalog.Catalog {
	t.Helper()
	recipe := "id: item\ntype: zim\nstrategy: download\nurl: " + url + "\n"
	if filename != "" {
		recipe += "filename: " + filename + "\n"
	}
	cat, err := catalog.LoadFromFS(fstest.MapFS{"item.yaml": &fstest.MapFile{Data: []byte(recipe)}}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// installV1 applies a vault holding "item" as zim/<filename> with content "v1".
func installV1(t *testing.T, filename string) (string, manifest.Manifest) {
	t.Helper()
	useMockHTTPResponses(t, map[string]string{"/v1.zim": "v1"})
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"item"}
	cat := zimCatalog(t, "http://example.test/v1.zim", filename)
	if err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"item"}}, cat); err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("item")
	if got := m.Realized.Entries[0].AcquisitionHash; got == "" || got != recipe.AcquisitionHash() {
		t.Fatalf("install recorded acquisition hash %q, want %q", got, recipe.AcquisitionHash())
	}
	return root, m
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*"+asideSuffix))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestUpdateSwapsInTheNewCopyAndRecordsItsHash(t *testing.T) {
	root, m := installV1(t, "item.zim")
	before := m.Realized.Entries[0]

	useMockHTTPResponses(t, map[string]string{"/v2.zim": "version two"})
	cat := zimCatalog(t, "http://example.test/v2.zim", "item.zim")
	if err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"item"}}, cat); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "zim", "item.zim")); got != "version two" {
		t.Errorf("content = %q", got)
	}
	after := m.Realized.Entries[0]
	recipe, _ := cat.RecipeByID("item")
	if len(m.Realized.Entries) != 1 || after.AcquisitionHash != recipe.AcquisitionHash() || after.AcquisitionHash == before.AcquisitionHash || after.ChecksumSHA256 == before.ChecksumSHA256 {
		t.Errorf("entries after update = %+v", m.Realized.Entries)
	}
	if left := leftovers(t, filepath.Join(root, "zim")); len(left) != 0 {
		t.Errorf("set-aside copies left behind: %v", left)
	}
}

func TestUpdateToANewFilenameRemovesTheOldFile(t *testing.T) {
	root, m := installV1(t, "")
	useMockHTTPResponses(t, map[string]string{"/v2.zim": "version two"})
	if err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"item"}}, zimCatalog(t, "http://example.test/v2.zim", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "zim", "v1.zim")); !os.IsNotExist(err) {
		t.Errorf("old file still present: %v", err)
	}
	if got := read(t, filepath.Join(root, "zim", "v2.zim")); got != "version two" || m.Realized.Entries[0].RelativePath != "zim/v2.zim" || len(m.Realized.Entries) != 1 {
		t.Errorf("content = %q, entries = %+v", got, m.Realized.Entries)
	}
}

func TestFailedUpdateKeepsTheInstalledCopy(t *testing.T) {
	root, m := installV1(t, "item.zim")
	before := m.Realized.Entries[0]

	// The new source is gone.
	useMockHTTPResponses(t, map[string]string{})
	err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"item"}}, zimCatalog(t, "http://example.test/gone.zim", "item.zim"))
	if err == nil {
		t.Fatal("update from a missing source should fail")
	}
	if got := read(t, filepath.Join(root, "zim", "item.zim")); got != "v1" {
		t.Errorf("installed copy = %q, want it untouched", got)
	}
	if len(m.Realized.Entries) != 1 || m.Realized.Entries[0] != before {
		t.Errorf("manifest changed by a failed update: %+v", m.Realized.Entries)
	}
	if left := leftovers(t, filepath.Join(root, "zim")); len(left) != 0 {
		t.Errorf("set-aside copies left behind: %v", left)
	}
}

func TestFailedReplaceUpdateLeavesTheItemPending(t *testing.T) {
	root, m := installV1(t, "item.zim")
	useMockHTTPResponses(t, map[string]string{})
	plan := planner.Plan{ToUpdate: []string{"item"}, Replace: true}
	if err := Run(context.Background(), root, &m, plan, zimCatalog(t, "http://example.test/gone.zim", "item.zim")); err == nil {
		t.Fatal("update from a missing source should fail")
	}
	if _, err := os.Stat(filepath.Join(root, "zim", "item.zim")); !os.IsNotExist(err) {
		t.Errorf("--replace should have deleted the installed copy: %v", err)
	}
	if len(m.Realized.Entries) != 0 {
		t.Errorf("manifest still lists the deleted item: %+v", m.Realized.Entries)
	}
	if p := planner.Build(m); len(p.ToDownload) != 1 {
		t.Errorf("item should be pending again: %+v", p)
	}
}

func TestApplyRestoresACopySetAsideByAnInterruptedUpdate(t *testing.T) {
	root, m := installV1(t, "item.zim")
	path := filepath.Join(root, "zim", "item.zim")
	// An update died after moving the copy aside and half-writing a new one.
	if err := os.Rename(path, path+asideSuffix); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), root, &m, planner.Plan{}, zimCatalog(t, "http://example.test/v1.zim", "item.zim")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "v1" {
		t.Errorf("content = %q, want the installed copy back", got)
	}
	if left := leftovers(t, filepath.Join(root, "zim")); len(left) != 0 {
		t.Errorf("set-aside copies left behind: %v", left)
	}
}

func TestRemovalIsReportedAsRemovingAndLeavesNoDownloadRecord(t *testing.T) {
	root, m := installV1(t, "item.zim")
	m.Desired.Items = nil
	cat := zimCatalog(t, "http://example.test/v1.zim", "item.zim")

	var steps []string
	record := func(ev ProgressEvent) { steps = append(steps, ev.Status+":"+ev.Step) }
	if err := Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"item"}}, cat, record); err != nil {
		t.Fatal(err)
	}
	if len(steps) == 0 || steps[0] != "active:removing" {
		t.Fatalf("progress = %v, want the removal reported as removing", steps)
	}
	left, _ := filepath.Glob(filepath.Join(root, "zim", "*"))
	if len(left) != 0 {
		t.Fatalf("removal left %v behind", left)
	}
}
