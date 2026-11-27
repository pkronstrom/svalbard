package apply

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// llamaArchive is a tar.gz shaped like llama-server's: a library reached
// through a link, and the server.
func llamaArchive(t *testing.T, release, body string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range []*tar.Header{
		{Name: release + "/llama-server", Mode: 0o755, Size: int64(len(body))},
		{Name: release + "/libllama.so.1", Typeflag: tar.TypeSymlink, Linkname: "libllama.so.1.0.0"},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			tw.Write([]byte(body))
		}
	}
	tw.WriteHeader(&tar.Header{Name: release + "/libllama.so.1.0.0", Mode: 0o755, Size: 3})
	tw.Write([]byte("lib"))
	tw.Close()
	gz.Close()
	return buf.String()
}

func llamaCatalog(t *testing.T, url string) *catalog.Catalog {
	t.Helper()
	recipe := "id: llama-server\ntype: binary\nstrategy: download\nplatforms:\n  linux-arm64: " + url + "\n"
	cat, err := catalog.LoadFromFS(fstest.MapFS{"llama-server.yaml": &fstest.MapFile{Data: []byte(recipe)}}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// The archive with links is unpacked at apply with the links resolved, so the
// stick needs no symlink and no write at first use; an update replaces the
// tree and a removal takes it away.
func TestApplyUnpacksBinaryArchiveWithLinksAndKeepsItInStep(t *testing.T) {
	useMockHTTPResponses(t, map[string]string{
		"/llama-b1-bin-ubuntu-arm64.tar.gz": llamaArchive(t, "llama-b1", "b1"),
		"/llama-b2-bin-ubuntu-arm64.tar.gz": llamaArchive(t, "llama-b2", "b2"),
	})
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"llama-server"}
	m.Desired.Options.HostPlatforms = []string{"linux-arm64"}
	binDir := filepath.Join(root, "bin", "linux-arm64")

	if err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"llama-server"}}, llamaCatalog(t, "http://example.test/llama-b1-bin-ubuntu-arm64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(binDir, "llama-b1-bin-ubuntu-arm64.tar.gz.unpacked", "llama-b1")
	if got := read(t, filepath.Join(tree, "llama-server")); got != "b1" {
		t.Fatalf("llama-server = %q, want the unpacked b1", got)
	}
	if info, err := os.Lstat(filepath.Join(tree, "libllama.so.1")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("libllama.so.1 = %v, %v; want a plain file", info, err)
	}

	// What a drive runtime from before .unpacked left beside the archive.
	legacy := filepath.Join(binDir, "llama-b1")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "llama-server"), []byte("b1"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"llama-server"}}, llamaCatalog(t, "http://example.test/llama-b2-bin-ubuntu-arm64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"llama-b1-bin-ubuntu-arm64.tar.gz.unpacked", "llama-b1"} {
		if _, err := os.Stat(filepath.Join(binDir, kept)); !os.IsNotExist(err) {
			t.Fatalf("b1 tree %s kept after the update: %v", kept, err)
		}
	}
	if got := read(t, filepath.Join(binDir, "llama-b2-bin-ubuntu-arm64.tar.gz.unpacked", "llama-b2", "llama-server")); got != "b2" {
		t.Fatalf("llama-server after update = %q, want b2", got)
	}

	if err := Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"llama-server"}}, llamaCatalog(t, "http://example.test/llama-b2-bin-ubuntu-arm64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(binDir)
	for _, e := range entries {
		// An update to a new file name leaves the old download record, as
		// for every type; that is not this tree's concern.
		if !strings.HasSuffix(e.Name(), ".download.json") {
			t.Errorf("left in bin/linux-arm64 after removal: %s", e.Name())
		}
	}
}

// An archive whose name stays the same across releases
// (duckdb-cli-linux-arm64.zip) gets the new release's tree, not the old one.
func TestUpdateUnderTheSameNameReplacesTheTree(t *testing.T) {
	useMockHTTPResponses(t, map[string]string{"/llama-bin.tar.gz": llamaArchive(t, "llama", "b1")})
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"llama-server"}
	m.Desired.Options.HostPlatforms = []string{"linux-arm64"}
	cat := llamaCatalog(t, "http://example.test/llama-bin.tar.gz")
	if err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"llama-server"}}, cat); err != nil {
		t.Fatal(err)
	}
	useMockHTTPResponses(t, map[string]string{"/llama-bin.tar.gz": llamaArchive(t, "llama", "b2")})
	if err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"llama-server"}}, cat); err != nil {
		t.Fatal(err)
	}
	server := filepath.Join(root, "bin", "linux-arm64", "llama-bin.tar.gz.unpacked", "llama", "llama-server")
	if got := read(t, server); got != "b2" {
		t.Fatalf("llama-server after update = %q, want b2", got)
	}
}

// A dataset is an archive too, but nothing unpacks it beside itself, so its
// removal leaves the directories beside it alone.
func TestRemovingANonBinaryArchiveLeavesItsNeighbours(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	neighbour := filepath.Join(dataDir, "llama", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(neighbour), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(neighbour, []byte("mine"), 0o644)
	os.WriteFile(filepath.Join(dataDir, "set.tar.gz"), []byte(llamaArchive(t, "llama", "x")), 0o644)
	m := manifest.New("test")
	m.Realized.Entries = []manifest.RealizedEntry{{ID: "set", Type: "dataset", Filename: "set.tar.gz", RelativePath: "data/set.tar.gz"}}
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"set"}}, cat); err != nil {
		t.Fatal(err)
	}
	if got := read(t, neighbour); got != "mine" {
		t.Fatalf("neighbour removed with the dataset")
	}
}
