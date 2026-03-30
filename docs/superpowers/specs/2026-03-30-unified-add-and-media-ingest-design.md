# Unified Add And Media Ingest Design

## Summary

Svalbard should replace the current split between local registration and website crawling with one ingestion command: `svalbard add <input>`.

That command should accept an existing local path or a remote URL and produce a reusable workspace-local source. Local paths should be registered directly. Remote URLs should be acquired through backend-specific runners, written into `generated/` as durable artifacts, and then registered the same way as any other local source.

The first remote backends should be:

- `web`: website-to-ZIM capture using Zimit
- `media`: media acquisition and packaging for any `yt-dlp`-supported site, plus Yle Areena via `yle-dl`

Remote work should default to Docker-backed runners so Svalbard does not need to bundle or install the heavy downloader, transcoder, and ZIM packaging toolchain as part of the base app.

## Goals

- Provide one clear user-facing ingestion command for local files, website capture, and media imports.
- Support remote media inputs without requiring Svalbard itself to vendor `yt-dlp`, `yle-dl`, `ffmpeg`, or ZIM tooling.
- Produce reusable workspace-local artifacts that can be selected for one or more drives later.
- Keep media imports compact enough for offline drives while preserving watchable quality for instructional content.
- Package imported media into a simple browsable offline site inside a ZIM, not a raw file dump.

## Non-Goals

- Backward compatibility with the unreleased `crawl` and `local add` command layout.
- A rich SPA-like offline media application in the first version.
- Site-specific UI tuning for every `yt-dlp` extractor.
- Authenticated media acquisition workflows in v1.
- Bundling downloader or transcoder binaries into the base Svalbard install.

## Current Context

Today, Svalbard has two related but separate flows:

- `svalbard local add <path>` registers an existing local file or directory
- `svalbard crawl ...` runs a website capture flow with Dockerized Zimit and then registers the generated ZIM

That split leaks implementation details into the CLI. Users think in terms of "add this content to the workspace", not in terms of whether the content already exists locally or needs a crawler or downloader first.

The current website flow already demonstrates the right operational model: Svalbard orchestrates an external backend, stores the resulting artifact in `generated/`, and registers it as a reusable local source. The new design generalizes that pattern across more acquisition types.

## User Experience

### Primary Command

The primary ingestion entrypoint should be:

```bash
svalbard add <input>
```

Where `<input>` may be:

- an existing local file path
- an existing local directory path
- an `http` or `https` URL for a website
- an `http` or `https` URL for downloadable media

### Optional Flags

The initial command surface should be:

```bash
svalbard add <input> \
  [--kind auto|local|web|media] \
  [--runner auto|docker|host] \
  [--quality 1080p|720p|480p|360p|source] \
  [--audio-only] \
  [--output <name>] \
  [--workspace <path>]
```

Behavior rules:

- `--kind` defaults to `auto`
- `--runner` defaults to `auto`
- remote `auto` runner resolves to `docker`
- `--quality` applies only to media imports
- default media quality is `720p`
- `--audio-only` overrides `--quality`
- `--output` overrides the generated artifact and source slug where valid

### Examples

```bash
svalbard add ./manuals/field-guide.zim
svalbard add https://example.com/docs
svalbard add https://www.youtube.com/playlist?list=...
svalbard add https://areena.yle.fi/... --quality 480p
svalbard add https://example.com/lecture --audio-only
```

## Detection And Routing

### Input Resolution

`svalbard add` should resolve the input in this order:

1. If the input resolves to an existing filesystem path, treat it as `local`
2. Otherwise, if it parses as an `http` or `https` URL, continue with remote detection
3. Otherwise, fail with a clear invalid-input error

### Backend Selection

When `--kind auto` is used:

- Existing local path -> `local`
- URL recognized as first-class media input -> `media`
- URL positively resolved by media probe -> `media`
- All other URLs -> `web`

The detection heuristic should bias toward correctness over cleverness. The first-class media URL patterns should include at least:

- YouTube URLs
- Yle Areena URLs

