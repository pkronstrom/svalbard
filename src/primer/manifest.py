from dataclasses import dataclass, field, asdict
from pathlib import Path
import yaml

@dataclass
class ManifestEntry:
    id: str
    type: str
    filename: str
    size_bytes: int
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"
    downloaded: str = ""  # ISO date
    url: str = ""
    checksum_sha256: str = ""

@dataclass
class Manifest:
    preset: str
    region: str
    target_path: str
    created: str = ""
    last_synced: str = ""
    enabled_groups: list[str] = field(default_factory=list)
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
        return cls(
            preset=data["preset"], region=data["region"],
            target_path=data["target_path"],
            created=data.get("created", ""),
            last_synced=data.get("last_synced", ""),
            enabled_groups=data.get("enabled_groups", []),
            entries=entries,
        )

    @classmethod
    def exists(cls, drive_path: Path) -> bool:
        return (drive_path / "manifest.yaml").exists()

    def entry_by_id(self, source_id: str) -> ManifestEntry | None:
        for e in self.entries:
            if e.id == source_id:
                return e
        return None
