from pathlib import Path

import yaml

from svalbard.models import License, Preset, Source

# Resolve paths relative to the project root (two levels up from this file)
_PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
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


def _build_recipe_index() -> dict[str, dict]:
    """Scan all recipe directories and build an id-keyed index."""
    index: dict[str, dict] = {}
    for recipes_dir in RECIPES_DIRS:
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


def load_preset(name: str) -> Preset:
    """Load a preset by name (e.g. 'finland-128'), resolving recipe references."""
    path = PRESETS_DIR / f"{name}.yaml"
    if not path.exists():
        raise FileNotFoundError(f"Preset not found: {path}")
    return parse_preset(path)


def parse_preset(path: Path) -> Preset:
    """Parse a preset YAML file, resolving source IDs from recipes."""
    with open(path) as f:
        data = yaml.safe_load(f)

    recipe_index = _build_recipe_index()
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


def list_presets() -> list[str]:
    """List available preset names."""
    if not PRESETS_DIR.exists():
        return []
    return sorted(p.stem for p in PRESETS_DIR.glob("*.yaml"))