For broader media support, Svalbard should allow a media probe step. One acceptable approach is to run a lightweight `yt-dlp --simulate` or equivalent metadata probe inside the selected media runner. If that probe succeeds, the URL should be treated as `media`; if it fails, Svalbard should fall back to `web`.

If the user supplies `--kind`, that explicit choice overrides auto-detection.

## Runner Model

### Host Orchestrator

The host Svalbard process should remain the orchestrator. It should:

- resolve the workspace root
- determine input kind and runner
- prepare staging and output paths
- invoke the selected backend runner
- register successful artifacts as local sources
- write provenance metadata

This host-side orchestration should not be moved wholesale into a container, because local recipe writing, path resolution, and source registration belong to the workspace model.

### Runners

The runner model should be:

- `local` runner:
  - host-only
  - registers an existing file or directory
- `web` runner:
  - default `docker`
  - uses Zimit to capture a website into a ZIM
- `media` runner:
  - default `docker`
  - uses downloader plus transcoder plus ZIM packaging tooling

The remote Docker preference is deliberate:

- avoids bundling heavyweight dependencies into the app
- keeps remote acquisition environments reproducible
- matches the existing Zimit operational model

Host runners may still be supported for advanced users who already have the relevant tools installed, but host mode is not the primary path for remote ingestion.

### Container Layout

Svalbard should keep separate Docker images per backend family rather than collapsing everything into one general toolbox image.

The initial layout should be:

- `docker/geodata/Dockerfile`
- `docker/media/Dockerfile`
- existing website-crawl container use remains separate through Zimit upstream

Container policy:

- prefer the same lightweight base distribution across Svalbard-managed images where practical
- Alpine is the preferred first choice because the current geodata image already uses it successfully
- do not force a shared base image yet
- if the media toolchain proves materially easier or more reliable on another minimal base such as Debian slim, that image may diverge

This keeps images small and purpose-specific while still encouraging a consistent operational footprint.

## Media Pipeline

### Supported Scope

The first media backend should support:

- any `yt-dlp`-supported site
- Yle Areena via `yle-dl`

This should be presented as one `media` backend, even though the underlying downloader may differ by site.

### Acquisition And Normalization

The media pipeline should work in these stages:

1. Probe the URL and collect metadata
2. Download media, thumbnails, subtitles, and metadata into a staging directory
3. Normalize filenames and metadata into a predictable internal structure
4. Transcode or remux media according to the selected Svalbard quality mode
5. Generate a simple static offline site
6. Package that site into a `.zim`
7. Register the resulting artifact as a local source

This design should treat downloader-specific quality flags only as acquisition aids. The canonical output size and compatibility policy should be enforced by one Svalbard-controlled normalization stage using `ffmpeg`. That keeps behavior predictable across `yt-dlp` sources and `yle-dl`.

### Quality Modes

User-facing media quality modes should be explicit resolution targets:

- `1080p`
- `720p` default
- `480p`
- `360p`
- `source`
- `--audio-only`

Recommended normalization policy:

- `1080p`: H.264 in MP4, AAC audio, target cap `1080p`
- `720p`: H.264 in MP4, AAC audio, target cap `720p`
- `480p`: H.264 in MP4, AAC audio, target cap `480p`
- `360p`: H.264 in MP4, AAC audio, target cap `360p`
- `source`: preserve the source as much as practical, only remux or minimally normalize when needed for compatibility
- `--audio-only`: extract or convert to a compact audio format and package as an audio collection instead of video playback pages

For instructional content, the recommended default should prioritize readability over aggressive minimization. `720p` is the best default for diagram-heavy or subtitle-heavy educational material, while `480p` and `360p` remain explicit storage-saving modes.

The quality ladder should be designed around a consistent transcode policy rather than assuming the ZIM container itself will save substantial space. Modern `mp4` and `webm` files are already compressed, so meaningful size reduction must happen before packaging.

### Audio-Only Mode

`--audio-only` should be a Svalbard-level mode rather than a downloader-specific passthrough flag. The underlying downloader may use native audio extraction when convenient, but the final behavior should be consistent:

