# Local Sources And Crawl Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add workspace-scoped local sources, deterministic drive import, and split crawl commands that generate and register local ZIM sources.

**Architecture:** Add a workspace-aware local source catalog on top of the existing preset/recipe system rather than replacing it. Extend manifest and sync logic to merge selected local sources with preset sources, then layer CLI and wizard support on top of that resolver.

**Tech Stack:** Python 3.12, Click, PyYAML, Rich, pytest, Docker/Zimit integration

---

### Task 1: Manifest And Source Model Foundations

**Files:**
- Modify: `src/svalbard/models.py`
- Modify: `src/svalbard/manifest.py`
- Test: `tests/test_manifest.py`

- [ ] **Step 1: Write the failing manifest/model tests**

```python
def test_manifest_roundtrip_preserves_workspace_and_local_sources(tmp_path):
    from svalbard.manifest import LocalSourceSnapshot, Manifest

    manifest = Manifest(
        preset="default-64",
        region="default",
        target_path="/tmp/drive",
        workspace_root="/tmp/workspace",
        local_sources=["local:example-docs"],
        local_source_snapshots=[
            LocalSourceSnapshot(
                id="local:example-docs",
                path="generated/example-docs.zim",
                kind="file",
                size_bytes=123,
                mtime=456.0,
            )
        ],
    )
    path = tmp_path / "manifest.yaml"
    manifest.save(path)
    loaded = Manifest.load(path)
    assert loaded.workspace_root == "/tmp/workspace"
    assert loaded.local_sources == ["local:example-docs"]
    assert loaded.local_source_snapshots[0].kind == "file"


def test_source_accepts_local_path_and_size_bytes():
    from svalbard.models import Source

    source = Source(
        id="local:example-docs",
        type="zim",
        group="practical",
        strategy="local",
        path="generated/example-docs.zim",
        size_bytes=123456789,
    )
    assert source.path == "generated/example-docs.zim"
    assert source.size_bytes == 123456789
    assert round(source.size_gb, 3) == round(123456789 / 1e9, 3)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest -q tests/test_manifest.py -k "workspace_and_local_sources or local_path_and_size_bytes"`
Expected: FAIL with missing manifest/source fields or import errors.

- [ ] **Step 3: Write minimal implementation**

```python
@dataclass
class LocalSourceSnapshot:
    id: str
    path: str
    kind: str
    size_bytes: int
    mtime: float = 0.0
    checksum_sha256: str = ""


@dataclass
class Manifest:
    ...
    workspace_root: str = ""
    local_sources: list[str] = field(default_factory=list)
    local_source_snapshots: list[LocalSourceSnapshot] = field(default_factory=list)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest -q tests/test_manifest.py -k "workspace_and_local_sources or local_path_and_size_bytes"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/test_manifest.py src/svalbard/models.py src/svalbard/manifest.py
git commit -m "feat: add local source manifest metadata"
```

### Task 2: Workspace And Local Source Discovery

**Files:**
- Modify: `src/svalbard/presets.py`
- Create: `src/svalbard/local_sources.py`
- Test: `tests/test_presets.py`
- Create: `tests/test_local_sources.py`

- [ ] **Step 1: Write the failing discovery tests**

```python
def test_workspace_root_is_repo_root():
    from svalbard.local_sources import workspace_root

    root = workspace_root()
    assert (root / "pyproject.toml").exists()


def test_load_local_sources_discovers_sidecars_and_derives_size_gb(tmp_path):
    from svalbard.local_sources import load_local_sources

    local_dir = tmp_path / "local"
    generated_dir = tmp_path / "generated"
    local_dir.mkdir()
    generated_dir.mkdir()
    artifact = generated_dir / "example.zim"
    artifact.write_bytes(b"x" * 100)
    (local_dir / "example.yaml").write_text(
        \"\"\"id: local:example
type: zim
group: practical
strategy: local
path: generated/example.zim
size_bytes: 100
\"\"\"
    )

    sources = load_local_sources(tmp_path)
    assert [s.id for s in sources] == ["local:example"]
    assert sources[0].path == "generated/example.zim"
    assert sources[0].size_gb > 0
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest -q tests/test_presets.py tests/test_local_sources.py -k "workspace_root or load_local_sources_discovers"`
Expected: FAIL because local source helpers do not exist.

- [ ] **Step 3: Write minimal implementation**

```python
def workspace_root(explicit: Path | None = None) -> Path:
    if explicit:
        return explicit.resolve()
    return Path(__file__).resolve().parent.parent.parent


def load_local_sources(root: Path | None = None) -> list[Source]:
    ...
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest -q tests/test_presets.py tests/test_local_sources.py -k "workspace_root or load_local_sources_discovers"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/test_presets.py tests/test_local_sources.py src/svalbard/presets.py src/svalbard/local_sources.py
git commit -m "feat: add workspace-scoped local source discovery"
```

### Task 3: Local Add And Sync Integration

**Files:**
- Modify: `src/svalbard/commands.py`
- Modify: `src/svalbard/cli.py`
- Test: `tests/test_commands.py`
- Modify: `tests/test_manifest.py`

- [ ] **Step 1: Write the failing sync/local-add tests**

