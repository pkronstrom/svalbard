from __future__ import annotations

from pathlib import Path

from svalbard.drive_config import remove_local_source_snapshot, write_local_source_snapshot
from svalbard.manifest import Manifest
from svalbard.paths import workspace_root as resolve_workspace_root


def resolve_drive_path(path: str | None = None) -> Path:
    candidate = Path(path or ".").resolve()
    if candidate.is_file():
        candidate = candidate.parent
    if not (candidate / "manifest.yaml").exists():
        raise FileNotFoundError(f"No drive manifest found at {candidate}")
    return candidate


def _resolve_workspace_for_drive(manifest: Manifest, workspace: Path | str | None = None) -> Path:
    if workspace is not None:
        return resolve_workspace_root(workspace)
    if manifest.workspace_root:
        return Path(manifest.workspace_root).resolve()
    return resolve_workspace_root()


def attach_local_source(drive_path: Path, source_id: str, workspace: Path | str | None = None) -> None:
    manifest_path = drive_path / "manifest.yaml"
    manifest = Manifest.load(manifest_path)
    if source_id not in manifest.local_sources:
        manifest.local_sources.append(source_id)
    manifest.save(manifest_path)

    workspace_root = _resolve_workspace_for_drive(manifest, workspace)
    write_local_source_snapshot(drive_path, source_id, workspace_root)


def detach_local_source(drive_path: Path, source_id: str) -> None:
    manifest_path = drive_path / "manifest.yaml"
    manifest = Manifest.load(manifest_path)
    manifest.local_sources = [item for item in manifest.local_sources if item != source_id]
    manifest.local_source_snapshots = [
        snapshot for snapshot in manifest.local_source_snapshots if snapshot.id != source_id
    ]
    manifest.entries = [entry for entry in manifest.entries if entry.id != source_id]
    manifest.save(manifest_path)

    remove_local_source_snapshot(drive_path, source_id)
