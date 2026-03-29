# Primer v1 — Remaining Work

Covers everything needed to bring Primer to a solid v1. Tasks 1–11 from the original plan are implemented. This plan covers the gaps.

---

### Task A: Data file bundling

**Problem:** `presets.py` uses `Path(__file__).parent.parent.parent / "presets"` which only works in dev. In an installed wheel, `presets/` and `data/` are outside the package.

**Fix:**
1. Move `presets/` → `src/primer/presets/`
2. Move `data/` → `src/primer/data/`
3. In `presets.py`, use `importlib.resources.files("primer") / "presets"` to locate preset dir
4. In `taxonomy.py`, use `importlib.resources.files("primer") / "data"` to locate taxonomy
5. Update `pyproject.toml` — no extra config needed since hatchling already packages `src/primer/`
6. Update `.gitignore` if needed

**Tests:** Existing preset/taxonomy tests should keep passing after the move.

---

### Task B: Auto-wizard on first run

**Problem:** Design says wizard launches automatically when no manifest is found, but code just prints a message.

**Fix in `cli.py`:**
```python
# In main(), when no manifest found:
if ctx.invoked_subcommand is None:
    if Manifest.exists(cwd):
        show_status(str(cwd))
        _show_menu(str(cwd))
    else:
        console.print("[bold]Primer[/bold] — Offline knowledge kit provisioner\n")
        console.print("No drive found in current directory.")
        if Confirm.ask("  Run setup wizard?", default=True):
            run_wizard()
        else:
            console.print("Run [bold]primer --help[/bold] for all commands.")
```

Prompt instead of auto-launch — less surprising, still zero-friction.

---

### Task C: Robust incremental sync

**Problem:** Manifest only saved at end of sync. Ctrl+C loses progress. No skip for already-downloaded sources.

**Changes to `commands.py` `sync_drive()`:**

1. **Skip already-downloaded sources** — before queuing a download, check if `manifest.entry_by_id(source.id)` exists AND the file is still on disk. Skip if both true (unless `--force`).

2. **Save manifest after each download** — move manifest.save() inside the results loop, after each successful ManifestEntry is added.

3. **Signal handling** — wrap the download loop in a try/except KeyboardInterrupt that saves the manifest before exiting.

4. **Add `--force` flag** to `sync` command — re-downloads everything even if already present.

**Changes to `downloader.py`:**
- No changes needed — file-exists skip is a good safety net but sync_drive should be the primary gate.

---

### Task D: Freshness checking & updates

**Problem:** No way to detect or fetch newer versions of already-downloaded sources.

**Changes:**

1. **In `sync_drive()`** — after resolving URL, compare against `manifest.entries[id].url`. If different:
   - Download new version
   - Delete old file (`drive_path / TYPE_DIRS[type] / old_entry.filename`)
   - Update manifest entry
   - Print `[yellow]Updated: source-id (old → new)[/yellow]`

2. **`--update` flag on `sync`** — default sync skips sources already in manifest. `--update` re-resolves URLs and checks for newer versions. Without `--update`, only missing sources are downloaded.

3. **Freshness info in `status`** — add `--check` flag to `primer status` that resolves URLs and compares:
   ```
   ID                     Type  Size     Downloaded   Status
   wikipedia-en-nopic     zim   25.0 GB  2026-03-29   ✓ current
   wikimed                zim   2.0 GB   2026-01-15   ↑ update available
   khan-academy-lite      zim   --       --           ✗ not downloaded
   ```

4. **Cleanup of old files** — when a source is updated, remove the old file. Track this in the download flow so we don't leave orphans.

---

### Task E: Tests

**Files to create:**

**`tests/conftest.py`** — shared fixtures:
- `tmp_drive` — tmp_path with manifest.yaml written
- `sample_preset` — a minimal 2-source preset for testing
- `sample_taxonomy` — minimal taxonomy

**`tests/test_models.py`**
- `total_size_gb` sums correctly
- `sources_for_options` filters by optional_group
- `sources_for_options` includes non-optional sources always

**`tests/test_presets.py`**
- `parse_preset` loads nordic-128 with correct source count
- `list_presets` finds at least nordic-128
- Missing preset raises FileNotFoundError

