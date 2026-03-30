# Unified Add And Media Ingest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the split local/crawl CLI with `svalbard add`, route local, web, and media inputs through one orchestration path, and add an initial Docker-backed media ingest backend that packages downloaded media as a ZIM.

**Architecture:** Keep host-side orchestration in the Python CLI, reuse the existing local-source registration flow, generalize generated-artifact provenance into one metadata format, and add a dedicated media backend plus Docker image. The implementation stays test-first and keeps backend execution mockable at the runner boundary.

**Tech Stack:** Python 3.12, Click, pytest, Docker, Zimit, yt-dlp, yle-dl, ffmpeg, libzim

---

### Task 1: Add Tests For Unified `svalbard add` Routing

**Files:**
- Modify: `tests/test_commands.py`
- Modify: `tests/test_crawler.py`
- Test: `tests/test_commands.py`
- Test: `tests/test_crawler.py`

- [ ] **Step 1: Write the failing add-command tests**

Add CLI coverage for:

```python
def test_add_command_registers_local_file(tmp_path):
    runner = CliRunner()
    artifact = tmp_path / "manual.zim"
    artifact.write_bytes(b"data")

    result = runner.invoke(main, ["add", str(artifact), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert (tmp_path / "local" / "manual.yaml").exists()


def test_add_command_routes_web_urls_to_zimit(tmp_path):
    runner = CliRunner()
    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.cli.run_add", return_value="local:example") as run_add:
        result = runner.invoke(main, ["add", "https://example.com/docs", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    run_add.assert_called_once()


def test_add_command_routes_media_urls_with_quality_flags(tmp_path):
    runner = CliRunner()

    with patch("svalbard.cli.run_add", return_value="local:playlist") as run_add:
        result = runner.invoke(
            main,
            ["add", "https://www.youtube.com/watch?v=abc", "--workspace", str(tmp_path), "--quality", "480p"],
        )

    assert result.exit_code == 0
    assert run_add.call_args.kwargs["quality"] == "480p"


def test_add_command_audio_only_overrides_quality(tmp_path):
    runner = CliRunner()

    with patch("svalbard.cli.run_add", return_value="local:audio") as run_add:
        result = runner.invoke(
            main,
            ["add", "https://areena.yle.fi/1-12345", "--workspace", str(tmp_path), "--quality", "1080p", "--audio-only"],
        )

    assert result.exit_code == 0
    assert run_add.call_args.kwargs["audio_only"] is True
```

- [ ] **Step 2: Run the targeted tests and verify they fail for the missing command**

Run: `uv run pytest -q tests/test_commands.py tests/test_crawler.py -k "add_command or generated_zim"`

Expected: FAIL because `svalbard add` and generalized generated-source helpers do not exist yet.

- [ ] **Step 3: Commit the failing-test checkpoint**

```bash
git add tests/test_commands.py tests/test_crawler.py
git commit -m "test: cover unified add routing"
```

### Task 2: Implement Host-Side `add` Orchestration And Generalized Provenance

**Files:**
- Create: `src/svalbard/add.py`
- Modify: `src/svalbard/cli.py`
- Modify: `src/svalbard/crawler.py`
- Modify: `README.md`
- Test: `tests/test_commands.py`
- Test: `tests/test_crawler.py`

- [ ] **Step 1: Add the failing generated-source provenance tests**

Add coverage like:

```python
def test_register_generated_zim_writes_recipe_and_source_metadata(tmp_path):
    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")

    source_id = register_generated_zim(
        workspace_root=tmp_path,
        artifact_path=artifact,
        origin_url="https://example.com/docs",
        kind="web",
        runner="docker",
        tool="zimit",
        source_id="local:example",
    )

    assert source_id == "local:example"
    assert (tmp_path / "local" / "example.yaml").exists()
    assert (tmp_path / "generated" / "example.source.yaml").exists()
```

- [ ] **Step 2: Run the provenance test and verify the expected failure**

Run: `uv run pytest -q tests/test_crawler.py -k "generated_zim"`

Expected: FAIL because `register_generated_zim` is not implemented.

- [ ] **Step 3: Implement the minimal orchestration and provenance code**

Implement:

- `src/svalbard/add.py`
  - input classification helpers
  - `run_add(...)`
  - remote runner dispatch
- `src/svalbard/crawler.py`
  - `register_generated_zim(...)`
  - keep a compatibility wrapper if needed for internal reuse
