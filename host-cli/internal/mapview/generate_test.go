package mapview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeVendorBundle(t *testing.T, root string) {
	t.Helper()

	// The layout the maplibre-vendor app-bundle recipe actually installs.
	for _, rel := range []string{
		filepath.Join("apps", "maplibre-vendor", "vendor", "maplibre-gl.js"),
		filepath.Join("apps", "maplibre-vendor", "vendor", "maplibre-gl.css"),
		filepath.Join("apps", "maplibre-vendor", "vendor", "pmtiles.js"),
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
	if !strings.Contains(html, "../maplibre-vendor/vendor/maplibre-gl.js") {
		t.Error("missing local MapLibre GL JS reference")
	}
	if !strings.Contains(html, "../maplibre-vendor/vendor/maplibre-gl.css") {
		t.Error("missing local MapLibre GL CSS reference")
	}
	if !strings.Contains(html, "../maplibre-vendor/vendor/pmtiles.js") {
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

func TestGenerateWritesAtlasTypefacesOnce(t *testing.T) {
	root := t.TempDir()
	writeVendorBundle(t, root)
	if err := Generate(root, []Layer{{Name: "OSM", Filename: "osm.pmtiles", Category: "basemap"}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "apps", "map")
	for _, rel := range []string{"glyphs/EB Garamond Regular/0-255.pbf", "glyphs/EB Garamond Italic/256-511.pbf", "glyphs/Plex Mono Regular/8192-8447.pbf", "fonts/eb.woff2"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil || info.Size() == 0 {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "glyphs", "README.md")); !os.IsNotExist(err) {
		t.Errorf("the regeneration note belongs in the source tree, not the drive: %v", err)
	}
	// An unchanged file keeps its modification time on the next apply.
	glyph := filepath.Join(dir, "glyphs", "EB Garamond Regular", "0-255.pbf")
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(glyph, past, past); err != nil {
		t.Fatal(err)
	}
	if err := Generate(root, nil); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(glyph); err != nil || !info.ModTime().Equal(past) {
		t.Errorf("unchanged glyph rewritten: %v", err)
	}
}