**`tests/test_taxonomy.py`**
- `load_taxonomy` returns all 8 groups
- `all_domains` returns ~35 domains
- `compute_coverage` scores a comprehensive source at 30
- Coverage caps at 100

**`tests/test_manifest.py`**
- Save/load roundtrip preserves all fields
- `exists()` returns True after save, False on empty dir
- `entry_by_id` returns correct entry or None

**`tests/test_resolver.py`**
- Static URL (no `{date}`) returns source.url unchanged
- URL pattern resolution with mocked httpx (monkeypatch `httpx.get` to return fake directory listing)

**`tests/test_downloader.py`**
- `find_downloader` returns a string (or None if nothing installed)
- `download_file` skips existing file and returns path

**`tests/test_audit.py`**
- `generate_audit` output contains expected sections (header, inventory, coverage matrix)

**`tests/test_commands.py`**
- `init_drive` creates manifest.yaml, serve.sh, README.md in tmp dir
- `sync_drive` with mocked downloader updates manifest

**`tests/test_wizard.py`**
- `find_best_preset` picks correct tier for given budget
- `detect_volumes` returns a list (may be empty in CI)

**Setup:** Add `[dependency-groups]` to pyproject.toml:
```toml
[dependency-groups]
dev = ["pytest>=8.0", "pytest-cov"]
```

---

### Task F: Additional presets

Create YAML presets following the tier design. Each higher tier uses `replaces` to upgrade sources from lower tiers.

**`nordic-32.yaml`** (~30 GB) — survival essentials:
- Wikipedia EN nopic (25 GB)
- WikiMed (2 GB)
- WikiHow (truncated or skipped — too large, use Practical Action + iFixit instead)
- iFixit (5 GB) — **note: won't fit at 32 GB with full Wikipedia, need to curate**
- Actually: Wikipedia EN nopic 25 + WikiMed 2 + Practical Action 1 + CyberChef 0.015 + binaries ≈ 28 GB
- This is the "absolute essentials" tier

**`nordic-64.yaml`** (~60 GB) — adds breadth:
- Everything from 32 + iFixit (5) + Wiktionary EN (5) + Wikipedia FI nopic (5) + Wiktionary FI (2) + Stack Exchange survival/diy (3.5) + Wikibooks (3) ≈ 52 GB
- Room for maps as optional

**`nordic-128.yaml`** — ✅ exists (20 sources, ~97 GB)

**`nordic-256.yaml`** (~240 GB) — adds richness:
- Replaces `wikipedia-en-nopic` with `wikipedia-en-maxi` (100 GB, with pictures)
- Adds Wikipedia SV nopic (~4 GB), Wikipedia NO nopic (~3 GB)
- Larger Gutenberg collection
- More Stack Exchanges (electronics, physics, math)
- Full Nordic maps (larger PMTiles extract)
- Khan Academy with some video content

**`nordic-512.yaml`** (~480 GB) — adds media + models:
- Everything from 256
- Khan Academy full video
- Small GGUF model (e.g. Llama 3.2 3B, ~2 GB)
- Linux installer ISOs (optional_group: installers)
- More language Wikipedias

**`nordic-1tb.yaml`** (~900 GB) — full coverage:
- Replaces `wikipedia-en-maxi` with full pics version
- Large GGUF model (e.g. Llama 3.1 70B Q4, ~40 GB)
- Full OpenStreetMap tiles
- Video courses
- All Stack Exchange sites
- More books

**`nordic-2tb.yaml`** (~1.8 TB) — everything:
- Multiple large models
- All language Wikipedias for Nordic region
- Full video libraries
- Complete maps

**Note:** Each preset is independently curated, not auto-generated from a smaller one. The `replaces` field lets the resolver know which source from a lower tier this one supersedes (for documentation/audit purposes).

**Also update** `wizard.py` `SIZE_PRESETS` dict to include all tiers.

---

### Task G: Checksum verification

**Changes to `models.py`:**
- Add `sha256: str = ""` field to `Source` dataclass

**Changes to `downloader.py`:**
- Add `verify_checksum(filepath: Path, expected_sha256: str) -> bool` — streams file through hashlib.sha256
- After download completes, if source has sha256 set, verify. If mismatch, delete file and raise error.

