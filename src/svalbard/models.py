from dataclasses import dataclass, field


@dataclass
class License:
    id: str = ""  # SPDX identifier (e.g. CC-BY-SA-3.0, Apache-2.0)
    attribution: str = ""
    url: str = ""
    noncommercial: bool = False
    redistribution: str = ""  # "allowed" (default), "prohibited"
    note: str = ""


@dataclass
class Source:
    id: str
    type: str  # zim, pmtiles, pdf, gguf, binary, app, iso
    group: str = ""  # reference, practical, education, maps, regional, models, tools
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"  # comprehensive, overview, reference-only
    size_gb: float = 0.0
    url: str = ""
    url_pattern: str = ""  # pattern with {date} placeholder
    platforms: dict[str, str] = field(default_factory=dict)
    description: str = ""
    sha256: str = ""  # expected hash (if empty, try fetching .sha256 sidecar)
    license: License | None = None
    strategy: str = "download"  # "download" or "build"
    build: dict = field(default_factory=dict)  # opaque config for builder
    path: str = ""
    size_bytes: int = 0

    def __post_init__(self) -> None:
        if self.size_bytes and not self.size_gb:
            self.size_gb = self.size_bytes / 1e9


@dataclass
class Preset:
    name: str
    description: str
    target_size_gb: float
    region: str
    sources: list[Source] = field(default_factory=list)

    @property
    def total_size_gb(self) -> float:
        return sum(s.size_gb for s in self.sources)
