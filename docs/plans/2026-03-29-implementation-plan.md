# Primer v1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a working `primer` CLI that can provision a USB stick or SSD with curated offline knowledge via an interactive wizard or CLI commands.

**Architecture:** Python package with Click for CLI routing, Rich for TUI/output, PyYAML for preset configs. Downloads via aria2c (fallback to wget/curl). Presets are YAML files defining sources with URLs, tags, and sizes. The tool writes a manifest to the drive and generates a serve.sh script.

**Tech Stack:** Python 3.11+, Click, Rich, PyYAML, httpx (for URL scraping), shutil (disk info)

---

### Task 1: Project scaffolding

**Files:**
- Create: `pyproject.toml`
- Create: `src/primer/__init__.py`
- Create: `src/primer/cli.py`
- Create: `src/primer/__main__.py`
- Create: `Brewfile`
- Create: `.gitignore`

**Step 1: Create pyproject.toml**

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "primer-kit"
version = "0.1.0"
description = "Offline knowledge kit provisioner"
requires-python = ">=3.11"
dependencies = [
    "click>=8.1",
    "rich>=13.0",
    "pyyaml>=6.0",
    "httpx>=0.27",
]

[project.scripts]
primer = "primer.cli:main"

[tool.hatch.build.targets.wheel]
packages = ["src/primer"]

[tool.pytest.ini_options]
testpaths = ["tests"]
```

**Step 2: Create src/primer/__init__.py**

```python
"""Primer — Offline knowledge kit provisioner."""
```

**Step 3: Create src/primer/__main__.py**

```python
from primer.cli import main

main()
```

**Step 4: Create src/primer/cli.py with basic Click group**

```python
import click
from rich.console import Console

console = Console()


@click.group(invoke_without_command=True)
@click.pass_context
def main(ctx):
    """Primer — Offline knowledge kit provisioner."""
    if ctx.invoked_subcommand is None:
        console.print("[bold]Primer[/bold] — Offline knowledge kit provisioner")
        console.print("Run [bold]primer wizard[/bold] to get started.")
        console.print("Run [bold]primer --help[/bold] for all commands.")


@main.command()
def wizard():
    """Interactive setup wizard."""
    console.print("[bold]Wizard not yet implemented.[/bold]")


@main.command()
@click.argument("path")
@click.option("--preset", required=True, help="Preset name (e.g. nordic-128)")
def init(path, preset):
    """Initialize a drive with a preset."""
    console.print(f"[bold]Init not yet implemented.[/bold] path={path} preset={preset}")


@main.command()
def sync():
    """Download/update content on initialized drive."""
    console.print("[bold]Sync not yet implemented.[/bold]")


@main.command()
def status():
    """Show what's downloaded, what's stale."""
    console.print("[bold]Status not yet implemented.[/bold]")


@main.command()
def audit():
    """Generate LLM-ready gap analysis report."""
    console.print("[bold]Audit not yet implemented.[/bold]")
```

**Step 5: Create .gitignore**

```
__pycache__/
*.pyc
*.egg-info/
dist/
build/
.venv/
*.zim
*.pmtiles
*.gguf
.DS_Store
```

**Step 6: Create Brewfile**

```ruby
brew "aria2"
brew "python@3.11"
brew "kiwix-tools"
cask "kiwix"
cask "lm-studio"
```

**Step 7: Install in dev mode and verify**

Run: `cd /Users/bembu/Projects/primer && python -m venv .venv && source .venv/bin/activate && pip install -e ".[dev]" 2>&1 | tail -5`

Then: `primer --help`

Expected: Shows Click help with wizard, init, sync, status, audit commands.

**Step 8: Commit**

```bash
git add pyproject.toml src/ .gitignore Brewfile
git commit -m "Project scaffolding: Click CLI, Rich, PyYAML, Brewfile"
```

---

### Task 2: Preset loader and data model

**Files:**
- Create: `src/primer/models.py`
- Create: `src/primer/presets.py`
- Create: `presets/nordic-128.yaml`
- Create: `tests/test_presets.py`

**Step 1: Create src/primer/models.py with dataclasses**

```python
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
```

**Step 2: Create the first preset presets/nordic-128.yaml**

```yaml
name: Nordic 128GB
description: Grab-and-go survival kit, Nordic/Finnish focus
target_size_gb: 120
region: nordic