- no video assets in the final package
- one audio file per item
- metadata, artwork, and descriptions preserved where available
- simple audio playback pages in the generated site

## Media Output Shape

The first version of the media ZIM should be a simple offline mini-site, not a rich client application.

Expected user-facing features:

- landing page with collection title and source URL
- playlist or collection grouping when metadata provides it
- thumbnail grid of items
- per-item detail pages
- embedded local playback
- title, duration, uploader or source name, and original URL
- description text when available
- subtitles linked or embedded when available

This is intentionally static, lightweight, and Kiwix-friendly.

### Staging Layout

One acceptable internal staging layout is:

```text
index.html
videos/<slug>.mp4
audio/<slug>.m4a
thumbs/<slug>.jpg
subs/<slug>.<lang>.vtt
metadata/*.json
assets/*
```

The exact filenames may vary, but the structure should separate normalized media, metadata, and generated site assets cleanly enough that packaging and debugging remain straightforward.

## Artifact Contract

Every successful remote add operation should end in one durable workspace artifact under `generated/`.

Final outputs:

- `generated/<slug>.zim`
- `generated/<slug>.source.yaml`

The provenance file should record at least:

- original input URL
- detected kind: `web` or `media`
- runner: `docker` or `host`
- backend tool chain used
- selected quality or `audio_only: true`
- creation timestamp
- final artifact path
- final artifact size

This should replace the narrower crawl-only provenance model with one general source-ingest metadata record.

Temporary staging should live separately, for example:

```text
generated/.staging/<slug>/
```

That staging directory should be cleaned on success unless the user asks to keep it.

## Registration Model

Successful `add` operations should register the final artifact through the same local-source path used for existing local content.

For local paths:

- validate the path
- infer or validate source type
- compute size metadata
- write `local/<id>.yaml`

For remote inputs:

- generate the final `.zim`
- write the general provenance metadata
- register the `.zim` as a local source

This keeps one consistent source model: remote acquisition produces a local artifact, and local artifacts are what the rest of Svalbard consumes.

## Failure Behavior

Failure handling should preserve debuggability and avoid half-registered state.

Rules:

- if acquisition fails, do not register anything
- if acquisition succeeds but packaging fails, keep staging and report its path clearly
- if packaging succeeds but registration fails, keep the `.zim` and provenance file and report the orphaned artifact clearly
- if Docker or the selected backend toolchain is unavailable, fail with an actionable prerequisite error
- if `--kind` or `--runner` is incompatible with the input, fail validation before starting work

Remote work should prefer transactional writes:

- write into staging or temporary output first
- atomically move successful outputs into their final `generated/` paths

## Testing Scope

The first implementation should cover automated tests for:

- path versus URL detection
- media versus web backend selection
- explicit `--kind` override behavior
- explicit `--runner` validation
- media quality flag validation
- `--audio-only` overriding quality mode
- remote runner command construction for `web` and `media`
- artifact and provenance path generation
- successful registration of produced artifacts
- failure cases where staging or orphaned outputs must be preserved

The tests do not need to download real media in normal unit coverage. Backend invocations should be mocked at the runner boundary.

## Open Questions Resolved For V1

- Primary command: `svalbard add`
- Backward compatibility: not required
- Remote execution default: Docker
- Media site scope: any `yt-dlp`-supported site, plus Yle Areena through `yle-dl`
- Default media quality: `720p`
- Lower-quality modes: `480p` and `360p`
- Higher-quality mode: `1080p`
- Audio mode: explicit `--audio-only`
- Output UX: static mini-site inside the ZIM, not a rich SPA

## Recommended Implementation Direction

Implement the feature around a small orchestrator plus pluggable runners:

- `add` command resolves input and high-level options
- `local`, `web`, and `media` runners own backend-specific execution
- runners return a standardized result describing the generated artifact
- registration and provenance writing stay in one host-side path

That structure should keep the CLI simple, the implementation modular, and future ingestion backends easy to add without reworking the user model again.
