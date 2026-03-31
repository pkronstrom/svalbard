# Manifest Integrity & Drift Detection

**Goal:** Add commands to detect foreign files, verify checksums, diff presets, and plan drive downsizing — turning the manifest into a reliable source of truth for what's on disk.

**Architecture:** Four new CLI subcommands (`inventory`, `verify`, `diff`, `budget`) that build on the existing `Manifest` and `Preset` infrastructure. All are read-only operations (no downloads, no deletes) — they report what *would* need to change and let the user decide.

**Tech Stack:** Python, Click, Rich tables, hashlib (sha256)

---

## Context

The manifest (`manifest.yaml`) already tracks every downloaded/built item with filename, size, checksum, URL, and download date. But it has blind spots:

| Gap | Description |
|-----|-------------|
| Foreign files | Files added to drive outside svalbard are invisible |
| Post-download corruption | Checksums verified at download, never again |
| Preset change planning | No way to preview add/remove when switching presets |
| Disk budget | No "what fits on a 64GB stick?" planning tool |

The existing `svalbard status --check` handles URL-level freshness. These new commands handle *disk-level truth*.

## Key design decisions

- **Read-only.** None of these commands modify files or manifest. They report. The user acts via `sync`, manual deletion, or future `svalbard prune`.
- **No new dataclasses.** Results are rendered directly with Rich tables/panels. No intermediate model layer.
- **Manifest module grows helpers, commands module grows commands.** Keep the same file topology.
- **`TYPE_DIRS` is the canonical mapping** from source type to subdirectory (already in `commands.py`). Reuse it — don't duplicate.

---

## Task 1: `svalbard inventory` — disk vs manifest comparison

**Purpose:** Walk the drive, compare every file against manifest entries, report three categories:

1. **Tracked & present** — file exists, manifest entry exists (happy path)
2. **Tracked & missing** — manifest entry exists, file gone (already covered by `status`, but include for completeness)
3. **Foreign** — file exists on disk, no manifest entry

**Files:**
- Modify: `src/svalbard/commands.py` — add `inventory_drive(path)` function
- Modify: `src/svalbard/cli.py` — add `inventory` subcommand
- Test: `tests/test_commands.py` — test inventory logic

**Design:**

```python
def inventory_drive(path: str):
    """Scan drive and compare disk contents against manifest."""
    drive_path = Path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    # Build set of expected files from manifest entries
    expected_files: dict[Path, ManifestEntry] = {}
    for entry in manifest.entries:
        type_dir = TYPE_DIRS.get(entry.type, "other")
        if entry.platform:
            file_path = drive_path / "bin" / entry.platform / entry.filename
        elif entry.type == "app":
            file_path = drive_path / type_dir / entry.filename  # directory name
        else:
            file_path = drive_path / type_dir / entry.filename
        expected_files[file_path] = entry

    # Walk all content directories, collect actual files
    content_dirs = set(TYPE_DIRS.values())  # zim, maps, books, models, bin, apps, infra, data
    actual_files: set[Path] = set()
    for dir_name in content_dirs:
        content_dir = drive_path / dir_name
        if content_dir.exists():
            for f in content_dir.rglob("*"):
                if f.is_file():
                    actual_files.add(f)

    # Categorize
    tracked_present = {p: e for p, e in expected_files.items() if p in actual_files}
    tracked_missing = {p: e for p, e in expected_files.items() if p not in actual_files}
    foreign = actual_files - set(expected_files.keys())

    # Render table with Rich
```

**Skip files:** `manifest.yaml`, `serve.sh`, `README.md`, `apps/map/index.html` — these are generated, not tracked sources. Hard-code a skip list for generated files at the drive root and in `apps/map/`.

**Output format:**

```
Svalbard Inventory — finland-128

  Tracked:  42 files (87.3 GB)
  Missing:   1 file  (wikipedia-fi-nopic — expected in zim/)
  Foreign:   3 files (2.1 GB)

Foreign files:
  zim/my-custom-notes.zim        (1.2 GB)
  maps/old-export.pmtiles        (0.8 GB)
  books/random.pdf               (0.1 GB)

Tip: Foreign files are not managed by svalbard and won't appear in status.
```

**CLI:**

```python
@main.command()
@click.argument("path", default=".")
def inventory(path: str) -> None:
    """Compare disk contents against manifest — find foreign and missing files."""
    from svalbard.commands import inventory_drive
    inventory_drive(path)
```

**Edge cases:**
- App-type entries (`type: "app"`) are directories, not files — compare directory existence
- Platform binaries live in `bin/{platform}/` — scan those subdirectories
- Symlinks: follow them (default `rglob` behavior)
- Empty drive (no content dirs yet): report "no content directories found"

---

## Task 2: `svalbard verify` — re-hash files against manifest checksums

