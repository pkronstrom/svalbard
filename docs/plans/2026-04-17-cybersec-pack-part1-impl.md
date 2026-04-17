# Cybersec Pack Part 1: Infrastructure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire up the new recipe types (`python-venv`, `python-package`, `dataset`) and add bare `.gz` decompression so the cybersec pack recipes can actually sync.

**Architecture:** Three changes: (1) Python-side — extend `Source` model, `TYPE_DIRS`, and add a `python-venv` builder that collects `python-package` recipes and runs `uv` at sync time. (2) Python-side — add `dataset` type handling for archive extraction to `data/`. (3) Go-side — add bare `.gz` decompression to the drive runtime binary resolver.

**Tech Stack:** Python 3.12+ (dataclasses, subprocess, pathlib), Go (compress/gzip), uv CLI

**Note:** No Python test suite exists in this project. Go tests exist in `drive-runtime/`. Python changes are validated manually via `svalbard sync`. Go changes have unit tests.

---

### Task 1: Add `dataset` and `python-venv` to TYPE_DIRS and Source model

**Files:**
- Modify: `src/svalbard/commands.py:32-43` (TYPE_DIRS, TYPE_GROUPS)
- Modify: `src/svalbard/models.py:15-33` (Source dataclass)
- Modify: `src/svalbard/presets.py:16-23` (_source_from_recipe)

**Step 1: Update TYPE_DIRS and TYPE_GROUPS**

In `src/svalbard/commands.py`, add to `TYPE_DIRS`:
```python
TYPE_DIRS = {
    "zim": "zim",
    "pmtiles": "maps",
    "pdf": "books",
    "epub": "books",
    "gguf": "models",
    "binary": "bin",
    "app": "apps",
    "iso": "infra",
    "sqlite": "data",
    "dataset": "data",                       # NEW
    "python-venv": "runtime/python",         # NEW
    "python-package": "runtime/python",      # NEW (logical; builder handles actual placement)
    "toolchain": "tools/platformio/packages",
}
```

Add to `TYPE_GROUPS`:
```python
TYPE_GROUPS = {
    ...
    "dataset": "tools",       # NEW
    "python-venv": "tools",   # NEW
    "python-package": "tools", # NEW
}
```

**Step 2: Add new fields to Source model**

In `src/svalbard/models.py`, add fields to `Source`:
```python
@dataclass
class Source:
    ...
    # Python venv/package fields
    python: str = ""            # e.g. ">=3.11" (python-venv only)
    venv: str = ""              # e.g. "svalbard-python" (python-package only)
    packages: list[str] = field(default_factory=list)  # pip packages
    entry_points: list[str] = field(default_factory=list)  # CLI entry points
```

**Step 3: Update _source_from_recipe to pass new fields**

No change needed — `_source_from_recipe` already uses `Source.__dataclass_fields__` to
auto-include any field present in the recipe dict. The new fields will be picked up
automatically since they match the YAML keys.

