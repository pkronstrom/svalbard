# Preset Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current `nordic-*` preset system with canonical `finland-*` and `default-*` presets, simplify preset selection around standalone files, and add multi-platform tool binary downloads without adding OSM extraction or vector topo download workflows.

**Architecture:** Keep preset files self-contained and canonical, with a thin compatibility layer so existing `nordic-*` manifests still work. Treat map assets as ordinary downloadable sources in this pass; do not add `extract`, `vectortiles`, or any other generated-output map pipeline. Expand multi-platform binary sources into per-platform downloads during `sync`, storing one manifest entry per downloaded platform artifact.

**Tech Stack:** Python 3.12, Click, Rich, PyYAML, httpx, pytest, uv

---

## Scope Decisions

- This plan **includes**: preset rename/reorganization, new preset schema (`group`, `platforms`), wizard and command simplification, legacy preset aliasing, manifest support for platform variants, and multi-platform binary downloads into `bin/{platform}/`.
- This plan **excludes**: `pmtiles extract`, bbox/maxzoom handling, vector topo downloads, `extract` or `vectortiles` schema fields, and any new map-processing binary/toolchain.
- New presets must only reference **directly downloadable** assets. If there is no vetted Finland/Nordics OSM artifact available at implementation time, leave those presets map-free rather than pointing at a world archive and labeling it as regional coverage.

## File Map

**Create:**
- `docs/plans/2026-03-29-preset-restructure-implementation-plan.md`
- `src/svalbard/presets/finland-32.yaml`
- `src/svalbard/presets/finland-64.yaml`
- `src/svalbard/presets/finland-128.yaml`
- `src/svalbard/presets/finland-256.yaml`
- `src/svalbard/presets/finland-512.yaml`
- `src/svalbard/presets/finland-1tb.yaml`
- `src/svalbard/presets/finland-2tb.yaml`
- `src/svalbard/presets/default-32.yaml`
- `src/svalbard/presets/default-64.yaml`
- `src/svalbard/presets/default-128.yaml`

**Modify:**
- `src/svalbard/models.py`
- `src/svalbard/presets.py`
- `src/svalbard/commands.py`
- `src/svalbard/manifest.py`
- `src/svalbard/wizard.py`
- `src/svalbard/readme_generator.py`
- `README.md`
- `tests/test_presets.py`
- `tests/test_commands.py`
- `tests/test_wizard.py`
- `tests/test_manifest.py`
- `tests/test_downloader.py`

**Delete after alias layer is in place and tests pass:**
- `src/svalbard/presets/nordic-32.yaml`
- `src/svalbard/presets/nordic-64.yaml`
- `src/svalbard/presets/nordic-128.yaml`
- `src/svalbard/presets/nordic-256.yaml`
- `src/svalbard/presets/nordic-512.yaml`
- `src/svalbard/presets/nordic-1tb.yaml`
- `src/svalbard/presets/nordic-2tb.yaml`

### Task 1: Canonical Preset Schema and Legacy Alias Layer

**Files:**
- Modify: `src/svalbard/models.py`
- Modify: `src/svalbard/presets.py`
- Modify: `tests/test_presets.py`

- [ ] **Step 1: Write failing parser and alias tests**

```python
from svalbard.presets import list_presets, load_preset


def test_load_preset_aliases_legacy_nordic_name():
    preset = load_preset("nordic-128")
    assert preset.name == "finland-128"
    assert preset.region == "finland"


def test_parse_finland_128_group_and_platforms():
    preset = load_preset("finland-128")
    tool = next(source for source in preset.sources if source.id == "kiwix-serve")
    assert tool.group == "tools"
    assert "linux-x86_64" in tool.platforms
    assert tool.platforms["linux-x86_64"].startswith("https://")


def test_list_presets_only_returns_canonical_names():
    presets = list_presets()
    assert "finland-128" in presets
    assert "default-64" in presets
    assert "nordic-128" not in presets
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `uv run pytest -q tests/test_presets.py -k "aliases_legacy_nordic_name or group_and_platforms or canonical_names"`

Expected: FAIL because `load_preset()` only loads exact filenames, `Source` has no `group` or `platforms`, and only `nordic-*` presets exist.

- [ ] **Step 3: Implement canonical preset parsing and alias resolution**

```python
from dataclasses import dataclass, field


@dataclass
class Source:
    id: str
    type: str
    group: str = ""
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"
    size_gb: float = 0.0
    url: str = ""
    url_pattern: str = ""
    platforms: dict[str, str] = field(default_factory=dict)
    description: str = ""
    sha256: str = ""


LEGACY_PRESET_ALIASES = {
    "nordic-32": "finland-32",
    "nordic-64": "finland-64",
    "nordic-128": "finland-128",
    "nordic-256": "finland-256",
    "nordic-512": "finland-512",
    "nordic-1tb": "finland-1tb",
    "nordic-2tb": "finland-2tb",
}


