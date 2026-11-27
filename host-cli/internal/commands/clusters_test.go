package commands

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func clusterCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	recipes := fstest.MapFS{
		"open-webui.yaml": {Data: []byte("id: open-webui\ntype: python-package\nsize_gb: 0.58\nfile_count: 21977\n")},
		"wiki.yaml":       {Data: []byte("id: wiki\ntype: zim\nsize_gb: 10\nurl: http://example.test/wiki.zim\n")},
		"drawio.yaml":     {Data: []byte("id: drawio\ntype: app\nsize_gb: 0.05\n")},
	}
	cat, err := catalog.LoadFromFS(recipes, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestEstimateSpaceCountsAClusterPerFile(t *testing.T) {
	cat := clusterCatalog(t)
	platforms := []string{"macos-arm64", "linux-x86_64"}
	e := EstimateSpace(cat, []string{"wiki", "open-webui", "drawio"}, platforms, 128<<10)
	// Open WebUI per platform: 0.58 GB + 21,977 x 128 KB = 3.46 GB (3.20 GB
	// measured), twice; the ZIM is one file.
	if want := 10 + 131072/1e9 + 2*(0.58+21977*131072/1e9); !near(e.AllocatedGB, want) {
		t.Errorf("AllocatedGB = %.3f, want %.3f", e.AllocatedGB, want)
	}
	if want := 10 + 4096/1e9 + 2*(0.58+21977*4096/1e9); !near(e.SmallGB, want) {
		t.Errorf("SmallGB = %.3f, want %.3f", e.SmallGB, want)
	}
	if strings.Join(e.Trees, ",") != "open-webui" || strings.Join(e.Unknown, ",") != "drawio" {
		t.Errorf("Trees = %v, Unknown = %v", e.Trees, e.Unknown)
	}
}

func TestWriteClusterWarningSaysWhatLargeClustersCost(t *testing.T) {
	cat := clusterCatalog(t)
	var buf bytes.Buffer
	WriteClusterWarning(&buf, EstimateSpace(cat, []string{"wiki", "open-webui", "drawio"}, []string{"linux-x86_64"}, 128<<10))
	out := buf.String()
	for _, want := range []string{"128 KB clusters", "open-webui holds many small files", "more than with 4 KB clusters", "drawio also holds many small files, not measured", "Preparing a stick"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning lacks %q:\n%s", want, out)
		}
	}

	buf.Reset()
	WriteClusterWarning(&buf, EstimateSpace(cat, []string{"wiki"}, nil, 1<<20))
	if out := buf.String(); !strings.Contains(out, "1 MB clusters") || !strings.Contains(out, "mostly large files") {
		t.Errorf("warning for large files only:\n%s", out)
	}

	buf.Reset()
	WriteClusterWarning(&buf, EstimateSpace(cat, []string{"open-webui"}, nil, 4<<10))
	if buf.Len() != 0 {
		t.Errorf("warned for 4 KB clusters:\n%s", buf.String())
	}
}

func TestClusterSizeMeasuresTheNearestExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	size, err := ClusterSize(filepath.Join(dir, "vault", "not-yet"))
	if err != nil {
		t.Fatal(err)
	}
	if size < 512 || size&(size-1) != 0 {
		t.Fatalf("ClusterSize = %d, want a power of two of at least 512", size)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("probe file left behind: %v", entries)
	}
}

func TestMeasureVaultAndStatusShowContentBesideAllocated(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "zim"), 0o755)
	os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte("small"), 0o644)
	os.WriteFile(filepath.Join(root, "zim", "a.zim"), bytes.Repeat([]byte("z"), 10000), 0o644)
	u, err := MeasureVault(root)
	if err != nil {
		t.Fatal(err)
	}
	if u.ContentBytes != 10005 || u.AllocatedBytes < u.ContentBytes || u.ClusterBytes == 0 {
		t.Fatalf("usage = %+v", u)
	}

	m := manifest.New("v")
	r := BuildStatus(clusterCatalog(t), m)
	r.Usage = &VaultUsage{ContentBytes: 3 << 30, AllocatedBytes: 9 << 30, ClusterBytes: 128 << 10}
	var buf bytes.Buffer
	if err := WriteStatusReport(&buf, r, m); err != nil {
		t.Fatal(err)
	}
	if want := "Size: 3.0 GB in files, 9.0 GB on disk (128 KB clusters)"; !strings.Contains(buf.String(), want) {
		t.Errorf("status lacks %q:\n%s", want, buf.String())
	}
}

// A disk the user cannot write is measured by the smallest file already on it.
func TestClusterSizeOfAnUnwritableDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	size, err := ClusterSize(dir)
	if err != nil || size < 512 {
		t.Fatalf("ClusterSize = %d, %v", size, err)
	}
}
