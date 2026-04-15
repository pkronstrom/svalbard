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
    basemaps = []
    overlays = []

    for source in preset.sources:
        if source.type != "pmtiles":
            continue

        recipe = _load_recipe(source.id)
        if recipe is None:
            continue

        viewer = recipe.get("viewer", {})
        build = recipe.get("build", {})
        entry = {
            "id": source.id,
            "name": viewer.get("name", source.id),
            "name_fi": viewer.get("name_fi", ""),
            "category": viewer.get("category", "other"),
            "style": viewer.get("style", {}),
            "tile_type": viewer.get("tile_type", "vector"),
            "filename": f"{source.id}.pmtiles",
            "source_layer": build.get("layer_name", source.id.replace("-", "_")),
        }

        if viewer.get("category") == "basemap":
            basemaps.append(entry)
        else:
            overlays.append(entry)

    return basemaps, overlays


def generate_map_viewer(drive_path: Path, preset_name: str) -> Path:
    """Generate apps/map/index.html with a MapLibre viewer for all pmtiles layers."""
    basemaps, overlays = _layer_defs_from_preset(preset_name, drive_path=drive_path)

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
    basemaps_json = json.dumps(basemaps, indent=2)
    categories_json = json.dumps(category_labels, indent=2)

    html = _MAP_VIEWER_TEMPLATE.replace("__LAYERS_JSON__", layers_json)
    html = html.replace("__BASEMAPS_JSON__", basemaps_json)
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
const BASEMAPS = __BASEMAPS_JSON__;
const CATEGORIES = __CATEGORIES_JSON__;

// Register PMTiles protocol
const protocol = new pmtiles.Protocol();
maplibregl.addProtocol("pmtiles", protocol.tile);

const pmtilesUrl = (filename) =>
  `pmtiles://${window.location.origin}/maps/${filename}`;

const style = { version: 8, sources: {}, layers: [] };

// ── Basemap layer builders per tile_type ──────────────────────────────────

function _protomapsLayers(srcId, vis) {
  return [
    { id: srcId+"-bg", type: "background", paint: { "background-color": "#f0f0f0" }, layout: { visibility: vis } },
    { id: srcId+"-earth", type: "fill", source: srcId, "source-layer": "earth",
      paint: { "fill-color": "#f5f5f3" }, layout: { visibility: vis } },
    { id: srcId+"-landcover", type: "fill", source: srcId, "source-layer": "landcover",
      paint: { "fill-color": "#e0eed8", "fill-opacity": 0.5 }, layout: { visibility: vis } },
    { id: srcId+"-water", type: "fill", source: srcId, "source-layer": "water",
      paint: { "fill-color": "#aad3df" }, layout: { visibility: vis } },
    { id: srcId+"-landuse", type: "fill", source: srcId, "source-layer": "landuse",
      paint: { "fill-color": "#c8facc", "fill-opacity": 0.3 }, layout: { visibility: vis } },
    { id: srcId+"-roads-other", type: "line", source: srcId, "source-layer": "roads", minzoom: 12,
      paint: { "line-color": "#e0e0e0", "line-width": 1 }, layout: { visibility: vis } },
    { id: srcId+"-roads-minor", type: "line", source: srcId, "source-layer": "roads", minzoom: 9,
      filter: ["any", ["has", "ref"], [">=", ["get", "sort_key"], 100]],
      paint: { "line-color": "#d0d0d0", "line-width": ["interpolate",["linear"],["zoom"],9,0.5,14,2] },
      layout: { visibility: vis } },
    { id: srcId+"-roads-major", type: "line", source: srcId, "source-layer": "roads",
      filter: [">=", ["get", "sort_key"], 200],
      paint: { "line-color": "#bbb", "line-width": ["interpolate",["linear"],["zoom"],6,1,14,4] },
      layout: { visibility: vis } },
    { id: srcId+"-boundaries", type: "line", source: srcId, "source-layer": "boundaries",
      paint: { "line-color": "#ccc", "line-width": 1, "line-dasharray": [3,2] }, layout: { visibility: vis } },
    { id: srcId+"-buildings", type: "fill", source: srcId, "source-layer": "buildings", minzoom: 13,
      paint: { "fill-color": "#ddd" }, layout: { visibility: vis } },
    { id: srcId+"-places", type: "symbol", source: srcId, "source-layer": "places",
      layout: { "text-field": "{name}", "text-size": ["interpolate",["linear"],["zoom"],6,10,14,14], visibility: vis },
      paint: { "text-color": "#333", "text-halo-color": "#fff", "text-halo-width": 1 } },
  ];
}

