# Extractor Architecture Refactor — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the makeityourself-zim.py extractors into a composable architecture with reusable fetch strategies, metadata/image/artifact collectors, and an orchestrator — all within the single file, ready to split into modules later.

**Architecture:** Replace the current 6 copy-paste extractor classes with a composable pipeline: `FetchChain` (HTTP → SSL bypass → Playwright → Wayback) feeds HTML into `MetadataCollector` (JSON-LD, OG, HTML selectors), `ImageCollector` (img tags, data-src, API, og:image), and `ArtifactCollector` (link scanning, API). Site-specific extractors become thin configuration classes that override which collectors/strategies to use. An `Orchestrator` replaces `_crawl_one` + post-crawl quality checks.

**Tech Stack:** Python 3.12, httpx, BeautifulSoup, Playwright (optional), libzim

---

## Current State

The file is 3269 lines. Six extractor classes (Printables, Thingiverse, GitHub, Instructables, Cults3D, Generic) each contain 130-274 lines of mostly duplicated logic:
- ProjectMeta construction (6 copies)
- `images_dir.mkdir()` / `artifacts_dir.mkdir()` (12 copies)
- Image download loops (different URL sources, same download logic)
- `_save_meta` + `_generate_project_page` calls (12 copies)
- Result construction (6 copies)
- Error handling (6 copies)

Post-crawl logic in `_crawl_one` adds another layer: quality check, Playwright fallback, optimization, README description extraction — all specific to extraction but outside the extractors.

## Target Architecture

```
SiteScraper (orchestrator)
  │
  ├── FetchChain             → gets HTML from a URL
  │     ├── HttpFetcher      → httpx GET with browser UA
  │     ├── SslBypassFetcher → httpx GET with verify=False
  │     ├── PlaywrightFetcher→ headless Chromium
  │     └── WaybackFetcher   → rewrites URL to web.archive.org
  │
  ├── MetadataCollector      → extracts title/desc/author/license
  │     ├── JsonLdStrategy   → <script type="application/ld+json">
  │     ├── OpenGraphStrategy→ og:title, og:description, etc.
  │     └── HtmlStrategy     → <title>, <meta name="author">, etc.
  │
  ├── ImageCollector         → downloads images
  │     ├── ImgTagStrategy   → <img src/data-src> scanning
  │     ├── ApiStrategy      → site-specific API (Thingiverse, Printables)
  │     └── OgImageStrategy  → og:image fallback
  │
  ├── ArtifactCollector      → downloads STLs/PDFs/ZIPs
  │     ├── LinkScanStrategy → <a href="*.stl"> scanning
  │     └── ApiStrategy      → site-specific API downloads
  │
  └── QualityChecker         → evaluates result, triggers retries

Site-specific extractors = thin classes that configure:
  - Which FetchChain order
  - Which MetadataCollector strategies (and custom API calls)
  - Which ImageCollector strategies
  - Which ArtifactCollector strategies
  - Any pre/post hooks
```

## Key Design Decisions

1. **Protocol-based, not deep inheritance.** Each strategy implements a simple protocol (method signature). No abstract base class hierarchies. Easy to add a new strategy without touching existing code.

2. **FetchChain is ordered fallback.** Each fetcher returns `FetchResult(html, status, cookies)` or raises. The chain tries each in order until one succeeds. Site configs just specify which fetchers to try: `[HttpFetcher, SslBypassFetcher, PlaywrightFetcher]`.

3. **Collectors are additive.** Multiple image strategies run in sequence, each adding to the same list. A Printables extractor runs `ApiImageStrategy` first, then `ImgTagStrategy` as fallback. Generic runs `ImgTagStrategy` then `OgImageStrategy`.

4. **The orchestrator owns the lifecycle.** Construction of `ProjectMeta`, `site_dir`, `CrawlResult`, `_save_meta`, `_generate_project_page` — all in one place. Extractors don't touch these.

5. **Site configs are data, not code** (where possible). A Cults3D extractor is mostly: "use JSON-LD for metadata, `data-src` images from `images.cults3d.com`, no artifact downloads."

---

### Task 1: Define Protocols and Data Types

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py:213-265` (Data Models section)

**Step 1: Add FetchResult dataclass and strategy protocols**

After the existing `ProjectMeta` dataclass (~line 265), add:

```python
@dataclass
class FetchResult:
    """Result of fetching a URL."""
    html: str
    url: str  # final URL after redirects
    status_code: int = 200
    cookies: dict[str, str] = field(default_factory=dict)


