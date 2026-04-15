"""Build handler registry for sources that require local processing.

Sources with strategy="build" are dispatched here instead of the downloader.
Each handler family (vector-static, osm-extract, etc.) converts raw data into
the final artifact that lives on the drive.
"""

from __future__ import annotations

import logging
import platform as _platform
import shutil
import subprocess
import tempfile
import zipfile
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlparse

import httpx

from svalbard.docker import TOOLS_IMAGE, has_docker, ensure_tools_image, run_container
from svalbard.models import Source

log = logging.getLogger(__name__)

CACHE_ROOT = Path.home() / ".cache" / "svalbard" / "build"


@dataclass
class BuildResult:
    source_id: str
    success: bool
    artifact: Path | None = None
    error: str = ""


# ── Tool checking ───────────────────────────────────────────────────────────

TOOL_REQUIREMENTS: dict[str, list[str]] = {
    "vector-static": ["ogr2ogr", "tippecanoe"],
    "vector-service": ["ogr2ogr", "tippecanoe"],
    "osm-extract": ["pmtiles"],
    "reference-static": [],
    "app-bundle": [],
    "raster-tms": ["pmtiles"],
    "mml-topo": ["ogr2ogr", "tippecanoe"],
}

# Tools that can fall back to Docker when not installed locally
DOCKER_TOOL_IMAGES: dict[str, str] = {
    "ogr2ogr": TOOLS_IMAGE,
    "tippecanoe": TOOLS_IMAGE,
    "pmtiles": TOOLS_IMAGE,
}


def _find_tool(name: str, drive_path: Path) -> str | None:
    """Find a tool binary: first on the drive, then on system PATH."""
    os_name = "macos" if _platform.system() == "Darwin" else "linux"
    arch = "arm64" if _platform.machine() in ("aarch64", "arm64") else "x86_64"
    platform_dir = f"{os_name}-{arch}"

    for search_dir in [drive_path / "bin" / platform_dir, drive_path / "bin"]:
        candidate = search_dir / name
        if candidate.exists() and candidate.stat().st_mode & 0o111:
            return str(candidate)

    # Fall back to system PATH
    return shutil.which(name)


def check_tools(families: list[str], drive_path: Path | None = None) -> list[str]:
    """Return list of missing tools needed by the given build families.

    Searches drive bin directories first, then system PATH, then checks
    whether Docker can provide the tool as a last resort.
    """
    needed: set[str] = set()
    for family in families:
        needed.update(TOOL_REQUIREMENTS.get(family, []))
    missing = []
    for tool in sorted(needed):
        if drive_path and _find_tool(tool, drive_path):
            continue
        if shutil.which(tool) is not None:
            continue
        # Accept Docker fallback for supported tools
        if tool in DOCKER_TOOL_IMAGES and has_docker() and ensure_tools_image():
            continue
        missing.append(tool)
    return missing


# ── Handler registry ────────────────────────────────────────────────────────

HANDLERS: dict[str, object] = {}


def _register(family: str):
    def decorator(fn):
        HANDLERS[family] = fn
        return fn
    return decorator


def run_build(source: Source, drive_path: Path, cache_dir: Path | None = None) -> BuildResult:
    """Dispatch a build source to its handler."""
    family = source.build.get("family", "")
    handler = HANDLERS.get(family)
    if handler is None:
        return BuildResult(source.id, False, error=f"Unknown build family: {family}")

    cache = (cache_dir or CACHE_ROOT) / source.id
    cache.mkdir(parents=True, exist_ok=True)

    try:
        return handler(source, drive_path, cache)
    except Exception as e:
        log.exception("Build failed for %s", source.id)
        return BuildResult(source.id, False, error=str(e))


# ── Helpers ─────────────────────────────────────────────────────────────────

