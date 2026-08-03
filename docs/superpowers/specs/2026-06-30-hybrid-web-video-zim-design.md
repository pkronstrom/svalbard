# Hybrid Web Video ZIM Design

Date: 2026-06-30
Status: Draft

## Summary

Svalbard should support a hybrid build family for websites whose pages are useful on their own, but whose instructional value depends on embedded external videos. The builder should first capture the website into an editable static mirror, then discover video embeds and links, download the referenced media with the existing media tooling, rewrite the mirror to use local video assets, and finally pack the completed mirror into one ZIM.

This is different from both existing generic website recipes and standalone media imports:

- `wget -> warc2zim` and `kage pack` capture the website, but leave YouTube embeds remote.
- `media-zim.py` downloads YouTube/Yle media into a ZIM, but produces a standalone media collection rather than preserving the original website context.

The new workflow combines those strengths without requiring mutation of an already-packed ZIM.

## Goals

- Preserve website structure, navigation, text, images, and downloadable project files.
- Replace supported external video embeds with local playable media where licensing and size policy allow it.
- Reuse existing Svalbard tooling for `yt-dlp`, `ffmpeg`, thumbnails, subtitles, and ZIM packaging patterns.
- Keep the intermediate mirror inspectable and patchable before final ZIM packing.
- Make media size policy explicit at the recipe level so practical websites do not accidentally produce very large artifacts.

## Non-Goals

- Build a general-purpose web archiving crawler that captures every third-party service.
- Mutate ZIM files in place after they are created.
- Archive arbitrary video platforms in v1 beyond providers already supported by `yt-dlp` and permitted by policy.
- Guarantee full-text search inside Kiwix for video transcript content in v1.
- Download videos by default for every website recipe.

## Current Context

Svalbard currently has three relevant paths:

- Generic build recipes can run pipeline steps such as `wget` and `warc2zim`.
- `media-zim.py` can download media with `yt-dlp` or `yle-dl`, normalize it with `ffmpeg`, generate local HTML pages, and package those files into a ZIM.
- Browser-rendered capture with `kage` has been evaluated as a possible replacement or complement for some website recipes. It captures pages and assets well for simple sites, but YouTube embeds remain remote iframes.

The Open Source Low Tech test showed the target shape clearly: pages and images are small and useful, while embedded YouTube videos carry much of the instructional value. A plain website ZIM is therefore incomplete, and a standalone video ZIM loses the original tutorial context.

## Recommended Architecture

The builder should operate on a static mirror directory as the main editable artifact:

1. Capture the website into `/work/mirror`.
2. Scan `/work/mirror/**/*.html` for supported video embeds and links.
3. Download unique media references into `/work/media`.
4. Normalize media into a local mirror subdirectory such as `/work/mirror/_svalbard/media`.
5. Rewrite HTML embeds and selected links to point at local media.
6. Pack `/work/mirror` into the final ZIM.

The key decision is to rewrite before final packing. ZIM should be treated as the final artifact, not as an editable intermediate format.

## Build Family

Add a recipe build family named `web-video-zim`.

Example recipe shape:

```yaml
id: opensourcelowtech
type: zim
strategy: build
description: Open Source Low Tech tutorials with local video embeds
build:
  family: web-video-zim
  source_url: https://opensourcelowtech.org/
  output: opensourcelowtech.zim
  title: Open Source Low Tech
  crawl_engine: kage
  max_pages: "100"
  scroll: "true"
  video_providers: youtube
  video_quality: 480p
  video_mode: media-page
  max_video_gb: "8"
  max_asset_mb: "100"
```

`BuildSpec.Config` currently captures unknown scalar keys as string template variables. That is sufficient for a first version, though a later typed config would make validation clearer.

## Crawl Engines

The build family should allow the crawl engine to be selected explicitly:

- `kage`: preferred for rendered static mirrors and post-processing.
- `wget`: possible fallback for simple static sites.

For v1, `kage` should be the primary implementation because it naturally creates the editable mirror that this workflow needs. `warc2zim` is less convenient here because post-processing a WARC or already-packed ZIM is awkward.

The builder should still keep the door open for a future `wget` mirror mode:

```text
wget --mirror --page-requisites --convert-links -> /work/mirror
post-process mirror
pack mirror
```

That would avoid WARC as the final path, while keeping wget useful for static sites.

## Video Discovery

The scanner should parse HTML with a real HTML parser rather than regex-only rewriting.

Supported v1 discoveries:

- `<iframe src="https://www.youtube.com/embed/<id>">`
- `<iframe src="https://www.youtube-nocookie.com/embed/<id>">`
- `<a href="https://www.youtube.com/watch?v=<id>">`
- `<a href="https://youtu.be/<id>">`

Discovery should normalize these to canonical source URLs before download. Duplicate embeds across pages should download once and reuse the same local asset.

The scanner should produce a manifest such as:

```json
{
  "items": [
    {
      "provider": "youtube",
      "id": "ywljr9RKExQ",
      "source_url": "https://www.youtube.com/watch?v=ywljr9RKExQ",
      "pages": ["rocketstove.html/index.html"]
    }
  ]
}
```

This manifest gives the build a debuggable checkpoint and makes retries easier.

## Download And Normalization

The implementation should reuse the behavior from `media-zim.py` where practical:

- `yt-dlp` for download and metadata.
- thumbnails via `--write-thumbnail`.
- subtitles via `--write-subs` and `--write-auto-subs`.
- `ffmpeg` normalization to browser-playable MP4.
- configurable quality: `1080p`, `720p`, `480p`, `360p`, or `source`.

The default for hybrid website recipes should be conservative, probably `480p`, because the website itself often includes multiple videos and project files.

Downloaded files should live inside the mirror under a reserved path:

```text
_svalbard/
  media/
    videos/<video-id-or-slug>.mp4
    thumbs/<video-id-or-slug>.jpg
    subs/<video-id-or-slug>-1.vtt
    manifest.json
```

The reserved path should not conflict with `kage`'s `_kage/` directory.

## Rewrite Modes

The recipe should choose one rewrite mode.

### `inline-video`

Replace an iframe with a local HTML5 player:

```html
<video controls preload="metadata" poster="_svalbard/media/thumbs/example.jpg">
  <source src="_svalbard/media/videos/example.mp4" type="video/mp4">
  <track kind="subtitles" src="_svalbard/media/subs/example-1.vtt">
</video>
```

This preserves the original page flow, but can make pages heavy and visually inconsistent.

### `media-page`

Replace an iframe with a compact local card linking to a generated media page:

```html
<a class="svalbard-video-card" href="_svalbard/media/pages/example.html">
  Watch offline video: Rocket Stove
</a>
```

This is the recommended default. It keeps tutorial pages readable, handles metadata consistently, and avoids embedding a large player many times.

### `thumbnail-link`

Replace an iframe with a thumbnail and a link to the local file or generated page. This is useful for low-storage presets or pages with many embeds.

## External Links And Provenance

Rewritten video blocks should preserve the original source URL near the local player or media page. The user should be able to see where the video came from, even offline.

Generated media pages should include:

- title
- source URL
- uploader/channel if available
- duration
- local player
- description when available
- subtitles if available

## Size Policy

Hybrid recipes need explicit media budgets.

Suggested controls:

- `video_quality`: default `480p`
- `max_video_gb`: total media budget for the recipe
- `max_video_count`: optional cap for broad sites
- `on_video_budget_exceeded`: `skip`, `keep-remote`, or `fail`
- `max_asset_mb`: crawler asset cap for non-video files

Recommended default behavior:

- If a video cannot be downloaded, leave the original external link and add a visible note in generated metadata.
- If the media budget is exceeded, skip remaining videos and keep their original embeds/links remote unless the recipe says `fail`.
- The final build should log and persist a summary of downloaded, skipped, failed, and budget-excluded videos.

## ZIM Packing

For the final pack step, there are two viable options:

1. Use `kage pack` on the rewritten mirror.
2. Use a Svalbard/libzim packer similar to `media-zim.py`.

The first version should prefer `kage pack` if `kage` is the crawl engine. This minimizes new packaging code and keeps the mirror-to-ZIM path deterministic. A later native Svalbard packer may be useful if we need full control over metadata, icons, redirects, or search indexing.

## Container Dependencies

The `svalbard-tools` image would need:

- `kage`
- Chromium or Chrome for `kage clone`
- `yt-dlp`
- `ffmpeg`
- existing ZIM tooling

`yt-dlp` and `ffmpeg` are already part of the tools image. Chromium is the largest new dependency. It should be added only if `kage-zim` or `web-video-zim` becomes a first-class build family.

## Error Handling

The build should fail for structural errors:

- no mirror produced
- final ZIM not produced
- final ZIM below minimum size
- invalid recipe config

The build should not fail by default for individual video failures. Those should be recorded in `_svalbard/media/manifest.json` and in build logs, unless the recipe asks for strict behavior.

Strict mode should be available:

```yaml
video_required: "true"
```

When strict mode is enabled, any discovered video that cannot be downloaded or rewritten should fail the build.

## Testing Strategy

Unit tests:

- extract YouTube IDs from iframe and link forms
- deduplicate equivalent YouTube URLs
- rewrite iframe to `inline-video`
- rewrite iframe to `media-page`
- preserve original source URL
- handle missing downloads according to policy
- enforce size budget decisions

Integration tests:

- run the builder against a tiny local HTML fixture with fake YouTube URLs and a mocked `yt-dlp`
- verify final mirror contains generated media pages and rewritten HTML
- verify final ZIM is produced

Manual validation:

- Open Source Low Tech as the first real-world candidate.
- Compare `kage-zim`, `wget/warc2zim`, and `web-video-zim` outputs for size, offline usefulness, and broken links.

## Open Questions

- Should the default rewrite mode be `media-page` for all recipes, or should small single-video pages use `inline-video`?
- Should video downloads be enabled only for explicit allowlisted recipes because of storage and licensing risk?
- Should final ZIM packing use `kage pack` initially, or should Svalbard own the packer from the start?
- Should transcripts/subtitles be indexed or exposed separately in v1?
- How should recipes represent license/attribution for embedded third-party videos?

## First Candidate

Open Source Low Tech is a strong first candidate because:

- the website itself is small
- the tutorials are practical and fit Svalbard's purpose
- the current plain mirror leaves YouTube embeds remote
- the number of pages and videos is manageable for manual review

The expected first implementation should target this one recipe before generalizing the builder for broader sites.