class FetchStrategy:
    """Protocol for URL fetchers. Subclasses try one approach to get HTML."""
    name: str = "base"

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        raise NotImplementedError


class MetadataStrategy:
    """Protocol for metadata extraction from HTML."""
    name: str = "base"

    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        """Mutate meta in-place with extracted fields."""
        raise NotImplementedError


class ImageStrategy:
    """Protocol for image discovery and download."""
    name: str = "base"

    def collect(
        self, soup: BeautifulSoup, url: str, images_dir: Path,
        client: httpx.Client, existing: list[str], **kwargs,
    ) -> list[str]:
        """Download images, return list of relative paths (e.g. 'images/img_00.jpg')."""
        raise NotImplementedError


class ArtifactStrategy:
    """Protocol for artifact discovery and download."""
    name: str = "base"

    def collect(
        self, soup: BeautifulSoup, url: str, artifacts_dir: Path,
        client: httpx.Client, existing: list[dict], **kwargs,
    ) -> list[dict]:
        """Download artifacts, return list of artifact dicts."""
        raise NotImplementedError
```

**Step 2: Verify compiles**

Run: `python3 -c "import py_compile; py_compile.compile('recipes/builders/makeityourself-zim.py', doraise=True)"`

**Step 3: Commit**

```bash
git add recipes/builders/makeityourself-zim.py
git commit -m "refactor(miy): add protocols for fetch/metadata/image/artifact strategies"
```

---

### Task 2: Implement FetchStrategy Variants

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — add after the protocols from Task 1

**Step 1: Implement the four fetch strategies**

```python
class HttpFetcher(FetchStrategy):
    name = "http"

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        r = client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
        r.raise_for_status()
        return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)


class SslBypassFetcher(FetchStrategy):
    name = "ssl_bypass"

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        with httpx.Client(verify=False, timeout=30) as insecure:
            r = insecure.get(url, headers=HEADERS, follow_redirects=True)
            r.raise_for_status()
            log.info("    SSL bypass OK for %s", url[:60])
            return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)


class PlaywrightFetcher(FetchStrategy):
    name = "playwright"

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        if not _has_playwright():
            raise RuntimeError("Playwright not available")
        from playwright.sync_api import sync_playwright
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            ctx = browser.new_context(
                viewport={"width": 1280, "height": 800},
                ignore_https_errors=True,
            )
            page = ctx.new_page()
            page.goto(url, wait_until="domcontentloaded", timeout=30000)
            page.wait_for_timeout(3000)
            page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
            page.wait_for_timeout(2000)
            html = page.content()
            final_url = page.url
            browser.close()
            log.info("    Playwright fetch OK for %s", url[:60])
            return FetchResult(html=html, url=final_url)


class WaybackFetcher(FetchStrategy):
    name = "wayback"

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        wayback_url = kwargs.get("wayback_url", "")
        if not wayback_url:
            raise RuntimeError("No wayback URL provided")
        r = client.get(wayback_url, headers=HEADERS, timeout=30, follow_redirects=True)
        r.raise_for_status()
        return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)
```

**Step 2: Implement FetchChain**

```python
class FetchChain:
    """Try fetchers in order until one succeeds."""

    def __init__(self, fetchers: list[FetchStrategy]):
        self.fetchers = fetchers

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        last_error = None
        for fetcher in self.fetchers:
            try:
                return fetcher.fetch(url, client, **kwargs)
            except Exception as e:
                last_error = e
                err_str = str(e)
                # SSL error → skip to next (SslBypassFetcher)
                if "CERTIFICATE_VERIFY_FAILED" in err_str:
                    continue
                # 403 → skip to next (PlaywrightFetcher)
                if "403" in err_str:
                    continue
                # Other errors → also try next
                continue
        raise last_error or RuntimeError(f"All fetchers failed for {url}")
