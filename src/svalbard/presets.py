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
            if data and "id" in data:
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


def parse_preset(path: Path, recipe_index: dict[str, dict] | None = None) -> Preset:
    """Parse a preset YAML file, resolving source IDs from recipes."""
    with open(path) as f:
        data = yaml.safe_load(f)

    recipe_index = recipe_index or _build_recipe_index()
    sources = []
    for entry in data.get("sources", []):
        if isinstance(entry, str):
            # Simple ID reference
            source_id = entry
            recipe = recipe_index.get(source_id)
            if recipe is None:
                raise ValueError(
                    f"Recipe '{source_id}' not found (referenced in preset '{data['name']}')"
                )
            sources.append(_source_from_recipe(recipe))
        elif isinstance(entry, dict):
            if "id" in entry and "type" not in entry:
                # ID reference with overrides
                source_id = entry["id"]
                recipe = recipe_index.get(source_id)
                if recipe is None:
                    raise ValueError(
                        f"Recipe '{source_id}' not found (referenced in preset '{data['name']}')"
                    )
                merged = dict(recipe)
                overrides = entry.get("override", {})
                merged.update(overrides)
                sources.append(_source_from_recipe(merged))
            else:
                # Inline source definition (backwards compat)
                sources.append(_source_from_recipe(entry))

    return Preset(
        name=data["name"],
        description=data["description"],
        target_size_gb=data["target_size_gb"],
        region=data["region"],
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