sources:
  - id: wikipedia-en-nopic
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim
    tags: [general-reference, medicine, agriculture, engineering, science]
    depth: comprehensive
    size_gb: 25

  - id: wikipedia-fi-nopic
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_fi_all_nopic_{date}.zim
    tags: [general-reference, language]
    depth: comprehensive
    size_gb: 5

  - id: wiktionary-en
    type: zim
    url_pattern: https://download.kiwix.org/zim/wiktionary/wiktionary_en_all_maxi_{date}.zim
    tags: [language, general-reference]
    depth: comprehensive
    size_gb: 5

  - id: wiktionary-fi
    type: zim
    url_pattern: https://download.kiwix.org/zim/wiktionary/wiktionary_fi_all_maxi_{date}.zim
    tags: [language]
    depth: comprehensive
    size_gb: 2

  - id: wikimed
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_maxi_{date}.zim
    tags: [medicine, emergency-medicine, first-aid]
    depth: comprehensive
    size_gb: 2

  - id: wikihow-en
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikihow/wikihow_en_all_maxi_{date}.zim
    tags: [water, fire-shelter, food-foraging, repair, cooking, general-reference]
    depth: comprehensive
    size_gb: 15

  - id: ifixit
    type: zim
    url_pattern: https://download.kiwix.org/zim/ifixit/ifixit_en_all_{date}.zim
    tags: [repair, electronics, mechanical]
    depth: comprehensive
    size_gb: 5

  - id: stackexchange-survival
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/survival.stackexchange.com_en_all_{date}.zim
    tags: [water, fire-shelter, food-foraging, navigation, first-aid, self-defense]
    depth: comprehensive
    size_gb: 0.5

  - id: stackexchange-diy
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/diy.stackexchange.com_en_all_{date}.zim
    tags: [civil-construction, woodworking, mechanical, repair]
    depth: comprehensive
    size_gb: 2

  - id: stackexchange-home-improvement
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/diy.stackexchange.com_en_all_{date}.zim
    tags: [civil-construction, repair, energy-power]
    depth: comprehensive
    size_gb: 3

  - id: stackexchange-gardening
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/gardening.stackexchange.com_en_all_{date}.zim
    tags: [gardening, agriculture, food-preservation]
    depth: comprehensive
    size_gb: 1

  - id: stackexchange-cooking
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/cooking.stackexchange.com_en_all_{date}.zim
    tags: [cooking, food-preservation]
    depth: comprehensive
    size_gb: 0.5

  - id: stackexchange-amateur-radio
    type: zim
    url_pattern: https://download.kiwix.org/zim/stack_exchange/ham.stackexchange.com_en_all_{date}.zim
    tags: [radio-comms, electronics]
    depth: comprehensive
    size_gb: 0.5

  - id: practical-action
    type: zim
    url_pattern: https://download.kiwix.org/zim/other/practical_action_en_all_{date}.zim
    tags: [water, energy-power, agriculture, civil-construction]
    depth: comprehensive
    size_gb: 1

  - id: wikibooks-en
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikibooks/wikibooks_en_all_maxi_{date}.zim
    tags: [education-pedagogy, science, engineering, computing]
    depth: comprehensive
    size_gb: 3

  - id: khan-academy-lite
    type: zim
    url_pattern: https://download.kiwix.org/zim/khan_academy/khan_academy_en_all_{date}.zim
    tags: [education-pedagogy, mathematics, physics, chemistry, biology]
    depth: comprehensive
    size_gb: 10

  - id: gutenberg-subset
    type: zim
    url_pattern: https://download.kiwix.org/zim/gutenberg/gutenberg_en_all_{date}.zim
    tags: [general-reference, agriculture, mechanical, medicine, history]
    depth: overview
    size_gb: 10

  - id: osm-finland-nordics
    type: pmtiles
    url: https://build.protomaps.com/20250301.pmtiles
    tags: [maps-geo, navigation]
    depth: comprehensive
    size_gb: 8
    optional_group: maps
    description: OpenStreetMap Finland + Nordics

  - id: cyberchef
    type: app
    url: https://gchq.github.io/CyberChef/CyberChef.html
    tags: [computing]
    depth: overview
    size_gb: 0.015
    description: Data encoding/decoding/analysis tool (single HTML file)

  - id: kiwix-serve-binaries
    type: binary
    url: https://download.kiwix.org/release/kiwix-tools/kiwix-tools_linux-x86_64.tar.gz
    tags: [general-reference]
    depth: comprehensive
    size_gb: 0.05
    description: kiwix-serve static binaries for 3 platforms

  - id: go-pmtiles-binaries
    type: binary
    url: https://github.com/protomaps/go-pmtiles/releases
    tags: [maps-geo]
    depth: comprehensive
    size_gb: 0.03
    optional_group: maps
    description: PMTiles server static binaries for 3 platforms
```

**Step 3: Create src/primer/presets.py**

```python
from pathlib import Path

import yaml

from primer.models import Preset, Source

PRESETS_DIR = Path(__file__).parent.parent.parent / "presets"


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
```

**Step 4: Write test**

```python
from pathlib import Path

from primer.presets import parse_preset, list_presets


def test_parse_nordic_128():
    path = Path(__file__).parent.parent / "presets" / "nordic-128.yaml"
    preset = parse_preset(path)
    assert preset.name == "Nordic 128GB"
    assert preset.region == "nordic"
    assert preset.target_size_gb == 120
    assert len(preset.sources) > 10
    assert preset.total_size_gb > 50


def test_sources_have_required_fields():
    path = Path(__file__).parent.parent / "presets" / "nordic-128.yaml"
    preset = parse_preset(path)
    for s in preset.sources:
        assert s.id, f"Source missing id"
        assert s.type, f"Source {s.id} missing type"
        assert s.tags, f"Source {s.id} missing tags"
        assert s.size_gb > 0, f"Source {s.id} missing size_gb"


def test_list_presets():
    names = list_presets()
    assert "nordic-128" in names


def test_optional_groups():
    path = Path(__file__).parent.parent / "presets" / "nordic-128.yaml"
    preset = parse_preset(path)
    all_sources = preset.sources_for_options({"maps", "models"})
    no_maps = preset.sources_for_options(set())
    assert len(all_sources) > len(no_maps)
```

**Step 5: Run tests**

Run: `cd /Users/bembu/Projects/primer && source .venv/bin/activate && pip install pytest && pytest tests/ -v`
Expected: All 4 tests PASS.

**Step 6: Commit**

```bash
git add src/primer/models.py src/primer/presets.py presets/ tests/
git commit -m "Add preset loader, data model, and nordic-128 preset"
```

---

### Task 3: Taxonomy and coverage scoring

**Files:**
- Create: `src/primer/taxonomy.py`
- Create: `data/taxonomy.yaml`
- Create: `tests/test_taxonomy.py`

**Step 1: Create data/taxonomy.yaml**

```yaml
groups:
  survival:
    domains: [water, fire-shelter, food-foraging, navigation, first-aid, self-defense]
  medical:
    domains: [medicine, dentistry, pharmacy, emergency-medicine, mental-health]
  food:
    domains: [agriculture, gardening, animal-husbandry, food-preservation, cooking]
  engineering:
    domains: [electronics, mechanical, civil-construction, metalworking, woodworking, energy-power]
  tech:
    domains: [computing, radio-comms, networking-mesh, 3d-printing, drones-robotics]
  science:
    domains: [chemistry, physics, biology, earth-science, mathematics]
  society:
    domains: [education-pedagogy, governance-law, trade-economics, history, language]
  reference:
    domains: [general-reference, maps-geo, repair]
```

**Step 2: Create src/primer/taxonomy.py**

```python
from dataclasses import dataclass
from pathlib import Path

import yaml

from primer.models import Source

DATA_DIR = Path(__file__).parent.parent.parent / "data"

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


def compute_coverage(sources: list[Source], taxonomy: dict[str, list[str]]) -> list[DomainCoverage]:
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

        results.append(DomainCoverage(
            domain=domain,
            group=group,
            score=min(raw_score, 100),
            sources=contributing,
            depth_breakdown=depth_breakdown,
        ))

    return results
```

**Step 3: Write test**

```python
from primer.models import Source
from primer.taxonomy import load_taxonomy, compute_coverage, all_domains


def test_load_taxonomy():
    tax = load_taxonomy()
    assert "survival" in tax
    assert "water" in tax["survival"]
    domains = all_domains(tax)
    assert len(domains) >= 28