**Purpose:** Recompute SHA-256 of every tracked file on disk, compare against manifest `checksum_sha256`. Reports:

1. **OK** — hash matches
2. **CORRUPTED** — hash mismatch
3. **NO HASH** — manifest entry has empty checksum (build artifacts, older entries)
4. **MISSING** — file not on disk

**Files:**
- Modify: `src/svalbard/commands.py` — add `verify_drive(path)` function
- Modify: `src/svalbard/cli.py` — add `verify` subcommand
- Test: `tests/test_commands.py`

**Design:**

```python
def verify_drive(path: str):
    """Re-hash tracked files and compare against manifest checksums."""
    drive_path = Path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")

    for entry in sorted(manifest.entries, key=lambda e: e.id):
        file_path = _resolve_entry_path(entry, drive_path)

        if not file_path.exists():
            print(f"  MISSING  {entry.id}")
            continue

        if not entry.checksum_sha256:
            print(f"  NO HASH  {entry.id}")
            continue

        computed = compute_sha256(file_path)
        if computed == entry.checksum_sha256:
            print(f"  OK       {entry.id}")
        else:
            print(f"  CORRUPT  {entry.id}  expected={entry.checksum_sha256[:12]}... got={computed[:12]}...")
```

**Helper to extract:** `_resolve_entry_path(entry, drive_path) -> Path` — shared between `inventory` and `verify`. Belongs in `commands.py` or `manifest.py`. Uses `TYPE_DIRS` + platform logic.

**Reuse `downloader.py` hash logic:** The downloader already computes SHA-256. Extract or reuse `compute_sha256(path) -> str` from there.

**Progress:** Use `rich.progress` bar since hashing large ZIM files (20+ GB) takes time. Show file name and progress percentage.

**CLI:**

```python
@main.command()
@click.argument("path", default=".")
def verify(path: str) -> None:
    """Re-hash all tracked files and check for corruption."""
    from svalbard.commands import verify_drive
    verify_drive(path)
```

**Exit code:** Return non-zero if any file is CORRUPT (useful for scripting). Click supports this via `ctx.exit(1)`.

---

## Task 3: `svalbard diff <preset>` — preview preset switch

