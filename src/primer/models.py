from dataclasses import dataclass, field


@dataclass
class Source:
    id: str
    type: str  # zim, pmtiles, pdf, gguf, binary, app, iso
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"  # comprehensive, overview, reference-only
    size_gb: float = 0.0
    url: str = ""
    url_pattern: str = ""  # pattern with {date} placeholder
    replaces: str = ""  # id of source this replaces in higher tiers
    optional_group: str = ""  # maps, models, installers, infra
    description: str = ""


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

    def sources_for_options(self, enabled_groups: set[str]) -> list[Source]:
        """Filter sources based on enabled optional groups."""
        result = []
        for s in self.sources:
            if s.optional_group and s.optional_group not in enabled_groups:
                continue
            result.append(s)
        return result