def test_compute_coverage():
    tax = load_taxonomy()
    sources = [
        Source(id="test1", type="zim", tags=["water", "fire-shelter"], depth="comprehensive", size_gb=1),
        Source(id="test2", type="zim", tags=["water"], depth="overview", size_gb=0.5),
    ]
    coverage = compute_coverage(sources, tax)
    water = next(c for c in coverage if c.domain == "water")
    assert water.score == 45  # 30 + 15
    assert len(water.sources) == 2

    fire = next(c for c in coverage if c.domain == "fire-shelter")
    assert fire.score == 30
    assert len(fire.sources) == 1

    # Uncovered domain
    dentistry = next(c for c in coverage if c.domain == "dentistry")
    assert dentistry.score == 0
    assert len(dentistry.sources) == 0


def test_score_caps_at_100():
    tax = load_taxonomy()
    sources = [
        Source(id=f"s{i}", type="zim", tags=["water"], depth="comprehensive", size_gb=1)
        for i in range(5)
    ]
    coverage = compute_coverage(sources, tax)
    water = next(c for c in coverage if c.domain == "water")
    assert water.score == 100  # 5 * 30 = 150, capped at 100
```

**Step 4: Run tests**

Run: `pytest tests/test_taxonomy.py -v`
Expected: All 3 tests PASS.

**Step 5: Commit**

```bash
git add src/primer/taxonomy.py data/ tests/test_taxonomy.py
git commit -m "Add taxonomy, coverage scoring engine"
```

---

### Task 4: Manifest and drive state

**Files:**
- Create: `src/primer/manifest.py`
- Create: `tests/test_manifest.py`

**Step 1: Create src/primer/manifest.py**

```python
from dataclasses import dataclass, field, asdict
from datetime import datetime
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
            preset=data["preset"],
            region=data["region"],
            target_path=data["target_path"],
            created=data.get("created", ""),
            last_synced=data.get("last_synced", ""),
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
```

**Step 2: Write test**

```python
from pathlib import Path
from primer.manifest import Manifest, ManifestEntry


def test_manifest_roundtrip(tmp_path):
    m = Manifest(
        preset="nordic-128",
        region="nordic",
        target_path="/Volumes/PRIMER",
        created="2026-03-29",
        last_synced="2026-03-29",
        entries=[
            ManifestEntry(
                id="wikipedia-en-nopic",
                type="zim",
                filename="wikipedia_en_all_nopic_2025-09.zim",
                size_bytes=25_000_000_000,
                tags=["general-reference"],
                depth="comprehensive",
                downloaded="2026-03-29",
            )
        ],
    )
    path = tmp_path / "manifest.yaml"
    m.save(path)
    loaded = Manifest.load(path)
    assert loaded.preset == "nordic-128"
    assert len(loaded.entries) == 1
    assert loaded.entries[0].id == "wikipedia-en-nopic"
    assert loaded.entries[0].size_bytes == 25_000_000_000


def test_manifest_exists(tmp_path):
    assert not Manifest.exists(tmp_path)
    (tmp_path / "manifest.yaml").write_text("preset: test\nregion: nordic\ntarget_path: /tmp\n")
    assert Manifest.exists(tmp_path)


def test_entry_by_id():
    m = Manifest(preset="test", region="nordic", target_path="/tmp", entries=[
        ManifestEntry(id="foo", type="zim", filename="foo.zim", size_bytes=100),
        ManifestEntry(id="bar", type="zim", filename="bar.zim", size_bytes=200),
    ])
    assert m.entry_by_id("foo").filename == "foo.zim"
    assert m.entry_by_id("baz") is None
```

**Step 3: Run tests**

Run: `pytest tests/test_manifest.py -v`
Expected: All 3 tests PASS.

**Step 4: Commit**

```bash
git add src/primer/manifest.py tests/test_manifest.py
git commit -m "Add manifest read/write for drive state tracking"
```

---

### Task 5: URL resolver (find latest ZIM dates)

**Files:**
- Create: `src/primer/resolver.py`
- Create: `tests/test_resolver.py`

**Step 1: Create src/primer/resolver.py**

```python
import re

import httpx

from primer.models import Source


def resolve_url(source: Source) -> str:
    """Resolve a source's URL pattern to a concrete URL by finding the latest date."""
    if source.url and not source.url_pattern:
        return source.url

    if not source.url_pattern:
        raise ValueError(f"Source {source.id} has no url or url_pattern")

    # Extract base directory URL from the pattern
    pattern = source.url_pattern
    # e.g. https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim
    base_url = pattern.rsplit("/", 1)[0] + "/"
    filename_pattern = pattern.rsplit("/", 1)[1]

    # Build regex from the filename pattern
    # Replace {date} with a capturing group for YYYY-MM format
    regex_str = re.escape(filename_pattern).replace(r"\{date\}", r"(\d{4}-\d{2})")
    regex = re.compile(regex_str)

    # Fetch directory listing
    response = httpx.get(base_url, follow_redirects=True, timeout=30)
    response.raise_for_status()

    # Find all matching filenames and their dates
    matches = []
    for match in regex.finditer(response.text):
        date = match.group(1)
        filename = match.group(0)
        matches.append((date, filename))

    if not matches:
        raise ValueError(f"No matching files found for {source.id} at {base_url}")

    # Sort by date, take latest
    matches.sort(key=lambda x: x[0])
    latest_date, latest_filename = matches[-1]

    return base_url + latest_filename


def resolve_all(sources: list[Source]) -> dict[str, str]:
    """Resolve all sources to concrete URLs. Returns {source_id: url}."""
    resolved = {}
    for s in sources:
        try:
            resolved[s.id] = resolve_url(s)
        except Exception as e:
            resolved[s.id] = f"ERROR: {e}"
    return resolved
```

**Step 2: Write test (with mocking)**

```python
from primer.models import Source
from primer.resolver import resolve_url


def test_resolve_static_url():
    s = Source(id="test", type="app", url="https://example.com/file.html", size_gb=0.01)
    assert resolve_url(s) == "https://example.com/file.html"


def test_resolve_url_pattern(monkeypatch):
    """Test URL pattern resolution with mocked HTTP response."""
    html = """
    <a href="wikipedia_en_all_nopic_2025-06.zim">wikipedia_en_all_nopic_2025-06.zim</a>
    <a href="wikipedia_en_all_nopic_2025-09.zim">wikipedia_en_all_nopic_2025-09.zim</a>
    <a href="wikipedia_en_all_nopic_2025-03.zim">wikipedia_en_all_nopic_2025-03.zim</a>
    """

    class MockResponse:
        text = html
        status_code = 200
        def raise_for_status(self): pass

    import primer.resolver as mod
    monkeypatch.setattr(mod.httpx, "get", lambda *a, **kw: MockResponse())

    s = Source(
        id="test",
        type="zim",
        url_pattern="https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim",
        size_gb=25,
    )
    url = resolve_url(s)
    assert url == "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_2025-09.zim"
