"""Generate a self-contained MapLibre map viewer for the drive.

Reads all pmtiles sources from the preset, builds a single-page HTML app
with layer toggles grouped by category, pointing at local tile and asset files.
"""

from __future__ import annotations

import json
from pathlib import Path

import yaml

from svalbard.drive_config import load_snapshot_preset
from svalbard.presets import load_preset, RECIPES_DIRS


def _load_recipe(source_id: str) -> dict | None:
    """Load a recipe YAML by source ID."""
    for recipes_dir in RECIPES_DIRS:
        for path in recipes_dir.rglob("*.yaml"):
            with open(path) as f:
                data = yaml.safe_load(f)
            if data and data.get("id") == source_id:
                return data
    return None


def _layer_defs_from_preset(
    preset_name: str,
    *,
    drive_path: Path | None = None,
    workspace: Path | str | None = None,
) -> tuple[dict | None, list[dict]]:
    """Return (basemap_def, overlay_defs) for all pmtiles sources in the preset."""
    preset = None
    if drive_path is not None:
        preset = load_snapshot_preset(drive_path)
    if preset is None:
        preset = load_preset(preset_name, workspace=workspace)
    basemap = None
    overlays = []

    for source in preset.sources:
        if source.type != "pmtiles":
            continue

        recipe = _load_recipe(source.id)
        if recipe is None:
            continue

        viewer = recipe.get("viewer", {})
        entry = {
            "id": source.id,
            "name": viewer.get("name", source.id),
            "name_fi": viewer.get("name_fi", ""),
            "category": viewer.get("category", "other"),
            "style": viewer.get("style", {}),
            "filename": f"{source.id}.pmtiles",
        }

        if viewer.get("category") == "basemap":
            basemap = entry
        else:
            overlays.append(entry)

    return basemap, overlays


def generate_map_viewer(drive_path: Path, preset_name: str) -> Path:
    """Generate apps/map/index.html with a MapLibre viewer for all pmtiles layers."""
    basemap, overlays = _layer_defs_from_preset(preset_name, drive_path=drive_path)

    # Category display order and labels
    category_labels = {
        "water": "Water",
        "food": "Food & Fishing",
        "shelter": "Shelter & Recreation",
        "context": "Environment",
        "other": "Other",
    }

    # Group overlays by category
    by_category: dict[str, list[dict]] = {}
    for layer in overlays:
        cat = layer["category"]
        by_category.setdefault(cat, []).append(layer)

    layers_json = json.dumps(overlays, indent=2)
    basemap_json = json.dumps(basemap) if basemap else "null"
    categories_json = json.dumps(category_labels, indent=2)

    html = _MAP_VIEWER_TEMPLATE.replace("__LAYERS_JSON__", layers_json)
    html = html.replace("__BASEMAP_JSON__", basemap_json)
    html = html.replace("__CATEGORIES_JSON__", categories_json)

    dest_dir = drive_path / "apps" / "map"
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / "index.html"
    dest.write_text(html)
    return dest


_MAP_VIEWER_TEMPLATE = r"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Offline Map Viewer</title>
<link rel="stylesheet" href="../maplibre-vendor/vendor/maplibre-gl.css">
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
#map { position: absolute; top: 0; left: 0; right: 0; bottom: 0; }
#sidebar {
  position: absolute; top: 10px; left: 10px; z-index: 1;
  background: rgba(255,255,255,0.95); border-radius: 8px;
  padding: 16px; width: 280px; max-height: calc(100vh - 20px);
  overflow-y: auto; box-shadow: 0 2px 8px rgba(0,0,0,0.15);
}
#sidebar h2 { font-size: 16px; margin-bottom: 12px; }
.category { margin-bottom: 12px; }
.category h3 { font-size: 13px; color: #666; text-transform: uppercase;
  letter-spacing: 0.5px; margin-bottom: 6px; }
.layer-toggle { display: flex; align-items: center; gap: 8px;
  padding: 4px 0; cursor: pointer; font-size: 14px; }
.layer-toggle input { cursor: pointer; }
.layer-toggle:hover { color: #2563eb; }
#toggle-btn {
  position: absolute; top: 10px; left: 10px; z-index: 2;
  background: white; border: none; border-radius: 6px; padding: 8px 12px;
  cursor: pointer; box-shadow: 0 2px 8px rgba(0,0,0,0.15);
  font-size: 18px; display: none;
}
</style>
</head>
<body>
<div id="map"></div>
<button id="toggle-btn" onclick="toggleSidebar()">&#9776;</button>
<div id="sidebar">
  <h2>Offline Map</h2>
  <div id="layer-list"></div>
</div>

<script src="../maplibre-vendor/vendor/maplibre-gl.js"></script>
<script src="../maplibre-vendor/vendor/pmtiles.js"></script>
<script>
const LAYERS = __LAYERS_JSON__;
const BASEMAP = __BASEMAP_JSON__;
const CATEGORIES = __CATEGORIES_JSON__;

// Register PMTiles protocol
const protocol = new pmtiles.Protocol();
maplibregl.addProtocol("pmtiles", protocol.tile);

// Build map style — use same origin so no CORS issues
const pmtilesUrl = (filename) =>
  `pmtiles://${window.location.origin}/maps/${filename}`;

