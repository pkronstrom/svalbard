package mapview

import (
	"html/template"
	"os"
	"path/filepath"
)

// Layer describes a PMTiles layer to render in the map viewer.
type Layer struct {
	Name     string // Display name (e.g., "OpenStreetMap Finland")
	Filename string // PMTiles filename (e.g., "osm-finland.pmtiles")
	Category string // "basemap" or overlay category
}

type templateData struct {
	Layers []Layer
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Svalbard Maps</title>
    <script src="https://unpkg.com/maplibre-gl@4.7.1/dist/maplibre-gl.js"></script>
    <link href="https://unpkg.com/maplibre-gl@4.7.1/dist/maplibre-gl.css" rel="stylesheet">
    <script src="https://unpkg.com/pmtiles@4.2.0/dist/pmtiles.js"></script>
    <style>
        body { margin: 0; }
        #map { width: 100%; height: 100vh; }
        #layers { position: absolute; top: 10px; right: 10px; background: rgba(255,255,255,0.9);
                   padding: 10px; border-radius: 4px; font-family: sans-serif; font-size: 13px; }
        #layers label { display: block; margin: 4px 0; }
    </style>
</head>
<body>
    <div id="map"></div>
    <div id="layers">
        <strong>Layers</strong>
        {{range .Layers}}
        <label><input type="checkbox" checked data-source="{{.Filename}}"> {{.Name}}</label>
        {{end}}
    </div>
    <script>
        const protocol = new pmtiles.Protocol();
        maplibregl.addProtocol("pmtiles", protocol.tile);

        const mapView = new maplibregl.Map({
            container: 'map',
            style: {
                version: 8,
                sources: {
                    {{range $i, $l := .Layers}}{{if $i}},{{end}}
                    "{{$l.Filename}}": {
                        type: "vector",
                        url: "pmtiles://../../maps/{{$l.Filename}}"
                    }
                    {{end}}
                },
                layers: []
            },
            center: [0, 20],
            zoom: 2
        });

        mapView.addControl(new maplibregl.NavigationControl());
    </script>
</body>
</html>
`

// Generate creates a standalone HTML map viewer at root/apps/map/index.html
// that renders the given PMTiles layers using MapLibre GL JS.
func Generate(root string, layers []Layer) error {
	dir := filepath.Join(root, "apps", "map")
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

	data := templateData{Layers: layers}
	return tmpl.Execute(f, data)
}