def canonical_preset_name(name: str) -> str:
    return LEGACY_PRESET_ALIASES.get(name, name)


def load_preset(name: str) -> Preset:
    canonical = canonical_preset_name(name)
    path = PRESETS_DIR / f"{canonical}.yaml"
    if not path.exists():
        raise FileNotFoundError(f"Preset not found: {path}")
    return parse_preset(path)


def list_presets() -> list[str]:
    return sorted(p.stem for p in PRESETS_DIR.glob("*.yaml"))
```

- [ ] **Step 4: Run tests to verify the new parser behavior passes**

Run: `uv run pytest -q tests/test_presets.py -k "aliases_legacy_nordic_name or group_and_platforms or canonical_names"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/svalbard/models.py src/svalbard/presets.py tests/test_presets.py
git commit -m "feat: add canonical preset schema and legacy aliases"
```

### Task 2: Replace the Preset Catalog With Standalone `finland-*` and `default-*` Files

**Files:**
- Create: `src/svalbard/presets/finland-32.yaml`
- Create: `src/svalbard/presets/finland-64.yaml`
- Create: `src/svalbard/presets/finland-128.yaml`
- Create: `src/svalbard/presets/finland-256.yaml`
- Create: `src/svalbard/presets/finland-512.yaml`
- Create: `src/svalbard/presets/finland-1tb.yaml`
- Create: `src/svalbard/presets/finland-2tb.yaml`
- Create: `src/svalbard/presets/default-32.yaml`
- Create: `src/svalbard/presets/default-64.yaml`
- Create: `src/svalbard/presets/default-128.yaml`
- Modify: `tests/test_presets.py`

- [ ] **Step 1: Write failing catalog-level tests for the new preset family**

```python
from svalbard.presets import load_preset, list_presets


def test_list_presets_contains_finland_and_default_families():
    presets = list_presets()
    assert "finland-32" in presets
    assert "finland-1tb" in presets
    assert "default-32" in presets
    assert "default-128" in presets


def test_default_64_is_region_neutral():
    preset = load_preset("default-64")
    assert preset.region == "default"
    assert all(source.group != "regional" for source in preset.sources)


def test_finland_128_uses_standalone_sources_only():
    preset = load_preset("finland-128")
    ids = {source.id for source in preset.sources}
    assert "wikipedia-en" in ids
    assert "kiwix-serve" in ids
    assert all(not source.platforms or source.type == "binary" for source in preset.sources)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `uv run pytest -q tests/test_presets.py -k "finland_and_default_families or region_neutral or standalone_sources_only"`

Expected: FAIL because the new preset files do not exist yet.

- [ ] **Step 3: Create canonical preset YAML files and remove cross-preset semantics**

```yaml
name: finland-128
description: Finnish + English offline reference set for a 128 GB drive
target_size_gb: 128
region: finland

sources:
  - id: wikipedia-en
    type: zim
    group: reference
    tags: [general-reference]
    depth: comprehensive
    size_gb: 25.0
    url_pattern: "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim"
    description: English Wikipedia without pictures

  - id: wikipedia-fi
    type: zim
    group: reference
    tags: [general-reference, language]
    depth: comprehensive
    size_gb: 5.0
    url_pattern: "https://download.kiwix.org/zim/wikipedia/wikipedia_fi_all_nopic_{date}.zim"
    description: Finnish Wikipedia without pictures

  - id: kiwix-serve
    type: binary
    group: tools
    tags: [general-reference]
    depth: reference-only
    size_gb: 0.2
    platforms:
      linux-x86_64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_linux-x86_64.tar.gz"
      linux-arm64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_linux-aarch64.tar.gz"
      macos-x86_64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_macos-x86_64.tar.gz"
      macos-arm64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_macos-arm64.tar.gz"
    description: Kiwix tools archive for all supported desktop platforms
```

Implementation notes:
- Do **not** carry over `optional_group` or `replaces`.
- `default-*` presets should stay English-first and region-neutral.
- `finland-*` presets may include Finnish-language and country-specific sources.
- Only include `group: maps` sources if the URL is a real vetted direct download. If not, omit the map source in this pass.
- Once the new preset files are in place and `LEGACY_PRESET_ALIASES` resolves old names, delete the old `src/svalbard/presets/nordic-*.yaml` files before the final test run.

- [ ] **Step 4: Run tests to verify the new catalog passes**

