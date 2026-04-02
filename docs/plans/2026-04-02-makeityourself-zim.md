# Make It Yourself — Offline Archive ZIM

**Date:** 2026-04-02
**Status:** Design
**Source:** https://makeityourself.org/ (PDF by NODE, Nov 2024)

## Goal

Archive the 1000+ DIY projects curated in the Make It Yourself PDF into a single,
browsable ZIM file with all instructions, images, and downloadable artifacts (STLs,
firmware, sewing patterns, etc.) preserved for offline use.

## Source Analysis

The PDF contains **917 unique URLs** across **220 pages**, organized into categories
(Gaming, Computing, Photography, Music, Cycling, Fashion, Home, Garden, Tools, etc.).

### Domain Distribution

| Domain | Count | Extractor | Crawl Difficulty |
|--------|-------|-----------|-----------------|
| printables.com | 298 (33%) | `PrintablesExtractor` | Medium — SSR + GraphQL API for files |
| thingiverse.com | 124 (14%) | `ThingiverseExtractor` | Hard — JS SPA, API fallback |
| github.com | 56 (6%) | `GitHubExtractor` | Easy — ZIP download + README |
| instructables.com | 53 (6%) | `InstructablesExtractor` | Medium — server-rendered |
| Woodworking blogs | ~100 (11%) | `GenericExtractor` | Easy — static HTML |
| Other | ~286 (31%) | `GenericExtractor` | Varies |

### Link Health (probed 2026-04-02)

- **596 (65%)** — alive (200 OK)
- **299 (33%)** — Printables rate-limited (502/connection error, actually alive)
- **11 (1.2%)** — truly dead (404)
- **11 (1.2%)** — other errors (403, 500, 503)

Dead links get a Wayback Machine lookup via `https://archive.org/wayback/available?url=`.

## Architecture

### Pipeline Overview

```
miy-archiver.py --workdir /data/miy
  ├── Stage 1: extract    MIY.pdf → links.jsonl
  ├── Stage 2: verify     links.jsonl → verified.jsonl
  ├── Stage 3: crawl      verified.jsonl → workdir/sites/...
  └── Stage 4: package    workdir/ → makeityourself.zim
```

Standalone Python script. Runs inside `svalbard-tools` container or locally with
dependencies installed. Each stage is **resumable** via checkpoint files.

### CLI Interface

```bash
# Normal run (resumes from where it left off)
python3 miy-archiver.py --workdir /data/miy

# Force full rerun (wipes state, re-downloads everything)
python3 miy-archiver.py --workdir /data/miy --force

# Force a specific stage and everything downstream
python3 miy-archiver.py --workdir /data/miy --force-stage verify

# Re-attempt only previously failed crawls
python3 miy-archiver.py --workdir /data/miy --retry-failed
```

## Stage 1: Extract (PDF → links.jsonl)

Parse `MIY.pdf` with PyMuPDF (`fitz`). Extract:

- **URLs** from link annotations on each page
- **Category/section structure** from page headings and table of contents
- **Project titles and descriptions** from text near each link
- **Thumbnail images** from the page (image closest to each link)

Output: `links.jsonl` — one record per project:

```jsonl
{"url": "https://www.printables.com/model/565344-...", "page": 73, "category": "Camping", "subcategory": "Rope & Cord", "title": "Automatic Rope Tensioner", "description": "...", "thumbnail": "thumbnails/p73_01.jpg"}
```

**Resume:** Skip if `links.jsonl` exists and PDF checksum matches.

## Stage 2: Verify (links.jsonl → verified.jsonl)

For each URL in `links.jsonl`:

1. **HTTP HEAD** with browser User-Agent, follow redirects (timeout: 15s)
2. If HEAD fails, **HTTP GET** as fallback
3. If dead (404, 410, DNS failure), check **Wayback Machine**:
   `https://archive.org/wayback/available?url={url}`
4. Classify domain → extractor type

Output: `verified.jsonl` — enriches each record:

```jsonl
{"url": "...", "status": "alive", "final_url": "...", "extractor": "printables", "content_type": "text/html", ...}
{"url": "...", "status": "dead", "wayback_url": "https://web.archive.org/web/2024.../...", "extractor": "generic", ...}
{"url": "...", "status": "dead", "wayback_url": null, "extractor": null, ...}
```

