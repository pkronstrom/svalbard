from pathlib import Path

import yaml

from svalbard.models import License, Preset, Source
from svalbard.paths import builtin_root, workspace_root as resolve_workspace_root

# Resolve built-in paths relative to the packaged project root.
_PROJECT_ROOT = builtin_root()
PRESETS_DIR = _PROJECT_ROOT / "presets"
RECIPES_DIRS = [
    _PROJECT_ROOT / "recipes",
]


def _source_from_recipe(recipe: dict) -> Source:
    """Build a Source from a recipe dict, converting nested structures."""
    kwargs = {k: v for k, v in recipe.items() if k in Source.__dataclass_fields__}
    if "license" in kwargs and isinstance(kwargs["license"], dict):
        kwargs["license"] = License(**kwargs["license"])
    return Source(**kwargs)


def _build_recipe_index(recipe_dirs: list[Path] | None = None) -> dict[str, dict]:
    """Scan all recipe directories and build an id-keyed index."""
    index: dict[str, dict] = {}
    for recipes_dir in recipe_dirs or RECIPES_DIRS:
        if not recipes_dir.exists():
            continue
        for path in recipes_dir.rglob("*.yaml"):
            with open(path) as f:
                data = yaml.safe_load(f)
            if data and "id" in data and data.get("strategy") != "local":
                index[data["id"]] = data
    return index


def builtin_recipe_ids() -> set[str]:
    """Return built-in recipe ids from checked-in recipe files."""
    return set(_build_recipe_index())


def workspace_presets_dir(workspace: Path | str | None = None) -> Path:
    """Return the workspace-owned preset directory."""
    root = resolve_workspace_root(workspace)
    if root == _PROJECT_ROOT:
        return root / ".svalbard" / "presets"
    return root / "presets"


def _workspace_preset_path(name: str, workspace: Path | str | None = None) -> Path:
    return workspace_presets_dir(workspace) / f"{name}.yaml"


def resolve_preset_path(name: str, workspace: Path | str | None = None) -> Path:
    """Resolve a preset path from workspace-owned or built-in presets."""
    workspace_path = _workspace_preset_path(name, workspace)
    builtin_path = PRESETS_DIR / f"{name}.yaml"
    if workspace_path.exists() and builtin_path.exists():
        raise ValueError(f"Workspace preset '{name}' collides with built-in preset")
    if workspace_path.exists():
        return workspace_path
    if builtin_path.exists():
        return builtin_path
    raise FileNotFoundError(f"Preset not found: {name}")


def load_preset(name: str, workspace: Path | str | None = None) -> Preset:
    """Load a preset by name (e.g. 'finland-128'), resolving recipe references."""
    path = resolve_preset_path(name, workspace)
    return parse_preset(path)


def parse_preset(
    path: Path,
    recipe_index: dict[str, dict] | None = None,
    _seen: set[str] | None = None,
) -> Preset:
    """Parse a preset YAML file, resolving source IDs from recipes.

    Supports ``extends: base-preset`` to inherit sources from another preset.
    Sources prefixed with ``-`` in the extending preset are removed from the
    inherited set.  New sources are appended.
    """
    with open(path) as f:
        data = yaml.safe_load(f)

    recipe_index = recipe_index or _build_recipe_index()

    # ── Resolve extends chain ────────────────────────────────────────────
    _seen = _seen or set()
    preset_name = data.get("name", path.stem)
    if preset_name in _seen:
        raise ValueError(f"Circular preset extends: {preset_name}")
    _seen.add(preset_name)

    inherited_sources: list[Source] = []
    if "extends" in data:
        extends = data["extends"]
        base_names = [extends] if isinstance(extends, str) else extends
        workspace = path.parent if path.parent.name == "presets" else None
        seen_ids: set[str] = set()
        for base_name in base_names:
            base_path = resolve_preset_path(base_name, workspace)
            base_preset = parse_preset(base_path, recipe_index=recipe_index, _seen=_seen)
            for s in base_preset.sources:
                if s.id not in seen_ids:
                    inherited_sources.append(s)
                    seen_ids.add(s.id)

    # ── Process this preset's source list ────────────────────────────────
    removals: set[str] = set()
    additions: list[Source] = []

    for entry in data.get("sources", []):
        if isinstance(entry, str):
            if entry.startswith("-"):
                # Remove inherited source
                removals.add(entry[1:].strip())
                continue
            source_id = entry
            recipe = recipe_index.get(source_id)
            if recipe is None:
                raise ValueError(
                    f"Recipe '{source_id}' not found (referenced in preset '{preset_name}')"
                )
            additions.append(_source_from_recipe(recipe))
        elif isinstance(entry, dict):
            if "id" in entry and "type" not in entry:
                source_id = entry["id"]
                recipe = recipe_index.get(source_id)
                if recipe is None:
                    raise ValueError(
                        f"Recipe '{source_id}' not found (referenced in preset '{preset_name}')"
                    )
                merged = dict(recipe)
                overrides = entry.get("override", {})
                merged.update(overrides)
                additions.append(_source_from_recipe(merged))
            else:
                additions.append(_source_from_recipe(entry))

    # ── Merge: inherited (minus removals) + additions ────────────────────
    sources = [s for s in inherited_sources if s.id not in removals]
    existing_ids = {s.id for s in sources}
    for s in additions:
        if s.id not in existing_ids:
            sources.append(s)
            existing_ids.add(s.id)

    return Preset(
        name=preset_name,
        description=data.get("description", ""),
        target_size_gb=data.get("target_size_gb", 0),
        region=data.get("region", ""),
        sources=sources,
    )


def recipe_data_by_id(source_id: str) -> dict:
    """Return the raw built-in recipe data for a source id."""
    recipe = _build_recipe_index().get(source_id)
    if recipe is None:
        raise KeyError(source_id)
    return recipe


def list_presets(workspace: Path | str | None = None) -> list[str]:
    """List available preset names."""
    names: set[str] = set()
    if PRESETS_DIR.exists():
        names.update(p.stem for p in PRESETS_DIR.glob("*.yaml"))
    workspace_dir = workspace_presets_dir(workspace)
    if workspace_dir.exists():
        names.update(p.stem for p in workspace_dir.glob("*.yaml"))
    return sorted(names)


def copy_preset_to_workspace(
    source_name: str,
    target_name: str,
    workspace: Path | str | None = None,
) -> Path:
    """Copy a preset into the workspace-owned preset directory."""
    source_path = resolve_preset_path(source_name, workspace)
    target_path = workspace_presets_dir(workspace) / f"{target_name}.yaml"
    target_path.parent.mkdir(parents=True, exist_ok=True)
    if target_path.exists():
        raise FileExistsError(f"Preset already exists: {target_path}")
    target_path.write_text(source_path.read_text())
    return target_path