function _rasterLayers(srcId, vis) {
  return [
    { id: srcId+"-raster", type: "raster", source: srcId, layout: { visibility: vis } },
  ];
}

// Simplified MML topo style — colours from MML backgroundmap.json
function _mmlTopoLayers(srcId, vis) {
  const kl = "kohdeluokka";
  return [
    { id: srcId+"-bg", type: "background",
      paint: { "background-color": "#dceacc" }, layout: { visibility: vis } },
    // Terrain areas (marsh, rock, sand)
    { id: srcId+"-maasto", type: "fill", source: srcId, "source-layer": "maasto_alue",
      paint: { "fill-color": ["match", ["get", kl],
        35411, "hsla(200,80%,90%,0.7)", 35412, "hsla(200,80%,90%,0.7)",
        35421, "hsla(200,80%,90%,0.7)", 35422, "hsla(200,80%,90%,0.7)",
        34100, "hsla(208,11%,75%,0.5)", 34700, "hsla(208,11%,75%,0.5)",
        39110, "hsla(44,100%,83%,0.84)", 39120, "hsla(44,100%,83%,0.84)",
        "hsla(200,80%,90%,0.4)"] }, layout: { visibility: vis } },
    // Land use
    { id: srcId+"-maankaytto", type: "fill", source: srcId, "source-layer": "maankaytto",
      paint: { "fill-color": ["match", ["get", kl],
        32611, "#f9f4d2", 32200, "hsl(87,45%,72%)", 32612, "hsl(87,45%,72%)",
        32800, "hsl(87,45%,72%)", 33100, "hsl(110,40%,65%)",
        40200, "hsl(0,0%,90%)", "#f7f7f3"] }, layout: { visibility: vis } },
    // Water bodies
    { id: srcId+"-water-fill", type: "fill", source: srcId, "source-layer": "vesisto_alue",
      paint: { "fill-color": "hsl(200,80%,85%)", "fill-outline-color": "hsl(200,80%,75%)" },
      layout: { visibility: vis } },
    // Water lines
    { id: srcId+"-water-line", type: "line", source: srcId, "source-layer": "vesisto_viiva",
      paint: { "line-color": "hsl(200,80%,85%)",
        "line-width": ["interpolate",["linear"],["zoom"],8,0.5,14,2] },
      layout: { visibility: vis } },
    // Terrain boundaries
    { id: srcId+"-maastoaluereuna", type: "line", source: srcId, "source-layer": "maastoaluereuna",
      paint: { "line-color": "#c8c4c5", "line-width": 0.5 }, minzoom: 11,
      layout: { visibility: vis } },
    // Contour lines
    { id: srcId+"-contours", type: "line", source: srcId, "source-layer": "korkeus",
      paint: { "line-color": "rgba(252,179,110,1)", "line-width": 0.5 }, minzoom: 10,
      layout: { visibility: vis } },
    // Buildings
    { id: srcId+"-buildings", type: "fill", source: srcId, "source-layer": "rakennus",
      paint: { "fill-color": "hsl(0,0%,65%)", "fill-outline-color": "hsl(0,0%,50%)" }, minzoom: 12,
      layout: { visibility: vis } },
    // Roads (from liikenne source-layer, filter by kohdeluokka ranges for road types)
    { id: srcId+"-roads-major", type: "line", source: srcId, "source-layer": "liikenne",
      filter: ["match", ["get", kl], [12111,12112,12121,12122], true, false],
      paint: { "line-color": "#cc4444", "line-width": ["interpolate",["linear"],["zoom"],6,1,14,4] },
      layout: { visibility: vis } },
    { id: srcId+"-roads-minor", type: "line", source: srcId, "source-layer": "liikenne",
      filter: ["match", ["get", kl], [12131,12132,12141,12151,12152,12153,12154,12155], true, false],
      paint: { "line-color": "#999", "line-width": ["interpolate",["linear"],["zoom"],9,0.5,14,2] },
      minzoom: 9, layout: { visibility: vis } },
    { id: srcId+"-roads-path", type: "line", source: srcId, "source-layer": "liikenne",
      filter: ["match", ["get", kl], [12316,12312,12313,12314], true, false],
      paint: { "line-color": "#90ca6f", "line-width": 1, "line-dasharray": [4,2] },
      minzoom: 11, layout: { visibility: vis } },
    // Railways
    { id: srcId+"-railway", type: "line", source: srcId, "source-layer": "liikenne",
      filter: ["match", ["get", kl], [14110,14121,14131], true, false],
      paint: { "line-color": "#555", "line-width": 1.5 },
      layout: { visibility: vis } },
    // Power lines
    { id: srcId+"-powerlines", type: "line", source: srcId, "source-layer": "liikenne",
      filter: ["match", ["get", kl], [22300,22311,22312], true, false],
      paint: { "line-color": "#aaa", "line-width": 0.5, "line-dasharray": [6,4] },
      minzoom: 11, layout: { visibility: vis } },
  ];
}

