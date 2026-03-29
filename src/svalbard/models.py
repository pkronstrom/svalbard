from dataclasses import dataclass, field


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
