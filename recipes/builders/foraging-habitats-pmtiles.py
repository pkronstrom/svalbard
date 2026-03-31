#!/usr/bin/env python3
"""Build a foraging habitat PMTiles layer from LUKE MS-NFI forest data.

Downloads Finnish national forest inventory rasters (site fertility, soil type,
tree species volumes), reclassifies them into foraging habitat types, polygonizes,
and produces a vector PMTiles layer with edible species properties per polygon.

Source: LUKE MS-NFI 2023, CC BY 4.0
License of output: CC BY 4.0

VASTUUVAPAUSLAUSEKE / DISCLAIMER:
  Tämä aineisto on tuotettu LUKE:n avoimista metsävaratiedoista parhaalla
  mahdollisella tarkkuudella. Lajiyhdistelmät ovat suuntaa-antavia ja
  perustuvat kasvupaikkatyypin ja puuston perusteella tehtyyn arvioon.
  Myrkylliset lajit voivat esiintyä samoilla kasvupaikoilla. Aineisto ei
  korvaa lajintunnistusopasta. Käyttäjä vastaa itse keräämistään kasveista
  ja sienistä.

  This dataset is derived from LUKE open forest resource data on a best-effort
  basis. Species associations are indicative estimates based on site fertility
  class and tree species composition. Poisonous species may occur in the same
  habitats. This does not replace a field identification guide. User assumes
  all responsibility for species they collect and consume.

Requirements: rasterio, numpy, pyproj, fiona, shapely
              GDAL CLI (gdal_translate), tippecanoe
Usage: python recipes/builders/foraging-habitats-pmtiles.py [--output foraging-habitats.pmtiles]
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import numpy as np

try:
    import rasterio
    from rasterio.features import shapes
    from rasterio.transform import from_bounds
except ImportError:
    print("rasterio is required: pip install rasterio", file=sys.stderr)
    sys.exit(1)

try:
    from pyproj import Transformer
except ImportError:
    print("pyproj is required: pip install pyproj", file=sys.stderr)
    sys.exit(1)

try:
    from shapely.geometry import shape, mapping
    from shapely.ops import unary_union
except ImportError:
    print("shapely is required: pip install shapely", file=sys.stderr)
    sys.exit(1)

# ── Source data ──────────────────────────────────────────────────────────────

LUKE_BASE = "https://www.nic.funet.fi/index/geodata/luke/vmi/2023"

RASTERS = {
    "kasvupaikka": f"{LUKE_BASE}/kasvupaikka_vmi1x_1923.tif",
    "paatyyppi": f"{LUKE_BASE}/paatyyppi_vmi1x_1923.tif",
    "maaluokka": f"{LUKE_BASE}/maaluokka_vmi1x_1923.tif",
    "manty": f"{LUKE_BASE}/manty_vmi1x_1923.tif",
    "kuusi": f"{LUKE_BASE}/kuusi_vmi1x_1923.tif",
    "koivu": f"{LUKE_BASE}/koivu_vmi1x_1923.tif",
}

CELL_SIZE = 500  # resample to 500m

# ── Foraging classes ─────────────────────────────────────────────────────────

FORAGING_CLASSES = {
    1: {
        "class": "herb_rich",
        "name": "Lehtometsä / Herb-rich forest",
        "berries": "Vadelma (raspberry) Jul-Aug",
        "herbs": "Nokkonen (nettle) May-Jul, Vuohenputki (ground elder) May-Jun, "
                 "Ketunleipä (wood sorrel) May-Sep, Mesiangervo (meadowsweet tea) Jun-Aug",
        "caution": "Kielo (lily of the valley) — myrkyllinen/poisonous, "
                   "Sudenmarja (herb-Paris) — myrkylliset marjat/poisonous berries",
    },
    2: {
        "class": "mixed_berry",
        "name": "Lehtomainen kangas / Mixed berry forest (OMT)",
        "berries": "Mustikka (bilberry) Jul-Aug, Vadelma (raspberry) Jul-Aug",
        "herbs": "Ketunleipä (wood sorrel) May-Sep",
        "caution": "",
    },
    3: {
        "class": "bilberry_forest",
        "name": "Mustikkametsä / Bilberry forest (MT)",
        "berries": "Mustikka (bilberry) Jul-Aug — paras kasvupaikka/peak habitat",
        "herbs": "",
        "caution": "",
    },
    4: {
        "class": "lingonberry_forest",
        "name": "Puolukkametsä / Lingonberry forest (VT)",
        "berries": "Puolukka (lingonberry) Aug-Oct — paras kasvupaikka/peak habitat, "
                   "Mustikka (bilberry) Jul-Aug",
        "herbs": "",
        "caution": "",
    },
    5: {
        "class": "dry_barren",
        "name": "Kuiva kangas / Dry barren forest (CT/ClT)",
        "berries": "Puolukka (lingonberry) Aug-Oct, Variksenmarja (crowberry) Aug-Sep",
        "herbs": "Kanerva (heather tea) Jun-Aug",
        "caution": "",
    },
    6: {
        "class": "bog_forest",
        "name": "Suometsä / Bog forest",
        "berries": "Lakka/hilla (cloudberry) Jul-Aug, Juolukka (bog bilberry) Aug-Sep, "
                   "Karpalo (cranberry) Sep-Oct",
        "herbs": "Suopursu (Labrador tea) — pieniä määriä/small amounts",
        "caution": "Myrkkykeiso (water hemlock) — erittäin myrkyllinen/extremely poisonous",
    },
    7: {
        "class": "open_bog",
        "name": "Avosuo / Open bog",
        "berries": "Lakka (cloudberry) Jul-Aug, Karpalo (cranberry) Sep-Oct",
        "herbs": "Rahkasammal (sphagnum) — haavanhoito/wound dressing",
        "caution": "Myrkkykeiso (water hemlock) — erittäin myrkyllinen/extremely poisonous",
    },
}

# Mushroom associations by dominant tree species
MUSHROOMS_BY_TREE = {
    "spruce": {
        3: "Herkkutatti (cep/porcini) Aug-Oct, Suppilovahvero (trumpet chanterelle) Sep-Nov",
        4: "Suppilovahvero (trumpet chanterelle) Sep-Nov",
    },
    "birch": {
        2: "Kantarelli (chanterelle) Jul-Sep",
        3: "Kantarelli (chanterelle) Jul-Sep, Koivunpunikkitatti (birch bolete) Jul-Oct",
    },
    "pine": {
        3: "Kangastatti (pine bolete) Aug-Oct",
        4: "Voitatti (slippery jack) Aug-Oct, Kangasrousku (rufous milkcap) Aug-Oct",
        5: "Voitatti (slippery jack) Aug-Oct",
    },
}

MUSHROOM_CAUTION = {
    "spruce": "Korvasieni (false morel) — myrkyllinen raakana, vaatii esikäsittelyn / toxic raw, requires parboiling",
    "pine": "Korvasieni (false morel) — myrkyllinen raakana / toxic raw, requires parboiling",
}

DISCLAIMER = "Suuntaa-antava / Indicative only — tunnista ennen käyttöä / identify before consuming"


# ── Pipeline helpers ─────────────────────────────────────────────────────────

def download(url: str, dest: Path) -> None:
    """Download a file with curl, skip if already exists."""
    if dest.exists():
        print(f"  Cached: {dest.name}")
        return
    print(f"  Downloading: {dest.name} ...")
    subprocess.run(
        ["curl", "-L", "--progress-bar", "-o", str(dest), url],
        check=True,
    )


def resample(src: Path, dst: Path, cell_size: int, method: str = "mode") -> None:
    """Resample a raster to a coarser resolution."""
    if dst.exists():
        print(f"  Cached: {dst.name}")
        return
    print(f"  Resampling {src.name} → {cell_size}m ({method})...")
    subprocess.run(
        [
            "gdal_translate",
            "-tr", str(cell_size), str(cell_size),
            "-r", method,
            "-co", "COMPRESS=DEFLATE",
            str(src), str(dst),
        ],
        check=True,
    )


def read_band(path: Path) -> tuple[np.ndarray, rasterio.Affine, rasterio.CRS]:
    """Read a single-band raster into a numpy array."""
    with rasterio.open(path) as src:
        return src.read(1), src.transform, src.crs


def classify(
    kasvupaikka: np.ndarray,
    paatyyppi: np.ndarray,
    maaluokka: np.ndarray,
    manty: np.ndarray,
    kuusi: np.ndarray,
    koivu: np.ndarray,
) -> tuple[np.ndarray, np.ndarray]:
    """Produce foraging class (1-7) and dominant tree (0=none, 1=pine, 2=spruce, 3=birch).

    Returns (foraging_class, dominant_tree) arrays.
    """
    h, w = kasvupaikka.shape
    fc = np.zeros((h, w), dtype=np.uint8)
    dt = np.zeros((h, w), dtype=np.uint8)

    # Only process forest land (maaluokka == 1)
    forest = maaluokka == 1

    mineral = forest & (paatyyppi == 1)
    mire = forest & ((paatyyppi == 2) | (paatyyppi == 3))
    open_bog = forest & (paatyyppi == 4)

    fc[mineral & (kasvupaikka == 1)] = 1  # herb_rich
    fc[mineral & (kasvupaikka == 2)] = 2  # mixed_berry
    fc[mineral & (kasvupaikka == 3)] = 3  # bilberry_forest
    fc[mineral & (kasvupaikka == 4)] = 4  # lingonberry_forest
    fc[mineral & (kasvupaikka >= 5)] = 5  # dry_barren
    fc[mire] = 6   # bog_forest
    fc[open_bog] = 7  # open_bog

    # Dominant tree on mineral soil
    tree_mask = mineral & (fc > 0)
    pine_dom = tree_mask & (manty >= kuusi) & (manty >= koivu) & (manty > 0)
    spruce_dom = tree_mask & (kuusi > manty) & (kuusi >= koivu) & (kuusi > 0)
    birch_dom = tree_mask & (koivu > manty) & (koivu > kuusi) & (koivu > 0)

    dt[pine_dom] = 1
    dt[spruce_dom] = 2
    dt[birch_dom] = 3

    return fc, dt


def polygonize_and_dissolve(
    fc: np.ndarray,
    dt: np.ndarray,
    transform: rasterio.Affine,
    crs: rasterio.CRS,
) -> list[dict]:
    """Convert raster classes to dissolved GeoJSON features in EPSG:4326."""
    print("  Polygonizing...")
    # Combine fc and dt into a single key for grouping
    # key = fc * 10 + dt (e.g., 32 = bilberry_forest + spruce)
    combined = fc.astype(np.int16) * 10 + dt.astype(np.int16)
    combined[fc == 0] = 0

    # Extract shapes
    geom_groups: dict[int, list] = {}
    for geom, value in shapes(combined, mask=(combined > 0), transform=transform):
        key = int(value)
        geom_groups.setdefault(key, []).append(shape(geom))

    print(f"  Dissolving {sum(len(v) for v in geom_groups.values())} polygons "
          f"into {len(geom_groups)} groups...")

    # Reproject to EPSG:4326
    transformer = Transformer.from_crs(str(crs), "EPSG:4326", always_xy=True)

    features = []
    tree_names = {0: "", 1: "pine", 2: "spruce", 3: "birch"}

    for key, geoms in geom_groups.items():
        fc_val = key // 10
        dt_val = key % 10

        if fc_val not in FORAGING_CLASSES:
            continue

        # Dissolve all polygons in this group
        merged = unary_union(geoms)
        # Simplify (200m tolerance for 500m source)
        merged = merged.simplify(200, preserve_topology=True)

        if merged.is_empty:
            continue

        # Reproject
        if merged.geom_type == "MultiPolygon":
            polys = list(merged.geoms)
        else:
            polys = [merged]

        for poly in polys:
            # Skip tiny polygons
            if poly.area < 100000:  # ~0.1 km² in projected coords
                continue

            coords = mapping(poly)
            # Reproject coordinates
            reproj = _reproject_geometry(coords, transformer)

            props = dict(FORAGING_CLASSES[fc_val])
            tree = tree_names[dt_val]
            props["tree"] = tree

            # Add tree-specific mushrooms
            mushrooms = MUSHROOMS_BY_TREE.get(tree, {}).get(fc_val, "")
            props["mushrooms"] = mushrooms

            # Add tree-specific caution
            existing_caution = props.get("caution", "")
            tree_caution = MUSHROOM_CAUTION.get(tree, "")
            if tree_caution and existing_caution:
                props["caution"] = f"{existing_caution}; {tree_caution}"
            elif tree_caution:
                props["caution"] = tree_caution

            props["disclaimer"] = DISCLAIMER

            features.append({
                "type": "Feature",
                "geometry": reproj,
                "properties": props,
            })

    print(f"  Generated {len(features)} features")
    return features


def _reproject_geometry(geom: dict, transformer: Transformer) -> dict:
    """Reproject a GeoJSON geometry dict from source CRS to target CRS."""
    def _transform_coords(coords):
        if isinstance(coords[0], (list, tuple)) and isinstance(coords[0][0], (list, tuple)):
            return [_transform_coords(ring) for ring in coords]
        xs, ys = zip(*[(c[0], c[1]) for c in coords])
        txs, tys = transformer.transform(list(xs), list(ys))
        return list(zip(txs, tys))

    return {
        "type": geom["type"],
        "coordinates": _transform_coords(geom["coordinates"]),
    }


def write_geojsonl(features: list[dict], path: Path) -> None:
    """Write newline-delimited GeoJSON (for tippecanoe)."""
    print(f"  Writing {len(features)} features to {path.name}...")
    with open(path, "w") as f:
        for feat in features:
            f.write(json.dumps(feat, ensure_ascii=False) + "\n")


def run_tippecanoe(geojsonl: Path, output: Path) -> None:
    """Run tippecanoe to produce PMTiles."""
    print(f"  Running tippecanoe → {output.name}...")
    subprocess.run(
        [
            "tippecanoe",
            "-o", str(output),
            "-l", "foraging",
            "-Z", "5",
            "-z", "12",
            "--coalesce-densest-as-needed",
            "--extend-zooms-if-still-dropping",
            "--no-tile-compression",
            "-n", "Foraging Habitats / Keräilykartta",
            "-A", DISCLAIMER,
            str(geojsonl),
        ],
        check=True,
    )


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Build foraging habitat PMTiles from LUKE MS-NFI data",
    )
    parser.add_argument(
        "--output", "-o",
        default="foraging-habitats.pmtiles",
        help="Output PMTiles file (default: foraging-habitats.pmtiles)",
    )
    parser.add_argument(
        "--cache-dir",
        default=None,
        help="Directory to cache downloaded rasters (default: temp dir)",
    )
    parser.add_argument(
        "--cell-size",
        type=int,
        default=CELL_SIZE,
        help=f"Resample cell size in meters (default: {CELL_SIZE})",
    )
    args = parser.parse_args()
    output = Path(args.output)

    # Check dependencies
    for tool in ("gdal_translate", "tippecanoe"):
        if not shutil.which(tool):
            print(f"Error: {tool} not found in PATH", file=sys.stderr)
            sys.exit(1)

    # Set up working directory
    cache_dir = Path(args.cache_dir) if args.cache_dir else None
    tmp_ctx = tempfile.TemporaryDirectory() if cache_dir is None else None

    try:
        work_dir = cache_dir or Path(tmp_ctx.name)
        work_dir.mkdir(parents=True, exist_ok=True)
        raw_dir = work_dir / "raw"
        resampled_dir = work_dir / "resampled"
        raw_dir.mkdir(exist_ok=True)
        resampled_dir.mkdir(exist_ok=True)

        # 1. Download
        print("\n[1/5] Downloading LUKE MS-NFI rasters...")
        raw_paths = {}
        for name, url in RASTERS.items():
            dest = raw_dir / f"{name}.tif"
            download(url, dest)
            raw_paths[name] = dest

        # 2. Resample
        print("\n[2/5] Resampling to {args.cell_size}m...".format(args=args))
        resampled = {}
        for name, src in raw_paths.items():
            dst = resampled_dir / f"{name}_{args.cell_size}m.tif"
            # Use 'mode' for categorical, 'average' for continuous
            method = "average" if name in ("manty", "kuusi", "koivu") else "mode"
            resample(src, dst, args.cell_size, method)
            resampled[name] = dst

        # 3. Classify
        print("\n[3/5] Reclassifying into foraging habitats...")
        data = {}
        transform = crs = None
        for name, path in resampled.items():
            arr, t, c = read_band(path)
            data[name] = arr
            if transform is None:
                transform, crs = t, c

        fc, dt = classify(
            data["kasvupaikka"], data["paatyyppi"], data["maaluokka"],
            data["manty"], data["kuusi"], data["koivu"],
        )
        classified = np.count_nonzero(fc)
        print(f"  Classified {classified:,} cells into foraging habitats")

        # 4. Polygonize, dissolve, reproject, attach species
        print("\n[4/5] Polygonizing and dissolving...")
        features = polygonize_and_dissolve(fc, dt, transform, crs)

        # 5. Tippecanoe
        print("\n[5/5] Building PMTiles...")
        geojsonl = work_dir / "foraging.geojsonl"
        write_geojsonl(features, geojsonl)
        run_tippecanoe(geojsonl, output)

        size_mb = output.stat().st_size / 1024 / 1024
        print(f"\nDone! {output} ({size_mb:.1f} MB)")
        print(f"  {len(features)} polygons, {len(FORAGING_CLASSES)} habitat classes")
        print(f"\n  Attribution: LUKE MS-NFI 2023, CC BY 4.0")
        print(f"  {DISCLAIMER}")

    finally:
        if tmp_ctx:
            tmp_ctx.cleanup()


if __name__ == "__main__":
    main()