**Step 4: Verify preset loading works**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -c "from svalbard.presets import load_preset; p = load_preset('cybersec'); print([(s.id, s.type) for s in p.sources])"`

Expected: list of (id, type) tuples for all cybersec sources, including `sqlmap` with `type='python-package'`.

**Step 5: Commit**
```
feat(models): add dataset, python-venv, python-package type support
```

---

### Task 2: Skip python-venv/python-package from normal download flow

**Files:**
- Modify: `src/svalbard/commands.py:468-471` (sync_drive source splitting)

**Step 1: Update source splitting in sync_drive**

The current code splits on `strategy != "build"` vs `strategy == "build"`. Python-package
recipes have `strategy: download` (default) but no URL — they'd crash in `resolve_url()`.
Python-venv recipes also have no URL.

Add these types to the build path so they're handled by the builder, not the downloader:

```python
# Split sources by strategy
_BUILDER_TYPES = {"python-venv", "python-package"}
download_sources_list = [
    s for s in preset.sources
    if s.strategy != "build" and s.type not in _BUILDER_TYPES
]
build_sources = [
    s for s in preset.sources
    if s.strategy == "build" or s.type in _BUILDER_TYPES
]
```

**Step 2: Verify sqlmap doesn't crash sync**

Run: `python -c "from svalbard.commands import expand_source_downloads; from svalbard.presets import load_preset; p = load_preset('cybersec'); print([s.id for s in p.sources if s.type in ('python-venv', 'python-package')])"`

Expected: `['sqlmap']` — confirming it would be routed to builder, not downloader.

**Step 3: Commit**
```
fix(sync): route python-venv/python-package to builder path
```

---

### Task 3: Register python-venv builder

**Files:**
- Modify: `src/svalbard/builder.py` (add new handler + TOOL_REQUIREMENTS entry)

**Step 1: Add tool requirements**

```python
TOOL_REQUIREMENTS: dict[str, list[str]] = {
    ...
    "python-venv": ["uv"],
}
```

**Step 2: Register the python-venv builder**

Add at the bottom of `builder.py`, before the `custom` handler:

```python
@_register("python-venv")
def build_python_venv(source: Source, drive_path: Path, cache: Path) -> BuildResult:
    """Build a shared Python venv using uv.

    Collects all python-package sources from the drive's preset snapshot,
    merges their packages lists, and installs into runtime/python/<platform>/.
    Generates wrapper scripts in bin/<platform>/ for each entry_point.
    """
    import platform as _platform

    os_name = "macos" if _platform.system() == "Darwin" else "linux"
    arch = "arm64" if _platform.machine() in ("aarch64", "arm64") else "x86_64"
    platform_dir = f"{os_name}-{arch}"

    # Find uv binary
    uv = _find_tool("uv", drive_path) or shutil.which("uv")
    if not uv:
        return BuildResult(source.id, False, error="uv not found on drive or PATH")

    python_spec = source.python or ">=3.11"
    venv_dir = drive_path / "runtime" / "python" / platform_dir
    venv_dir.mkdir(parents=True, exist_ok=True)

    # Install Python if needed
    if not (venv_dir / "bin" / "python3").exists():
        log.info("Installing Python %s via uv", python_spec)
        result = subprocess.run(
            [uv, "python", "install", "--install-dir", str(venv_dir / ".pythons"), python_spec],
            capture_output=True, text=True,
        )
        if result.returncode != 0:
            return BuildResult(source.id, False, error=f"uv python install failed: {result.stderr[-300:]}")

        # Create venv
        # Find the installed python
        python_bins = list((venv_dir / ".pythons").rglob("python3*"))
        python_bin = next((p for p in python_bins if p.is_file() and p.stat().st_mode & 0o111), None)
        if not python_bin:
            return BuildResult(source.id, False, error="Could not find installed Python binary")

        result = subprocess.run(
            [uv, "venv", str(venv_dir), "--python", str(python_bin)],
            capture_output=True, text=True,
        )
        if result.returncode != 0:
            return BuildResult(source.id, False, error=f"uv venv failed: {result.stderr[-300:]}")

    # Collect python-package sources from preset snapshot
    from svalbard.drive_config import load_snapshot_preset
    snapshot = load_snapshot_preset(drive_path)
    if snapshot is None:
        return BuildResult(source.id, False, error="No preset snapshot on drive")

    all_packages: list[str] = []
    all_entry_points: dict[str, str] = {}  # entry_point -> source_id
    for s in snapshot.sources:
        if s.type == "python-package" and s.venv == source.id:
            all_packages.extend(s.packages)
            for ep in s.entry_points:
                all_entry_points[ep] = s.id

    if all_packages:
        log.info("Installing %d packages into %s", len(all_packages), venv_dir)
        result = subprocess.run(
            [uv, "pip", "install", "--python", str(venv_dir / "bin" / "python3"), *all_packages],
            capture_output=True, text=True,
        )
        if result.returncode != 0:
            return BuildResult(source.id, False, error=f"uv pip install failed: {result.stderr[-500:]}")

    # Generate wrapper scripts in bin/<platform>/
    bin_dir = drive_path / "bin" / platform_dir
    bin_dir.mkdir(parents=True, exist_ok=True)
    for ep_name in all_entry_points:
        wrapper = bin_dir / ep_name
        wrapper.write_text(
            f'#!/bin/sh\n'
            f'DRIVE="$(cd "$(dirname "$0")/../.." && pwd)"\n'
            f'exec "$DRIVE/runtime/python/{platform_dir}/bin/{ep_name}" "$@"\n'
        )
        wrapper.chmod(0o755)
        log.info("Generated wrapper: %s", wrapper)

    return BuildResult(source.id, True, artifact=venv_dir)
```

**Step 3: Handle python-package in sync — don't build individually**

In `sync_drive`, python-package sources should NOT be built individually — they're
handled as part of the python-venv build. Add a filter in the build loop:

In `src/svalbard/commands.py`, where `pending_builds` is constructed (~line 632):
```python
pending_builds = []
for source in build_sources:
    # python-package sources are installed by the python-venv builder, not individually
    if source.type == "python-package":
        continue
    if force or _artifact_path_for_build(source, drive_path) is None:
        pending_builds.append(source)
    else:
        skipped += 1
```

Also, python-venv sources use family implicitly — update `run_build` dispatch
to handle `type == "python-venv"` without requiring `build.family`:

In `builder.py`, update `run_build`:
```python
def run_build(source: Source, drive_path: Path, cache_dir: Path | None = None) -> BuildResult:
    """Dispatch a build source to its handler."""
    # python-venv type uses its own handler directly
    if source.type == "python-venv":
        family = "python-venv"
    else:
        family = source.build.get("family", "")
    handler = HANDLERS.get(family)
    ...
