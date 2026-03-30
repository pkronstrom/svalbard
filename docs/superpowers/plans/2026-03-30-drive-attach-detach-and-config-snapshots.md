# Drive Attach/Detach And Config Snapshots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CLI-first management for attaching and detaching workspace-local sources to existing drives, snapshot the exact preset and recipe configuration onto each drive, and support editable workspace-owned custom presets without mutating packaged defaults.

**Architecture:** Keep built-in presets and recipes as packaged read-only inputs, introduce a workspace layer for user-owned state (`generated/`, `local/`, custom `presets/`), and snapshot the exact drive configuration into `.svalbard/config/` on each drive. `attach` and `detach` mutate both `manifest.yaml` and the drive snapshot, while `sync` and status/readme generation prefer drive-local config snapshots over host-installed copies.

**Tech Stack:** Python 3.12, Click, PyYAML, pytest

---

### Task 1: Introduce Workspace/Built-In Data Resolution

**Files:**
- Create: `src/svalbard/paths.py`
- Modify: `src/svalbard/local_sources.py`
- Modify: `src/svalbard/presets.py`
- Create: `tests/test_paths.py`

- [ ] **Step 1: Write the failing path-resolution tests**

Add tests covering:

```python
from pathlib import Path


def test_default_workspace_root_falls_back_to_user_data_dir(tmp_path, monkeypatch):
    from svalbard.paths import workspace_root

    monkeypatch.setenv("HOME", str(tmp_path))

    root = workspace_root(None, cwd=tmp_path / "not-a-workspace")

    assert root == tmp_path / ".local" / "share" / "svalbard"


def test_workspace_root_prefers_explicit_workspace(tmp_path):
    from svalbard.paths import workspace_root

    explicit = tmp_path / "my-workspace"
    assert workspace_root(explicit) == explicit.resolve()


def test_workspace_root_prefers_repo_style_workspace_when_present(tmp_path):
    from svalbard.paths import workspace_root

    (tmp_path / "presets").mkdir()
    (tmp_path / "recipes").mkdir()

    assert workspace_root(None, cwd=tmp_path) == tmp_path.resolve()
```

- [ ] **Step 2: Run the new test file and verify it fails**

Run: `uv run pytest -q tests/test_paths.py`

Expected: FAIL because `src/svalbard/paths.py` does not exist yet and workspace resolution is still repo-only.

- [ ] **Step 3: Implement the minimal path-resolution module**

Create `src/svalbard/paths.py` with helpers like:

```python
def builtin_root() -> Path:
    return Path(__file__).resolve().parent.parent.parent


def default_workspace_root() -> Path:
    return Path.home() / ".local" / "share" / "svalbard"


def looks_like_workspace(path: Path) -> bool:
    return (path / "recipes").exists() or (path / "local").exists() or (path / "generated").exists()


def workspace_root(explicit: Path | str | None = None, *, cwd: Path | None = None) -> Path:
    if explicit is not None:
        return Path(explicit).resolve()
    current = (cwd or Path.cwd()).resolve()
    if looks_like_workspace(current):
        return current
    return default_workspace_root().resolve()
```

Update:
- `src/svalbard/local_sources.py` to use `paths.workspace_root`
- `src/svalbard/presets.py` to separate built-in preset/recipe roots from workspace-owned preset roots

- [ ] **Step 4: Run the path tests and existing local source tests**

Run: `uv run pytest -q tests/test_paths.py tests/test_local_sources.py`

Expected: PASS

- [ ] **Step 5: Commit the path-resolution slice**

```bash
git add src/svalbard/paths.py src/svalbard/local_sources.py src/svalbard/presets.py tests/test_paths.py
git commit -m "feat: add workspace and packaged data resolution"
```

### Task 2: Snapshot Preset And Recipe Config Onto Drives

**Files:**
- Create: `src/svalbard/drive_config.py`
- Modify: `src/svalbard/manifest.py`
- Modify: `src/svalbard/commands.py`
- Modify: `src/svalbard/readme_generator.py`
- Create: `tests/test_drive_config.py`

- [ ] **Step 1: Write the failing drive-snapshot tests**

Add tests covering:

