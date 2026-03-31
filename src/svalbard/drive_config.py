from __future__ import annotations

import re
import shutil
from pathlib import Path

import yaml

from svalbard.local_sources import load_local_sources
from svalbard.models import Preset
from svalbard.presets import (
    load_preset,
    parse_preset,
    recipe_data_by_id,
    resolve_preset_path,
)


def _slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "config"


def config_root(drive_path: Path) -> Path:
    return drive_path / ".svalbard" / "config"


def _copy_yaml(path: Path, dest: Path) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, dest)


def _local_snapshot_path(drive_path: Path, source_id: str) -> Path:
    slug = source_id.split(":", 1)[-1]
    return config_root(drive_path) / "local" / f"{_slugify(slug)}.yaml"


def _local_recipe_path(workspace_root: Path, source_id: str) -> Path:
    local_dir = workspace_root / "recipes" / "local"
    for path in sorted(local_dir.glob("*.yaml")):
        data = yaml.safe_load(path.read_text()) or {}
        if data.get("id") == source_id:
            return path
    raise FileNotFoundError(f"Local source sidecar not found for {source_id}")


def write_local_source_snapshot(drive_path: Path, source_id: str, workspace_root: Path) -> None:
    source_path = _local_recipe_path(workspace_root, source_id)
    _copy_yaml(source_path, _local_snapshot_path(drive_path, source_id))


def remove_local_source_snapshot(drive_path: Path, source_id: str) -> None:
    snapshot = _local_snapshot_path(drive_path, source_id)
    if snapshot.exists():
        snapshot.unlink()


def write_drive_snapshot(
    drive_path: Path,
    *,
    preset_name: str,
    workspace_root: Path,
    local_source_ids: list[str],
) -> None:
    root = config_root(drive_path)
    recipes_dir = root / "recipes"
    local_dir = root / "local"
    root.mkdir(parents=True, exist_ok=True)
    recipes_dir.mkdir(parents=True, exist_ok=True)
    local_dir.mkdir(parents=True, exist_ok=True)

    preset_path = resolve_preset_path(preset_name, workspace_root)
    _copy_yaml(preset_path, root / "preset.yaml")

    # Replace the recipe snapshot set with the exact recipes referenced by the preset.
    for existing in recipes_dir.glob("*.yaml"):
        existing.unlink()

    preset = load_preset(preset_name, workspace=workspace_root)
    for source in preset.sources:
        recipe = recipe_data_by_id(source.id)
        recipe_path = recipes_dir / f"{_slugify(source.id)}.yaml"
        recipe_path.write_text(yaml.safe_dump(recipe, sort_keys=False))

    for existing in local_dir.glob("*.yaml"):
        existing.unlink()
    for source_id in local_source_ids:
        write_local_source_snapshot(drive_path, source_id, workspace_root)


def load_snapshot_preset(drive_path: Path) -> Preset | None:
    root = config_root(drive_path)
    preset_path = root / "preset.yaml"
    recipes_dir = root / "recipes"
    if not preset_path.exists() or not recipes_dir.exists():
        return None

    recipe_index: dict[str, dict] = {}
    for path in sorted(recipes_dir.glob("*.yaml")):
        data = yaml.safe_load(path.read_text()) or {}
        if "id" in data:
            recipe_index[data["id"]] = data
    return parse_preset(preset_path, recipe_index=recipe_index)