```

**Step 3: Run tests**

Run: `pytest tests/test_resolver.py -v`
Expected: All 2 tests PASS.

**Step 4: Commit**

```bash
git add src/primer/resolver.py tests/test_resolver.py
git commit -m "Add URL resolver for finding latest ZIM file dates"
```

---

### Task 6: Download engine

**Files:**
- Create: `src/primer/downloader.py`
- Create: `tests/test_downloader.py`

**Step 1: Create src/primer/downloader.py**

```python
import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path

from rich.console import Console
from rich.progress import Progress, SpinnerColumn, BarColumn, DownloadColumn, TransferSpeedColumn, TimeRemainingColumn

console = Console()


@dataclass
class DownloadResult:
    source_id: str
    success: bool
    filepath: Path | None = None
    error: str = ""


def find_downloader() -> str | None:
    """Find available download tool: aria2c preferred, fallback to wget, then curl."""
    for tool in ["aria2c", "wget", "curl"]:
        if shutil.which(tool):
            return tool
    return None


def download_file(url: str, dest_dir: Path, tool: str | None = None) -> Path:
    """Download a file to dest_dir using the best available tool. Returns filepath."""
    dest_dir.mkdir(parents=True, exist_ok=True)
    filename = url.rsplit("/", 1)[-1]
    dest_path = dest_dir / filename

    if dest_path.exists():
        console.print(f"  [dim]Already exists: {filename}[/dim]")
        return dest_path

    if tool is None:
        tool = find_downloader()

    if tool is None:
        raise RuntimeError("No download tool found. Install aria2c, wget, or curl.")

    console.print(f"  [bold]Downloading:[/bold] {filename}")

    if tool == "aria2c":
        cmd = ["aria2c", "-x", "4", "-d", str(dest_dir), "-o", filename, "-c", url]
    elif tool == "wget":
        cmd = ["wget", "-c", "-q", "--show-progress", "-O", str(dest_path), url]
    else:
        cmd = ["curl", "-L", "-C", "-", "-o", str(dest_path), url]

    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"Download failed: {url}")

    return dest_path


def download_sources(sources: list[tuple[str, str, Path]]) -> list[DownloadResult]:
    """Download multiple sources. Input: [(source_id, url, dest_dir), ...]."""
    tool = find_downloader()
    results = []

    for source_id, url, dest_dir in sources:
        try:
            path = download_file(url, dest_dir, tool)
            results.append(DownloadResult(source_id=source_id, success=True, filepath=path))
        except Exception as e:
            results.append(DownloadResult(source_id=source_id, success=False, error=str(e)))
            console.print(f"  [red]Failed: {source_id}: {e}[/red]")

    return results
```

**Step 2: Write test**

```python
from pathlib import Path
from primer.downloader import find_downloader, DownloadResult


def test_find_downloader():
    tool = find_downloader()
    # At least one of aria2c, wget, curl should be available
    assert tool in ("aria2c", "wget", "curl", None)


def test_download_result():
    r = DownloadResult(source_id="test", success=True, filepath=Path("/tmp/test.zim"))
    assert r.success
    assert r.filepath.name == "test.zim"
```

**Step 3: Run tests**

Run: `pytest tests/test_downloader.py -v`
Expected: All 2 tests PASS.

**Step 4: Commit**

```bash
git add src/primer/downloader.py tests/test_downloader.py
git commit -m "Add download engine with aria2c/wget/curl fallback"
```

---

### Task 7: Init and sync commands

**Files:**
- Modify: `src/primer/cli.py`
- Create: `src/primer/commands.py`

**Step 1: Create src/primer/commands.py**

```python
from datetime import datetime
from pathlib import Path

from rich.console import Console
from rich.table import Table

from primer.downloader import download_sources
from primer.manifest import Manifest, ManifestEntry
from primer.models import Preset, Source
from primer.presets import load_preset
from primer.resolver import resolve_url
from primer.taxonomy import compute_coverage, load_taxonomy

console = Console()

# Map source types to subdirectories on the drive
TYPE_DIRS = {
    "zim": "zim",
    "pmtiles": "maps",
    "pdf": "books",
    "epub": "books",
    "gguf": "models",
    "binary": "bin",
    "app": "apps",
    "iso": "infra",
}


def init_drive(path: str, preset_name: str, enabled_groups: set[str] | None = None):
    """Initialize a drive with a preset."""
    drive_path = Path(path)
    drive_path.mkdir(parents=True, exist_ok=True)

    preset = load_preset(preset_name)
    if enabled_groups is None:
        enabled_groups = {"maps"}  # default: include maps

    sources = preset.sources_for_options(enabled_groups)

    manifest = Manifest(
        preset=preset_name,
        region=preset.region,
        target_path=str(drive_path),
        created=datetime.now().isoformat(timespec="seconds"),
    )
    manifest.save(drive_path / "manifest.yaml")

    console.print(f"[bold green]Initialized:[/bold green] {drive_path}")
    console.print(f"  Preset: {preset.name}")
    console.print(f"  Sources: {len(sources)}")
    console.print(f"  Estimated size: {sum(s.size_gb for s in sources):.1f} GB")
    console.print(f"\nRun [bold]primer sync[/bold] to download content.")