def _download_to(url: str, dest: Path, *, timeout: float = 300) -> Path:
    """Download a URL to dest (file path). Returns dest."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    with httpx.stream("GET", url, follow_redirects=True, timeout=timeout) as r:
        r.raise_for_status()
        with open(dest, "wb") as f:
            for chunk in r.iter_bytes(chunk_size=65536):
                f.write(chunk)
    return dest


def _safe_extract_zip(zip_path: Path, dest: Path) -> None:
    """Extract a ZIP file, rejecting entries with path traversal."""
    with zipfile.ZipFile(zip_path) as zf:
        for info in zf.infolist():
            member_path = (dest / info.filename).resolve()
            if not str(member_path).startswith(str(dest.resolve())):
                raise ValueError(f"Zip entry escapes target: {info.filename}")
        zf.extractall(dest)


def _run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Run a subprocess, raising on failure."""
    log.info("Running: %s", " ".join(cmd))
    return subprocess.run(cmd, check=True, capture_output=True, text=True, **kwargs)


def _resolve_tool(
    name: str, drive_path: Path,
) -> tuple[str | None, str | None]:
    """Resolve a tool to either a local binary path or a Docker image.

    Returns (binary_path, docker_image).  Exactly one will be non-None when
    the tool is available, or both None when it cannot be found.
    """
    local = _find_tool(name, drive_path)
    if local:
        return local, None
    if name in DOCKER_TOOL_IMAGES and has_docker() and ensure_tools_image():
        return None, DOCKER_TOOL_IMAGES[name]
    return None, None


# ── vector-static ───────────────────────────────────────────────────────────

def _docker_path(host_path: Path | str, mounts: dict) -> str:
    """Convert a host path to its Docker container equivalent."""
    hp = str(host_path)
    for host_mount, container_mount in mounts.items():
        if hp.startswith(host_mount):
            return hp.replace(host_mount, container_mount, 1)
    return hp


def _run_ogr2ogr(args: list[str], drive_path: Path, mounts: dict[str, str] | None = None) -> subprocess.CompletedProcess:
    """Run ogr2ogr using local binary or Docker fallback."""
    binary, image = _resolve_tool("ogr2ogr", drive_path)
    if binary:
        return _run([binary, *args])
    if image:
        return run_container(["ogr2ogr", *args], mounts=mounts or {}, check=True, capture_output=True, text=True)
    raise RuntimeError("ogr2ogr not found. Install GDAL or Docker.")


def _run_tippecanoe(args: list[str], drive_path: Path, mounts: dict[str, str] | None = None) -> subprocess.CompletedProcess:
    """Run tippecanoe using local binary or Docker fallback."""
    binary, image = _resolve_tool("tippecanoe", drive_path)
    if binary:
        return _run([binary, *args])
    if image:
        return run_container(["tippecanoe", *args], mounts=mounts or {}, check=True, capture_output=True, text=True)
    raise RuntimeError("tippecanoe not found. Install tippecanoe or Docker.")


