package mapview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesHTMLFile(t *testing.T) {
	root := t.TempDir()
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

	if !strings.Contains(html, "maplibregl") || !strings.Contains(html, "maplibre-gl") {
		t.Error("missing MapLibre GL JS reference")
	}
	if !strings.Contains(html, "pmtiles") {
		t.Error("missing PMTiles JS reference")
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
	err := Generate(root, nil)
	// Should succeed but maybe skip file creation or create minimal page
	if err != nil {
		t.Fatal(err)
	}
}