def sync_drive(path: str):
    """Download/update content on an initialized drive."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found. Run primer init or primer wizard first.[/red]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    console.print(f"[bold]Syncing:[/bold] {manifest.preset} → {drive_path}")

    # Resolve URLs
    console.print("\n[bold]Resolving latest versions...[/bold]")
    downloads = []
    for source in preset.sources:
        try:
            url = resolve_url(source)
            dest_dir = drive_path / TYPE_DIRS.get(source.type, "other")
            downloads.append((source.id, url, dest_dir))
            console.print(f"  [green]✓[/green] {source.id}")
        except Exception as e:
            console.print(f"  [red]✗[/red] {source.id}: {e}")

    # Download
    console.print(f"\n[bold]Downloading {len(downloads)} files...[/bold]")
    results = download_sources(downloads)

    # Update manifest
    for r in results:
        if r.success and r.filepath:
            source = next((s for s in preset.sources if s.id == r.source_id), None)
            if source:
                entry = ManifestEntry(
                    id=r.source_id,
                    type=source.type,
                    filename=r.filepath.name,
                    size_bytes=r.filepath.stat().st_size,
                    tags=source.tags,
                    depth=source.depth,
                    downloaded=datetime.now().isoformat(timespec="seconds"),
                    url=str(r.filepath),
                )
                # Replace or add entry
                manifest.entries = [e for e in manifest.entries if e.id != r.source_id]
                manifest.entries.append(entry)

    manifest.last_synced = datetime.now().isoformat(timespec="seconds")
    manifest.save(drive_path / "manifest.yaml")

    succeeded = sum(1 for r in results if r.success)
    failed = sum(1 for r in results if not r.success)
    console.print(f"\n[bold green]Done:[/bold green] {succeeded} downloaded, {failed} failed.")


def show_status(path: str):
    """Show drive status."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[dim]No primer drive found at this path.[/dim]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")

    table = Table(title=f"Primer — {manifest.preset}")
    table.add_column("ID", style="cyan")
    table.add_column("Type")
    table.add_column("Size", justify="right")
    table.add_column("Downloaded")

    total_bytes = 0
    for e in sorted(manifest.entries, key=lambda x: x.id):
        size_str = f"{e.size_bytes / 1e9:.1f} GB" if e.size_bytes > 1e9 else f"{e.size_bytes / 1e6:.0f} MB"
        table.add_row(e.id, e.type, size_str, e.downloaded[:10] if e.downloaded else "—")
        total_bytes += e.size_bytes

    console.print(table)
    console.print(f"\n  Total: {total_bytes / 1e9:.1f} GB | Last synced: {manifest.last_synced or '—'}")
```

**Step 2: Update src/primer/cli.py to wire up commands**

```python
import click
from rich.console import Console

console = Console()


@click.group(invoke_without_command=True)
@click.pass_context
def main(ctx):
    """Primer — Offline knowledge kit provisioner."""
    if ctx.invoked_subcommand is None:
        from primer.manifest import Manifest
        from pathlib import Path

        # Check if current dir or common paths have a manifest
        cwd = Path.cwd()
        if Manifest.exists(cwd):
            from primer.commands import show_status
            show_status(str(cwd))
            _show_menu(str(cwd))
        else:
            console.print("[bold]Primer[/bold] — Offline knowledge kit provisioner")
            console.print("\nNo drive found. Run [bold]primer wizard[/bold] to get started.")
            console.print("Run [bold]primer --help[/bold] for all commands.")


def _show_menu(path: str):
    """Show interactive menu for an initialized drive."""
    console.print("\n  [bold][s][/bold] Sync (check for updates)")
    console.print("  [bold][a][/bold] Audit report")
    console.print("  [bold][w][/bold] Wizard (reconfigure)")
    console.print("  [bold][q][/bold] Quit")

    choice = console.input("\n  > ")
    if choice == "s":
        from primer.commands import sync_drive
        sync_drive(path)
    elif choice == "a":
        ctx = click.Context(audit)
        ctx.invoke(audit)
    elif choice == "w":
        ctx = click.Context(wizard)
        ctx.invoke(wizard)


@main.command()
def wizard():
    """Interactive setup wizard."""
    console.print("[bold]Wizard not yet implemented.[/bold]")


@main.command()
@click.argument("path")
@click.option("--preset", required=True, help="Preset name (e.g. nordic-128)")
def init(path, preset):
    """Initialize a drive with a preset."""
    from primer.commands import init_drive
    init_drive(path, preset)


@main.command()
@click.argument("path", default=".")
def sync(path):
    """Download/update content on initialized drive."""
    from primer.commands import sync_drive
    sync_drive(path)


@main.command()
@click.argument("path", default=".")
def status(path):
    """Show what's downloaded, what's stale."""
    from primer.commands import show_status
    show_status(path)


@main.command()
def audit():
    """Generate LLM-ready gap analysis report."""
    console.print("[bold]Audit not yet implemented.[/bold]")
```

**Step 3: Verify CLI works**

Run: `primer --help`
Expected: Shows all commands.

Run: `primer init /tmp/test-primer --preset nordic-128`
Expected: Creates manifest at /tmp/test-primer/manifest.yaml, prints summary.

Run: `primer status /tmp/test-primer`
Expected: Shows empty table (nothing downloaded yet).

**Step 4: Commit**

```bash
git add src/primer/commands.py src/primer/cli.py
git commit -m "Wire up init, sync, and status commands"
```

---

### Task 8: Wizard (interactive setup)

**Files:**
- Modify: `src/primer/cli.py`
- Create: `src/primer/wizard.py`

**Step 1: Create src/primer/wizard.py**

```python
import shutil
from pathlib import Path

from rich.console import Console
from rich.panel import Panel
from rich.prompt import Prompt, Confirm
from rich.table import Table

from primer.commands import init_drive, sync_drive
from primer.presets import list_presets, load_preset

console = Console()

SIZE_PRESETS = {
    32: "nordic-32",
    64: "nordic-64",
    128: "nordic-128",
    256: "nordic-256",
    512: "nordic-512",
    1024: "nordic-1tb",
    2048: "nordic-2tb",
}


def detect_volumes() -> list[dict]:
    """Detect mounted volumes with size info."""
    volumes = []
    volumes_path = Path("/Volumes")
    if not volumes_path.exists():
        return volumes

    for v in sorted(volumes_path.iterdir()):
        if v.name == "Macintosh HD":
            continue
        try:
            usage = shutil.disk_usage(v)
            volumes.append({
                "path": str(v),
                "name": v.name,
                "total_gb": usage.total / 1e9,
                "free_gb": usage.free / 1e9,
            })
        except (PermissionError, OSError):
            continue
    return volumes


def find_best_preset(size_gb: float, region: str = "nordic") -> str | None:
    """Find the largest preset that fits the given size."""
    available = list_presets()
    best = None
    for size, preset_name in sorted(SIZE_PRESETS.items()):
        if preset_name in available and size <= size_gb:
            best = preset_name
    return best