**Purpose:** Compare the current manifest (what's downloaded) against a different preset (what *would* be needed). Reports:

1. **Keep** — source in both current manifest and target preset
2. **Add** — source in target preset but not in manifest
3. **Remove** — source in manifest but not in target preset
4. **Size delta** — net change in GB

**Files:**
- Modify: `src/svalbard/commands.py` — add `diff_presets(path, target_preset)` function
- Modify: `src/svalbard/cli.py` — add `diff` subcommand
- Test: `tests/test_commands.py`

**Design:**

```python
def diff_presets(path: str, target_preset_name: str):
    """Show what would change if switching to a different preset."""
    drive_path = Path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")
    current_preset = load_preset(manifest.preset)
    target_preset = load_preset(target_preset_name)

    current_ids = {s.id for s in current_preset.sources}
    target_ids = {s.id for s in target_preset.sources}

    keep = current_ids & target_ids
    add = target_ids - current_ids
    remove = current_ids - target_ids

    # Build lookup for sizes
    target_sources = {s.id: s for s in target_preset.sources}
    current_entries = {e.id: e for e in manifest.entries}

    # Render table
```

**Output format:**

```
Diff: finland-128 → finland-64

  Keep:    38 sources (72.4 GB)
  Add:      2 sources (~3.1 GB)
    + osm-europe          pmtiles  ~2.8 GB
    + stackexchange-math   zim     ~0.3 GB
  Remove:   6 sources (14.2 GB)
    - libretexts-medicine  zim      4.1 GB
    - gutenberg            zim      3.8 GB
    - libretexts-eng       zim      3.2 GB
    ...

  Net change: -11.1 GB
  Target capacity: 64 GB  |  Estimated usage: ~61.3 GB
```

**Size source:** For "keep" and "remove", use `manifest.entries[].size_bytes` (actual). For "add", use `target_preset.sources[].size_gb` (estimated from recipe).

**CLI:**

```python
@main.command(name="diff")
@click.argument("path", default=".")
@click.argument("target_preset")
def diff_cmd(path: str, target_preset: str) -> None:
    """Preview what changes if you switch to a different preset."""
    from svalbard.commands import diff_presets
    diff_presets(path, target_preset)
```

---

## Task 4: `svalbard budget <size_gb>` — capacity planning

**Purpose:** Given a target drive size, show what fits and what doesn't from the current preset. Prioritize by a simple heuristic: keep required items first, then by group priority, then by size (smaller first to maximize count).

**Files:**
- Modify: `src/svalbard/commands.py` — add `budget_plan(path, target_gb)` function
- Modify: `src/svalbard/cli.py` — add `budget` subcommand
- Test: `tests/test_commands.py`

**Design:**

```python
# Priority order for groups (lower = kept first)
GROUP_PRIORITY = {
    "tools": 0,       # tiny, essential for accessing content
    "practical": 1,    # survival guides
    "maps": 2,         # navigation
    "regional": 3,     # local knowledge
    "reference": 4,    # wikipedia etc — large but important
    "education": 5,    # textbooks
    "models": 6,       # LLMs — large, nice-to-have
}

def budget_plan(path: str, target_gb: float):
    """Show what fits within a target drive size."""
    drive_path = Path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    # Use actual sizes from manifest where available, estimated from recipe otherwise
    items = []
    for source in preset.sources:
        entry = manifest.entry_by_id(source.id)
        size_gb = entry.size_bytes / 1e9 if entry else source.size_gb
        items.append((source, size_gb))

    # Sort by priority then size
    items.sort(key=lambda x: (GROUP_PRIORITY.get(x[0].group, 99), x[1]))

    # Greedy knapsack
    fits = []
    drops = []
    used_gb = 0
    overhead_gb = 0.5  # manifest, serve.sh, readme, map viewer, filesystem overhead

    for source, size_gb in items:
        if used_gb + size_gb + overhead_gb <= target_gb:
            fits.append((source, size_gb))
            used_gb += size_gb
        else:
            drops.append((source, size_gb))

    # Render
```

**Output format:**

```
Budget: finland-128 → 64 GB target

  Fits (38 sources, 61.2 GB):
    Group        Sources  Size
    tools        4        0.3 GB
    practical    8        2.1 GB
    maps         6        4.8 GB
    regional     5        1.2 GB
    reference    10       48.1 GB
    education    5        4.7 GB

  Dropped (6 sources, 18.4 GB):
    - llama-3.2-3b         models    3.2 GB
    - llama-3.2-1b         models    1.1 GB
    - libretexts-eng       education 4.8 GB
    - gutenberg            reference 3.8 GB
    ...

  Tip: Use "svalbard diff" to compare against an existing smaller preset.
```

**No writes.** This is purely advisory. A future `svalbard prune` could act on this, but that's out of scope.

**CLI:**

```python
@main.command()
@click.argument("path", default=".")
@click.argument("target_gb", type=float)
def budget(path: str, target_gb: float) -> None:
    """Show what fits within a target drive size."""
    from svalbard.commands import budget_plan
    budget_plan(path, target_gb)
```

---

## Task 5: Shared helper — `_resolve_entry_path`

Both `inventory` and `verify` need to resolve a `ManifestEntry` to a `Path` on disk. Extract this into a shared function in `commands.py`:

```python
def _resolve_entry_path(entry: ManifestEntry, drive_path: Path) -> Path:
    """Resolve a manifest entry to its expected file path on disk."""
    if entry.platform:
        return drive_path / "bin" / entry.platform / entry.filename
    type_dir = TYPE_DIRS.get(entry.type, "other")
    return drive_path / type_dir / entry.filename
```

---

## Task 6: Add interactive menu entries

The existing interactive menu in `cli.py:_show_menu` should get entries for the new commands:

```
  [s] Sync (check for updates)
  [i] Inventory (check disk vs manifest)     ← new
  [v] Verify (re-hash checksums)             ← new
  [a] Audit report
  [p] Provision laptop (install apps)
  [w] Wizard (reconfigure)
  [q] Quit
```

Keep `diff` and `budget` as CLI-only — they take arguments and aren't well-suited to the simple menu.

---

## Task 7: Tests

Each new command needs:

1. **Happy path** — initialized drive with fake manifest entries and matching files on disk
2. **Foreign files** — drop extra files in content dirs, confirm inventory finds them
3. **Missing files** — manifest entry with no file on disk
4. **Checksum match/mismatch** — verify with known content and hash
5. **Diff** — two presets with overlapping sources, confirm add/remove/keep sets
6. **Budget** — preset that exceeds target_gb, confirm correct items dropped

All tests use `tmp_path`, `init_drive`, and mock network calls. Follow existing patterns in `test_commands.py`.

---

## Sequence

```
Task 5 (shared helper)  →  Task 1 (inventory)  →  Task 2 (verify)
                                                →  Task 3 (diff)
                                                →  Task 4 (budget)
                                                →  Task 6 (menu)
                                                →  Task 7 (tests throughout)
```

Tasks 1-4 are independent of each other after the shared helper. Tests should be written alongside each command (TDD).

---

## Out of scope

- **`svalbard prune`** — actually deleting foreign or dropped files (future, separate plan)
- **Manifest migration** — adding new fields to ManifestEntry (not needed — all new logic reads existing fields)
- **Manifest signing/tampering detection** — overkill for a personal offline drive
- **Automatic preset switching** — `diff` reports, user decides