```

**Step 3: Verify compiles, commit**

```bash
git commit -m "refactor(miy): implement FetchChain with HTTP/SSL/Playwright/Wayback strategies"
```

---

### Task 3: Implement Metadata Strategies

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — add after fetch strategies

**Step 1: Implement three metadata strategies**

```python
class JsonLdMetadata(MetadataStrategy):
    """Extract metadata from JSON-LD script blocks."""
    name = "jsonld"

    def __init__(self, type_filter: str | None = None):
        self.type_filter = type_filter  # e.g. "Product", "MediaObject"

    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        for script in soup.find_all("script", type="application/ld+json"):
            try:
                data = json.loads(script.string)
                if not isinstance(data, dict):
                    continue
                if self.type_filter and data.get("@type") != self.type_filter:
                    continue
                meta.title = meta.title or data.get("name", "")
                meta.description = meta.description or (data.get("description") or "")[:2000]
                # Author: try multiple paths
                for author_path in [
                    data.get("creator", {}),
                    data.get("brand", {}),
                    (data.get("mainEntityOfPage") or {}).get("author", {}),
                ]:
                    if isinstance(author_path, dict) and author_path.get("name"):
                        meta.author = meta.author or author_path["name"]
                        break
                meta.license = meta.license or (data.get("mainEntityOfPage") or {}).get("license", "") or data.get("license", "")
                break
            except (json.JSONDecodeError, TypeError):
                continue


class OpenGraphMetadata(MetadataStrategy):
    """Extract metadata from OpenGraph meta tags."""
    name = "opengraph"

    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        og = lambda prop: (soup.find("meta", property=f"og:{prop}") or {}).get("content", "")
        meta.title = meta.title or og("title")
        meta.description = meta.description or og("description")[:2000]
        meta.author = meta.author or og("site_name")


class HtmlMetadata(MetadataStrategy):
    """Extract metadata from standard HTML elements."""
    name = "html"

    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        if not meta.title:
            title_tag = soup.find("title")
            if title_tag:
                meta.title = title_tag.get_text().strip()[:200]
        if not meta.description:
            desc_tag = soup.find("meta", attrs={"name": "description"})
            if desc_tag:
                meta.description = desc_tag.get("content", "")[:2000]
        if not meta.author:
            author_tag = soup.find("meta", attrs={"name": "author"})
            if author_tag:
                meta.author = author_tag.get("content", "")
```

**Step 2: Verify compiles, commit**

```bash
git commit -m "refactor(miy): implement JSON-LD, OpenGraph, HTML metadata strategies"
```

---

### Task 4: Implement Image and Artifact Strategies

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — add after metadata strategies

**Step 1: Implement image strategies**

```python
class ImgTagImages(ImageStrategy):
    """Collect images from <img> tags, filtering by domain and size."""
    name = "img_tags"

    def __init__(self, *, domain_filter: str | None = None, data_src: bool = False, max_images: int = 12):
        self.domain_filter = domain_filter
        self.data_src = data_src
        self.max_images = max_images

    def collect(self, soup, url, images_dir, client, existing, **kwargs):
        new_images = []
        seen_bases = {Path(e).stem for e in existing}
        base_domain = urlparse(url).netloc.replace("www.", "")
        count = len(existing)

        for img in soup.find_all("img"):
            if count >= self.max_images:
                break
            src = (img.get("data-src") if self.data_src else None) or img.get("src") or ""
            if not src.startswith("http"):
                if src.startswith("/"):
                    src = urljoin(url, src)
                else:
                    continue
            if src.startswith("data:"):
                continue
            # Domain filter
            img_domain = urlparse(src).netloc.replace("www.", "")
            if self.domain_filter and self.domain_filter not in img_domain:
                continue
            elif not self.domain_filter:
                # By default, accept same domain + major CDNs
                if img_domain != base_domain and not any(
                    cdn in img_domain for cdn in ["cloudfront", "cdn", "imgur", "wp.com", "squarespace"]
                ):
                    continue
            base = src.split("?")[0]
            slug = _slugify(Path(urlparse(base).path).stem)[:20]
            if slug in seen_bases:
                continue
            seen_bases.add(slug)
            fname = f"img_{count:02d}.jpg"
            if _download_file(client, src, images_dir / fname, cookies=kwargs.get("cookies")):
                new_images.append(f"images/{fname}")
                count += 1
            time.sleep(0.3)
        return new_images


class OgImages(ImageStrategy):
    """Fallback: collect og:image."""
    name = "og_image"

    def collect(self, soup, url, images_dir, client, existing, **kwargs):
        if existing:
            return []
        og_img = soup.find("meta", property="og:image")
        if og_img and og_img.get("content", "").startswith("http"):
            fname = "og.jpg"
            if _download_file(client, og_img["content"], images_dir / fname):
                return [f"images/{fname}"]
        return []