def run_wizard():
    """Run the interactive setup wizard."""
    console.print(Panel("[bold]Primer — Offline Knowledge Kit[/bold]\n\nThis wizard will help you set up an offline knowledge drive.", style="blue"))

    # Step 1: Target
    console.print("\n[bold]Step 1/5 — Target[/bold]")
    console.print("Where should the kit be provisioned?\n")

    volumes = detect_volumes()
    choices = {}
    for i, v in enumerate(volumes, 1):
        label = f"{v['name']} ({v['total_gb']:.0f} GB total, {v['free_gb']:.0f} GB free)"
        console.print(f"  [bold]{i}[/bold]) {v['path']}  [dim]{label}[/dim]")
        choices[str(i)] = v["path"]
    console.print(f"  [bold]c[/bold]) Custom path...")

    choice = Prompt.ask("\n  Select", choices=list(choices.keys()) + ["c"])
    if choice == "c":
        target_path = Prompt.ask("  Enter path")
    else:
        target_path = choices[choice]

    # Step 2: Budget
    console.print("\n[bold]Step 2/5 — Budget[/bold]")
    try:
        usage = shutil.disk_usage(target_path)
        default_gb = int(usage.total / 1e9 * 0.9)
    except OSError:
        default_gb = 128

    budget_gb = int(Prompt.ask(f"  How much space to use (GB)?", default=str(default_gb)))

    # Step 3: Region
    console.print("\n[bold]Step 3/5 — Region[/bold]")
    console.print("  [bold]1[/bold]) Nordic (Finland, Nordics, Northern Europe)")
    console.print("  [dim]2) US (coming soon)[/dim]")
    console.print("  [dim]3) Global (coming soon)[/dim]")
    region_choice = Prompt.ask("  Select", default="1", choices=["1"])
    region = "nordic"

    # Find best preset
    preset_name = find_best_preset(budget_gb, region)
    if not preset_name:
        console.print(f"[red]No preset found for {budget_gb} GB. Minimum is 32 GB.[/red]")
        return

    preset = load_preset(preset_name)

    # Step 4: Options
    console.print(f"\n[bold]Step 4/5 — Options[/bold] (preset: {preset.name})")
    enabled_groups = set()

    include_maps = Confirm.ask("  Include offline maps?", default=True)
    if include_maps:
        enabled_groups.add("maps")

    if budget_gb >= 512:
        include_models = Confirm.ask("  Include LLM models?", default=True)
        if include_models:
            enabled_groups.add("models")

        include_installers = Confirm.ask("  Include app installers (Kiwix.dmg, etc.)?", default=False)
        if include_installers:
            enabled_groups.add("installers")

        include_infra = Confirm.ask("  Include Linux ISO + package cache?", default=False)
        if include_infra:
            enabled_groups.add("infra")

    # Step 5: Review
    sources = preset.sources_for_options(enabled_groups)
    total_gb = sum(s.size_gb for s in sources)

    console.print(f"\n[bold]Step 5/5 — Review[/bold]")
    table = Table()
    table.add_column("Category")
    table.add_column("Sources", justify="right")
    table.add_column("Size", justify="right")

    by_type: dict[str, list] = {}
    for s in sources:
        by_type.setdefault(s.type, []).append(s)
    for type_name, type_sources in sorted(by_type.items()):
        size = sum(s.size_gb for s in type_sources)
        table.add_row(type_name.upper(), str(len(type_sources)), f"{size:.1f} GB")

    console.print(table)
    console.print(f"\n  [bold]Target:[/bold]  {target_path}")
    console.print(f"  [bold]Preset:[/bold]  {preset.name}")
    console.print(f"  [bold]Total:[/bold]   {total_gb:.1f} GB / {budget_gb} GB")

    if not Confirm.ask("\n  Proceed?", default=True):
        console.print("[dim]Cancelled.[/dim]")
        return

    # Execute
    init_drive(target_path, preset_name, enabled_groups)
    if Confirm.ask("\n  Start downloading now?", default=True):
        sync_drive(target_path)
```

**Step 2: Wire wizard in cli.py**

Replace the wizard command:

```python
@main.command()
def wizard():
    """Interactive setup wizard."""
    from primer.wizard import run_wizard
    run_wizard()
```

**Step 3: Test manually**

Run: `primer wizard`
Expected: Walks through 5 steps interactively.

**Step 4: Commit**

```bash
git add src/primer/wizard.py src/primer/cli.py
git commit -m "Add interactive wizard with Rich prompts"
```

---

### Task 9: Audit report generator

**Files:**
- Create: `src/primer/audit.py`
- Modify: `src/primer/cli.py`

**Step 1: Create src/primer/audit.py**

```python
from datetime import datetime
from pathlib import Path
import shutil

from primer.manifest import Manifest
from primer.models import Source
from primer.presets import load_preset
from primer.taxonomy import compute_coverage, load_taxonomy


FORMAT_ACCESSIBILITY = """| Format | macOS | iOS | Android | Linux | Viewer on drive? |
|--------|-------|-----|---------|-------|-------------------|
| ZIM    | ✓     | ✓   | ✓       | ✓     | ✓ kiwix-serve     |
| PMTiles| ✓     | ✓   | ✓       | ✓     | ✓ go-pmtiles      |
| PDF    | ✓     | ✓   | ✓       | ✓     | ✗ OS built-in     |
| EPUB   | ✓     | ✓   | ✓       | ~     | ✗ OS built-in     |
| GGUF   | ✓     | ✗   | ✗       | ✓     | ✓ llama-server    |
| HTML   | ✓     | ✓   | ✓       | ✓     | ✓ native          |
| WebM   | ✓     | ✓   | ✓       | ✓     | ✓ via kiwix-serve |"""