// ── Build basemap sources & layers ────────────────────────────────────────

const basemapLayerIds = {};  // basemap id → [layer ids]

if (BASEMAPS.length === 0) {
  style.layers.push({ id: "bg", type: "background", paint: { "background-color": "#e8e8e0" } });
}

BASEMAPS.forEach((bm, i) => {
  const vis = i === 0 ? "visible" : "none";
  const srcId = bm.id;

  if (bm.tile_type === "raster") {
    style.sources[srcId] = { type: "raster", url: pmtilesUrl(bm.filename), tileSize: 256 };
    const layers = _rasterLayers(srcId, vis);
    layers.forEach(l => style.layers.push(l));
    basemapLayerIds[srcId] = layers.map(l => l.id);
  } else if (bm.tile_type === "mml-topo") {
    style.sources[srcId] = { type: "vector", url: pmtilesUrl(bm.filename) };
    const layers = _mmlTopoLayers(srcId, vis);
    layers.forEach(l => style.layers.push(l));
    basemapLayerIds[srcId] = layers.map(l => l.id);
  } else {
    // Default: Protomaps vector basemap
    style.sources[srcId] = { type: "vector", url: pmtilesUrl(bm.filename) };
    const layers = _protomapsLayers(srcId, vis);
    layers.forEach(l => style.layers.push(l));
    basemapLayerIds[srcId] = layers.map(l => l.id);
  }
});

// ── Overlay sources & layers ──────────────────────────────────────────────

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
      "source-layer": (layer.source_layer || layer.id.replace(/-/g, "_")),
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
        "source-layer": (layer.source_layer || layer.id.replace(/-/g, "_")),
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
      "source-layer": (layer.source_layer || layer.id.replace(/-/g, "_")),
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
      "source-layer": (layer.source_layer || layer.id.replace(/-/g, "_")),
      paint: {
        "circle-color": s.points["circle-color"] || "#888",
        "circle-radius": s.points["circle-radius"] || 5,
      },
      layout: { visibility: "none" },
    });
  }
});

// ── Initialize map ────────────────────────────────────────────────────────

const map = new maplibregl.Map({
  container: "map",
  style: style,
  center: [25.5, 64.0],
  zoom: 5,
});
map.addControl(new maplibregl.NavigationControl());

// ── Build sidebar ─────────────────────────────────────────────────────────

const listEl = document.getElementById("layer-list");

// Basemap selector (radio buttons if >1 basemap)
if (BASEMAPS.length > 1) {
  const bmDiv = document.createElement("div");
  bmDiv.className = "category";
  bmDiv.innerHTML = "<h3>Basemap</h3>";
  BASEMAPS.forEach((bm, i) => {
    const label = document.createElement("label");
    label.className = "layer-toggle";
    label.innerHTML = `<input type="radio" name="basemap" value="${bm.id}" ${i === 0 ? "checked" : ""}> ${bm.name}`;
    label.querySelector("input").addEventListener("change", (e) => {
      BASEMAPS.forEach((b) => {
        const vis = b.id === e.target.value ? "visible" : "none";
        (basemapLayerIds[b.id] || []).forEach((lid) => {
          if (map.getLayer(lid)) map.setLayoutProperty(lid, "visibility", vis);
        });
      });
    });
    bmDiv.appendChild(label);
  });
  listEl.appendChild(bmDiv);
}

// Overlay toggles grouped by category
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

if (BASEMAPS.length === 0 && LAYERS.length === 0) {
  document.getElementById("sidebar").style.display = "none";
}

function toggleSidebar() {
  const sb = document.getElementById("sidebar");
  sb.style.display = sb.style.display === "none" ? "block" : "none";
}
</script>
</body>
</html>
"""