```python
def test_init_drive_writes_config_snapshot(tmp_path):
    from svalbard.commands import init_drive

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    assert (drive / ".svalbard" / "config" / "preset.yaml").exists()
    assert (drive / ".svalbard" / "config" / "recipes").exists()


def test_drive_snapshot_contains_selected_local_source_metadata(tmp_path):
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path), local_sources=[source_id])

    snapshot = drive / ".svalbard" / "config" / "local" / "example.yaml"
    assert snapshot.exists()
```

- [ ] **Step 2: Run the new snapshot tests and verify they fail**

Run: `uv run pytest -q tests/test_drive_config.py -k "config_snapshot"`

Expected: FAIL because no drive snapshot writer exists.

- [ ] **Step 3: Implement the drive-config snapshot module**

Create `src/svalbard/drive_config.py` with functions like:

```python
def config_root(drive_path: Path) -> Path:
    return drive_path / ".svalbard" / "config"


def write_drive_snapshot(
    drive_path: Path,
    *,
    preset_name: str,
    workspace_root: Path,
    local_source_ids: list[str],
) -> None:
    ...


def load_snapshot_preset(drive_path: Path) -> Preset | None:
    ...
```

Snapshot layout:

```text
.svalbard/config/
  preset.yaml
  recipes/<id>.yaml
  local/<id>.yaml
```

Update:
- `src/svalbard/commands.py:init_drive()` to write the initial snapshot
- `src/svalbard/readme_generator.py` and any other drive readers to prefer the drive snapshot when available
- `src/svalbard/manifest.py` only if a small helper field is needed; do not overextend the manifest if the filesystem snapshot is sufficient

- [ ] **Step 4: Run snapshot tests plus manifest/readme tests**

Run: `uv run pytest -q tests/test_drive_config.py tests/test_manifest.py tests/test_commands.py`

Expected: PASS

- [ ] **Step 5: Commit the drive snapshot slice**

```bash
git add src/svalbard/drive_config.py src/svalbard/manifest.py src/svalbard/commands.py src/svalbard/readme_generator.py tests/test_drive_config.py tests/test_manifest.py tests/test_commands.py
git commit -m "feat: snapshot drive preset and recipe config"
```

### Task 3: Add `attach` And `detach` For Existing Drives

**Files:**
- Create: `src/svalbard/attach.py`
- Modify: `src/svalbard/cli.py`
- Modify: `src/svalbard/commands.py`
- Modify: `src/svalbard/drive_config.py`
- Create: `tests/test_attach.py`

- [ ] **Step 1: Write the failing attach/detach tests**

Add tests covering:

```python
from click.testing import CliRunner


def test_attach_adds_local_source_to_manifest_and_snapshot(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    result = CliRunner().invoke(main, ["attach", source_id, str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()
    assert (drive / ".svalbard" / "config" / "local" / "example.yaml").exists()


def test_detach_removes_local_source_from_manifest_and_snapshot(tmp_path):
    ...


def test_attach_defaults_drive_path_from_cwd(tmp_path, monkeypatch):
    ...
```

- [ ] **Step 2: Run the attach tests and verify they fail**

Run: `uv run pytest -q tests/test_attach.py`

Expected: FAIL because `attach` and `detach` do not exist.

- [ ] **Step 3: Implement minimal attach/detach behavior**

Create `src/svalbard/attach.py` with helpers like:

```python
def resolve_drive_path(path: str | None = None) -> Path:
    candidate = Path(path or ".").resolve()
    if not (candidate / "manifest.yaml").exists():
        raise FileNotFoundError("No drive manifest found")
    return candidate


def attach_local_source(drive_path: Path, source_id: str, workspace_root: Path) -> None:
    ...


def detach_local_source(drive_path: Path, source_id: str) -> None:
    ...
```

Update `src/svalbard/cli.py`:

```python
@main.command("attach")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option("--workspace", default=None)
def attach_command(...): ...


@main.command("detach")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option("--workspace", default=None)
def detach_command(...): ...
```

Behavior:
- `attach` appends to `manifest.local_sources` if absent
- `attach` writes or refreshes the corresponding `.svalbard/config/local/<id>.yaml`
- `detach` removes the source from `manifest.local_sources`
- `detach` removes the drive-local snapshot file
- both commands resolve the drive from `cwd` when no path is given

- [ ] **Step 4: Run attach tests plus sync regression coverage**

Run: `uv run pytest -q tests/test_attach.py tests/test_commands.py tests/test_local_sources.py`

Expected: PASS

- [ ] **Step 5: Commit the attach/detach slice**