class LinkArtifacts(ArtifactStrategy):
    """Scan <a> links for downloadable artifact files."""
    name = "link_scan"

    def collect(self, soup, url, artifacts_dir, client, existing, **kwargs):
        new_artifacts = []
        for link in soup.find_all("a", href=True):
            href = link["href"]
            if not href.startswith("http"):
                href = urljoin(url, href)
            ext = Path(urlparse(href).path).suffix.lower()
            if ext not in ARTIFACT_EXTS:
                continue
            fname = _sanitize_filename(Path(urlparse(href).path).name)
            if _download_file(client, href, artifacts_dir / fname):
                fsize = (artifacts_dir / fname).stat().st_size
                new_artifacts.append({
                    "filename": f"artifacts/{fname}",
                    "type": ext.lstrip("."),
                    "size_bytes": fsize,
                })
            time.sleep(0.3)
        return new_artifacts
```

**Step 2: Verify compiles, commit**

```bash
git commit -m "refactor(miy): implement ImgTag, OgImage, LinkArtifact collector strategies"
```

---

### Task 5: Build the SiteScraper Orchestrator

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — add after collector strategies

This is the core: replaces the duplicated extract() boilerplate in every extractor.

**Step 1: Implement SiteScraper**

```python
@dataclass
class SiteConfig:
    """Configuration for a site-specific scraper."""
    name: str
    domain: str
    fetch_chain: list[FetchStrategy]
    metadata_strategies: list[MetadataStrategy]
    image_strategies: list[ImageStrategy]
    artifact_strategies: list[ArtifactStrategy]
    # Optional hooks for site-specific logic
    pre_fetch: Callable | None = None   # (verified, client) -> dict of extra kwargs
    post_parse: Callable | None = None  # (soup, meta, site_dir, client) -> None
    rate_limit: float = CRAWL_DELAY_PER_DOMAIN