Rate limit: 5 concurrent requests, 200ms delay between batches.

**Resume:** Load existing `verified.jsonl`, only probe URLs not already in it.

## Stage 3: Crawl (verified.jsonl → sites/)

Dispatches each verified URL to the appropriate extractor. All extractors produce a
standard output structure:

```
workdir/sites/{domain}/{slug}/
  ├── index.html          ← cleaned page content
  ├── images/             ← photos, renders, diagrams
  ├── artifacts/          ← STLs, PDFs, DXFs, ZIPs, firmware, KiCad files
  └── meta.json           ← structured metadata
```

### meta.json Schema

```json
{
  "url": "https://www.printables.com/model/565344-...",
  "title": "Automatic Rope Tensioner",
  "description": "Paracord/rope tensioner for camping",
  "author": "username",
  "license": "CC-BY-SA-4.0",
  "category": "Camping",
  "source_domain": "printables.com",
  "images": ["images/01.jpg", "images/02.jpg"],
  "artifacts": [
    {"filename": "artifacts/tensioner_body.stl", "type": "stl", "size_bytes": 245000},
    {"filename": "artifacts/tensioner_pin.stl", "type": "stl", "size_bytes": 12000}
  ],
  "crawled_at": "2026-04-02T14:23:00Z",
  "source_status": "alive"
}
```

### Extractor: Printables (298 links)

1. Fetch page with browser UA header
2. Parse embedded `__data` JSON (SvelteKit SSR payload) for:
   - Title, description, author, license
   - Image gallery URLs
   - File list with download URLs (STL, 3MF, STEP, etc.)
   - Auto-generated PDF URL
3. Download all files and images
4. Generate clean `index.html` from structured data

Rate limit: 1 req/sec with exponential backoff on 429/502.

### Extractor: Thingiverse (124 links)

1. Extract thing ID from URL
2. Fetch metadata: `GET api.thingiverse.com/things/{id}`
3. Fetch files: `GET api.thingiverse.com/things/{id}/files`
4. Download STLs and images via API URLs
5. Generate clean `index.html`

Fallback: If API is blocked, use Playwright headless to snapshot the page
and attempt to extract download links from rendered DOM.

### Extractor: GitHub (56 links)

1. Fetch repo metadata via GitHub API (`gh api repos/{owner}/{repo}`)
2. Download repo ZIP: `/{owner}/{repo}/archive/refs/heads/{default_branch}.zip`
3. Render README.md → `index.html`
4. Preserve relevant artifacts (filter out `.git`, `node_modules`, `__pycache__`, etc.)
5. For large repos (>50MB), only keep docs + artifact files (STL, STEP, KiCad, gerber, etc.)

### Extractor: Instructables (53 links)

1. Fetch page (server-rendered HTML)
2. Parse with BeautifulSoup: step-by-step instructions, images, attached files
3. Download all images and file attachments
4. Generate clean `index.html` preserving step structure

### Extractor: Generic (286 links)

For the long tail of blogs, shops, and specialty sites:

1. Load page with Playwright (headless Chromium), wait up to 30s for content
2. Snapshot full rendered DOM as static HTML
3. Download linked assets from same domain: images, PDFs, ZIPs, STLs
4. Rewrite internal links to point to local assets
5. If page fails to load, mark as `failed` in progress

### Progress Tracking

`workdir/crawl_progress.jsonl` — one entry per completed/failed URL:

```jsonl
{"url": "...", "status": "completed", "extractor": "printables", "files": 8, "size_bytes": 4521000, "ts": "2026-04-02T14:23:00Z"}
{"url": "...", "status": "failed", "error": "timeout", "attempts": 3, "ts": "2026-04-02T14:24:12Z"}
```

**Resume:** Load `crawl_progress.jsonl`, skip `completed` entries. Re-attempt `failed`
entries up to 3 times with exponential backoff. `--retry-failed` resets attempt count
on failed entries.

**`--force-stage crawl`:** Deletes `crawl_progress.jsonl` and `sites/` directory,
re-crawls everything. Does NOT re-run stages 1-2.

