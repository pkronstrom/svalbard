from pathlib import Path

import yaml

from svalbard.manifest import Manifest
from svalbard.models import License, Source
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.presets import builtin_recipe_ids


def workspace_root(explicit: Path | str | None = None) -> Path:
    """Return the canonical workspace root."""
    return resolve_workspace_root(explicit)


def _source_from_recipe(recipe: dict) -> Source:
    kwargs = {k: v for k, v in recipe.items() if k in Source.__dataclass_fields__}
    if "license" in kwargs and isinstance(kwargs["license"], dict):
        kwargs["license"] = License(**kwargs["license"])
    return Source(**kwargs)


def _artifact_exists(root: Path, source: Source) -> bool:
    path = Path(source.path)
    if not path.is_absolute():
        path = root / path
    return path.exists()


def load_local_sources(root: Path | str | None = None) -> list[Source]:
    """Load local source sidecars from workspace local/."""
    root_path = workspace_root(root)
    local_dir = root_path / "local"
    if not local_dir.exists():
        return []

    builtin_ids = builtin_recipe_ids()
    sources: list[Source] = []
    for recipe_path in sorted(local_dir.glob("*.yaml")):
        with open(recipe_path) as f:
            recipe = yaml.safe_load(f) or {}
        source = _source_from_recipe(recipe)
        if source.id in builtin_ids:
            raise ValueError(f"Local source id '{source.id}' collides with built-in recipe id")
        if not _artifact_exists(root_path, source):
            continue
        if source.size_bytes and not source.size_gb:
            source.size_gb = source.size_bytes / 1e9
        sources.append(source)
    return sources


def active_sources_for_manifest(manifest: Manifest, preset) -> list[Source]:
    """Return preset sources plus selected local sources for a drive manifest."""
    local_map = {source.id: source for source in load_local_sources(manifest.workspace_root or None)}
    snapshots = {snapshot.id: snapshot for snapshot in manifest.local_source_snapshots}
    selected: list[Source] = []
    for source_id in manifest.local_sources:
        if source_id in local_map:
            selected.append(local_map[source_id])
            continue
        snapshot = snapshots.get(source_id)
        selected.append(
            Source(
                id=source_id,
                type="unknown",
                group="local",
                strategy="local",
                path=snapshot.path if snapshot else "",
                size_bytes=snapshot.size_bytes if snapshot else 0,
                description=f"Missing local source ({source_id})",
            )
        )
    return [*preset.sources, *selected]
