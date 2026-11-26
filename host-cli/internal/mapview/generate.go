package mapview

import (
	_ "embed"
	"html/template"
	"os"
	"path/filepath"
)

// Layer describes a PMTiles layer to render in the map viewer.
type Layer struct {
	Name        string
	Filename    string
	Path        string
	Category    string
	SourceLayer string
	Style       map[string]map[string]any
	Attribution string
	LicenseURL  string
}

type templateData struct {
	Layers          []Layer
	MapLibreJSPath  string
	MapLibreCSSPath string
	PMTilesJSPath   string
}

//go:embed viewer.html
var htmlTemplate string

// The maplibre-vendor app-bundle installs into apps/maplibre-vendor/vendor/,
// a sibling of this viewer's apps/map/ directory.
const (
	mapLibreJSPath  = "../maplibre-vendor/vendor/maplibre-gl.js"
	mapLibreCSSPath = "../maplibre-vendor/vendor/maplibre-gl.css"
	pmTilesJSPath   = "../maplibre-vendor/vendor/pmtiles.js"
)

func ensureVendorAssets(root string) error {
	vendor := filepath.Join("apps", "maplibre-vendor", "vendor")
	required := []string{
		filepath.Join(vendor, "maplibre-gl.js"),
		filepath.Join(vendor, "maplibre-gl.css"),
		filepath.Join(vendor, "pmtiles.js"),
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			return err
		}
	}
	return nil
}

// Generate creates a standalone HTML map viewer at root/apps/map/index.html
// that renders the given PMTiles layers using MapLibre GL JS.
func Generate(root string, layers []Layer) error {
	dir := filepath.Join(root, "apps", "map")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := ensureVendorAssets(root); err != nil {
		return err
	}

	tmpl, err := template.New("mapview").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	outPath := filepath.Join(dir, "index.html")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := templateData{
		Layers:          layers,
		MapLibreJSPath:  mapLibreJSPath,
		MapLibreCSSPath: mapLibreCSSPath,
		PMTilesJSPath:   pmTilesJSPath,
	}
	return tmpl.Execute(f, data)
}