```

**Step 4: Verify the builder registers**

Run: `python -c "from svalbard.builder import HANDLERS; print(list(HANDLERS.keys()))"`

Expected: list includes `"python-venv"`.

**Step 5: Commit**
```
feat(builder): add python-venv builder with uv integration
```

---

### Task 4: Add dataset type handling to sync

**Files:**
- Modify: `src/svalbard/commands.py:56-62` (_ARCHIVE_SUFFIXES, _is_archive)

**Step 1: Add .gz to archive suffixes**

The Python downloader saves the file as-is. For `dataset` type sources with `.zip` URLs
(like SecLists), the downloader saves the archive. We need to ensure the file ends up in
`data/` which is already handled by `TYPE_DIRS["dataset"] = "data"` from Task 1.

No additional extraction logic is needed at download time — datasets are stored as
archives on the drive and extracted by the user or tooling as needed. The existing
download flow with `TYPE_DIRS` routing is sufficient.

**Step 2: Verify dataset routing**

Run: `python -c "from svalbard.commands import TYPE_DIRS; print(TYPE_DIRS.get('dataset'))"`

Expected: `data`

**Step 3: Commit**
```
feat(sync): add dataset type routing to data/ directory
```

---

### Task 5: Add bare .gz decompression to Go runtime resolver

**Files:**
- Modify: `drive-runtime/internal/binary/runtimebinary.go:59-77` (add .gz case)
- Modify: `drive-runtime/internal/binary/runtimebinary_test.go` (add test)

**Step 1: Write the failing test**

In `drive-runtime/internal/binary/runtimebinary_test.go`, add:

```go
func TestResolve_BareGzExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin", "linux-x86_64", "chisel")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a bare .gz file containing a fake binary
	content := []byte("#!/bin/sh\necho chisel")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(content); err != nil {
		t.Fatalf("gzip write error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close error = %v", err)
	}

	gzPath := filepath.Join(binDir, "chisel_1.0_linux_amd64.gz")
	if err := os.WriteFile(gzPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("chisel", tmpDir, func() (string, error) {
		return "linux-x86_64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if filepath.Base(got) != "chisel" {
		t.Errorf("Resolve() = %v, want chisel binary", got)
	}

	// Verify the extracted file is executable and has correct content
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("extracted content = %q, want %q", data, content)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd drive-runtime && go test ./internal/binary/ -run TestResolve_BareGzExtraction -v`

Expected: FAIL

**Step 3: Add extractBareGz function and wire it in**

In `drive-runtime/internal/binary/runtimebinary.go`, add the case in `resolveFromDir`:

```go
case strings.HasSuffix(entry.Name(), ".gz") && !strings.HasSuffix(entry.Name(), ".tar.gz"):
    if err := extractBareGz(path, dir); err != nil {
        return "", err
    }
    extracted = true
```

Add the extraction function:

```go
func extractBareGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Derive output name: strip .gz suffix from filename
	outName := strings.TrimSuffix(filepath.Base(archivePath), ".gz")
	// Strip version/platform suffixes to get the tool name
	// e.g. "chisel_1.11.5_linux_amd64" -> keep as-is, findMatchingBinary handles it
	outPath := filepath.Join(destDir, outName)

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gzr); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
```

**Step 4: Run test to verify it passes**

Run: `cd drive-runtime && go test ./internal/binary/ -run TestResolve_BareGzExtraction -v`

Expected: PASS

**Step 5: Run full binary test suite**

Run: `cd drive-runtime && go test ./internal/binary/ -v`

Expected: all tests PASS

**Step 6: Commit**
```
feat(runtime): add bare .gz decompression to binary resolver
```

---

### Task 6: Add .gz to Python-side archive suffixes

**Files:**
- Modify: `src/svalbard/commands.py:57` (_ARCHIVE_SUFFIXES)

**Step 1: Add .gz**

```python
_ARCHIVE_SUFFIXES = {".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip", ".gz"}
```

**Step 2: Commit**
```
feat(sync): recognize bare .gz as archive suffix
```

---

### Task 7: Integration verification

**Step 1: Verify preset loads all cybersec sources**

Run: `python -c "
from svalbard.presets import load_preset
p = load_preset('cybersec')
for s in p.sources:
    print(f'{s.id:30s} type={s.type:20s} strategy={s.strategy}')
"`

Expected: all 17 sources listed with correct types.

**Step 2: Verify cybersec-ctf loads**

Run: `python -c "
from svalbard.presets import load_preset
p = load_preset('cybersec-ctf')
for s in p.sources:
    print(f'{s.id:30s} type={s.type:20s} packages={s.packages}')
"`

Expected: 7 sources, python-package ones show their packages.

**Step 3: Verify builder dispatch**

Run: `python -c "
from svalbard.builder import HANDLERS
print('Registered handlers:', list(HANDLERS.keys()))
assert 'python-venv' in HANDLERS, 'python-venv handler missing'
print('OK')
"`

**Step 4: Run Go tests**

Run: `cd drive-runtime && go test ./... 2>&1 | tail -20`

Expected: all pass

**Step 5: Commit (if any fixups needed)**
```
fix: integration fixups for cybersec pack infrastructure
```