## Stage 4: Package (sites/ → ZIM)

### Frontpage Generation

The PDF's category structure becomes a responsive HTML index — the ZIM's entry point.

```
output/
  ├── index.html                     ← main catalog page
  ├── style.css
  ├── category/
  │     ├── gaming/index.html
  │     ├── cycling/index.html
  │     ├── camping/index.html
  │     └── .../index.html
  ├── sites/                         ← copied from workdir/sites/
  │     ├── printables.com/...
  │     ├── github.com/...
  │     └── .../
  └── dead/
        └── index.html               ← dead links listing with wayback status
```

**index.html** contains:
- Title, intro text, credit to NODE and makeityourself.org
- Stats: "X projects archived, Y artifacts, Z total size"
- Category grid: cards with project count per category
- Search hint (Kiwix FTS works across all project titles/descriptions)

**Per-category pages** contain:
- Project cards with: thumbnail, title, 1-line description, source domain badge,
  artifact count ("3 STL files"), status badge (archived / wayback / dead)
- Cards link to the project's `index.html` within `sites/`
- Filter tags by making method (3D printing, woodworking, sewing, electronics, etc.)

Styled with minimal inline CSS. No JavaScript required. Mobile-friendly.

### ZIM Creation

Uses `zimwriterfs` (available in svalbard-tools container):

```bash
zimwriterfs \
  --welcome index.html \
  --title "Make It Yourself" \
  --description "1000+ curated DIY projects archived for offline use" \
  --creator "NODE / makeityourself.org" \
  --publisher "svalbard" \
  --language eng \
  --illustration favicon.png \
  --withFullTextIndex \
  output/ \
  makeityourself.zim
```

### Estimated Output

- **ZIM size:** 2-5 GB (depending on STL file sizes)
  - ~298 Printables models × ~5-20MB STLs each = 1-3 GB
  - ~124 Thingiverse models = 0.5-1 GB
  - Images across all projects = 0.5-1 GB
  - HTML/text: negligible
- **Runtime:** 2-4 hours (stage 3 dominates, polite rate limits)

## Resumability Summary

| Flag | Behavior |
|------|----------|
| *(no flag)* | Resume from last checkpoint. Skip completed stages/URLs. |
| `--force` | Wipe entire workdir, start from scratch. |
| `--force-stage extract` | Re-parse PDF. Invalidates verify + crawl. |
| `--force-stage verify` | Re-probe all links. Invalidates crawl for changed statuses. |
| `--force-stage crawl` | Re-crawl all sites. Keeps verify results. |
| `--force-stage package` | Re-generate frontpage + ZIM. Keeps crawled data. |
| `--retry-failed` | Re-attempt only `failed` entries in crawl progress. |

Downstream invalidation: `--force-stage verify` checks if any URL's status changed
from alive→dead or dead→alive. Only those URLs are re-crawled; unchanged ones keep
their existing crawl data.

## Svalbard Integration

### Recipe: `recipes/content/makeityourself.yaml`

```yaml
id: makeityourself
type: zim
group: practical
tags: [diy, maker, 3d-printing, woodworking, sewing, electronics]
size_gb: 3.5
description: "1000+ curated DIY projects archived from makeityourself.org"
strategy: build
build:
  family: custom
  builder: makeityourself-zim.py
  source_url: https://makeityourself.org/MIY.pdf
license:
  id: mixed
  attribution: "Curated by NODE (makeityourself.org). Individual project licenses vary."
  note: "PDF has no explicit license. Projects are CC-BY-SA, CC-BY-NC, MIT, etc."
```

### Container Requirements

The script needs these tools (all available or addable in `svalbard-tools`):
- Python 3.10+ with: `pymupdf`, `aiohttp`, `beautifulsoup4`, `playwright`
- Playwright Chromium (for generic extractor)
- `zimwriterfs` (from openzim)

### Licensing Considerations

- The MIY PDF has **no explicit license** (defaults to all rights reserved by NODE)
- We do NOT redistribute the PDF — we build an independent archive of the linked sources
- Each project's license is preserved in `meta.json` and displayed on project pages
- NODE and makeityourself.org are credited prominently on the frontpage
- Recommend reaching out to NODE for explicit permission
