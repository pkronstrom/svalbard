# ZIM Dither Tool Design

**Goal:** A Go CLI tool that reprocesses ZIM files by resizing and
Bayer-dithering all images, producing a much smaller ZIM with
visually useful 8-color indexed images. Enables compact Wikipedia
variants for space-constrained tiers (2 GB bugout kits).

**Motivation:** Kiwix publishes `mini` (stub articles, no images)
and `maxi` (full articles, full images) but nothing in between.
We want full article text with dithered images — readable diagrams,
identifiable photos, at a fraction of the size.

---

## Architecture

```
build-tools/
├── go.mod
├── go.sum
├── cmd/
│   └── zim-dither/
│       └── main.go
├── pkg/
│   └── imaging/
│       ├── bayer.go       # 4x4 ordered dithering (from ogma)
│       ├── common.go      # palette extraction, resize, clamp
│       └── dither.go      # algorithm dispatcher
```

First Go binary in the svalbard repo. `build-tools/` is for
build-time tooling (runs on the host / in Docker during provisioning).
Not packaged onto the stick. Future build tools share `go.mod` and
reuse `pkg/imaging`.

## Pipeline

```
input.zim
  → zimdump --dir /tmp/work/
  → walk all image entries (jpg, png, webp, gif)
  → for each image:
      1. decode
      2. resize to max width (default 400px, Lanczos3)
      3. extract palette (k-means, 8 colors, k-means++ seeding)
      4. Bayer 4x4 ordered dither
      5. encode as indexed PNG
  → zimwriterfs /tmp/work/ → output.zim
```

SVG files are passed through unchanged (already vector).

## CLI Interface

```bash
zim-dither [flags] input.zim output.zim

Flags:
  --width     int     Max image width in pixels (default 400)
  --colors    int     Palette size (default 8)
  --dither    string  Algorithm: bayer, floyd, atkinson (default "bayer")
  --workers   int     Parallel image processing goroutines (default NumCPU)
  --verbose           Print progress
```

## Integration with Svalbard

### Recipe

```yaml
id: wikipedia-en-top-dithered
type: zim
display_group: reference
tags: [general-reference, encyclopedia]
depth: standard
size_gb: 1.5  # estimated — needs measurement after first build
strategy: build
description: English Wikipedia Top 50k — full articles with dithered
  8-color images (Bayer ordered dithering)

build:
  family: zim-dither
  source_url: https://download.kiwix.org/zim/wikipedia/wikipedia_en_top_maxi_{date}.zim
  width: 400
  colors: 8
  dither: bayer
```

### Python builder dispatch

The Python builder (`commands.py`) downloads the source ZIM, then
shells out to `zim-dither`:

```python
subprocess.run([
    "zim-dither",
    "--width", str(build["width"]),
    "--colors", str(build["colors"]),
    "--dither", build["dither"],
    source_path, output_path,
], check=True)
```

### Docker container

The `svalbard-tools` Docker image adds:
- Go build of `zim-dither` (multi-stage, copies static binary)
- `zimdump` and `zimwriterfs` (already present via libzim)

## Dithering Algorithm

Ported from ogma (`internal/imaging/bayer.go`):

1. **Bayer matrix:** Standard 4x4 threshold matrix normalized to 0–1
2. **Palette extraction:** k-means clustering with k-means++ seeding,
   10 iterations, sampling every 3rd pixel
3. **Dithering:** Per-pixel threshold from matrix position, spread
   factor of 64.0 applied to each RGB channel, nearest palette color
4. **Output:** `image.Paletted` — indexed color PNG with 8-entry
   color table

This produces the distinctive ordered-dither look with visible
pattern structure. The 8-color indexed PNGs compress extremely well
(often 5-10x smaller than the source JPEG).

## Expected Size Reduction

Rough estimates for `wikipedia_en_top_maxi` (7.7 GB):
- Text content: ~2.1 GB (same as nopic — unchanged)
- Images original: ~5.6 GB (full resolution JPEGs)
- Images dithered: ~0.3–0.8 GB (400px indexed PNGs)
- **Estimated output: 1.5–3 GB** (needs measurement)

## Dependencies

- `github.com/nfnt/resize` — Lanczos3 image resizing (from ogma)
- `zimdump` — ZIM extraction (C++, in Docker image)
- `zimwriterfs` — ZIM creation (C++, in Docker image)
- Go standard library `image/*` — decode/encode, no CGo

## Future Extensions

- Additional dither algorithms (floyd-steinberg, atkinson, halftone)
- Warm/sepia/mono palette presets (already in ogma)
- Film grain overlay
- Per-image adaptive palette vs global palette
- Apply to any ZIM, not just Wikipedia
