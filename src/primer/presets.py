from importlib.resources import files
from pathlib import Path

import yaml

from primer.models import Preset, Source

PRESETS_DIR = Path(str(files("primer") / "presets"))


def load_preset(name: str) -> Preset:
    """Load a preset by name (e.g. 'nordic-128')."""
    path = PRESETS_DIR / f"{name}.yaml"
    if not path.exists():
        raise FileNotFoundError(f"Preset not found: {path}")
    return parse_preset(path)


def parse_preset(path: Path) -> Preset:
    """Parse a preset YAML file into a Preset object."""
    with open(path) as f:
        data = yaml.safe_load(f)
    sources = [Source(**s) for s in data.get("sources", [])]
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
