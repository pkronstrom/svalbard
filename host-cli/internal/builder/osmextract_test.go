package builder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestOSMExtractCutsTheLatestBuildToTheBBox(t *testing.T) {
	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"key":"20261124.pmtiles"},{"key":"20261126.pmtiles"},{"key":"20261125.pmtiles"}]`))
	}))
	defer index.Close()
	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	var call []string
	runToolCommand = func(_ context.Context, _, _ string, image, tool string, args []string) (string, error) {
		if image != BaseToolsImage || tool != "pmtiles" {
			t.Fatalf("tool call = %s in %s", tool, image)
		}
		call = args
		return "", os.WriteFile(args[2], []byte("pmtiles"), 0o644)
	}
	recipe := catalog.Item{
		ID: "osm-test", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "osm-extract",
			Config: map[string]string{
				"bbox": "23.8,59.8,26.8,61.7", "maxzoom": "14",
				"builds_index": index.URL, "builds_base": "https://build.example.test/",
			},
		},
	}
	entries, err := buildOSMExtract(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != filepath.Join("maps", "osm-test.pmtiles") {
		t.Fatalf("entries = %+v", entries)
	}
	if data, err := os.ReadFile(filepath.Join(root, "maps", "osm-test.pmtiles")); err != nil || string(data) != "pmtiles" {
		t.Fatalf("output = %q, %v", data, err)
	}
	want := []string{"extract", "https://build.example.test/20261126.pmtiles", "", "--bbox=23.8,59.8,26.8,61.7", "--maxzoom=14"}
	if len(call) != len(want) || call[0] != want[0] || call[1] != want[1] || call[3] != want[3] || call[4] != want[4] {
		t.Fatalf("pmtiles args = %q, want %q (output path aside)", call, want)
	}
	if !strings.HasPrefix(call[2], filepath.Join(root, ".staging")) {
		t.Errorf("extract writes %s, want a staging path so a failed run leaves no partial map", call[2])
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", "build", "osm-test")); !os.IsNotExist(err) {
		t.Errorf("build files left on the vault: %v", err)
	}
}

func TestOSMExtractUsesAPinnedSourceWithoutTheIndex(t *testing.T) {
	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	var source string
	runToolCommand = func(_ context.Context, _, _ string, _, _ string, args []string) (string, error) {
		source = args[1]
		return "", os.WriteFile(args[2], []byte("pmtiles"), 0o644)
	}
	recipe := catalog.Item{
		ID: "osm-pinned", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family:    "osm-extract",
			SourceURL: "https://build.example.test/20261101.pmtiles",
			Config:    map[string]string{"bbox": "24,60,25,61", "builds_index": "http://127.0.0.1:1/unreachable"},
		},
	}
	if _, err := buildOSMExtract(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if source != "https://build.example.test/20261101.pmtiles" {
		t.Fatalf("source = %q", source)
	}
}
