package builder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestVectorServiceBuildsPMTilesFromDeclaredLayers(t *testing.T) {
	var request url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.URL.Query()
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer server.Close()
	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	runToolCommand = func(_ context.Context, _, _ string, image, tool string, args []string) (string, error) {
		if image != BaseToolsImage || tool != "tippecanoe" {
			t.Fatalf("tool call = %s in %s", tool, image)
		}
		if !slices.Contains(args, "--drop-densest-as-needed") {
			t.Fatalf("tippecanoe args = %v, want density limit", args)
		}
		for index, arg := range args {
			if arg == "-o" && index+1 < len(args) {
				return "", os.WriteFile(args[index+1], []byte("pmtiles"), 0o644)
			}
		}
		t.Fatal("tippecanoe output argument missing")
		return "", nil
	}
	recipe := catalog.Item{
		ID: "lipas", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "vector-service",
			Config: map[string]string{
				"service_type": "wfs", "service_url": server.URL,
				"output_format": "application/json", "srs": "EPSG:4326",
				"layer_name": "lipas", "max_zoom": "14", "drop_densest_as_needed": "true",
			},
			Layers: []catalog.BuildLayer{{Name: "source:points", Filter: "kind = 1"}},
		},
	}
	entries, err := buildVectorService(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != filepath.Join("maps", "lipas.pmtiles") {
		t.Fatalf("entries = %+v", entries)
	}
	if request.Get("typeNames") != "source:points" || request.Get("CQL_FILTER") != "kind = 1" {
		t.Fatalf("WFS query = %v", request)
	}
}

func TestVectorStaticBuildsPMTilesFromShapefileArchive(t *testing.T) {
	fixtureDir := t.TempDir()
	archive := filepath.Join(fixtureDir, "source.zip")
	writeZip(t, archive, "data/layer.shp", "shape")
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	var calls []string
	// macOS creates this AppleDouble companion beside extracted files on exFAT.
	sidecar := filepath.Join(root, ".staging", "build", "protected", "source-01", "data", "._layer.shp")
	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte("metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	runToolCommand = func(_ context.Context, _, _ string, image, tool string, args []string) (string, error) {
		if image != BaseToolsImage {
			t.Fatalf("image = %q", image)
		}
		calls = append(calls, tool)
		switch tool {
		case "ogr2ogr":
			if got := filepath.Base(args[len(args)-1]); got != "layer.shp" {
				t.Fatalf("ogr2ogr source = %q, want layer.shp", got)
			}
			output := args[len(args)-2]
			return "", os.WriteFile(output, []byte(`{"type":"FeatureCollection","features":[]}`), 0o644)
		case "tippecanoe":
			for index, arg := range args {
				if arg == "-o" && index+1 < len(args) {
					return "", os.WriteFile(args[index+1], []byte("pmtiles"), 0o644)
				}
			}
		}
		return "", nil
	}
	recipe := catalog.Item{
		ID: "protected", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "vector-static", SourceURL: server.URL,
			Config: map[string]string{
				"source_format": "shapefile", "source_srs": "EPSG:3067",
				"layer_name": "protected", "max_zoom": "14",
			},
		},
	}
	entries, err := buildVectorStatic(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != filepath.Join("maps", "protected.pmtiles") {
		t.Fatalf("entries = %+v", entries)
	}
	if strings.Join(calls, ",") != "ogr2ogr,tippecanoe" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestVectorStaticMergesMultipleShapefileAssets(t *testing.T) {
	fixtureDir := t.TempDir()
	archive := filepath.Join(fixtureDir, "source.zip")
	writeZip(t, archive, "data/layer.shp", "shape")
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	ogrCalls := 0
	var tippecanoeInputs []string
	runToolCommand = func(_ context.Context, _, _ string, _, tool string, args []string) (string, error) {
		switch tool {
		case "ogr2ogr":
			ogrCalls++
			return "", os.WriteFile(args[len(args)-2], []byte(`{"type":"FeatureCollection","features":[]}`), 0o644)
		case "tippecanoe":
			for index, arg := range args {
				if arg == "--force" {
					tippecanoeInputs = args[index+1:]
				}
				if arg == "-o" && index+1 < len(args) {
					if err := os.WriteFile(args[index+1], []byte("pmtiles"), 0o644); err != nil {
						return "", err
					}
				}
			}
		}
		return "", nil
	}
	recipe := catalog.Item{
		ID: "protected", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "vector-static",
			Assets: []catalog.BuildAsset{{URL: server.URL + "/a.zip"}, {URL: server.URL + "/b.zip"}},
			Config: map[string]string{"source_format": "shapefile"},
		},
	}
	if _, err := buildVectorStatic(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if ogrCalls != 2 || len(tippecanoeInputs) != 2 {
		t.Fatalf("ogr2ogr calls = %d, tippecanoe inputs = %v", ogrCalls, tippecanoeInputs)
	}
}