```bash
git add src/svalbard/attach.py src/svalbard/cli.py src/svalbard/commands.py src/svalbard/drive_config.py tests/test_attach.py tests/test_commands.py tests/test_local_sources.py
git commit -m "feat: add attach and detach commands for drives"
```

### Task 4: Add Workspace-Owned Custom Preset Management

**Files:**
- Modify: `src/svalbard/presets.py`
- Modify: `src/svalbard/cli.py`
- Create: `tests/test_presets_cli.py`
- Modify: `README.md`

- [ ] **Step 1: Write the failing preset CLI tests**

Add tests covering:

```python
from click.testing import CliRunner


def test_preset_list_shows_builtin_and_workspace_presets(tmp_path):
    from svalbard.cli import main

    presets_dir = tmp_path / "presets"
    presets_dir.mkdir()
    (presets_dir / "my-pack.yaml").write_text(
        "name: my-pack\ndescription: test\ntarget_size_gb: 1\nregion: default\nsources: []\n"
    )

    result = CliRunner().invoke(main, ["preset", "list", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert "my-pack" in result.output


def test_preset_copy_writes_workspace_owned_preset(tmp_path):
    from svalbard.cli import main

    result = CliRunner().invoke(main, ["preset", "copy", "default-32", "my-pack", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert (tmp_path / "presets" / "my-pack.yaml").exists()
```

- [ ] **Step 2: Run the preset CLI tests and verify they fail**

Run: `uv run pytest -q tests/test_presets_cli.py`

Expected: FAIL because the `preset` command group does not exist.

- [ ] **Step 3: Implement minimal preset management**

Update `src/svalbard/presets.py`:
- allow loading presets from built-in roots plus `workspace/presets`
- keep built-ins read-only
- do not support in-place override of built-ins in v1

Update `src/svalbard/cli.py`:

```python
@main.group("preset")
def preset_group() -> None:
    ...


@preset_group.command("list")
@click.option("--workspace", default=None)
def preset_list(...): ...


@preset_group.command("copy")
@click.argument("source_name")
@click.argument("target_name")
@click.option("--workspace", default=None)
def preset_copy(...): ...
```

Implementation rule:
- `preset copy` writes a full YAML file to `workspace/presets/<target>.yaml`
- later editing can be manual; do not add an editor-launch feature in v1

- [ ] **Step 4: Run the preset CLI tests and preset loader regressions**

Run: `uv run pytest -q tests/test_presets_cli.py tests/test_presets.py`

Expected: PASS

- [ ] **Step 5: Commit the custom preset slice**

```bash
git add src/svalbard/presets.py src/svalbard/cli.py tests/test_presets_cli.py tests/test_presets.py README.md
git commit -m "feat: add workspace preset management commands"
```

### Task 5: Final Verification And Documentation Pass

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-03-30-drive-attach-detach-and-config-snapshots.md`
- Test: `tests/test_paths.py`
- Test: `tests/test_drive_config.py`
- Test: `tests/test_attach.py`
- Test: `tests/test_presets_cli.py`

- [ ] **Step 1: Run the full focused verification suite**

Run: `uv run pytest -q tests/test_paths.py tests/test_drive_config.py tests/test_attach.py tests/test_presets_cli.py tests/test_commands.py tests/test_local_sources.py tests/test_manifest.py tests/test_presets.py`

Expected: PASS

- [ ] **Step 2: Update README command documentation**

Document the CLI flow:

```text
svalbard add <input>
svalbard attach <source-id> [drive-path]
svalbard detach <source-id> [drive-path]
svalbard preset list
svalbard preset copy <built-in> <custom-name>
svalbard sync [drive-path]
```

Mention:
- drive path defaults from `cwd` for `attach`, `detach`, and `sync`
- workspace defaults to the current workspace when detected, else user data dir
- drives carry `.svalbard/config/` snapshots of the preset and used recipes

- [ ] **Step 3: Review implementation against the agreed UX**

Confirm the code covers:
- `add / attach / detach` as the main trio
- CLI-first drive membership management
- packaged built-ins plus workspace-owned custom presets
- drive-local snapshot independence from host install layout
- no mutation of packaged preset files

- [ ] **Step 4: Commit the final integration state**

```bash
git add README.md docs/superpowers/plans/2026-03-30-drive-attach-detach-and-config-snapshots.md tests src
git commit -m "feat: add drive attach/detach and config snapshots"
```