const style = {
  version: 8,
  sources: {},
  layers: [],
};

// Add basemap if available
if (BASEMAP) {
  style.sources["basemap"] = {
    type: "vector",
    url: pmtilesUrl(BASEMAP.filename),
  };
  // Protomaps basemap layers (simplified)
  style.layers.push(
    { id: "bg", type: "background", paint: { "background-color": "#f0f0f0" } },
    { id: "water", type: "fill", source: "basemap", "source-layer": "water",
      paint: { "fill-color": "#aad3df" } },
    { id: "landuse-park", type: "fill", source: "basemap", "source-layer": "landuse",
      filter: ["==", "pmap:kind", "park"],
      paint: { "fill-color": "#c8facc", "fill-opacity": 0.5 } },
    { id: "roads-minor", type: "line", source: "basemap", "source-layer": "roads",
      filter: ["in", "pmap:kind", "minor_road", "other"],
      paint: { "line-color": "#e0e0e0", "line-width": 1 } },
    { id: "roads-major", type: "line", source: "basemap", "source-layer": "roads",
      filter: ["in", "pmap:kind", "major_road", "highway"],
      paint: { "line-color": "#bbb", "line-width": 2 } },
    { id: "buildings", type: "fill", source: "basemap", "source-layer": "buildings",
      paint: { "fill-color": "#ddd" } },
    { id: "places", type: "symbol", source: "basemap", "source-layer": "places",
      layout: { "text-field": "{name}", "text-size": 12 },
      paint: { "text-color": "#333", "text-halo-color": "#fff", "text-halo-width": 1 } }
  );
} else {
  style.layers.push(
    { id: "bg", type: "background", paint: { "background-color": "#e8e8e0" } }
  );
}

// Add overlay sources + layers
LAYERS.forEach((layer) => {
  style.sources[layer.id] = {
    type: "vector",
    url: pmtilesUrl(layer.filename),
  };

  const s = layer.style || {};
  if (s.polygons) {
    style.layers.push({
      id: layer.id + "-fill",
      type: "fill",
      source: layer.id,
      "source-layer": layer.id.replace(/-/g, "_"),
      paint: {
        "fill-color": s.polygons["fill-color"] || "#888",
        "fill-opacity": s.polygons["fill-opacity"] || 0.3,
      },
      layout: { visibility: "none" },
    });
    if (s.polygons["line-color"]) {
      style.layers.push({
        id: layer.id + "-line",
        type: "line",
        source: layer.id,
        "source-layer": layer.id.replace(/-/g, "_"),
        paint: {
          "line-color": s.polygons["line-color"],
          "line-width": s.polygons["line-width"] || 1,
        },
        layout: { visibility: "none" },
      });
    }
  }
  if (s.lines) {
    style.layers.push({
      id: layer.id + "-line",
      type: "line",
      source: layer.id,
      "source-layer": layer.id.replace(/-/g, "_"),
      paint: {
        "line-color": s.lines["line-color"] || "#888",
        "line-width": s.lines["line-width"] || 2,
      },
      layout: { visibility: "none" },
    });
  }
  if (s.points) {
    style.layers.push({
      id: layer.id + "-circle",
      type: "circle",
      source: layer.id,
      "source-layer": layer.id.replace(/-/g, "_"),
      paint: {
        "circle-color": s.points["circle-color"] || "#888",
        "circle-radius": s.points["circle-radius"] || 5,
      },
      layout: { visibility: "none" },
    });
  }
});

// Initialize map — centered on Finland
const map = new maplibregl.Map({
  container: "map",
  style: style,
  center: [25.5, 64.0],
  zoom: 5,
});
map.addControl(new maplibregl.NavigationControl());

// Build sidebar
const listEl = document.getElementById("layer-list");
const grouped = {};
LAYERS.forEach((l) => {
  const cat = l.category || "other";
  if (!grouped[cat]) grouped[cat] = [];
  grouped[cat].push(l);
});

Object.keys(CATEGORIES).forEach((cat) => {
  if (!grouped[cat]) return;
  const div = document.createElement("div");
  div.className = "category";
  div.innerHTML = `<h3>${CATEGORIES[cat]}</h3>`;
  grouped[cat].forEach((layer) => {
    const label = document.createElement("label");
    label.className = "layer-toggle";
    label.innerHTML = `<input type="checkbox" data-layer="${layer.id}"> ${layer.name}`;
    label.querySelector("input").addEventListener("change", (e) => {
      const vis = e.target.checked ? "visible" : "none";
      map.getStyle().layers.forEach((ml) => {
        if (ml.id.startsWith(layer.id + "-")) {
          map.setLayoutProperty(ml.id, "visibility", vis);
        }
      });
    });
    div.appendChild(label);
  });
  listEl.appendChild(div);
});

// Hide sidebar if no overlay layers
if (LAYERS.length === 0) {
  document.getElementById("sidebar").style.display = "none";
}

// Sidebar toggle for mobile
function toggleSidebar() {
  const sb = document.getElementById("sidebar");
  sb.style.display = sb.style.display === "none" ? "block" : "none";
}
</script>
</body>
</html>
"""