**Changes to preset YAMLs:**
- Add sha256 where known. For Kiwix ZIM files, these change with each release so we can't hardcode them.

**Kiwix .sha256 sidecar files:**
- Kiwix publishes `filename.zim.sha256` alongside each ZIM
- Add logic: if source is type `zim` and no sha256 in preset, try fetching `{url}.sha256`, parse it, verify
- This gives us automatic checksum verification for all ZIM downloads without maintaining hashes in presets

**Changes to `manifest.py`:**
- Add `sha256: str = ""` to ManifestEntry — record the verified hash for later integrity checks
- `primer status` could optionally re-verify on-disk files against stored hashes

---

### Task H: Rich download progress

**Current state:** subprocess.run calls aria2c/wget/curl which print their own output. No Rich integration.

**Approach:** Use httpx streaming as the primary download method with Rich Progress. Keep CLI tools as fallback.

**Changes to `downloader.py`:**

```python
def download_file_httpx(url: str, dest_path: Path, progress: Progress, task_id) -> Path:
    """Download via httpx with Rich progress bar."""
    with httpx.stream("GET", url, follow_redirects=True) as r:
        total = int(r.headers.get("content-length", 0))
        progress.update(task_id, total=total)
        with open(dest_path, "wb") as f:
            for chunk in r.iter_bytes(chunk_size=65536):
                f.write(chunk)
                progress.advance(task_id, len(chunk))
    return dest_path
```

**Resume support:** Check if partial file exists, send `Range` header, append to file.

**Fallback:** If httpx download fails (e.g. server doesn't support Range, or connection is flaky), fall back to aria2c/wget/curl subprocess. aria2c is still better for very large files (multi-connection).

**Integration in `download_sources`:**
```python
with Progress(
    SpinnerColumn(),
    TextColumn("[bold]{task.fields[filename]}"),
    BarColumn(),
    DownloadColumn(),
    TransferSpeedColumn(),
    TimeRemainingColumn(),
) as progress:
    for source_id, url, dest_dir in sources:
        task_id = progress.add_task("dl", filename=filename, total=None)
        # ... download with progress updates
```

**User choice:** Add `--aria2c` flag to `primer sync` to force aria2c for users who prefer multi-connection downloads.

---

### Task I: `primer crawl`

**New file: `src/primer/crawler.py`**

**Config format** (from existing examples):
```yaml
name: nordic-emergency
output: zim/custom/nordic-emergency.zim
description: Finnish emergency services
tags: [first-aid, fire-shelter]
sites:
  - url: https://example.fi
    scope: domain
    max_pages: 500
limits:
  max_size_mb: 512
  time_limit_minutes: 60
```

**Logic:**
1. `load_crawl_config(path)` — parse YAML
2. `check_docker()` — verify Docker is running (`docker info`)
3. `check_zimit()` — verify image exists or pull `openzim/zimit`
4. `run_crawl(config, drive_path)` — build Docker command:
   ```
   docker run -v {output_dir}:/output openzim/zimit
     --url {url} --scope {scope}
     --output /output/{output_name}
     --limit {max_pages} --sizeLimit {max_size_mb}
     --timeLimit {time_limit_minutes}
   ```
5. For multi-site configs: crawl each site, then merge ZIMs (or crawl sequentially into one)

**CLI in `cli.py`:**
```python
@main.command()
@click.argument("config", required=False)
@click.option("--all", "crawl_all", is_flag=True, help="Run all configs in crawl/")
def crawl(config, crawl_all):
    """Crawl websites into ZIM files."""
```

**Behavior:**
- `primer crawl nordic-emergency.yaml` — run single config
- `primer crawl --all` — run all `.yaml` (not `.yaml.example`) in `crawl/`
- Graceful error if Docker not available
- Output goes to drive's `zim/custom/` directory
- After crawl, update manifest with new custom ZIM entries

---

## Implementation order

1. **A** — Data file bundling (structural fix, do first)
2. **B** — Auto-wizard (tiny change)
3. **C** — Robust incremental sync (foundational for D)
4. **D** — Freshness checking & updates
5. **E** — Tests (validate A–D plus existing code)
6. **G** — Checksum verification
7. **H** — Rich download progress
8. **F** — Additional presets (can research sizes while coding G/H)
9. **I** — `primer crawl`
