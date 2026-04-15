# MIY Quality Badges, Extractors & Optimization

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve makeityourself ZIM quality with visual badges, fix broken extractors, add two new domain extractors, and optimize output size.

**Architecture:** All changes in `recipes/builders/makeityourself-zim.py`. The file uses a strategy-pattern SiteConfig system for per-domain extractors, a `_quality_score()` function for post-crawl evaluation, and a `_optimize_project()` pass for image/artifact optimization. The ZIM is assembled in `stage_package()`.

**Tech Stack:** Python 3, httpx, BeautifulSoup, Pillow, libzim

---

### Task 1: Add quality badges to category cards

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `_make_category_page()` (~line 2985), `FRONTPAGE_CSS` (~line 2890), `stage_package()` (~line 2655)

**What:** Map the existing `_quality_score()` (0–8 scale) to visual badge tiers on category page project cards. Score 6–8 = "★★★", 4–5 = "★★", 1–3 = "★". Score 0 = no badge. Add CSS classes `.badge-quality-high`, `.badge-quality-good`, `.badge-quality-basic`. Compute quality during `stage_package` loop and inject into project dicts.

**Commit:** `feat(miy): add quality badges to category page cards`

---

### Task 2: Fix Cults3D extractor

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `Cults3DJsonLdMetadata` (~line 1791), `Cults3DImages` (~line 1815)

**What:** Cults3D now uses `@type: "3DModel"` in JSON-LD (was `MediaObject`). Broaden the type check to accept both. Image CDN also serves from `static.cults3d.com` and `files.cults3d.com` — add these to the domain check alongside `images.cults3d.com`.

**Commit:** `fix(miy): broaden Cults3D JSON-LD type and CDN domain checks`

---

### Task 3: Add dead links link to index page

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `_make_index_page()` (~line 2937)

**What:** The dead links page (`dead/index.html`) exists but the main index has no link to it. Add a "View N dead links" link in the stats/footer area, only when dead_count > 0.

**Commit:** `feat(miy): add dead links link to index page`

---

### Task 4: Add Ana-White extractor

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — new `_anawhite_post_parse()`, `ANAWHITE_CONFIG`, register in `EXTRACTOR_MAP` and `SITE_CONFIGS`

**What:** ana-white.com is a woodworking/DIY site. Create a SiteConfig with:
- Metadata: `OpenGraphMetadata()`, `HtmlMetadata()` (JSON-LD is unreliable on this site)
- Images: handled in `post_parse` — scan for `img` tags within `.entry-content` with domain filter
- Artifacts: `LinkArtifacts()` for PDF plans
- Post-parse: extract author from `.entry-meta`, download images from content area
- Rate limit: 1.5s (default)

Register `ana-white.com` and `www.ana-white.com` in `EXTRACTOR_MAP`.

**Commit:** `feat(miy): add Ana-White domain extractor`

---

### Task 5: Add Hackaday.io extractor

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — new `_hackaday_post_parse()`, `HACKADAY_CONFIG`, register in `EXTRACTOR_MAP` and `SITE_CONFIGS`

**What:** hackaday.io is a maker project hosting site. Create a SiteConfig with:
- Metadata: `JsonLdMetadata()`, `OpenGraphMetadata()`, `HtmlMetadata()`
- Images: handled in `post_parse` — extract from `.project-image` and gallery sections on `cdn.hackaday.io`
- Artifacts: `LinkArtifacts()` for downloadable files
- Post-parse: extract project description from `.project-description`, author from `.project-creator`
- Rate limit: 1.5s (default)

Register `hackaday.io` and `www.hackaday.io` in `EXTRACTOR_MAP`.

**Commit:** `feat(miy): add Hackaday.io domain extractor`

---

### Task 6: Reduce image size limits

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — constants `IMAGE_MAX_DIM` (line 2469), `IMAGE_JPEG_QUALITY` (line 2470)

**What:** Reduce `IMAGE_MAX_DIM` from 1200 → 800 and `IMAGE_JPEG_QUALITY` from 85 → 80. This significantly reduces ZIM size while keeping images adequate for offline reference on tablets/phones.

**Commit:** `perf(miy): reduce image max dimension to 800px and quality to 80`

---

### Task 7: Increase ZIM cluster size

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `stage_package()` (~line 2754)

**What:** Increase `config_clustersize` from 2048 → 2097152 (2 MB). Larger clusters let the LZMA compressor find more redundancy across HTML pages and similar images, typically reducing ZIM size 20–30%.

**Commit:** `perf(miy): increase ZIM cluster size to 2MB for better compression`

---

### Task 8: Convert PNG to WebP during optimization

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `_optimize_project()` (~line 2473), image validation in `stage_package()` (~line 2709)

**What:** In the optimization pass, convert PNG images to WebP (lossless quality 80). WebP is ~30% smaller than optimized PNG. Update `meta.images` references from `.png` to `.webp`. Add `.webp` handling in the resize step. Keep the accepted extensions list including `.webp`.

**Commit:** `perf(miy): convert PNG images to WebP during optimization`

---

### Task 9: Add quality score logging

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — `stage_package()` (~line 2655)

**What:** After collecting projects in stage_package, compute quality scores for all and log a distribution summary: count per tier (high/good/basic/none), average score, and list of top issues. This helps track archive quality over builds.

**Commit:** `feat(miy): log quality score distribution during packaging`