class SiteScraper:
    """Orchestrator that runs a configured extraction pipeline."""

    def __init__(self, config: SiteConfig, client: httpx.Client, sites_dir: Path):
        self.config = config
        self.client = client
        self.sites_dir = sites_dir
        self.fetch_chain = FetchChain(config.fetch_chain)

    def site_dir_for(self, url: str) -> Path:
        parsed = urlparse(url)
        domain = parsed.netloc.replace("www.", "")
        path_slug = _slugify(parsed.path.strip("/"))
        d = self.sites_dir / domain / path_slug
        d.mkdir(parents=True, exist_ok=True)
        return d

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        site_dir = self.site_dir_for(url)
        result = CrawlResult(url=verified.url, extractor=self.config.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain=self.config.domain or urlparse(verified.url).netloc.replace("www.", ""),
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        # Pre-fetch hook (e.g. prepare API headers, cookies)
        extra_kwargs = {}
        if self.config.pre_fetch:
            extra_kwargs = self.config.pre_fetch(verified, self.client) or {}

        # Fetch HTML
        try:
            fetch_result = self.fetch_chain.fetch(
                url, self.client,
                wayback_url=verified.wayback_url if verified.status == "wayback" else "",
                **extra_kwargs,
            )
            html = fetch_result.html
            (site_dir / "raw.html").write_text(html)
        except Exception as e:
            result.status = "failed"
            result.error = str(e)[:200]
            return result

        soup = BeautifulSoup(html, "html.parser")

        # Extract metadata (each strategy fills in what it can)
        for strategy in self.config.metadata_strategies:
            strategy.extract(soup, url, meta)

        # Post-parse hook (e.g. Printables GraphQL, Thingiverse API, GitHub ZIP)
        if self.config.post_parse:
            try:
                self.config.post_parse(soup, meta, site_dir, self.client, **extra_kwargs)
            except Exception as e:
                log.debug("    post_parse hook error: %s", e)

        # Collect images
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        for strategy in self.config.image_strategies:
            new_imgs = strategy.collect(
                soup, url, images_dir, self.client, meta.images, **extra_kwargs,
            )
            meta.images.extend(new_imgs)
            total_size += sum(
                (images_dir / Path(p).name).stat().st_size
                for p in new_imgs if (images_dir / Path(p).name).exists()
            )

        # Collect artifacts
        artifacts_dir = site_dir / "artifacts"
        artifacts_dir.mkdir(exist_ok=True)
        for strategy in self.config.artifact_strategies:
            new_arts = strategy.collect(
                soup, url, artifacts_dir, self.client, meta.artifacts, **extra_kwargs,
            )
            meta.artifacts.extend(new_arts)
            total_size += sum(a.get("size_bytes", 0) for a in new_arts)

        # Finalize
        meta.title = meta.title or verified.title or "Untitled"
        _generate_project_page(site_dir, meta)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result
```

**Step 2: Verify compiles, commit**

```bash
git commit -m "refactor(miy): implement SiteScraper orchestrator with composable pipeline"
```

---

### Task 6: Define Site Configs for All Extractors

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — replace old extractor classes

Each old 130-274 line extractor becomes a ~20-40 line site config + optional hook functions.

**Step 1: Define Generic config (simplest)**

```python
GENERIC_CONFIG = SiteConfig(
    name="generic",
    domain="",
    fetch_chain=[HttpFetcher(), SslBypassFetcher(), PlaywrightFetcher()],
    metadata_strategies=[JsonLdMetadata(), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[ImgTagImages(), OgImages()],
    artifact_strategies=[LinkArtifacts()],
)
```

**Step 2: Define Cults3D config**

```python
CULTS3D_CONFIG = SiteConfig(
    name="cults3d",
    domain="cults3d.com",
    fetch_chain=[HttpFetcher()],
    metadata_strategies=[JsonLdMetadata(type_filter="MediaObject"), OpenGraphMetadata()],
    image_strategies=[ImgTagImages(domain_filter="images.cults3d.com", data_src=True)],
    artifact_strategies=[],  # STLs require auth
)
```

**Step 3: Define Instructables config**

Uses same generic strategies but adds a `post_parse` hook for step extraction.

```python
def _instructables_post_parse(soup, meta, site_dir, client, **kwargs):
    """Extract step-by-step instructions from Instructables pages."""
    steps = []
    for step in soup.find_all("div", class_="step") or soup.find_all("section", class_="step"):
        step_title = (step.find(["h2", "h3"]) or {}).get_text("").strip() if step.find(["h2", "h3"]) else ""
        step_body = step.find("div", class_="step-body")
        step_text = step_body.get_text(strip=True) if step_body else step.get_text(strip=True)
        steps.append({"title": step_title, "text": step_text[:2000]})
    if steps:
        (site_dir / "steps.json").write_text(json.dumps(steps, ensure_ascii=False, indent=2))


INSTRUCTABLES_CONFIG = SiteConfig(
    name="instructables",
    domain="instructables.com",
    fetch_chain=[HttpFetcher()],
    metadata_strategies=[OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[ImgTagImages(domain_filter="content.instructables.com")],
    artifact_strategies=[LinkArtifacts()],
    post_parse=_instructables_post_parse,
)
```

**Step 4: Define GitHub config**

GitHub is unique — downloads repo ZIP, renders README. Needs a custom `post_parse` hook.

```python
def _github_post_parse(soup, meta, site_dir, client, **kwargs):
    """Download repo ZIP and extract README."""
    # Parse owner/repo from URL
    m = re.match(r"https?://github\.com/([^/]+)/([^/]+)", meta.url)
    if not m:
        return
    owner, repo = m.group(1), m.group(2)

    # API metadata
    try:
        r = client.get(f"https://api.github.com/repos/{owner}/{repo}", headers=HEADERS, timeout=15)
        if r.status_code == 200:
            data = r.json()
            meta.description = meta.description or (data.get("description") or "")[:2000]
            meta.license = (data.get("license") or {}).get("spdx_id", "")
            meta.author = data.get("owner", {}).get("login", "")
            default_branch = data.get("default_branch", "main")
    except Exception:
        default_branch = "main"

    # Download repo ZIP
    zip_url = f"https://github.com/{owner}/{repo}/archive/refs/heads/{default_branch}.zip"
    zip_path = site_dir / "artifacts" / f"{repo}.zip"
    (site_dir / "artifacts").mkdir(exist_ok=True)
    if _download_file(client, zip_url, zip_path):
        meta.artifacts.append({
            "filename": f"artifacts/{repo}.zip",
            "type": "zip",
            "size_bytes": zip_path.stat().st_size,
        })

    # Fetch README
    for readme_name in ["README.md", "readme.md", "Readme.md"]:
        raw_url = f"https://raw.githubusercontent.com/{owner}/{repo}/{default_branch}/{readme_name}"
        try:
            r = client.get(raw_url, headers=HEADERS, timeout=15)
            if r.status_code == 200:
                (site_dir / "README.md").write_text(r.text)
                break
        except Exception:
            continue


GITHUB_CONFIG = SiteConfig(
    name="github",
    domain="github.com",
    fetch_chain=[HttpFetcher()],
    metadata_strategies=[OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[OgImages()],  # GitHub pages rarely have inline images
    artifact_strategies=[],  # ZIP download is in post_parse
    post_parse=_github_post_parse,
)
```

**Step 5: Define Printables config**

The most complex — GraphQL API for metadata, signed download URLs for STLs. Needs a custom `post_parse` hook and a custom `ArtifactStrategy` subclass.

```python
class PrintablesApiImages(ImageStrategy):
    """Fetch images via Printables GraphQL API."""
    name = "printables_api"

    def collect(self, soup, url, images_dir, client, existing, **kwargs):
        model_id = re.search(r"/model/(\d+)", url)
        if not model_id:
            return []
        model_id = model_id.group(1)
        # ... (move existing Printables GraphQL image logic here)
        # Returns list of 'images/img_XX.jpg' paths


class PrintablesApiArtifacts(ArtifactStrategy):
    """Download STL files via Printables GraphQL signed URLs."""
    name = "printables_api"

    def collect(self, soup, url, artifacts_dir, client, existing, **kwargs):
        # ... (move existing Printables STL download logic here)
        # Returns list of artifact dicts


def _printables_pre_fetch(verified, client):
    """Prepare auth token for Printables API calls."""
    return {"printables_token": getattr(client, "_printables_token", "")}


PRINTABLES_CONFIG = SiteConfig(
    name="printables",
    domain="printables.com",
    fetch_chain=[HttpFetcher()],
    metadata_strategies=[],  # metadata comes from GraphQL in post_parse
    image_strategies=[PrintablesApiImages()],
    artifact_strategies=[PrintablesApiArtifacts()],
    pre_fetch=_printables_pre_fetch,
    post_parse=_printables_post_parse,  # GraphQL metadata extraction
)
```

**Step 6: Define Thingiverse config**

Similar to Printables — API-driven with CDN cookie bypass.

```python
class ThingiverseApiImages(ImageStrategy):
    name = "thingiverse_api"
    # ... (move Thingiverse API image download logic)

class ThingiverseApiArtifacts(ArtifactStrategy):
    name = "thingiverse_api"
    # ... (move Thingiverse API STL download logic)

THINGIVERSE_CONFIG = SiteConfig(
    name="thingiverse",
    domain="thingiverse.com",
    fetch_chain=[HttpFetcher()],
    metadata_strategies=[JsonLdMetadata(type_filter="Product"), HtmlMetadata()],
    image_strategies=[ThingiverseApiImages()],
    artifact_strategies=[ThingiverseApiArtifacts()],
    post_parse=_thingiverse_post_parse,  # API metadata enrichment
    rate_limit=2.0,
)
```

**Step 7: Update EXTRACTORS and stage_crawl**

Replace the old class-based `EXTRACTORS` dict and extractor instantiation:

```python
SITE_CONFIGS: dict[str, SiteConfig] = {
    "printables": PRINTABLES_CONFIG,
    "thingiverse": THINGIVERSE_CONFIG,
    "github": GITHUB_CONFIG,
    "instructables": INSTRUCTABLES_CONFIG,
    "cults3d": CULTS3D_CONFIG,
    "generic": GENERIC_CONFIG,
}
```

In `stage_crawl`, replace extractor instantiation with:

```python
scrapers: dict[str, SiteScraper] = {}
for name, config in SITE_CONFIGS.items():
    scrapers[name] = SiteScraper(config, client, state.sites_dir)
```

**Step 8: Remove old extractor classes**

Delete `PrintablesExtractor`, `ThingiverseExtractor`, `GitHubExtractor`, `InstructablesExtractor`, `Cults3DExtractor`, `GenericExtractor`, and `BaseExtractor`. This should remove ~900 lines.

**Step 9: Verify compiles, run end-to-end test on 3 links, commit**

```bash
git commit -m "refactor(miy): replace 6 extractor classes with composable SiteConfig + SiteScraper"
```

---

### Task 7: Move Post-Crawl Logic Into Orchestrator

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py` — simplify `_crawl_one`

**Step 1: Move quality check + Playwright fallback into SiteScraper**

Add a `post_extract` method to `SiteScraper` that runs after `extract()`:

```python
def post_extract(self, site_dir: Path, meta: ProjectMeta, url: str) -> None:
    """Quality check: if no images, try Playwright fallback."""
    quality = _quality_score(site_dir)
    if "no_images" in quality["issues"] and _has_playwright():
        _playwright_recrawl(url, site_dir, meta)
        _save_meta(site_dir, meta)

    # Enrich description from raw HTML if weak
    better = _extract_desc_from_raw(site_dir, meta.description)
    if better:
        meta.description = better
        _save_meta(site_dir, meta)
```

**Step 2: Simplify `_crawl_one` in `stage_crawl`**

The `_crawl_one` function shrinks from ~75 lines to ~30 lines:

```python
def _crawl_one(v: VerifiedLink) -> CrawlResult:
    ext_name = _classify_extractor(v.url)
    if v.status == "wayback" and v.wayback_url:
        ext_name = "generic"
    scraper = scrapers.get(ext_name, scrapers["generic"])
    attempts = progress.get(v.url, {}).get("attempts", 0) + 1
    sem = ext_sems.get(ext_name, ext_sems["generic"])

    with sem:
        try:
            result = scraper.extract(v)
            result.attempts = attempts
        except Exception as e:
            return CrawlResult(
                url=v.url, status="failed", extractor=ext_name,
                error=str(e)[:200], attempts=attempts,
                ts=datetime.now(timezone.utc).isoformat(),
            )

    # Post-extract quality improvements
    if result.status == "completed":
        site_dir = scraper.site_dir_for(v.final_url or v.url)
        meta_path = site_dir / "meta.json"
        if meta_path.exists():
            meta_obj = _meta_from_dict(json.loads(meta_path.read_text()))
            scraper.post_extract(site_dir, meta_obj, v.final_url or v.url)
            if optimize:
                _optimize_project(site_dir, meta_obj)
            _save_meta(site_dir, meta_obj)
            result.files = len(meta_obj.artifacts) + len(meta_obj.images)

    # ... progress tracking (unchanged)
    return result
```

**Step 3: Verify compiles, run end-to-end test, commit**

```bash
git commit -m "refactor(miy): move post-crawl quality check into SiteScraper.post_extract"
```

---

### Task 8: Final Cleanup and Verification

**Files:**
- Modify: `recipes/builders/makeityourself-zim.py`

**Step 1: Remove dead code**

- Remove `_playwright_recrawl` standalone function (now integrated into orchestrator flow)
- Remove any orphaned helper functions
- Remove unused imports

**Step 2: Run full end-to-end test**

```bash
# Test with 3 diverse links: one Printables, one generic, one GitHub
~/.pyenv/miy-env/bin/python3 -c "
# Quick integration test...
" 
```

**Step 3: Run on full dataset with --force-stage package**

```bash
~/.pyenv/miy-env/bin/python3 recipes/builders/makeityourself-zim.py --force-stage package
```

Verify ZIM output matches previous build (~10 GB, ~896 projects).

**Step 4: Final commit**

```bash
git commit -m "refactor(miy): cleanup dead code after extractor architecture refactor"
```

---

## Expected Line Count Change

| Section | Before | After | Delta |
|---------|--------|-------|-------|
| Protocols + strategies | 0 | ~250 | +250 |
| SiteScraper orchestrator | 0 | ~100 | +100 |
| Site configs + hooks | 0 | ~300 | +300 |
| Old extractor classes | ~900 | 0 | -900 |
| _crawl_one | ~75 | ~30 | -45 |
| Total | 3269 | ~2975 | ~-295 |

Net reduction of ~300 lines while gaining clean interfaces, composable strategies, and zero duplicated fetch/save/generate boilerplate.

## What This Enables Later

1. **New ZIM builders** import strategies: `from makeityourself_zim import HttpFetcher, ImgTagImages, SiteScraper`
2. **Module extraction**: each strategy class → own file, `SiteConfig` → YAML
3. **Testing**: strategy classes are pure functions, easy to unit test with mock HTML
4. **Plugin system**: drop a `SiteConfig` dict into a registry → auto-detected
