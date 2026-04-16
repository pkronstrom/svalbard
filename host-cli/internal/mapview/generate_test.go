package mapview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVendorBundle(t *testing.T, root string) {
	t.Helper()

	for _, rel := range []string{
		filepath.Join("vendor", "maplibre-gl.js"),
		filepath.Join("vendor", "maplibre-gl.css"),
		filepath.Join("vendor", "pmtiles.js"),
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("test asset"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func TestGenerateCreatesHTMLFile(t *testing.T) {
	root := t.TempDir()
	writeVendorBundle(t, root)
	layers := []Layer{
		{Name: "OSM Finland", Filename: "osm-finland.pmtiles", Category: "basemap"},
	}
	if err := Generate(root, layers); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "apps", "map", "index.html")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)

	if strings.Contains(html, "unpkg.com") {
		t.Error("generated HTML should not reference CDN assets")
	}
	if !strings.Contains(html, "../../vendor/maplibre-gl.js") {
		t.Error("missing local MapLibre GL JS reference")
	}
	if !strings.Contains(html, "../../vendor/maplibre-gl.css") {
		t.Error("missing local MapLibre GL CSS reference")
	}
	if !strings.Contains(html, "../../vendor/pmtiles.js") {
		t.Error("missing local PMTiles JS reference")
	}
	if !strings.Contains(html, "osm-finland.pmtiles") {
		t.Error("missing layer filename")
	}
	if !strings.Contains(html, "OSM Finland") {
		t.Error("missing layer display name")
	}
}

func TestGenerateWithMultipleLayers(t *testing.T) {
	root := t.TempDir()
	writeVendorBundle(t, root)
	layers := []Layer{
		{Name: "OSM", Filename: "osm.pmtiles", Category: "basemap"},
		{Name: "Water Points", Filename: "water.pmtiles", Category: "overlay"},
	}
	if err := Generate(root, layers); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, "apps", "map", "index.html"))
	html := string(raw)

	if !strings.Contains(html, "osm.pmtiles") {
		t.Error("missing basemap")
	}
	if !strings.Contains(html, "water.pmtiles") {
		t.Error("missing overlay")
	}
}

func TestGenerateEmptyLayers(t *testing.T) {
	root := t.TempDir()
	writeVendorBundle(t, root)
	err := Generate(root, nil)
	// Should succeed but maybe skip file creation or create minimal page
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRequiresVendorBundle(t *testing.T) {
	root := t.TempDir()

	err := Generate(root, []Layer{{Name: "OSM", Filename: "osm.pmtiles"}})
	if err == nil {
		t.Fatal("expected error when vendor bundle is missing")
	}
}