Run: `uv run pytest -q tests/test_presets.py`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/svalbard/presets tests/test_presets.py
git commit -m "feat: replace nordic presets with standalone finland and default families"
```

### Task 3: Simplify Wizard, Init, Sync, and Status Around Standalone Presets

**Files:**
- Modify: `src/svalbard/commands.py`
- Modify: `src/svalbard/manifest.py`
- Modify: `src/svalbard/wizard.py`
- Modify: `tests/test_commands.py`
- Modify: `tests/test_wizard.py`
- Modify: `tests/test_manifest.py`

- [ ] **Step 1: Write failing behavior tests for preset-only flows and legacy-manifest compatibility**

```python
from svalbard.commands import init_drive
from svalbard.manifest import Manifest
from svalbard.wizard import presets_for_space


def test_init_drive_records_canonical_preset_without_enabled_groups(tmp_path):
    init_drive(str(tmp_path), "finland-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert manifest.preset == "finland-128"
    assert manifest.enabled_groups == []


def test_init_drive_accepts_legacy_preset_name_and_writes_canonical_name(tmp_path):
    init_drive(str(tmp_path), "nordic-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert manifest.preset == "finland-128"
    assert manifest.region == "finland"


def test_presets_for_space_filters_by_region_family():
    result = presets_for_space(122, region="default")
    names = [name for name, _, _ in result]
    assert "default-64" in names
    assert "finland-64" not in names
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `uv run pytest -q tests/test_commands.py tests/test_wizard.py tests/test_manifest.py -k "canonical_preset_without_enabled_groups or writes_canonical_name or filters_by_region_family"`

Expected: FAIL because `init_drive()` still stores whatever name it receives and the wizard logic still assumes the old region/options model.

- [ ] **Step 3: Implement the simplified preset-only command flow**

```python
def init_drive(path: str, preset_name: str):
    drive_path = Path(path)
    drive_path.mkdir(parents=True, exist_ok=True)

    preset = load_preset(preset_name)
    manifest = Manifest(
        preset=preset.name,
        region=preset.region,
        target_path=str(drive_path),
        created=datetime.now().isoformat(timespec="seconds"),
        enabled_groups=[],
    )
    manifest.save(drive_path / "manifest.yaml")


def sync_drive(path: str, update: bool = False, force: bool = False):
    manifest = Manifest.load(Path(path) / "manifest.yaml")
    preset = load_preset(manifest.preset)
    active_sources = preset.sources


def presets_for_space(free_gb: float, region: str) -> list[tuple[str, float, bool]]:
    result = []
    for preset_name in list_presets():
        preset = load_preset(preset_name)
        if preset.region != region:
            continue
        content_gb = sum(source.size_gb for source in preset.sources)
        result.append((preset.name, content_gb, content_gb <= free_gb))
    return sorted(result, key=lambda item: item[1])
```

Wizard behavior:
- Replace the old options step with region selection followed by tier selection.
- Regions come from discovered canonical preset filenames.
- Review screen shows all sources in the chosen preset.
- Remove prompts for maps/models/installers/infra in this pass.

Manifest behavior:
- Keep `enabled_groups` in the dataclass for backward compatibility, but treat it as deprecated and unused for new drives.

- [ ] **Step 4: Run tests to verify the simplified flow passes**

Run: `uv run pytest -q tests/test_commands.py tests/test_wizard.py tests/test_manifest.py`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/svalbard/commands.py src/svalbard/manifest.py src/svalbard/wizard.py tests/test_commands.py tests/test_wizard.py tests/test_manifest.py
git commit -m "feat: simplify preset selection around standalone canonical presets"
```

### Task 4: Add Multi-Platform Binary Downloads and Platform-Aware Manifest Entries

**Files:**
- Modify: `src/svalbard/manifest.py`
- Modify: `src/svalbard/commands.py`
- Modify: `tests/test_commands.py`
- Modify: `tests/test_manifest.py`

- [ ] **Step 1: Write failing tests for platform expansion and manifest round-tripping**

```python
from svalbard.manifest import Manifest, ManifestEntry
from svalbard.presets import load_preset
from svalbard.commands import expand_source_downloads


def test_expand_source_downloads_creates_one_job_per_platform(tmp_path):
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.id == "kiwix-serve")
    jobs = expand_source_downloads(source, tmp_path)
    assert [job.platform for job in jobs] == [
        "linux-x86_64",
        "linux-arm64",
        "macos-x86_64",
        "macos-arm64",
    ]
    assert all(job.dest_dir.parent == tmp_path / "bin" for job in jobs)


def test_manifest_roundtrip_preserves_platform(tmp_path):
    manifest = Manifest(
        preset="finland-128",
        region="finland",
        target_path="/mnt/drive",
        entries=[
            ManifestEntry(
                id="kiwix-serve",
                type="binary",
                platform="linux-x86_64",
                filename="kiwix-tools_linux-x86_64.tar.gz",
                size_bytes=123,
            )
        ],
    )
    path = tmp_path / "manifest.yaml"
    manifest.save(path)
    loaded = Manifest.load(path)
    assert loaded.entries[0].platform == "linux-x86_64"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `uv run pytest -q tests/test_commands.py tests/test_manifest.py -k "one_job_per_platform or preserves_platform"`

Expected: FAIL because there is no job-expansion helper and `ManifestEntry` has no `platform` field.

- [ ] **Step 3: Implement platform-aware download expansion**

```python
@dataclass
class ManifestEntry:
    id: str
    type: str
    filename: str
    size_bytes: int
    platform: str = ""
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"
    downloaded: str = ""
    url: str = ""
    checksum_sha256: str = ""


@dataclass
class DownloadJob:
    source_id: str
    source_type: str
    url: str
    dest_dir: Path
    platform: str = ""


def expand_source_downloads(source: Source, drive_path: Path) -> list[DownloadJob]:
    if source.platforms:
        return [
            DownloadJob(
                source_id=source.id,
                source_type=source.type,
                url=url,
                dest_dir=drive_path / "bin" / platform,
                platform=platform,
            )
            for platform, url in sorted(source.platforms.items())
        ]

    return [
        DownloadJob(
            source_id=source.id,
            source_type=source.type,
            url=resolve_url(source),
            dest_dir=drive_path / TYPE_DIRS.get(source.type, "other"),
        )
    ]
```

Integration notes:
- `sync_drive()` should iterate expanded jobs instead of one download tuple per source.
- When matching existing manifest entries, use `(source_id, platform)` as the identity.
- `show_status()` should aggregate platformed binary entries into one row per source, marking the source current only when all declared platforms are present on disk.

- [ ] **Step 4: Run tests to verify platform-aware downloads pass**

Run: `uv run pytest -q tests/test_commands.py tests/test_manifest.py`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/svalbard/manifest.py src/svalbard/commands.py tests/test_commands.py tests/test_manifest.py
git commit -m "feat: support multi-platform binary downloads and manifest entries"
```

### Task 5: Refresh User-Facing Docs, Status Output, and Final Test Coverage

**Files:**
- Modify: `src/svalbard/readme_generator.py`
- Modify: `README.md`
- Modify: `tests/test_downloader.py`
- Modify: `tests/test_commands.py`
- Modify: `tests/test_wizard.py`

- [ ] **Step 1: Write failing docs and smoke-level behavior tests**

```python
from svalbard.commands import init_drive, show_status


def test_init_drive_generates_readme_for_canonical_preset(tmp_path):
    init_drive(str(tmp_path), "finland-128")
    readme = (tmp_path / "README.md").read_text()
    assert "Svalbard Drive — finland-128" in readme
    assert "bin/" in readme


def test_show_status_handles_platformed_binary_sources(tmp_path):
    init_drive(str(tmp_path), "finland-128")
    show_status(str(tmp_path))
    assert (tmp_path / "manifest.yaml").exists()
```

- [ ] **Step 2: Run tests to verify they fail where docs/output still reflect the old model**

Run: `uv run pytest -q tests/test_commands.py tests/test_wizard.py tests/test_downloader.py -k "canonical_preset or platformed_binary_sources"`

Expected: FAIL or incomplete assertions because the generated README and help text still describe the old preset families and flat `bin/` layout.

- [ ] **Step 3: Update README content and remove outdated design language**

```markdown
## Presets

Presets are discovered from `src/svalbard/presets/*.yaml` and grouped by region:

- `default-*`: region-neutral, English-first starter drives
- `finland-*`: Finnish + English drives with Finland-specific content where available

This release does not implement generated regional OSM extracts or vector topo downloads.
Map sources, when present, must be ordinary downloadable assets.
```

Drive README notes:
- mention binaries may live in `bin/<platform>/`
- keep `serve.sh` guidance unchanged because it already looks in platform-specific directories

- [ ] **Step 4: Run the full relevant test suite**

Run: `uv run pytest -q tests/test_presets.py tests/test_commands.py tests/test_wizard.py tests/test_manifest.py tests/test_downloader.py`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add README.md src/svalbard/readme_generator.py tests/test_presets.py tests/test_commands.py tests/test_wizard.py tests/test_manifest.py tests/test_downloader.py
git commit -m "docs: update preset restructure messaging and coverage"
```

## Self-Review

- **Spec coverage:** This plan covers canonical preset renaming, standalone preset files, wizard/command simplification, legacy aliasing, and multi-platform binaries. It intentionally does not cover OSM extraction or topo-vector downloading.
- **Placeholder scan:** There are no `TODO` or `TBD` placeholders. Each task names exact files, concrete tests, expected failures, implementation shape, verification commands, and commit messages.
- **Type consistency:** `Source.group`, `Source.platforms`, `ManifestEntry.platform`, `canonical_preset_name()`, and `expand_source_downloads()` are used consistently across tasks.
