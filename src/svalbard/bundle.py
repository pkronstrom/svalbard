"""Document bundle builder — packages PDFs, EPUBs, etc. into a browsable ZIM."""

from __future__ import annotations

import json
import re
import shutil
import subprocess
import zipfile
from pathlib import Path

from rich.console import Console

from svalbard.crawler import register_generated_zim
from svalbard.docker import TOOLS_IMAGE, has_docker, ensure_tools_image

console = Console()


def _title_from_filename(filename: str) -> str:
    """Derive a human-readable title from a filename."""
    stem = Path(filename).stem
    cleaned = re.sub(r"[-_]+", " ", stem)
    return cleaned.title()


def _build_collection_json(files: list[Path]) -> list[dict]:
    """Generate nautilus collection entries from a list of file paths."""
    return [
        {
            "title": _title_from_filename(f.name),
            "description": "",
            "files": [f.name],
        }
        for f in files
    ]


def _create_bundle_zip(files: list[Path], zip_path: Path) -> Path:
    """Create a ZIP archive with files and collection.json for nautiluszim."""
    collection = _build_collection_json(files)
    zip_path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(zip_path, "w", compression=zipfile.ZIP_STORED) as zf:
        zf.writestr("collection.json", json.dumps(collection, indent=2))
        for f in files:
            zf.write(f, f.name)
    return zip_path


def run_bundle_build(
    *,
    files: list[Path],
    name: str,
    workspace_root: Path,
    title: str | None = None,
    description: str | None = None,
    language: str = "eng",
) -> str:
    """Build a document bundle ZIM and register it as a local source.

    Returns the local source ID (e.g. 'local:my-bundle').
    """
    # Validate inputs early (before Docker checks)
    resolved_files: list[Path] = []
    for f in files:
        p = Path(f).expanduser().resolve()
        if not p.is_file():
            raise FileNotFoundError(f"Input file not found: {f}")
        resolved_files.append(p)

    if not has_docker():
        raise RuntimeError("Docker is not available. Install Docker to build bundles.")
    if not ensure_tools_image():
        raise RuntimeError("Failed to build svalbard-tools image.")

    resolved_title = title or _title_from_filename(name)
    resolved_description = description or f"Document bundle: {resolved_title}"

    # Prepare staging
    staging_dir = workspace_root / "library" / ".staging" / name
    staging_dir.mkdir(parents=True, exist_ok=True)

    # Copy files to staging (flat namespace, handle collisions)
    staged_files: list[Path] = []
    seen_names: set[str] = set()
    for f in resolved_files:
        dest_name = f.name
        if dest_name in seen_names:
            dest_name = f"{f.stem}-{len(seen_names)}{f.suffix}"
        seen_names.add(dest_name)
        dest = staging_dir / dest_name
        if not dest.exists() or dest.stat().st_mtime < f.stat().st_mtime:
            shutil.copy2(f, dest)
        staged_files.append(dest)

    # Create ZIP
    zip_path = staging_dir / f"{name}.zip"
    _create_bundle_zip(staged_files, zip_path)

    # Run nautiluszim in Docker
    cmd = [
        "docker", "run", "--rm",
        "-v", f"{staging_dir}:/input:ro",
        "-v", f"{workspace_root / 'library'}:/output",
        TOOLS_IMAGE,
        "nautiluszim",
        "--archive", f"/input/{zip_path.name}",
        "--name", name,
        "--title", resolved_title,
        "--description", resolved_description,
        "--language", language,
        "--output", "/output",
    ]
    console.print(f"[bold]Building bundle:[/bold] {resolved_title} ({len(resolved_files)} files)")
    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"nautiluszim failed with exit code {result.returncode}")

    # Find the output ZIM (nautiluszim may add date suffix)
    artifact_path = workspace_root / "library" / f"{name}.zim"
    if not artifact_path.exists():
        candidates = sorted((workspace_root / "library").glob(f"{name}*.zim"))
        if not candidates:
            raise RuntimeError("nautiluszim did not produce an output ZIM")
        artifact_path = candidates[-1]

    return register_generated_zim(
        workspace_root=workspace_root,
        artifact_path=artifact_path,
        origin_url="bundle:" + ",".join(f.name for f in resolved_files),
        kind="bundle",
        runner="docker",
        tool="nautiluszim",
        source_id=f"local:{name}",
    )