- `src/svalbard/cli.py`
  - new `@main.command("add")`
  - options for `--kind`, `--runner`, `--quality`, `--audio-only`, `--output`, `--workspace`
  - remove or stop exposing the old `crawl` / `local add` entrypoints in the primary CLI surface
- `README.md`
  - replace command table entries for `local add` and `crawl` with `svalbard add`

- [ ] **Step 4: Run the focused command tests and make them pass**

Run: `uv run pytest -q tests/test_commands.py tests/test_crawler.py -k "add_command or generated_zim"`

Expected: PASS

- [ ] **Step 5: Commit the orchestration slice**

```bash
git add src/svalbard/add.py src/svalbard/crawler.py src/svalbard/cli.py README.md tests/test_commands.py tests/test_crawler.py
git commit -m "feat: add unified add command orchestration"
```

### Task 3: Add The Initial Docker-Backed Media Backend

**Files:**
- Create: `src/svalbard/media.py`
- Create: `docker/media/Dockerfile`
- Create: `docker/media/build-media-zim.py`
- Modify: `src/svalbard/add.py`
- Test: `tests/test_commands.py`
- Test: `tests/test_crawler.py`

- [ ] **Step 1: Add failing media-runner tests**

Add coverage like:

```python
def test_run_add_uses_media_backend_for_youtube_urls(tmp_path):
    artifact = tmp_path / "generated" / "playlist.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.add.run_media_ingest", return_value=artifact) as media_mock:
        source_id = run_add("https://www.youtube.com/watch?v=abc", workspace_root=tmp_path)

    assert source_id == "local:playlist"
    media_mock.assert_called_once()


def test_run_add_writes_media_provenance(tmp_path):
    artifact = tmp_path / "generated" / "lecture.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.add.run_media_ingest", return_value=artifact):
        run_add("https://areena.yle.fi/1-12345", workspace_root=tmp_path, audio_only=True)

    text = (tmp_path / "generated" / "lecture.source.yaml").read_text()
    assert "kind: media" in text
    assert "audio_only: true" in text
```

- [ ] **Step 2: Run the media test subset and verify it fails**

Run: `uv run pytest -q tests/test_commands.py tests/test_crawler.py -k "media_backend or media_provenance"`

Expected: FAIL because the media runner and metadata format are not wired in yet.

- [ ] **Step 3: Implement the minimal media backend**

Implement:

- `src/svalbard/media.py`
  - Docker image constant and ensure/build helper
  - URL probing helpers for YouTube/Yle-first detection
  - `run_media_ingest(...)` that shells into the media container
- `docker/media/Dockerfile`
  - lightweight Alpine-based image when practical
  - installs `ffmpeg`, Python, and Python packages `yt-dlp`, `yle-dl`, and `libzim`
- `docker/media/build-media-zim.py`
  - `probe` mode for URL detection
  - `build` mode that downloads media, writes a simple static site, and creates a ZIM
- `src/svalbard/add.py`
  - media detection and dispatch
  - quality/audio-only validation

- [ ] **Step 4: Run the media subset again and make it pass**

Run: `uv run pytest -q tests/test_commands.py tests/test_crawler.py -k "media_backend or media_provenance"`

Expected: PASS

- [ ] **Step 5: Commit the media backend slice**

```bash
git add src/svalbard/add.py src/svalbard/media.py docker/media/Dockerfile docker/media/build-media-zim.py tests/test_commands.py tests/test_crawler.py
git commit -m "feat: add docker-backed media ingest runner"
```

### Task 4: Run Full Verification For The Touched Surface

**Files:**
- Modify: `docs/superpowers/plans/2026-03-30-unified-add-and-media-ingest.md`
- Test: `tests/test_commands.py`
- Test: `tests/test_crawler.py`
- Test: `tests/test_local_sources.py`

- [ ] **Step 1: Run the full focused verification suite**

Run: `uv run pytest -q tests/test_commands.py tests/test_crawler.py tests/test_local_sources.py`

Expected: PASS

- [ ] **Step 2: Review the implementation against the approved spec**

Check that the code covers:

- one `svalbard add` entrypoint
- local-path registration
- web URL routing to Zimit
- media URL routing to the media backend
- generalized `generated/<slug>.source.yaml` provenance
- `--quality` and `--audio-only`
- separate `docker/geodata` and `docker/media` images

- [ ] **Step 3: Commit the final implementation state**

```bash
git add src/svalbard README.md docker tests docs/superpowers/plans/2026-03-30-unified-add-and-media-ingest.md
git commit -m "feat: implement unified add and media ingest flow"
```
