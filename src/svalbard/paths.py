from __future__ import annotations

from pathlib import Path


def builtin_root() -> Path:
    """Return the packaged/built-in Svalbard data root."""
    return Path(__file__).resolve().parent.parent.parent


def default_workspace_root() -> Path:
    """Return the default user-owned workspace root."""
    return Path.home() / ".local" / "share" / "svalbard"


def looks_like_workspace(path: Path) -> bool:
    """Return True when the path looks like a Svalbard workspace."""
    return any(
        (path / name).exists()
        for name in ("presets", "recipes", "generated")
    )


def workspace_root(explicit: Path | str | None = None, *, cwd: Path | None = None) -> Path:
    """Resolve the active workspace root."""
    if explicit is not None:
        return Path(explicit).resolve()
    current = (cwd or Path.cwd()).resolve()
    if looks_like_workspace(current):
        return current
    return default_workspace_root().resolve()