@_register("vector-static")
def build_vector_static(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Download a ZIP with shapefiles, convert to GeoPackage, then to PMTiles."""
    b = source.build
    source_url = b["source_url"]
    source_srs = b.get("source_srs", "EPSG:4326")
    layer_name = b.get("layer_name", source.id)
    max_zoom = b.get("max_zoom", 14)

    raw_dir = cache / "raw"
    raw_dir.mkdir(parents=True, exist_ok=True)

    # Download
    zip_name = Path(urlparse(source_url).path).name
    zip_path = raw_dir / zip_name
    if not zip_path.exists():
        log.info("Downloading %s", source_url)
        _download_to(source_url, zip_path)

    # Unzip
    extract_dir = cache / "extracted"
    if not extract_dir.exists():
        extract_dir.mkdir(parents=True)
        _safe_extract_zip(zip_path, extract_dir)

    # Find shapefile
    shp_files = list(extract_dir.rglob("*.shp"))
    if not shp_files:
        return BuildResult(source.id, False, error="No .shp file found in archive")
    shp = shp_files[0]

    # Convert to GeoPackage
    gpkg_dir = cache / "canonical"
    gpkg_dir.mkdir(parents=True, exist_ok=True)
    gpkg = gpkg_dir / f"{source.id}.gpkg"

    docker_mounts = {str(cache): "/data", str(drive_path): "/drive"}

    if not gpkg.exists():
        _run_ogr2ogr([
            "-f", "GPKG",
            "-t_srs", "EPSG:4326",
            "-s_srs", source_srs,
            _docker_path(gpkg, docker_mounts),
            _docker_path(shp, docker_mounts),
            "-nln", layer_name,
        ], drive_path, docker_mounts)

    # Convert to PMTiles via tippecanoe
    dest_dir = drive_path / "maps"
    dest_dir.mkdir(parents=True, exist_ok=True)
    pmtiles_path = dest_dir / f"{source.id}.pmtiles"

    if not pmtiles_path.exists():
        tmp_geojson = cache / f"{source.id}.geojsonseq"
        try:
            _run_ogr2ogr([
                "-f", "GeoJSONSeq",
                _docker_path(tmp_geojson, docker_mounts),
                _docker_path(gpkg, docker_mounts),
            ], drive_path, docker_mounts)
            _run_tippecanoe([
                "-o", _docker_path(pmtiles_path, docker_mounts),
                f"-z{max_zoom}",
                "--drop-densest-as-needed",
                "-P",
                "-l", layer_name,
                _docker_path(tmp_geojson, docker_mounts),
            ], drive_path, docker_mounts)
        finally:
            tmp_geojson.unlink(missing_ok=True)

    return BuildResult(source.id, True, artifact=pmtiles_path)


# ── vector-service ──────────────────────────────────────────────────────────

@_register("vector-service")
def build_vector_service(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Fetch layers from a WFS service, merge into GeoPackage, then PMTiles."""
    b = source.build
    service_url = b["service_url"]
    layers = b.get("layers", [])
    layer_name = b.get("layer_name", source.id)
    max_zoom = b.get("max_zoom", 14)

    gpkg_dir = cache / "canonical"
    gpkg_dir.mkdir(parents=True, exist_ok=True)
    gpkg = gpkg_dir / f"{source.id}.gpkg"

    docker_mounts = {str(cache): "/data", str(drive_path): "/drive"}

    if not gpkg.exists():
        wfs_url = f"WFS:{service_url}"
        for i, layer_def in enumerate(layers):
            cmd = [
                "-f", "GPKG",
                "-t_srs", "EPSG:4326",
                _docker_path(gpkg, docker_mounts), wfs_url,
                layer_def["name"],
                "-nln", layer_name,
            ]
            if i > 0:
                cmd.append("-append")
            filt = layer_def.get("filter")
            if filt:
                cmd.extend(["-where", filt])
            _run_ogr2ogr(cmd, drive_path, docker_mounts)

    # Convert to PMTiles
    dest_dir = drive_path / "maps"
    dest_dir.mkdir(parents=True, exist_ok=True)
    pmtiles_path = dest_dir / f"{source.id}.pmtiles"

    if not pmtiles_path.exists():
        tmp_geojson = cache / f"{source.id}.geojsonseq"
        try:
            _run_ogr2ogr([
                "-f", "GeoJSONSeq",
                _docker_path(tmp_geojson, docker_mounts),
                _docker_path(gpkg, docker_mounts),
            ], drive_path, docker_mounts)
            _run_tippecanoe([
                "-o", _docker_path(pmtiles_path, docker_mounts),
                f"-z{max_zoom}",
                "--drop-densest-as-needed",
                "-P",
                "-l", layer_name,
                _docker_path(tmp_geojson, docker_mounts),
            ], drive_path, docker_mounts)
        finally:
            tmp_geojson.unlink(missing_ok=True)

    return BuildResult(source.id, True, artifact=pmtiles_path)


# ── osm-extract ─────────────────────────────────────────────────────────────

def _resolve_protomaps_url() -> str:
    """Resolve the latest Protomaps daily build URL.

    Tries today's date, then walks backwards up to 7 days.
    """
    from datetime import datetime, timedelta

    for days_back in range(8):
        date = datetime.now() - timedelta(days=days_back)
        url = f"https://build.protomaps.com/{date.strftime('%Y%m%d')}.pmtiles"
        try:
            r = httpx.head(url, follow_redirects=True, timeout=10)
            if r.status_code == 200:
                return url
        except httpx.HTTPError:
            continue
    raise RuntimeError(
        "Could not find a Protomaps daily build from the last 7 days. "
        "Check https://build.protomaps.com/ for availability."
    )


@_register("osm-extract")
def build_osm_extract(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Extract a regional PMTiles from the Protomaps global daily build."""
    b = source.build
    bbox = b.get("bbox", "19.5,59.0,32.0,70.1")
    maxzoom = b.get("maxzoom", 15)

    dest_dir = drive_path / "maps"
    dest_dir.mkdir(parents=True, exist_ok=True)
    pmtiles_path = dest_dir / f"{source.id}.pmtiles"

    if not pmtiles_path.exists():
        daily_url = _resolve_protomaps_url()
        log.info("Extracting from %s", daily_url)
        binary, image = _resolve_tool("pmtiles", drive_path)
        cmd_args = [
            "extract", daily_url, str(pmtiles_path),
            f"--bbox={bbox}", f"--maxzoom={maxzoom}",
        ]
        if binary:
            _run([binary, *cmd_args])
        elif image:
            run_container(["pmtiles", *cmd_args], mounts={
                str(dest_dir): str(dest_dir),
            }, check=True, capture_output=True, text=True)
        else:
            raise RuntimeError("pmtiles not found. Install go-pmtiles or Docker.")

    return BuildResult(source.id, True, artifact=pmtiles_path)


# ── reference-static ────────────────────────────────────────────────────────

@_register("reference-static")
def build_reference_static(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Download reference data and build a SQLite database with FTS5.

    This is a placeholder that creates the database structure.
    Actual parsing is source-specific and will be extended per-source.
    """
    b = source.build
    source_url = b.get("source_url", "")
    tables = b.get("tables", [])

    dest_dir = drive_path / "data"
    dest_dir.mkdir(parents=True, exist_ok=True)
    db_path = dest_dir / f"{source.id}.sqlite"

    if not db_path.exists():
        import sqlite3

        raw_dir = cache / "raw"
        raw_dir.mkdir(parents=True, exist_ok=True)

        # Download source if build config marks it as downloadable
        if b.get("downloadable", False) and source_url:
            filename = Path(urlparse(source_url).path).name
            raw_path = raw_dir / filename
            if not raw_path.exists():
                _download_to(source_url, raw_path)

        # Create database with schema
        conn = sqlite3.connect(str(db_path))
        conn.execute("PRAGMA journal_mode=WAL")

        for table_def in tables:
            table_name = table_def["name"]
            # Create a basic table — actual column definitions depend on the
            # source format and will be populated by source-specific parsers
            conn.execute(f"""
                CREATE TABLE IF NOT EXISTS "{table_name}" (
                    id INTEGER PRIMARY KEY,
                    data TEXT
                )
            """)
            if table_def.get("fts"):
                fts_cols = table_def.get("fts_columns", ["data"])
                cols = ", ".join(fts_cols)
                conn.execute(f"""
                    CREATE VIRTUAL TABLE IF NOT EXISTS "{table_name}_fts"
                    USING fts5({cols}, content="{table_name}")
                """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS _meta (
                key TEXT PRIMARY KEY,
                value TEXT
            )
        """)
        conn.execute(
            "INSERT OR REPLACE INTO _meta (key, value) VALUES (?, ?)",
            ("source_id", source.id),
        )
        conn.execute(
            "INSERT OR REPLACE INTO _meta (key, value) VALUES (?, ?)",
            ("description", source.description),
        )
        conn.commit()
        conn.close()

    return BuildResult(source.id, True, artifact=db_path)


# ── app-bundle ──────────────────────────────────────────────────────────────

@_register("app-bundle")
def build_app_bundle(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Download and extract a web application bundle to the apps directory."""
    b = source.build
    source_url = b.get("source_url", "")
    assets = b.get("assets", [])

    dest_dir = drive_path / "apps" / source.id
    dest_dir.mkdir(parents=True, exist_ok=True)

    # Check if already populated
    if any(dest_dir.iterdir()):
        return BuildResult(source.id, True, artifact=dest_dir)

    if assets:
        # Download individual asset files
        for asset in assets:
            asset_url = asset["url"]
            asset_dest = dest_dir / asset["dest"]
            asset_dest.parent.mkdir(parents=True, exist_ok=True)
            _download_to(asset_url, asset_dest)
    elif source_url:
        # Download and extract archive
        raw_dir = cache / "raw"
        raw_dir.mkdir(parents=True, exist_ok=True)

        parsed = urlparse(source_url)
        filename = Path(parsed.path).name or f"{source.id}.zip"
        archive_path = raw_dir / filename

        if not archive_path.exists():
            _download_to(source_url, archive_path)

        if archive_path.suffix == ".zip":
            _safe_extract_zip(archive_path, dest_dir)
        else:
            import tarfile
            with tarfile.open(archive_path) as tf:
                tf.extractall(dest_dir, filter="data")

    return BuildResult(source.id, True, artifact=dest_dir)


# ── raster-tms ─────────────────────────────────────────────────────────────

@_register("raster-tms")
def build_raster_tms(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Download raster tiles from a TMS endpoint and package as raster PMTiles."""
    import math
    import sqlite3

    b = source.build
    tms_url = b["tms_url"]  # e.g. "https://tiles.kartat.kapsi.fi/maastokartta"
    bbox_str = b.get("bbox", "23.8,59.8,26.8,61.7")
    min_zoom = b.get("min_zoom", 0)
    max_zoom = b.get("max_zoom", 13)
    tile_ext = b.get("tile_format", "jpg")

    west, south, east, north = (float(x) for x in bbox_str.split(","))

    def _lon_to_x(lon: float, z: int) -> int:
        n = 2 ** z
        return max(0, min(n - 1, int((lon + 180) / 360 * n)))

    def _lat_to_y(lat: float, z: int) -> int:
        n = 2 ** z
        r = math.radians(lat)
        return max(0, min(n - 1, int((1 - math.log(math.tan(r) + 1 / math.cos(r)) / math.pi) / 2 * n)))

    mbtiles_path = cache / f"{source.id}.mbtiles"

    if not mbtiles_path.exists():
        conn = sqlite3.connect(str(mbtiles_path))
        conn.execute("CREATE TABLE metadata (name text, value text)")
        conn.execute("CREATE TABLE tiles (zoom_level integer, tile_column integer, tile_row integer, tile_data blob)")
        conn.execute("CREATE UNIQUE INDEX tile_index ON tiles (zoom_level, tile_column, tile_row)")
        fmt = "jpeg" if tile_ext == "jpg" else tile_ext
        for k, v in [
            ("name", source.id), ("format", fmt), ("type", "baselayer"),
            ("bounds", bbox_str), ("minzoom", str(min_zoom)), ("maxzoom", str(max_zoom)),
        ]:
            conn.execute("INSERT INTO metadata VALUES (?, ?)", (k, v))
        conn.commit()

        total = 0
        with httpx.Client(timeout=30, follow_redirects=True) as client:
            for z in range(min_zoom, max_zoom + 1):
                x0, x1 = _lon_to_x(west, z), _lon_to_x(east, z)
                y0, y1 = _lat_to_y(north, z), _lat_to_y(south, z)
                batch = []
                for x in range(x0, x1 + 1):
                    for y in range(y0, y1 + 1):
                        url = f"{tms_url}/{z}/{x}/{y}.{tile_ext}"
                        try:
                            r = client.get(url)
                            r.raise_for_status()
                            tms_y = (2 ** z) - 1 - y  # MBTiles uses TMS-flipped y
                            batch.append((z, x, tms_y, r.content))
                        except httpx.HTTPError:
                            pass
                conn.executemany("INSERT OR REPLACE INTO tiles VALUES (?,?,?,?)", batch)
                conn.commit()
                total += len(batch)
                log.info("z%d: %d tiles for %s", z, len(batch), source.id)

        log.info("Total: %d raster tiles for %s", total, source.id)
        conn.close()

    # Convert MBTiles → PMTiles
    dest_dir = drive_path / "maps"
    dest_dir.mkdir(parents=True, exist_ok=True)
    pmtiles_path = dest_dir / f"{source.id}.pmtiles"

    if not pmtiles_path.exists():
        binary, image = _resolve_tool("pmtiles", drive_path)
        if binary:
            _run([binary, "convert", str(mbtiles_path), str(pmtiles_path)])
        elif image:
            dm = {str(cache): "/cache", str(dest_dir): "/dest"}
            run_container(
                ["pmtiles", "convert", _docker_path(mbtiles_path, dm), _docker_path(pmtiles_path, dm)],
                mounts=dm, check=True, capture_output=True, text=True,
            )
        else:
            raise RuntimeError("pmtiles CLI not found. Install go-pmtiles or Docker.")

    return BuildResult(source.id, True, artifact=pmtiles_path)


# ── mml-topo ───────────────────────────────────────────────────────────────

# Mapping from maastotietokanta GeoPackage table names to MML taustakartta
# vector-tile source-layer names.  Features carry a `kohdeluokka` attribute
# that the MML style JSON uses for filtering/coloring.
_MML_LAYER_GROUPS: dict[str, list[str]] = {
    "vesisto_alue": [
        "jarvi", "virtavesialue", "meri", "matalikko", "kaislikko",
        "maatuvavesialue", "tulvaalue",
    ],
    "vesisto_viiva": [
        "virtavesikapea", "koski", "lahde", "vesikivi", "vesikivikko",
    ],
    "maasto_alue": [
        "suo", "soistuma", "kallioalue", "kalliohalkeama", "kivikko",
        "hietikko", "jyrkanne", "luiska", "kivi", "niitty",
        "kansallispuisto", "luonnonpuisto", "luonnonsuojelualue",
        "rauhoitettukohde", "retkeilyalue", "suojametsa",
        "suojelualueenreunaviiva", "merkittavaluontokohde",
    ],
    "maankaytto": [
        "maatalousmaa", "metsamaankasvillisuus", "metsamaanmuokkaus",
        "metsanraja", "taajama", "puisto", "puu", "puurivi",
        "muuavoinalue", "autoliikennealue", "lentokenttaalue",
        "satamaalue", "hautausmaa", "kaatopaikka", "louhos",
        "maaaineksenottoalue", "varastoalue", "urheilujavirkistysalue",
        "taytemaa",
    ],
    "liikenne": [
        "tieviiva", "tiesymboli", "tienroteksti",
        "rautatie", "rautatieliikennepaikka", "rautatiensymboli",
        "vesikulkuvayla", "turvalaite", "ankkuripaikka", "hylky",
        "sahkolinja", "sahkolinjansymboli", "suurjannitelinjanpylvas",
        "muuntaja", "muuntoasema", "vesitorni",
        "aallonmurtaja", "aita", "allas", "masto", "mastonkorkeus",
        "savupiippu", "savupiipunkorkeus", "tuulivoimala",
        "nakotorni", "muistomerkki", "tulentekopaikka", "portti",
        "kellotapuli", "tervahauta", "pato", "tunnelinaukko",
        "rakennelma", "rakennusreunaviiva",
    ],
    "rakennus": ["rakennus"],
    "korkeus": [
        "korkeuskayra", "korkeuskayrankorkeusarvo",
        "syvyyskayra", "syvyyskayransyvyysarvo",
        "syvyyspiste", "viettoviiva",
    ],
    "maastoaluereuna": ["maastokuvionreuna"],
}


def _download_large(url: str, dest: Path, *, timeout: float = 7200) -> Path:
    """Stream-download a large file with progress logging."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    with httpx.stream("GET", url, follow_redirects=True, timeout=timeout) as r:
        r.raise_for_status()
        total = int(r.headers.get("content-length", 0))
        downloaded = 0
        last_log_mb = 0
        with open(dest, "wb") as f:
            for chunk in r.iter_bytes(chunk_size=1_048_576):
                f.write(chunk)
                downloaded += len(chunk)
                mb = downloaded // (1024 * 1024)
                if mb - last_log_mb >= 500:
                    total_mb = total // (1024 * 1024) if total else "?"
                    log.info("Downloaded %d / %s MB", mb, total_mb)
                    last_log_mb = mb
    return dest


def _gpkg_tables(gpkg_path: Path) -> set[str]:
    """Return the set of feature-table names in a GeoPackage."""
    import sqlite3
    conn = sqlite3.connect(str(gpkg_path))
    try:
        rows = conn.execute(
            "SELECT table_name FROM gpkg_contents WHERE data_type='features'"
        ).fetchall()
        return {r[0] for r in rows}
    finally:
        conn.close()


@_register("mml-topo")
def build_mml_topo(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Build vector PMTiles from MML maastotietokanta GeoPackage.

    Downloads the all-Finland GeoPackage(s) from kartat.kapsi.fi, clips to
    the requested bounding box, groups tables into MML taustakartta source-
    layers, and runs tippecanoe to produce a single multi-layer PMTiles file.
    """
    b = source.build
    maasto_url = b.get(
        "maasto_url",
        "http://kartat.kapsi.fi/files/maastotietokanta/geopackage_maasto/mtkmaasto.zip",
    )
    korkeus_url = b.get(
        "korkeus_url",
        "http://kartat.kapsi.fi/files/maastotietokanta/geopackage_korkeus/mtkkorkeus.zip",
    )
    bbox_str = b.get("bbox", "23.8,59.8,26.8,61.7")
    max_zoom = b.get("max_zoom", 14)

    raw_dir = cache / "raw"
    raw_dir.mkdir(parents=True, exist_ok=True)
    layers_dir = cache / "layers"
    layers_dir.mkdir(parents=True, exist_ok=True)

    docker_mounts = {str(cache): "/data", str(drive_path): "/drive"}

    # ── 1. Download & extract GeoPackages ─────────────────────────────────
    gpkg_files: list[Path] = []
    for label, url in [("maasto", maasto_url), ("korkeus", korkeus_url)]:
        zip_name = Path(urlparse(url).path).name
        zip_path = raw_dir / zip_name
        if not zip_path.exists():
            log.info("Downloading %s (%s)…", label, url)
            _download_large(url, zip_path)

        extract_dir = raw_dir / label
        if not extract_dir.exists():
            extract_dir.mkdir()
            log.info("Extracting %s…", zip_name)
            _safe_extract_zip(zip_path, extract_dir)

        for gpkg in extract_dir.rglob("*.gpkg"):
            gpkg_files.append(gpkg)

    if not gpkg_files:
        return BuildResult(source.id, False, error="No .gpkg files found after extraction")

    # ── 2. Export & merge tables per source-layer ─────────────────────────
    gpkg_table_cache: dict[str, set[str]] = {}
    for gpkg in gpkg_files:
        gpkg_table_cache[str(gpkg)] = _gpkg_tables(gpkg)

    available: set[str] = set()
    for tables in gpkg_table_cache.values():
        available.update(tables)

    west, south, east, north = bbox_str.split(",")

    for layer_name, tables in _MML_LAYER_GROUPS.items():
        merged = layers_dir / f"{layer_name}.geojsonseq"
        if merged.exists():
            continue

        parts_written = 0
        with open(merged, "wb") as out:
            for table in tables:
                if table not in available:
                    continue
                gpkg = next(
                    (g for g in gpkg_files if table in gpkg_table_cache[str(g)]),
                    None,
                )
                if gpkg is None:
                    continue

                part = layers_dir / f"_part_{table}.geojsonseq"
                try:
                    _run_ogr2ogr([
                        "-f", "GeoJSONSeq",
                        "-t_srs", "EPSG:4326",
                        "-spat", west, south, east, north,
                        "-spat_srs", "EPSG:4326",
                        _docker_path(part, docker_mounts),
                        _docker_path(gpkg, docker_mounts),
                        table,
                    ], drive_path, docker_mounts)
                    if part.exists() and part.stat().st_size > 0:
                        with open(part, "rb") as inp:
                            shutil.copyfileobj(inp, out)
                        parts_written += 1
                except Exception:
                    log.warning("Skipping table %s (ogr2ogr failed)", table)
                finally:
                    if part.exists():
                        part.unlink()

        if parts_written == 0:
            merged.unlink(missing_ok=True)
        else:
            log.info("Exported %s: %d tables merged", layer_name, parts_written)

    # ── 3. Run tippecanoe ─────────────────────────────────────────────────
    dest_dir = drive_path / "maps"
    dest_dir.mkdir(parents=True, exist_ok=True)
    pmtiles_path = dest_dir / f"{source.id}.pmtiles"

    if not pmtiles_path.exists():
        tc_args = [
            "-o", _docker_path(pmtiles_path, docker_mounts),
            f"-z{max_zoom}",
            "--drop-densest-as-needed",
            "--extend-zooms-if-still-dropping",
            "-P",  # parallel reads
        ]
        layer_files = sorted(layers_dir.glob("*.geojsonseq"))
        if not layer_files:
            return BuildResult(source.id, False, error="No layer data exported")

        for lf in layer_files:
            lname = lf.stem
            tc_args.extend(["-L", f"{lname}:{_docker_path(lf, docker_mounts)}"])

        _run_tippecanoe(tc_args, drive_path, docker_mounts)

    return BuildResult(source.id, True, artifact=pmtiles_path)