def generate_audit(drive_path: Path) -> str:
    """Generate a markdown audit report for AI analysis."""
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)
    taxonomy = load_taxonomy()

    # Disk usage
    try:
        usage = shutil.disk_usage(drive_path)
        total_gb = usage.total / 1e9
        used_gb = usage.used / 1e9
        free_gb = usage.free / 1e9
    except OSError:
        total_gb = used_gb = free_gb = 0

    # Build source objects from manifest entries for coverage
    sources = [
        Source(id=e.id, type=e.type, tags=e.tags, depth=e.depth, size_gb=e.size_bytes / 1e9)
        for e in manifest.entries
    ]
    coverage = compute_coverage(sources, taxonomy)

    lines = []
    lines.append("# Primer Audit Report")
    lines.append(f"Generated: {datetime.now().strftime('%Y-%m-%d')}")
    lines.append(f"Preset: {manifest.preset}")
    lines.append(f"Drive: {drive_path} ({total_gb:.0f}GB total, {used_gb:.0f}GB used, {free_gb:.0f}GB free)")
    lines.append("")

    # System prompt
    lines.append("## System Prompt for AI Analysis")
    lines.append("")
    lines.append("You are analyzing an offline knowledge kit designed for survival and")
    lines.append("civilization rebuilding scenarios, with a Nordic/Finnish focus.")
    lines.append("The kit must be usable with:")
    lines.append("- MacBook (M4, macOS, 128GB RAM)")
    lines.append("- iPhone/iPad (iOS, Kiwix reader)")
    lines.append("- Android phone (Kiwix, OsmAnd)")
    lines.append("- Any x86/ARM Linux machine")
    lines.append("")
    lines.append("Analyze the inventory below and identify:")
    lines.append("1. Critical knowledge gaps for survival (Nordic climate, -30°C winters)")
    lines.append("2. Knowledge gaps for rebuilding (agriculture, manufacturing, governance)")
    lines.append("3. Missing practical formats (theory but no step-by-step guides)")
    lines.append("4. Accessibility gaps (content that can't be opened without specific software)")
    lines.append("5. Redundancies worth eliminating to free space")
    lines.append("6. Specific freely-available resources that would fill the top 10 gaps")
    lines.append("7. Regional blind spots (Nordic flora, fauna, building codes, law)")
    lines.append(f"\nAvailable free space: {free_gb:.0f} GB")
    lines.append("")

    # Inventory
    lines.append("## Inventory")
    lines.append("")
    lines.append("| ID | Type | Size | Tags | Depth | Downloaded |")
    lines.append("|----|------|------|------|-------|------------|")
    for e in sorted(manifest.entries, key=lambda x: x.id):
        size = f"{e.size_bytes / 1e9:.1f} GB" if e.size_bytes > 1e9 else f"{e.size_bytes / 1e6:.0f} MB"
        tags = ", ".join(e.tags[:5])
        date = e.downloaded[:10] if e.downloaded else "—"
        lines.append(f"| {e.id} | {e.type} | {size} | {tags} | {e.depth} | {date} |")
    lines.append("")

    # Coverage
    lines.append("## Coverage Matrix")
    lines.append("")
    lines.append("| Domain | Group | Score | Sources | Gaps |")
    lines.append("|--------|-------|-------|---------|------|")
    for c in sorted(coverage, key=lambda x: x.score):
        bar = "█" * (c.score // 10) + "░" * (10 - c.score // 10)
        gap = ""
        if c.score == 0:
            gap = "✗ No sources"
        elif c.score < 30:
            gap = "⚠ Weak coverage"
        sources_str = str(len(c.sources))
        lines.append(f"| {c.domain} | {c.group} | {bar} {c.score}% | {sources_str} | {gap} |")
    lines.append("")

    # Format accessibility
    lines.append("## Format Accessibility Matrix")
    lines.append("")
    lines.append(FORMAT_ACCESSIBILITY)
    lines.append("")

    return "\n".join(lines)
```

**Step 2: Wire audit command in cli.py**

Replace the audit command:

```python
@main.command()
@click.argument("path", default=".")
def audit(path):
    """Generate LLM-ready gap analysis report."""
    from pathlib import Path as P
    from primer.manifest import Manifest
    drive_path = P(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found.[/red]")
        return
    from primer.audit import generate_audit
    report = generate_audit(drive_path)
    click.echo(report)
```

**Step 3: Commit**

```bash
git add src/primer/audit.py src/primer/cli.py
git commit -m "Add audit report generator with LLM prompt"
```

---

### Task 10: serve.sh generator

**Files:**
- Create: `src/primer/serve_generator.py`
- Modify: `src/primer/commands.py` (call it from init)

**Step 1: Create src/primer/serve_generator.py**

```python
from pathlib import Path


SERVE_SH = r'''#!/usr/bin/env bash
# Primer Drive — Self-contained knowledge server
# Auto-detects OS/arch and starts available services

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Detect OS and architecture
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        darwin) os="macos" ;;
        linux)  os="linux" ;;
        *)      echo "Unsupported OS: $os"; exit 1 ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="x86_64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)             echo "Unsupported arch: $arch"; exit 1 ;;
    esac

    if [ "$os" = "macos" ]; then
        PLATFORM="macos-arm64"
    else
        PLATFORM="linux-${arch}"
    fi
}

detect_platform

BIN_DIR="${SCRIPT_DIR}/bin/${PLATFORM}"
ZIM_DIR="${SCRIPT_DIR}/zim"
MAPS_DIR="${SCRIPT_DIR}/maps"
MODELS_DIR="${SCRIPT_DIR}/models"

# Count available content
ZIM_COUNT=$(find "$ZIM_DIR" -name "*.zim" 2>/dev/null | wc -l | tr -d ' ')
MAP_COUNT=$(find "$MAPS_DIR" -name "*.pmtiles" 2>/dev/null | wc -l | tr -d ' ')
MODEL_COUNT=$(find "$MODELS_DIR" -name "*.gguf" 2>/dev/null | wc -l | tr -d ' ')

HAS_KIWIX=false
HAS_PMTILES=false
HAS_LLAMA=false

[ -x "${BIN_DIR}/kiwix-serve" ] && [ "$ZIM_COUNT" -gt 0 ] && HAS_KIWIX=true
[ -x "${BIN_DIR}/go-pmtiles" ] && [ "$MAP_COUNT" -gt 0 ] && HAS_PMTILES=true
[ -x "${BIN_DIR}/llama-server" ] && [ "$MODEL_COUNT" -gt 0 ] && HAS_LLAMA=true

# Track running services
PIDS=()
cleanup() {
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
}
trap cleanup EXIT

start_kiwix() {
    echo "  Starting kiwix-serve on http://localhost:8080 ..."
    "${BIN_DIR}/kiwix-serve" --port 8080 "${ZIM_DIR}"/*.zim "${ZIM_DIR}"/custom/*.zim 2>/dev/null &
    PIDS+=($!)
    echo "  Serving ${ZIM_COUNT} ZIM files."
}

start_maps() {
    echo "  Starting map viewer on http://localhost:8081 ..."
    "${BIN_DIR}/go-pmtiles" serve "${MAPS_DIR}" --port 8081 &
    PIDS+=($!)
    echo "  Serving ${MAP_COUNT} map files."
}

start_llama() {
    local model
    model=$(find "$MODELS_DIR" -name "*.gguf" | head -1)
    echo "  Starting LLM server on http://localhost:8082 ..."
    "${BIN_DIR}/llama-server" --model "$model" --host 127.0.0.1 --port 8082 &
    PIDS+=($!)
    echo "  Model: $(basename "$model")"
    echo "  API: http://localhost:8082/v1/chat/completions"
}

show_contents() {
    echo ""
    echo "  Drive contents:"
    [ "$ZIM_COUNT" -gt 0 ] && echo "    ZIM files:  ${ZIM_COUNT}" && ls -1 "$ZIM_DIR"/*.zim 2>/dev/null | while read f; do echo "      $(basename "$f")"; done
    [ "$MAP_COUNT" -gt 0 ] && echo "    Map files:  ${MAP_COUNT}"
    [ "$MODEL_COUNT" -gt 0 ] && echo "    LLM models: ${MODEL_COUNT}"
    echo ""
}

# Main menu
while true; do
    echo ""
    echo "  Primer Drive"
    echo "  Detected: ${PLATFORM}"
    echo ""

    N=0
    OPTIONS=()

    if $HAS_KIWIX; then
        N=$((N+1)); echo "  [${N}] Start knowledge base (kiwix-serve, ${ZIM_COUNT} ZIM files)"
        OPTIONS+=("kiwix")
    fi
    if $HAS_PMTILES; then
        N=$((N+1)); echo "  [${N}] Start map viewer (go-pmtiles)"
        OPTIONS+=("maps")
    fi
    if $HAS_LLAMA; then
        N=$((N+1)); echo "  [${N}] Start LLM server (llama-server)"
        OPTIONS+=("llama")
    fi
    if [ ${#OPTIONS[@]} -gt 1 ]; then
        N=$((N+1)); echo "  [${N}] Start all"
        OPTIONS+=("all")
    fi
    N=$((N+1)); echo "  [${N}] Show drive contents"
    OPTIONS+=("contents")
    echo "  [q] Quit"

    read -r -p "  > " choice

    case "$choice" in
        q|Q) echo "  Bye."; exit 0 ;;
    esac

    # Map number to action
    if [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -le ${#OPTIONS[@]} ]; then
        action="${OPTIONS[$((choice-1))]}"
        case "$action" in
            kiwix) start_kiwix ;;
            maps) start_maps ;;
            llama) start_llama ;;
            all)
                $HAS_KIWIX && start_kiwix
                $HAS_PMTILES && start_maps
                $HAS_LLAMA && start_llama
                ;;
            contents) show_contents ;;
        esac

        if [ "$action" != "contents" ]; then
            echo ""
            echo "  Services running. Press Enter to return to menu, or Ctrl+C to stop all."
            read -r
        fi
    fi
done
'''


def generate_serve_sh(drive_path: Path):
    """Write serve.sh to the drive root."""
    serve_path = drive_path / "serve.sh"
    serve_path.write_text(SERVE_SH)
    serve_path.chmod(0o755)
```

**Step 2: Call from init_drive in commands.py**

Add after manifest save:

```python
from primer.serve_generator import generate_serve_sh
generate_serve_sh(drive_path)
```

**Step 3: Commit**

```bash
git add src/primer/serve_generator.py src/primer/commands.py
git commit -m "Add serve.sh generator for self-contained drive menu"
```

---

### Task 11: README and drive README generator

**Files:**
- Create: `src/primer/readme_generator.py`
- Create: `README.md` (project README)

**Step 1: Create src/primer/readme_generator.py**

```python
from pathlib import Path

from primer.manifest import Manifest
from primer.presets import load_preset


def generate_drive_readme(drive_path: Path):
    """Write a README.md to the drive explaining what it is and how to use it."""
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    content = f"""# Primer — Offline Knowledge Kit

**Preset:** {preset.name}
**Region:** {preset.region}
**Created:** {manifest.created}

## Quick Start

### Browse knowledge (any computer)

```bash
./serve.sh
```

This starts a local web server. Open http://localhost:8080 in your browser to access
Wikipedia, WikiHow, medical references, and all other content.

### On iPhone/iPad

Install "Kiwix" from the App Store, then open any `.zim` file from the `zim/` folder.

### On Android

Install "Kiwix" from Google Play, then open any `.zim` file from the `zim/` folder.

## What's on this drive

| Directory | Contents |
|-----------|----------|
| `zim/` | Kiwix ZIM files — Wikipedia, WikiHow, Stack Exchange, medical refs, etc. |
| `maps/` | Offline maps (PMTiles format) — view via serve.sh or browser |
| `books/` | PDFs and EPUBs |
| `models/` | AI language models (GGUF format) |
| `apps/` | Portable apps (CyberChef, etc.) |
| `bin/` | Server binaries for macOS and Linux |
| `serve.sh` | Start browsing — auto-detects your OS |

## Formats

All content uses open, universal formats:

- **ZIM** — Open with Kiwix (any platform) or kiwix-serve (included)
- **PMTiles** — Open with go-pmtiles (included) or any PMTiles viewer
- **PDF/EPUB** — Open with any reader
- **GGUF** — Open with LM Studio, llama.cpp, or llama-server (included)
- **HTML** — Open in any browser

## No internet required

Everything on this drive works completely offline. No accounts, no servers, no cloud.
"""
    (drive_path / "README.md").write_text(content)
```

**Step 2: Create project README.md**

```markdown
# Primer

Offline knowledge kit provisioner for survival and civilization rebuilding.

## Install

```bash
pip install -e .
```

## Quick start

```bash
primer wizard
```

Or non-interactively:

```bash
primer init /Volumes/MY_USB --preset nordic-128
primer sync /Volumes/MY_USB
```

## Presets

| Preset | Size | Focus |
|--------|------|-------|
| nordic-128 | 128 GB | Grab-and-go survival, Nordic focus |
| nordic-256 | 256 GB | + Full Wikipedia with images, all Stack Exchanges |
| nordic-1tb | 1 TB | Civilization rebuild, LLMs, Linux ISO |

## Commands

- `primer wizard` — Interactive setup
- `primer init <path> --preset <name>` — Initialize a drive
- `primer sync <path>` — Download/update content
- `primer status <path>` — Show drive status
- `primer audit <path>` — Generate gap analysis report

## Docs

- [Design document](docs/plans/2026-03-29-primer-design.md)
```

**Step 3: Call readme generator from init_drive**

Add to commands.py init_drive after serve.sh generation:

```python
from primer.readme_generator import generate_drive_readme
generate_drive_readme(drive_path)
```

**Step 4: Commit**

```bash
git add src/primer/readme_generator.py README.md src/primer/commands.py
git commit -m "Add README and drive README generator"
```

---

## Execution order

Tasks 1-6 are foundational (data model, loader, resolver, downloader). Tasks 7-11 wire everything together into the CLI. Each task is independently testable and committable.

After Task 11, the tool is functional end-to-end: `primer wizard` walks you through setup, `primer sync` downloads content, `primer status` shows what you have, `primer audit` generates a gap analysis report, and the drive has a self-contained `serve.sh`.

## Not in v1 (deferred)

- Additional presets (nordic-32, nordic-64, nordic-256, nordic-512, nordic-2tb)
- `primer crawl` (custom website crawling via Zimit)
- Binary downloading (kiwix-serve, go-pmtiles, llama-server static binaries)
- Checksum verification
- PMTiles map viewer HTML bundle
- Freshness checking against remote