```python
def test_add_local_file_writes_sidecar(tmp_path):
    from svalbard.commands import add_local_source

    artifact = tmp_path / "manual.zim"
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")
    sidecar = tmp_path / "local" / "manual.yaml"
    assert source_id == "local:manual"
    assert sidecar.exists()


def test_sync_copies_selected_local_source(tmp_path):
    from svalbard.commands import init_drive, sync_drive
    from svalbard.manifest import Manifest

    generated = tmp_path / "generated"
    local = tmp_path / "local"
    generated.mkdir()
    local.mkdir()
    (generated / "example.zim").write_bytes(b"data")
    (local / "example.yaml").write_text(
        \"\"\"id: local:example
type: zim
group: practical
strategy: local
path: generated/example.zim
size_bytes: 4
\"\"\"
    )

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path), local_sources=["local:example"])
    sync_drive(str(drive))
    assert (drive / "zim" / "local-example.zim").exists()
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.entry_by_id("local:example") is not None
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest -q tests/test_commands.py -k "add_local_file_writes_sidecar or sync_copies_selected_local_source"`
Expected: FAIL because commands and init/sync signatures do not yet support local sources.

- [ ] **Step 3: Write minimal implementation**

```python
def add_local_source(path: Path, workspace_root: Path, source_type: str | None = None, source_id: str | None = None) -> str:
    ...


def _active_sources(manifest: Manifest) -> list[Source]:
    preset = load_preset(manifest.preset)
    local_sources = resolve_selected_local_sources(...)
    return preset.sources + local_sources
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run pytest -q tests/test_commands.py -k "add_local_file_writes_sidecar or sync_copies_selected_local_source"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/test_commands.py tests/test_manifest.py src/svalbard/commands.py src/svalbard/cli.py src/svalbard/manifest.py
git commit -m "feat: add local source registration and sync import"
```

### Task 4: Wizard And User-Facing Integration

**Files:**
- Modify: `src/svalbard/wizard.py`
- Modify: `src/svalbard/readme_generator.py`
- Modify: `src/svalbard/audit.py`
- Test: `tests/test_wizard.py`

- [ ] **Step 1: Write the failing wizard/user-facing tests**

```python
def test_presets_for_space_can_add_local_size(tmp_path):
    from svalbard.local_sources import LocalSourceInfo, summarize_local_sources

    local_sources = [LocalSourceInfo(id="local:example", size_bytes=2_000_000_000, description="Example", type="zim")]
    summary = summarize_local_sources(local_sources)
    assert summary.total_size_gb >= 1.9
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest -q tests/test_wizard.py -k "local"`
Expected: FAIL because local-source summary helpers are missing from the wizard flow.

- [ ] **Step 3: Write minimal implementation**

```python
def discovered_local_sources(root: Path | None = None) -> list[Source]:
    return load_local_sources(root)
```

- [ ] **Step 4: Run targeted tests**

Run: `uv run pytest -q tests/test_wizard.py tests/test_commands.py -k "local or sync_copies_selected_local_source"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/test_wizard.py src/svalbard/wizard.py src/svalbard/readme_generator.py src/svalbard/audit.py
git commit -m "feat: surface local sources in wizard and drive metadata"
```

### Task 5: Split Crawl CLI And Generated Registration

**Files:**
- Modify: `src/svalbard/crawler.py`
- Modify: `src/svalbard/cli.py`
- Test: `tests/test_commands.py`
- Create: `tests/test_crawler.py`

- [ ] **Step 1: Write the failing crawl tests**

```python
def test_crawl_url_writes_generated_artifact_and_recipe(tmp_path):
    from svalbard.crawler import register_crawled_zim

    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = register_crawled_zim(
        workspace_root=tmp_path,
        artifact_path=artifact,
        origin_url="https://example.com/docs",
        source_id="local:example",
    )
    assert source_id == "local:example"
    assert (tmp_path / "local" / "example.yaml").exists()
    assert (tmp_path / "generated" / "example.crawl.yaml").exists()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run pytest -q tests/test_crawler.py`
Expected: FAIL because crawl registration helpers and split CLI modes do not exist.

- [ ] **Step 3: Write minimal implementation**

```python
@crawl.group()
def crawl() -> None:
    ...


@crawl.command("url")
def crawl_url(url: str, output: str, workspace: str | None) -> None:
    ...
```

- [ ] **Step 4: Run targeted tests**

Run: `uv run pytest -q tests/test_crawler.py tests/test_commands.py -k "crawl or local"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/test_crawler.py src/svalbard/crawler.py src/svalbard/cli.py
git commit -m "feat: split crawl commands and register generated zim sources"
```

### Final Verification

**Files:**
- Test: `tests/test_manifest.py`
- Test: `tests/test_presets.py`
- Test: `tests/test_commands.py`
- Test: `tests/test_wizard.py`
- Test: `tests/test_crawler.py`

- [ ] **Step 1: Run full focused verification**

Run: `uv run pytest -q tests/test_manifest.py tests/test_presets.py tests/test_commands.py tests/test_wizard.py tests/test_crawler.py`
Expected: PASS with 0 failures.

- [ ] **Step 2: Review spec coverage manually**

Check that the implementation covers workspace root, namespaced local IDs, manifest snapshots, deterministic destinations, split crawl commands, and wizard/local-source integration.

- [ ] **Step 3: Commit final integration fixes**

```bash
git add src/svalbard tests docs/superpowers/plans/2026-03-30-local-sources-and-crawl-import.md
git commit -m "feat: implement local sources and crawl import flow"
```
