from dataclasses import dataclass, field, asdict
from pathlib import Path
import yaml


@dataclass
class LocalSourceSnapshot:
    id: str
    path: str
    kind: str
    size_bytes: int
    mtime: float = 0.0
    checksum_sha256: str = ""


@dataclass
class ManifestEntry:
    id: str
    type: str
    filename: str
    size_bytes: int
    platform: str = ""
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"
    downloaded: str = ""  # ISO date
    url: str = ""
    checksum_sha256: str = ""
    relative_path: str = ""
    kind: str = "file"
    source_strategy: str = ""

@dataclass
class Manifest:
    preset: str
    region: str
    target_path: str
    created: str = ""
    last_synced: str = ""
    workspace_root: str = ""
    local_sources: list[str] = field(default_factory=list)
    local_source_snapshots: list[LocalSourceSnapshot] = field(default_factory=list)
    entries: list[ManifestEntry] = field(default_factory=list)

    def save(self, path: Path):
        """Save manifest to YAML."""
        data = asdict(self)
        with open(path, "w") as f:
            yaml.dump(data, f, default_flow_style=False, sort_keys=False)

    @classmethod
    def load(cls, path: Path) -> "Manifest":
        """Load manifest from YAML."""
        with open(path) as f:
            data = yaml.safe_load(f)
        entries = [ManifestEntry(**e) for e in data.get("entries", [])]
        snapshots = [LocalSourceSnapshot(**e) for e in data.get("local_source_snapshots", [])]
        return cls(
            preset=data["preset"], region=data["region"],
            target_path=data["target_path"],
            created=data.get("created", ""),
            last_synced=data.get("last_synced", ""),
            workspace_root=data.get("workspace_root", ""),
            local_sources=data.get("local_sources", []),
            local_source_snapshots=snapshots,
            entries=entries,
        )

    @classmethod
    def exists(cls, drive_path: Path) -> bool:
        return (drive_path / "manifest.yaml").exists()

    def entry_by_id(self, source_id: str, platform: str = "") -> ManifestEntry | None:
        for e in self.entries:
            if e.id == source_id and e.platform == platform:
                return e
        return None

    def entries_by_id(self, source_id: str) -> list[ManifestEntry]:
        return [entry for entry in self.entries if entry.id == source_id]
