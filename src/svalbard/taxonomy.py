from dataclasses import dataclass
from importlib.resources import files
from pathlib import Path

import yaml

from svalbard.models import Source

DATA_DIR = Path(str(files("svalbard") / "data"))

DEPTH_SCORES = {
    "comprehensive": 30,
    "overview": 15,
    "reference-only": 10,
}


@dataclass
class DomainCoverage:
    domain: str
    group: str
    score: int  # 0-100
    sources: list[str]  # source ids contributing
    depth_breakdown: dict[str, int]  # depth -> count


def load_taxonomy() -> dict[str, list[str]]:
    """Load taxonomy: returns {group_name: [domain, ...]}."""
    path = DATA_DIR / "taxonomy.yaml"
    with open(path) as f:
        data = yaml.safe_load(f)
    return {g: info["domains"] for g, info in data["groups"].items()}


def all_domains(taxonomy: dict[str, list[str]]) -> dict[str, str]:
    """Returns {domain: group} mapping."""
    result = {}
    for group, domains in taxonomy.items():
        for d in domains:
            result[d] = group
    return result


def compute_coverage(
    sources: list[Source], taxonomy: dict[str, list[str]]
) -> list[DomainCoverage]:
    """Compute coverage scores for all domains based on provided sources."""
    domain_to_group = all_domains(taxonomy)
    results = []
    for domain, group in sorted(domain_to_group.items()):
        contributing = []
        depth_breakdown: dict[str, int] = {}
        raw_score = 0
        for s in sources:
            if domain in s.tags:
                contributing.append(s.id)
                depth_breakdown[s.depth] = depth_breakdown.get(s.depth, 0) + 1
                raw_score += DEPTH_SCORES.get(s.depth, 0)
        results.append(
            DomainCoverage(
                domain=domain,
                group=group,
                score=min(raw_score, 100),
                sources=contributing,
                depth_breakdown=depth_breakdown,
            )
        )
    return results
