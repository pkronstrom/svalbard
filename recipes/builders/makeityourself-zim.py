#!/usr/bin/env python3
"""Archive 1000+ DIY projects from makeityourself.org into a browsable ZIM.

Downloads the Make It Yourself PDF, extracts all project links, verifies them,
crawls each site with domain-specific extractors, and packages everything into
a single searchable ZIM file with a responsive HTML frontpage.

Requirements: pip install pymupdf httpx beautifulsoup4 libzim Pillow
Optional: pip install playwright (for JS-heavy generic sites)

Usage:
    python3 makeityourself-zim.py --workdir /data/miy
    python3 makeityourself-zim.py --workdir /data/miy --force
    python3 makeityourself-zim.py --workdir /data/miy --force-stage verify
    python3 makeityourself-zim.py --workdir /data/miy --retry-failed
    python3 makeityourself-zim.py --workdir /data/miy --retry-dead

Environment variables:
    MIY_THINGIVERSE_TOKEN  — Thingiverse API app token (required for STL downloads).
                             Register at https://www.thingiverse.com/developers
                             Without it, Thingiverse metadata is archived but STLs are not.

    MIY_PRINTABLES_TOKEN   — Printables API bearer token (optional).
                             STL downloads work without auth via signed URLs, but
                             providing a token may avoid rate limits on heavy runs.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import logging
import mimetypes
import re
import sys
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from html import escape, unescape
from pathlib import Path
from urllib.parse import urlparse, urljoin

import fitz  # pymupdf
import httpx
from bs4 import BeautifulSoup
from libzim.writer import Creator, Item, StringProvider, FileProvider, Hint

log = logging.getLogger("miy")

PDF_URL = "https://makeityourself.org/MIY.pdf"
WAYBACK_API = "https://archive.org/wayback/available"

# Default paths relative to svalbard project root
DEFAULT_WORKDIR = "library/workspace/miy"
DEFAULT_OUTPUT = "library/makeityourself.zim"

BROWSER_UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)
HEADERS = {"User-Agent": BROWSER_UA}

# File extensions we consider artifacts (downloadable project files)
ARTIFACT_EXTS = {
    ".stl", ".3mf", ".step", ".stp", ".obj", ".dxf", ".svg", ".pdf",
    ".scad", ".f3d", ".fcstd", ".kicad_pcb", ".sch", ".gbr", ".zip",
    ".gz", ".tar", ".7z", ".rar", ".gcode", ".ino", ".hex", ".bin",
    ".gerber", ".brd", ".drl",
}

MAX_RETRIES = 3
VERIFY_CONCURRENCY = 10
VERIFY_DELAY = 0.2
CRAWL_DELAY_PER_DOMAIN = 1.5  # seconds between requests to same domain

THINGIVERSE_COOKIES_FILE = "thingiverse_cookies.json"

# Headers that match a real Chrome browser — Cloudflare checks these alongside cookies
_TV_CDN_HEADERS = {
    "User-Agent": BROWSER_UA,
    "Accept": "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.9",
    "Accept-Encoding": "gzip, deflate, br",
    "Referer": "https://www.thingiverse.com/",
    "Sec-Ch-Ua": '"Chromium";v="120", "Google Chrome";v="120", "Not?A_Brand";v="8"',
    "Sec-Ch-Ua-Mobile": "?0",
    "Sec-Ch-Ua-Platform": '"macOS"',
    "Sec-Fetch-Dest": "image",
    "Sec-Fetch-Mode": "no-cors",
    "Sec-Fetch-Site": "same-site",
}


def _test_tv_cookies(cookies: dict[str, str]) -> bool:
    """Test if Thingiverse CDN cookies work."""
    try:
        r = httpx.get(
            "https://cdn.thingiverse.com/site/img/favicons/favicon-32x32.png",
            cookies=cookies, headers=_TV_CDN_HEADERS, timeout=10,
        )
        return r.status_code == 200
    except Exception:
        return False


def _load_thingiverse_cookies(workdir: Path) -> dict[str, str] | None:
    """Load cached Thingiverse Cloudflare cookies from disk."""
    cookie_path = workdir / THINGIVERSE_COOKIES_FILE
    if not cookie_path.exists():
        return None
    try:
        data = json.loads(cookie_path.read_text())
        cookies = data.get("cookies", data) if isinstance(data, dict) and "cookies" in data else data
        ua = data.get("user_agent", BROWSER_UA) if isinstance(data, dict) and "user_agent" in data else BROWSER_UA
        # Update the CDN headers to match the captured UA
        _TV_CDN_HEADERS["User-Agent"] = ua
        if _test_tv_cookies(cookies):
            log.info("  Thingiverse cookies: valid (loaded from cache)")
            return cookies
        log.info("  Thingiverse cookies: expired")
    except Exception:
        pass
    return None


def _capture_thingiverse_cookies(workdir: Path) -> dict[str, str] | None:
    """Open a visible browser to Thingiverse, wait for Cloudflare, capture cookies."""
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        log.warning("  Playwright not installed — cannot capture Thingiverse cookies")
        return None

    log.info("  Opening browser to pass Thingiverse Cloudflare challenge...")
    log.info("  (solve the CAPTCHA if prompted, or just wait)")

    try:
        with sync_playwright() as p:
            # Don't override user_agent — let Playwright use its default so
            # Cloudflare's TLS/UA fingerprint check is consistent
            browser = p.chromium.launch(headless=False)
            ctx = browser.new_context()
            page = ctx.new_page()

            # Capture the actual User-Agent the browser is using
            actual_ua = page.evaluate("() => navigator.userAgent")
            log.debug("  Browser UA: %s", actual_ua[:80])

            page.goto("https://www.thingiverse.com/thing:1015178", timeout=60000)

            # Wait until we get past Cloudflare
            for _ in range(120):
                title = page.title()
                if "just a moment" not in title.lower() and "checking" not in title.lower():
                    break
                page.wait_for_timeout(1000)
            else:
                log.warning("  Timed out waiting for Cloudflare challenge")
                browser.close()
                return None

            page.wait_for_timeout(2000)

            # Test a CDN download from within the browser context
            cdn_test = page.evaluate("""
                async () => {
                    const r = await fetch('https://cdn.thingiverse.com/site/img/favicons/favicon-32x32.png');
                    return {status: r.status, ok: r.ok};
                }
            """)
            log.info("  In-browser CDN test: %s", cdn_test)

            # Extract all cookies
            raw_cookies = ctx.cookies()
            cookies = {}
            for c in raw_cookies:
                if c["domain"].endswith("thingiverse.com") or c["domain"].endswith("cloudflare.com"):
                    cookies[c["name"]] = c["value"]

            browser.close()

            if "cf_clearance" in cookies:
                log.info("  Cloudflare cleared — captured %d cookies (cf_clearance present)", len(cookies))
            else:
                log.info("  Captured %d cookies (no cf_clearance — may not have been challenged)", len(cookies))

            # Update CDN headers to match the browser's actual UA
            _TV_CDN_HEADERS["User-Agent"] = actual_ua

            if _test_tv_cookies(cookies):
                log.info("  Cookie+UA test passed — CDN downloads will work")
                (workdir / THINGIVERSE_COOKIES_FILE).write_text(
                    json.dumps({"cookies": cookies, "user_agent": actual_ua}, indent=2)
                )
                return cookies

            log.warning("  Cookie test failed — Cloudflare may require matching TLS fingerprint")
            log.warning("  Saving cookies anyway in case they work for some requests")
            (workdir / THINGIVERSE_COOKIES_FILE).write_text(
                json.dumps({"cookies": cookies, "user_agent": actual_ua}, indent=2)
            )
            return cookies

    except Exception as e:
        log.warning("  Browser cookie capture failed: %s", str(e)[:100])
        return None


# ── Data Models ────────────────────────────────────────────────────────────


@dataclass
class LinkEntry:
    url: str
    page: int
    category: str = ""
    subcategory: str = ""
    title: str = ""
    description: str = ""
    thumbnail: str = ""


@dataclass
class VerifiedLink(LinkEntry):
    status: str = ""  # alive, dead, wayback, error
    final_url: str = ""
    wayback_url: str = ""
    extractor: str = ""
    http_status: int = 0
    error: str = ""


@dataclass
class CrawlResult:
    url: str
    status: str = ""  # completed, failed
    extractor: str = ""
    title: str = ""
    files: int = 0
    size_bytes: int = 0
    attempts: int = 0
    error: str = ""
    ts: str = ""


@dataclass
class ProjectMeta:
    url: str
    title: str = ""
    description: str = ""
    author: str = ""
    license: str = ""
    category: str = ""
    subcategory: str = ""
    source_domain: str = ""
    images: list[str] = field(default_factory=list)
    artifacts: list[dict] = field(default_factory=list)
    crawled_at: str = ""
    source_status: str = ""


# ── JSONL helpers ──────────────────────────────────────────────────────────


def _write_jsonl(path: Path, items: list) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w") as f:
        for item in items:
            obj = asdict(item) if hasattr(item, "__dataclass_fields__") else item
            f.write(json.dumps(obj, ensure_ascii=False) + "\n")


def _read_jsonl(path: Path) -> list[dict]:
    if not path.exists():
        return []
    items = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if line:
                items.append(json.loads(line))
    return items


def _append_jsonl(path: Path, item) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    obj = asdict(item) if hasattr(item, "__dataclass_fields__") else item
    with open(path, "a") as f:
        f.write(json.dumps(obj, ensure_ascii=False) + "\n")


# ── State Management ──────────────────────────────────────────────────────


@dataclass
class PipelineState:
    workdir: Path
    pdf_checksum: str = ""
    stage: str = ""  # extract, verify, crawl, package
    started_at: str = ""

    @property
    def state_file(self) -> Path:
        return self.workdir / "state.json"

    @property
    def links_file(self) -> Path:
        return self.workdir / "links.jsonl"

    @property
    def verified_file(self) -> Path:
        return self.workdir / "verified.jsonl"

    @property
    def progress_file(self) -> Path:
        return self.workdir / "crawl_progress.jsonl"

    @property
    def sites_dir(self) -> Path:
        return self.workdir / "sites"

    @property
    def output_dir(self) -> Path:
        return self.workdir / "output"

    @property
    def thumbnails_dir(self) -> Path:
        return self.workdir / "thumbnails"

    def save(self) -> None:
        self.workdir.mkdir(parents=True, exist_ok=True)
        self.state_file.write_text(json.dumps({
            "pdf_checksum": self.pdf_checksum,
            "stage": self.stage,
            "started_at": self.started_at,
        }))

    @classmethod
    def load(cls, workdir: Path) -> PipelineState:
        state = cls(workdir=workdir)
        if state.state_file.exists():
            data = json.loads(state.state_file.read_text())
            state.pdf_checksum = data.get("pdf_checksum", "")
            state.stage = data.get("stage", "")
            state.started_at = data.get("started_at", "")
        return state

    def invalidate_from(self, stage: str) -> None:
        """Invalidate a stage and everything downstream."""
        stages = ["extract", "verify", "crawl", "package"]
        idx = stages.index(stage)
        for s in stages[idx:]:
            if s == "extract" and self.links_file.exists():
                self.links_file.unlink()
            elif s == "verify" and self.verified_file.exists():
                self.verified_file.unlink()
            elif s == "crawl" and self.progress_file.exists():
                self.progress_file.unlink()
                import shutil
                if self.sites_dir.exists():
                    shutil.rmtree(self.sites_dir)
            elif s == "package":
                import shutil
                if self.output_dir.exists():
                    shutil.rmtree(self.output_dir)


# ── Stage 1: Extract ──────────────────────────────────────────────────────


def _slugify(text: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")
    return slug[:80] or "untitled"


def stage_extract(state: PipelineState, pdf_path: Path | None = None) -> list[LinkEntry]:
    """Parse the MIY PDF and extract all project links with context."""
    if state.links_file.exists():
        existing = _read_jsonl(state.links_file)
        log.info("Stage 1: Loaded %d links from cache", len(existing))
        return [LinkEntry(**e) for e in existing]

    log.info("Stage 1: Extracting links from PDF...")

    # Download PDF if not provided
    if pdf_path is None:
        pdf_path = state.workdir / "MIY.pdf"
        if not pdf_path.exists():
            log.info("  Downloading PDF from %s", PDF_URL)
            pdf_path.parent.mkdir(parents=True, exist_ok=True)
            with httpx.stream("GET", PDF_URL, follow_redirects=True, timeout=60) as r:
                r.raise_for_status()
                with open(pdf_path, "wb") as f:
                    for chunk in r.iter_bytes(65536):
                        f.write(chunk)

    # Checksum
    state.pdf_checksum = hashlib.sha256(pdf_path.read_bytes()).hexdigest()

    doc = fitz.open(str(pdf_path))
    log.info("  PDF has %d pages", len(doc))

    # Extract table of contents for category structure
    toc = doc.get_toc(simple=True)
    # Build page → category mapping from TOC
    page_categories: dict[int, tuple[str, str]] = {}
    current_cat = ""
    current_sub = ""
    for level, title, page_num in toc:
        if level == 1:
            current_cat = title.strip()
            current_sub = ""
        elif level == 2:
            current_sub = title.strip()
        page_categories[page_num] = (current_cat, current_sub)

    # Extract links with context
    links: list[LinkEntry] = []
    seen_urls: set[str] = set()

    for page_num in range(len(doc)):
        page = doc[page_num]
        page_no = page_num + 1

        # Find category for this page (use nearest preceding TOC entry)
        cat, sub = "", ""
        for p in range(page_no, 0, -1):
            if p in page_categories:
                cat, sub = page_categories[p]
                break

        for link in page.get_links():
            uri = link.get("uri", "")
            if not uri or not uri.startswith("http"):
                continue
            if uri in seen_urls:
                continue
            seen_urls.add(uri)

            # Get text near the link
            rect = link.get("from")
            text = ""
            if rect:
                text = page.get_text("text", clip=rect).strip()

            # Try to get surrounding context for title/description
            # The PDF has project descriptions as text blocks near links
            title = text.split("\n")[0][:120] if text else ""

            links.append(LinkEntry(
                url=uri,
                page=page_no,
                category=cat,
                subcategory=sub,
                title=title,
            ))

    # Also extract thumbnail images per page
    thumbnails_dir = state.thumbnails_dir
    thumbnails_dir.mkdir(parents=True, exist_ok=True)
    log.info("  Extracting thumbnails...")
    for page_num in range(len(doc)):
        page = doc[page_num]
        images = page.get_images(full=True)
        for img_idx, img_info in enumerate(images):
            xref = img_info[0]
            try:
                pix = fitz.Pixmap(doc, xref)
                if pix.n > 4:  # CMYK, convert
                    pix = fitz.Pixmap(fitz.csRGB, pix)
                thumb_path = thumbnails_dir / f"p{page_num + 1}_{img_idx:02d}.jpg"
                if not thumb_path.exists():
                    pix.save(str(thumb_path))
            except Exception:
                continue

    doc.close()

    # Assign thumbnails to links (closest image on same page)
    for link in links:
        page_thumbs = sorted(thumbnails_dir.glob(f"p{link.page}_*.jpg"))
        if page_thumbs:
            link.thumbnail = str(page_thumbs[0].relative_to(state.workdir))

    _write_jsonl(state.links_file, links)
    state.stage = "extract"
    state.save()
    log.info("  Extracted %d unique links across %d categories", len(links), len({l.category for l in links if l.category}))
    return links


# ── Stage 2: Verify ───────────────────────────────────────────────────────


EXTRACTOR_MAP: dict[str, str] = {
    "www.printables.com": "printables",
    "printables.com": "printables",
    "www.thingiverse.com": "thingiverse",
    "thingiverse.com": "thingiverse",
    "github.com": "github",
    "www.github.com": "github",
    "www.instructables.com": "instructables",
    "instructables.com": "instructables",
    "cults3d.com": "cults3d",
    "www.cults3d.com": "cults3d",
    "ana-white.com": "anawhite",
    "www.ana-white.com": "anawhite",
    "hackaday.io": "hackaday",
    "www.hackaday.io": "hackaday",
}


def _classify_extractor(url: str) -> str:
    parsed = urlparse(url)
    domain = parsed.netloc.lower()
    ext = EXTRACTOR_MAP.get(domain, "generic")
    # Thingiverse make: URLs are user prints, not things — use generic
    if ext == "thingiverse" and "/make:" in parsed.path:
        return "generic"
    # GitHub org/user pages without /repo — still use github extractor (handles orgs)
    return ext


async def _check_wayback(client: httpx.AsyncClient, url: str) -> str | None:
    """Check Wayback Machine for an archived version of a URL."""
    for attempt in range(3):
        try:
            r = await client.get(WAYBACK_API, params={"url": url}, timeout=20)
            if r.status_code == 200:
                data = r.json()
                snap = data.get("archived_snapshots", {}).get("closest", {})
                if snap.get("available"):
                    return snap["url"]
                return None
            if r.status_code == 503:
                await asyncio.sleep(2 * (attempt + 1))
                continue
            return None
        except Exception:
            await asyncio.sleep(1)
    return None


async def _verify_one(
    client: httpx.AsyncClient, sem: asyncio.Semaphore, entry: LinkEntry
) -> VerifiedLink:
    async with sem:
        verified = VerifiedLink(
            url=entry.url, page=entry.page, category=entry.category,
            subcategory=entry.subcategory, title=entry.title,
            description=entry.description, thumbnail=entry.thumbnail,
            extractor=_classify_extractor(entry.url),
        )

        try:
            r = await client.head(
                entry.url, follow_redirects=True, timeout=15, headers=HEADERS
            )
            verified.http_status = r.status_code
            verified.final_url = str(r.url)

            if 200 <= r.status_code < 400:
                verified.status = "alive"
                return verified
        except Exception:
            pass

        # Fallback: GET request
        try:
            r = await client.get(
                entry.url, follow_redirects=True, timeout=15, headers=HEADERS
            )
            verified.http_status = r.status_code
            verified.final_url = str(r.url)
            if 200 <= r.status_code < 400:
                verified.status = "alive"
                return verified
        except Exception as e:
            verified.error = str(e)[:200]

        # Dead — check Wayback Machine
        wayback = await _check_wayback(client, entry.url)
        if wayback:
            verified.status = "wayback"
            verified.wayback_url = wayback
        else:
            verified.status = "dead"

        await asyncio.sleep(VERIFY_DELAY)
        return verified


async def _run_verify(links: list[LinkEntry], existing: dict[str, dict]) -> list[VerifiedLink]:
    sem = asyncio.Semaphore(VERIFY_CONCURRENCY)
    results: list[VerifiedLink] = []

    # Keep already-verified entries
    for url, data in existing.items():
        results.append(VerifiedLink(**data))

    # Only verify new ones
    to_verify = [l for l in links if l.url not in existing]
    if not to_verify:
        return results

    log.info("  Verifying %d new links (%d already cached)...", len(to_verify), len(existing))
    async with httpx.AsyncClient() as client:
        tasks = [_verify_one(client, sem, entry) for entry in to_verify]
        for i, coro in enumerate(asyncio.as_completed(tasks)):
            result = await coro
            results.append(result)
            if (i + 1) % 50 == 0:
                log.info("    %d/%d verified...", i + 1, len(to_verify))

    return results


async def _retry_dead_wayback(existing: dict[str, dict]) -> list[VerifiedLink]:
    """Re-check Wayback Machine for entries currently marked 'dead'."""
    dead = [VerifiedLink(**v) for v in existing.values() if v.get("status") == "dead" and not v.get("wayback_url")]
    if not dead:
        log.info("  No dead links without Wayback URLs to retry")
        return [VerifiedLink(**v) for v in existing.values()]

    log.info("  Retrying Wayback for %d dead links...", len(dead))
    recovered = 0
    async with httpx.AsyncClient() as client:
        for v in dead:
            wayback = await _check_wayback(client, v.url)
            if wayback:
                v.status = "wayback"
                v.wayback_url = wayback
                existing[v.url] = asdict(v)
                recovered += 1
                log.info("    Recovered: %s → %s", v.url[:60], wayback[:60])
            await asyncio.sleep(0.5)

    log.info("  Wayback retry: %d/%d recovered", recovered, len(dead))
    return [VerifiedLink(**v) for v in existing.values()]


def stage_verify(
    state: PipelineState, links: list[LinkEntry], *, retry_dead: bool = False,
) -> list[VerifiedLink]:
    """Verify all links and classify by extractor type."""
    existing = {}
    if state.verified_file.exists():
        for entry in _read_jsonl(state.verified_file):
            existing[entry["url"]] = entry

    if retry_dead and existing:
        log.info("Stage 2: Retrying Wayback for dead links...")
        results = asyncio.run(_retry_dead_wayback(existing))
        _write_jsonl(state.verified_file, results)
        state.stage = "verify"
        state.save()
        by_status = {}
        for r in results:
            by_status.setdefault(r.status, []).append(r)
        for status, items in sorted(by_status.items()):
            log.info("  %s: %d", status, len(items))
        return results

    if len(existing) >= len(links):
        log.info("Stage 2: All %d links already verified", len(existing))
        return [VerifiedLink(**e) for e in existing.values()]

    log.info("Stage 2: Verifying links...")
    results = asyncio.run(_run_verify(links, existing))

    _write_jsonl(state.verified_file, results)
    state.stage = "verify"
    state.save()

    # Summary
    by_status = {}
    for r in results:
        by_status.setdefault(r.status, []).append(r)
    for status, items in sorted(by_status.items()):
        log.info("  %s: %d", status, len(items))

    return results


# ── Stage 3: Crawl — Extractors ───────────────────────────────────────────


def _download_file_sync(url: str, dest: Path, *, timeout: float = 60) -> None:
    """Download a file without a pre-existing client."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    with httpx.stream("GET", url, follow_redirects=True, timeout=timeout, headers=HEADERS) as r:
        r.raise_for_status()
        with open(dest, "wb") as f:
            for chunk in r.iter_bytes(65536):
                f.write(chunk)


def _download_file(
    client: httpx.Client, url: str, dest: Path, *,
    timeout: float = 120, headers: dict | None = None, cookies: dict | None = None,
) -> bool:
    """Download a file to dest. Returns True on success."""
    if dest.exists() and dest.stat().st_size > 0:
        return True
    dest.parent.mkdir(parents=True, exist_ok=True)
    try:
        kwargs: dict = dict(follow_redirects=True, timeout=timeout, headers=headers or HEADERS)
        if cookies:
            kwargs["cookies"] = cookies
        with client.stream("GET", url, **kwargs) as r:
            r.raise_for_status()
            with open(dest, "wb") as f:
                for chunk in r.iter_bytes(65536):
                    f.write(chunk)
        return True
    except Exception as e:
        log.warning("    Failed to download %s: %s", url[:80], e)
        dest.unlink(missing_ok=True)
        return False


def _save_meta(site_dir: Path, meta: ProjectMeta) -> None:
    meta_path = site_dir / "meta.json"
    meta_path.write_text(json.dumps(asdict(meta), ensure_ascii=False, indent=2))


_PROJECT_META_FIELDS = frozenset(ProjectMeta.__dataclass_fields__)


def _meta_from_dict(d: dict) -> ProjectMeta:
    """Construct a ProjectMeta from a raw dict, ignoring unknown keys."""
    return ProjectMeta(**{k: v for k, v in d.items() if k in _PROJECT_META_FIELDS})


def _patch_meta(proj_dir: Path, **fields) -> None:
    """Update specific fields in a project's meta.json without rewriting the whole object."""
    meta_path = proj_dir / "meta.json"
    try:
        meta_data = json.loads(meta_path.read_text())
        meta_data.update(fields)
        meta_path.write_text(json.dumps(meta_data, ensure_ascii=False, indent=2))
    except Exception:
        pass


# CSS selectors tried in order to find content paragraphs in raw HTML.
_CONTENT_SELECTORS = [
    "article p", "main p", ".step-body p", ".entry-content p",
    ".post-content p", ".content p", "#content p", "body p",
]


def _extract_desc_from_raw(site_dir: Path, current_desc: str) -> str | None:
    """Try to extract a better description from raw.html or README.md.

    Returns the improved description, or None if nothing better was found.
    """
    raw_path = site_dir / "raw.html"
    if raw_path.exists() and raw_path.stat().st_size > 500:
        try:
            raw_soup = BeautifulSoup(raw_path.read_text(), "html.parser")
            for sel in _CONTENT_SELECTORS:
                paras = raw_soup.select(sel)
                paras = [p for p in paras if len(p.get_text(strip=True)) > 30]
                if paras:
                    candidate = " ".join(p.get_text(strip=True) for p in paras[:3])
                    if len(candidate) > len(current_desc):
                        return unescape(candidate[:2000])
                    break
        except Exception:
            pass

    readme_path = site_dir / "README.md"
    if readme_path.exists():
        try:
            lines = [
                l.strip() for l in readme_path.read_text().split("\n")
                if l.strip() and not l.startswith("#") and not l.startswith("!")
                and not l.startswith("[") and len(l.strip()) > 20
            ]
            if lines:
                candidate = " ".join(lines[:3])[:2000]
                if len(candidate) > len(current_desc):
                    return candidate
        except Exception:
            pass
    return None


def _sanitize_filename(name: str) -> str:
    """Make a filename safe for all platforms."""
    name = re.sub(r'[<>:"/\\|?*]', "_", name)
    name = name.strip(". ")
    return name[:200] or "file"


# ── Composable Strategies ──────────────────────────────────────────────


@dataclass
class FetchResult:
    html: str
    url: str
    status_code: int = 200
    cookies: dict[str, str] = field(default_factory=dict)


class FetchStrategy:
    name: str = "base"
    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        raise NotImplementedError


class MetadataStrategy:
    name: str = "base"
    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        raise NotImplementedError


class ImageStrategy:
    name: str = "base"
    def collect(self, soup: BeautifulSoup, url: str, images_dir: Path,
                client: httpx.Client, existing: list[str], **kwargs) -> list[str]:
        raise NotImplementedError


class ArtifactStrategy:
    name: str = "base"
    def collect(self, soup: BeautifulSoup, url: str, artifacts_dir: Path,
                client: httpx.Client, existing: list[dict], **kwargs) -> list[dict]:
        raise NotImplementedError


# ── Fetch Strategy Implementations ────────────────────────────────────


class HttpFetcher(FetchStrategy):
    name = "http"
    def fetch(self, url, client, **kwargs):
        r = client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
        r.raise_for_status()
        return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)


class SslBypassFetcher(FetchStrategy):
    name = "ssl_bypass"
    def fetch(self, url, client, **kwargs):
        with httpx.Client(verify=False, timeout=30) as insecure:
            r = insecure.get(url, headers=HEADERS, follow_redirects=True)
            r.raise_for_status()
            log.info("    SSL bypass OK for %s", url[:60])
            return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)


class PlaywrightFetcher(FetchStrategy):
    name = "playwright"
    def fetch(self, url, client, **kwargs):
        if not _has_playwright():
            raise RuntimeError("Playwright not available")
        from playwright.sync_api import sync_playwright
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            ctx = browser.new_context(viewport={"width": 1280, "height": 800}, ignore_https_errors=True)
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
    def fetch(self, url, client, **kwargs):
        wayback_url = kwargs.get("wayback_url", "")
        if not wayback_url:
            raise RuntimeError("No wayback URL provided")
        r = client.get(wayback_url, headers=HEADERS, timeout=30, follow_redirects=True)
        r.raise_for_status()
        return FetchResult(html=r.text, url=str(r.url), status_code=r.status_code)


class FetchChain:
    def __init__(self, fetchers: list[FetchStrategy]):
        self.fetchers = fetchers

    def fetch(self, url: str, client: httpx.Client, **kwargs) -> FetchResult:
        last_error = None
        for fetcher in self.fetchers:
            try:
                return fetcher.fetch(url, client, **kwargs)
            except Exception as e:
                last_error = e
                continue
        raise last_error or RuntimeError(f"All fetchers failed for {url}")


# ── Metadata Strategy Implementations ─────────────────────────────────


class JsonLdMetadata(MetadataStrategy):
    name = "jsonld"
    def __init__(self, type_filter: str | None = None):
        self.type_filter = type_filter

    def extract(self, soup, url, meta):
        for script in soup.find_all("script", type="application/ld+json"):
            try:
                data = json.loads(script.string)
                if not isinstance(data, dict):
                    continue
                if self.type_filter and data.get("@type") != self.type_filter:
                    continue
                meta.title = meta.title or data.get("name", "")
                meta.description = meta.description or (data.get("description") or "")[:2000]
                for path in [data.get("creator", {}), data.get("brand", {}),
                             (data.get("mainEntityOfPage") or {}).get("author", {})]:
                    if isinstance(path, dict) and path.get("name"):
                        meta.author = meta.author or path["name"]
                        break
                meta.license = meta.license or (data.get("mainEntityOfPage") or {}).get("license", "") or data.get("license", "")
                break
            except (json.JSONDecodeError, TypeError):
                continue


class OpenGraphMetadata(MetadataStrategy):
    name = "opengraph"
    def extract(self, soup, url, meta):
        def og(prop):
            tag = soup.find("meta", property=f"og:{prop}")
            return tag.get("content", "") if tag else ""
        meta.title = meta.title or og("title")
        meta.description = meta.description or og("description")[:2000]
        meta.author = meta.author or og("site_name")


class HtmlMetadata(MetadataStrategy):
    name = "html"
    def extract(self, soup, url, meta):
        if not meta.title:
            t = soup.find("title")
            if t:
                meta.title = t.get_text().strip()[:200]
        if not meta.description:
            d = soup.find("meta", attrs={"name": "description"})
            if d:
                meta.description = d.get("content", "")[:2000]
        if not meta.author:
            a = soup.find("meta", attrs={"name": "author"})
            if a:
                meta.author = a.get("content", "")


# ── Image & Artifact Strategy Implementations ─────────────────────────


class ImgTagImages(ImageStrategy):
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
            img_domain = urlparse(src).netloc.replace("www.", "")
            if self.domain_filter and self.domain_filter not in img_domain:
                continue
            elif not self.domain_filter:
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
    name = "og_image"
    def collect(self, soup, url, images_dir, client, existing, **kwargs):
        if existing:
            return []
        og_img = soup.find("meta", property="og:image")
        if og_img and og_img.get("content", "").startswith("http"):
            if _download_file(client, og_img["content"], images_dir / "og.jpg"):
                return ["images/og.jpg"]
        return []


class LinkArtifacts(ArtifactStrategy):
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
                new_artifacts.append(_artifact_entry(fname, artifacts_dir))
            time.sleep(0.3)
        return new_artifacts


# ── SiteScraper Orchestrator ───────────────────────────────────────────────


@dataclass
class SiteConfig:
    """Declarative configuration for a site-specific scraper."""
    name: str
    domain: str
    fetch_chain: list[FetchStrategy]
    metadata_strategies: list[MetadataStrategy]
    image_strategies: list[ImageStrategy]
    artifact_strategies: list[ArtifactStrategy]
    pre_fetch: object | None = None    # (verified, client, **kw) -> dict
    post_parse: object | None = None   # (soup, meta, site_dir, client, **kw) -> None
    rate_limit: float = CRAWL_DELAY_PER_DOMAIN


class SiteScraper:
    """Orchestrator: runs a configured extraction pipeline for one URL."""

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

        # Pre-fetch hook (prepare API headers, cookies, tokens)
        extra_kwargs = {}
        if self.config.pre_fetch:
            extra_kwargs = self.config.pre_fetch(verified, self.client) or {}

        # Fetch HTML via chain (HTTP → SSL bypass → Playwright → Wayback)
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

        # Extract metadata (each strategy fills what it can, first wins per field)
        for strategy in self.config.metadata_strategies:
            strategy.extract(soup, url, meta)

        # Post-parse hook (Printables GraphQL, Thingiverse API, GitHub ZIP, etc.)
        if self.config.post_parse:
            try:
                self.config.post_parse(soup, meta, site_dir, self.client, **extra_kwargs)
            except Exception as e:
                log.debug("    post_parse hook error: %s", e)

        # Collect images
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        for strategy in self.config.image_strategies:
            new_imgs = strategy.collect(soup, url, images_dir, self.client, meta.images, **extra_kwargs)
            meta.images.extend(new_imgs)
            total_size += sum(
                (images_dir / Path(p).name).stat().st_size
                for p in new_imgs if (images_dir / Path(p).name).exists()
            )

        # Collect artifacts
        artifacts_dir = site_dir / "artifacts"
        artifacts_dir.mkdir(exist_ok=True)
        for strategy in self.config.artifact_strategies:
            new_arts = strategy.collect(soup, url, artifacts_dir, self.client, meta.artifacts, **extra_kwargs)
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

    def post_extract(self, site_dir: Path, meta: ProjectMeta, url: str) -> None:
        """Quality check: Playwright fallback for missing images, description enrichment."""
        quality = _quality_score(site_dir)
        if "no_images" in quality["issues"] and _has_playwright():
            _playwright_recrawl(url, site_dir, meta)
            _save_meta(site_dir, meta)

        better = _extract_desc_from_raw(site_dir, meta.description)
        if better:
            meta.description = better
            _save_meta(site_dir, meta)


# ── Site Configurations ──────────────────────────────────────────────────

# Module-level config populated by stage_crawl before creating scrapers
_RUNTIME_CONFIG: dict = {}

_DEFAULT_FETCH_CHAIN = [HttpFetcher(), SslBypassFetcher(), PlaywrightFetcher()]


def _artifact_entry(fname: str, artifacts_dir: Path) -> dict:
    """Build a standard artifact metadata dict."""
    fsize = (artifacts_dir / fname).stat().st_size if (artifacts_dir / fname).exists() else 0
    return {
        "filename": f"artifacts/{fname}",
        "type": Path(fname).suffix.lstrip("."),
        "size_bytes": fsize,
    }


# ── Printables ───────────────────────────────────────────────────────────


def _printables_extract_model_id(url: str) -> str | None:
    m = re.search(r"/model/(\d+)", url)
    return m.group(1) if m else None


def _printables_fetch_graphql(client: httpx.Client, model_id: str) -> dict | None:
    """Fetch rich metadata via Printables GraphQL API."""
    query = """
    query PrintProfile($id: ID!) {
        print(id: $id) {
            id
            name
            slug
            description
            license { name }
            category { name }
            user { publicUsername }
            images { filePath }
            stls { id name fileSize privateFile }
            gcodes { id name fileSize privateFile }
            otherFiles { id name fileSize privateFile }
            pdfFilePath
        }
    }
    """
    try:
        r = client.post(
            "https://api.printables.com/graphql/",
            json={"query": query, "variables": {"id": model_id}},
            headers={**HEADERS, "Content-Type": "application/json"},
            timeout=30,
        )
        if r.status_code == 200:
            data = r.json()
            if "errors" in data:
                log.debug("  Printables GraphQL errors: %s", data["errors"])
                return None
            return data.get("data", {}).get("print")
    except Exception as e:
        log.debug("  Printables GraphQL failed for %s: %s", model_id, e)
    return None


def _printables_get_download_links(
    client: httpx.Client, model_id: str, file_ids: list[dict], auth_token: str = "",
) -> dict[str, str]:
    """Get signed download URLs via GraphQL mutation."""
    by_type: dict[str, list[str]] = {}
    for f in file_ids:
        ft = f.get("type", "stl")
        by_type.setdefault(ft, []).append(f["id"])

    files_arg = [{"fileType": ft, "ids": ids} for ft, ids in by_type.items()]

    mutation = """
    mutation GetDownloadLink($printId: ID!, $files: [DownloadFileInput!]!, $source: DownloadSourceEnum!) {
        getDownloadLink(printId: $printId, files: $files, source: $source) {
            ok
            output {
                files { id link ttl fileType }
            }
            errors { field messages }
        }
    }
    """
    headers = {
        **HEADERS,
        "Content-Type": "application/json",
        "Origin": "https://www.printables.com",
        "Referer": "https://www.printables.com/",
    }
    if auth_token:
        headers["Authorization"] = f"Bearer {auth_token}"

    result_map: dict[str, str] = {}
    try:
        r = client.post(
            "https://api.printables.com/graphql/",
            json={
                "query": mutation,
                "variables": {
                    "printId": model_id,
                    "files": files_arg,
                    "source": "model_detail",
                },
            },
            headers=headers,
            timeout=30,
        )
        if r.status_code == 200:
            data = r.json()
            dl = data.get("data", {}).get("getDownloadLink", {})
            if dl.get("ok") and dl.get("output"):
                for f in dl["output"].get("files", []):
                    if f.get("link"):
                        result_map[f["id"]] = f["link"]
            elif dl.get("errors"):
                log.debug("  Printables download errors: %s", dl["errors"])
    except Exception as e:
        log.debug("  Printables download link failed: %s", e)
    return result_map


def _printables_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Printables post-parse: GraphQL metadata, images, PDF, signed STL downloads."""
    url = meta.url
    model_id = _printables_extract_model_id(url)
    if not model_id:
        return

    auth_token = _RUNTIME_CONFIG.get("printables_token", "")
    gql_data = _printables_fetch_graphql(client, model_id)
    time.sleep(CRAWL_DELAY_PER_DOMAIN)

    if gql_data:
        meta.title = gql_data.get("name", "") or meta.title
        meta.description = (gql_data.get("description") or meta.description or "")[:2000]
        meta.author = (gql_data.get("user") or {}).get("publicUsername", "") or meta.author
        lic = gql_data.get("license") or {}
        meta.license = lic.get("name", "") or meta.license

        # Download images (publicly accessible)
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        for img in (gql_data.get("images") or []):
            img_path = img.get("filePath", "")
            if not img_path:
                continue
            img_url = f"https://media.printables.com/{img_path}"
            ext = Path(img_path).suffix or ".jpg"
            fname = _sanitize_filename(Path(img_path).stem) + ext
            if _download_file(client, img_url, images_dir / fname):
                meta.images.append(f"images/{fname}")
            time.sleep(0.3)

        # Download PDF description if available
        artifacts_dir = site_dir / "artifacts"
        artifacts_dir.mkdir(exist_ok=True)
        pdf_path = gql_data.get("pdfFilePath")
        if pdf_path:
            pdf_url = f"https://files.printables.com/{pdf_path}"
            if _download_file(client, pdf_url, artifacts_dir / "description.pdf"):
                fsize = (artifacts_dir / "description.pdf").stat().st_size
                meta.artifacts.append({
                    "filename": "artifacts/description.pdf",
                    "type": "pdf",
                    "size_bytes": fsize,
                })
            time.sleep(0.3)

        # Download STLs and other files via signed URLs
        stls = gql_data.get("stls") or []
        gcodes = gql_data.get("gcodes") or []
        other = gql_data.get("otherFiles") or []
        all_files = [
            *[{**f, "_type": "stl"} for f in stls],
            *[{**f, "_type": "gcode"} for f in gcodes],
            *[{**f, "_type": "other"} for f in other],
        ]
        if all_files:
            file_ids = [{"id": f["id"], "type": f["_type"]} for f in all_files]
            dl_urls = _printables_get_download_links(client, model_id, file_ids, auth_token)
            time.sleep(CRAWL_DELAY_PER_DOMAIN)

            for f_info in all_files:
                fid = f_info["id"]
                dl_url = dl_urls.get(fid)
                fname = _sanitize_filename(f_info.get("name", f"file_{fid}"))
                if dl_url:
                    if _download_file(client, dl_url, artifacts_dir / fname):
                        meta.artifacts.append(_artifact_entry(fname, artifacts_dir))
                    time.sleep(0.5)
                else:
                    meta.artifacts.append({
                        "filename": fname,
                        "type": Path(fname).suffix.lstrip(".") or f_info["_type"],
                        "size_bytes": f_info.get("fileSize", 0),
                        "download_failed": True,
                    })

    # If no GraphQL data, fallback title
    if not meta.title:
        meta.title = f"Printables Model {model_id}"


PRINTABLES_CONFIG = SiteConfig(
    name="printables",
    domain="printables.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[JsonLdMetadata(), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # Images handled in post_parse via GraphQL
    artifact_strategies=[],
    post_parse=_printables_post_parse,
)


# ── Thingiverse ──────────────────────────────────────────────────────────


def _thingiverse_extract_thing_id(url: str) -> str | None:
    m = re.search(r"thing:(\d+)", url)
    return m.group(1) if m else None


def _thingiverse_pre_fetch(verified: VerifiedLink, client: httpx.Client, **kw) -> dict:
    return {
        "api_token": _RUNTIME_CONFIG.get("thingiverse_token", ""),
        "cookies": _RUNTIME_CONFIG.get("thingiverse_cookies"),
    }


def _thingiverse_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Thingiverse post-parse: JSON-LD metadata, API enrichment, images, STL files."""
    url = meta.url
    thing_id = _thingiverse_extract_thing_id(url)
    if not thing_id:
        return

    api_token = kw.get("api_token", "")
    cdn_cookies = kw.get("cookies") or {}

    # Extract JSON-LD for metadata (already partially done by JsonLdMetadata("Product"),
    # but we need the thumbnail URL and Thingiverse-specific fallback)
    jsonld = None
    for script in soup.find_all("script", type="application/ld+json"):
        try:
            data = json.loads(script.string)
            if isinstance(data, dict) and data.get("@type") == "Product":
                jsonld = data
                break
        except (json.JSONDecodeError, TypeError):
            continue

    thumb_url = None
    if jsonld:
        # Enrich with JSON-LD fields not captured by generic JsonLdMetadata
        meop = jsonld.get("mainEntityOfPage") or {}
        author_obj = meop.get("author") or {}
        if author_obj.get("name"):
            meta.author = author_obj["name"]
        meta.license = meta.license or meop.get("license", "")
        img_obj = jsonld.get("image") or jsonld.get("thumbnailUrl")
        if isinstance(img_obj, dict):
            thumb_url = img_obj.get("url", "")
        elif isinstance(img_obj, str):
            thumb_url = img_obj
    else:
        # Fallback: parse title for "Name by Author - Thingiverse"
        title_tag = soup.find("title")
        if title_tag:
            t = title_tag.get_text()
            meta.title = meta.title or re.sub(r"\s*[-\u2013]\s*Thingiverse\s*$", "", t).strip()
            m = re.match(r"(.+?)\s+by\s+(.+)", meta.title)
            if m:
                meta.title, meta.author = m.group(1), m.group(2)

    images_dir = site_dir / "images"
    images_dir.mkdir(exist_ok=True)

    if api_token:
        api_base = "https://api.thingiverse.com"
        api_headers = {**HEADERS, "Authorization": f"Bearer {api_token}"}
        cdn_headers = {**_TV_CDN_HEADERS}
        cdn_auth_headers = {**cdn_headers, "Authorization": f"Bearer {api_token}"}

        # API metadata (richer than JSON-LD)
        try:
            r = client.get(
                f"{api_base}/things/{thing_id}", headers=api_headers, timeout=30,
            )
            if r.status_code == 200:
                api_data = r.json()
                meta.title = meta.title or api_data.get("name", "")
                if not meta.description or len(meta.description) < 50:
                    meta.description = (api_data.get("description") or "")[:2000]
                creator = api_data.get("creator") or {}
                meta.author = meta.author or creator.get("name", "")
                meta.license = meta.license or api_data.get("license", "")
            time.sleep(0.3)
        except Exception:
            pass

        # API images
        try:
            r = client.get(
                f"{api_base}/things/{thing_id}/images",
                headers=api_headers, timeout=30,
            )
            if r.status_code == 200:
                for i, img_data in enumerate(r.json()[:12]):
                    sizes = img_data.get("sizes", [])
                    large = [s for s in sizes if s.get("type") == "display" and s.get("size") == "large"]
                    medium = [s for s in sizes if s.get("type") == "display" and s.get("size") == "medium"]
                    url_entry = large[0] if large else (medium[0] if medium else (sizes[0] if sizes else None))
                    if not url_entry:
                        continue
                    img_url = url_entry.get("url", "")
                    if not img_url.startswith("http"):
                        continue
                    fname = f"img_{i:02d}.jpg"
                    if _download_file(client, img_url, images_dir / fname, headers=cdn_headers, cookies=cdn_cookies or None):
                        meta.images.append(f"images/{fname}")
                    time.sleep(0.3)
            time.sleep(CRAWL_DELAY_PER_DOMAIN)
        except Exception as e:
            log.debug("  Thingiverse images API error: %s", e)

        # API files (STL downloads)
        try:
            r = client.get(
                f"{api_base}/things/{thing_id}/files",
                headers=api_headers, timeout=30,
            )
            if r.status_code == 200:
                artifacts_dir = site_dir / "artifacts"
                artifacts_dir.mkdir(exist_ok=True)
                for f_info in r.json():
                    dl_url = f_info.get("download_url") or f_info.get("direct_url") or f_info.get("public_url")
                    if not dl_url:
                        continue
                    fname = _sanitize_filename(f_info.get("name", "file"))
                    if _download_file(client, dl_url, artifacts_dir / fname, headers=cdn_auth_headers, cookies=cdn_cookies or None):
                        meta.artifacts.append(_artifact_entry(fname, artifacts_dir))
                    time.sleep(0.5)
            elif r.status_code == 401:
                log.warning("  Thingiverse API returned 401 — token may be invalid")
            time.sleep(CRAWL_DELAY_PER_DOMAIN)
        except Exception as e:
            log.debug("  Thingiverse files API error: %s", e)

    else:
        # No token: try CDN image URLs from JSON-LD / HTML (often blocked)
        if thumb_url and thumb_url.startswith("http"):
            if _download_file(client, thumb_url, images_dir / "thumb.jpg"):
                meta.images.append("images/thumb.jpg")
            time.sleep(0.3)
        if not meta.images:
            raw_path = site_dir / "raw.html"
            html = raw_path.read_text() if raw_path.exists() else ""
            cdn_urls = re.findall(r'https?://cdn\.thingiverse\.com/[^"\'<>\s]+\.(?:jpg|png|jpeg)', html)
            for i, img_url in enumerate(dict.fromkeys(cdn_urls)):
                if i >= 5:
                    break
                fname = f"img_{i:02d}.jpg"
                if _download_file(client, img_url, images_dir / fname):
                    meta.images.append(f"images/{fname}")
                time.sleep(0.2)

    meta.title = meta.title or f"Thingiverse Thing {thing_id}"


THINGIVERSE_CONFIG = SiteConfig(
    name="thingiverse",
    domain="thingiverse.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[JsonLdMetadata(type_filter="Product"), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # Images handled in post_parse via API
    artifact_strategies=[],
    pre_fetch=_thingiverse_pre_fetch,
    post_parse=_thingiverse_post_parse,
    rate_limit=2.0,
)


# ── GitHub ───────────────────────────────────────────────────────────────


def _github_parse_repo(url: str) -> tuple[str, str] | None:
    parsed = urlparse(url)
    parts = parsed.path.strip("/").split("/")
    if len(parts) >= 2:
        return parts[0], parts[1]
    return None


def _github_is_org_url(url: str) -> str | None:
    """Check if URL is a GitHub org/user (single path segment)."""
    parsed = urlparse(url)
    parts = [p for p in parsed.path.strip("/").split("/") if p]
    if len(parts) == 1:
        return parts[0]
    return None


def _github_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """GitHub post-parse: API metadata, repo ZIP, README, OG image."""
    url = meta.url
    repo_info = _github_parse_repo(url)

    # Handle org/user URLs
    org_name = _github_is_org_url(url)
    if org_name and not repo_info:
        try:
            r = client.get(
                f"https://api.github.com/orgs/{org_name}/repos?sort=stars&per_page=5",
                headers={**HEADERS, "Accept": "application/vnd.github.v3+json"},
                timeout=30,
            )
            if r.status_code == 200:
                repos = r.json()
                if repos:
                    top = repos[0]
                    repo_info = (org_name, top["name"])
                    log.info("    GitHub org %s → using top repo: %s", org_name, top["name"])
            if not repo_info:
                r = client.get(
                    f"https://api.github.com/users/{org_name}/repos?sort=stars&per_page=5",
                    headers={**HEADERS, "Accept": "application/vnd.github.v3+json"},
                    timeout=30,
                )
                if r.status_code == 200:
                    repos = r.json()
                    if repos:
                        top = repos[0]
                        repo_info = (org_name, top["name"])
                        log.info("    GitHub user %s → using top repo: %s", org_name, top["name"])
        except Exception as e:
            log.debug("    GitHub org/user lookup failed: %s", e)

    if not repo_info:
        return

    owner, repo = repo_info

    # Fetch repo metadata via API
    default_branch = "main"
    try:
        r = client.get(
            f"https://api.github.com/repos/{owner}/{repo}",
            headers={**HEADERS, "Accept": "application/vnd.github.v3+json"},
            timeout=30,
        )
        if r.status_code == 200:
            repo_data = r.json()
            meta.title = repo_data.get("name", "") or meta.title
            meta.description = (repo_data.get("description") or meta.description or "")[:2000]
            meta.author = (repo_data.get("owner") or {}).get("login", "") or meta.author
            lic = repo_data.get("license") or {}
            meta.license = lic.get("spdx_id", "") or meta.license
            default_branch = repo_data.get("default_branch", "main")
        else:
            meta.title = meta.title or repo
            meta.author = meta.author or owner
        time.sleep(0.5)
    except Exception:
        meta.title = meta.title or repo
        meta.author = meta.author or owner

    # Download repo as ZIP
    artifacts_dir = site_dir / "artifacts"
    artifacts_dir.mkdir(exist_ok=True)
    zip_url = f"https://github.com/{owner}/{repo}/archive/refs/heads/{default_branch}.zip"
    zip_path = artifacts_dir / f"{repo}-{default_branch}.zip"

    if _download_file(client, zip_url, zip_path):
        fsize = zip_path.stat().st_size
        meta.artifacts.append({
            "filename": f"artifacts/{zip_path.name}",
            "type": "zip",
            "size_bytes": fsize,
        })
    time.sleep(0.5)

    # Fetch README for display
    for readme_name in ("README.md", "readme.md", "README.rst", "README"):
        try:
            r = client.get(
                f"https://raw.githubusercontent.com/{owner}/{repo}/{default_branch}/{readme_name}",
                headers=HEADERS, timeout=15,
            )
            if r.status_code == 200:
                (site_dir / "README.md").write_text(r.text)
                break
            time.sleep(0.2)
        except Exception:
            continue

    # Download OG image
    images_dir = site_dir / "images"
    images_dir.mkdir(exist_ok=True)
    og_img = soup.find("meta", property="og:image")
    if og_img:
        img_url = og_img.get("content", "")
        if img_url:
            if _download_file(client, img_url, images_dir / "social.png"):
                meta.images.append("images/social.png")

    meta.title = meta.title or f"{owner}/{repo}"


GITHUB_CONFIG = SiteConfig(
    name="github",
    domain="github.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # OG image handled in post_parse
    artifact_strategies=[],  # ZIP handled in post_parse
    post_parse=_github_post_parse,
)


# ── Instructables ────────────────────────────────────────────────────────


def _instructables_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Instructables post-parse: CDN images, step extraction, file attachments."""
    url = meta.url

    # Author
    author_tag = soup.find("a", class_="member-header-display-name") or soup.find("a", attrs={"rel": "author"})
    if author_tag:
        meta.author = meta.author or author_tag.get_text().strip()

    # Clean up title
    if meta.title:
        meta.title = meta.title.replace(" : ", " - ").strip()

    # Download images from content.instructables.com CDN
    images_dir = site_dir / "images"
    images_dir.mkdir(exist_ok=True)
    img_count = 0
    seen_urls: set[str] = set()
    raw_path = site_dir / "raw.html"
    html = raw_path.read_text() if raw_path.exists() else ""
    img_urls = re.findall(
        r'https?://content\.instructables\.com/[A-Z0-9/]+\.[a-z]+\?[^"\'&\s<>]+',
        html,
    )
    for src in img_urls:
        if img_count >= 20:
            break
        src = re.sub(r'&amp;', '&', src)
        base_path = src.split("?")[0]
        if base_path in seen_urls:
            continue
        seen_urls.add(base_path)
        if "height=620&width=620" in src or "width=320" in src:
            continue
        ext = Path(urlparse(base_path).path).suffix or ".jpg"
        if ext == ".webp":
            ext = ".jpg"
        fname = f"img_{img_count:02d}{ext}"
        dl_url = base_path + "?frame=1&width=1024"
        if _download_file(client, dl_url, images_dir / fname):
            meta.images.append(f"images/{fname}")
            img_count += 1
        time.sleep(0.2)

    # Fallback OG image
    if not meta.images:
        og_img = soup.find("meta", property="og:image")
        if og_img and og_img.get("content", "").startswith("http"):
            if _download_file(client, og_img["content"], images_dir / "og.jpg"):
                meta.images.append("images/og.jpg")

    # Extract steps
    steps = []
    step_elements = soup.find_all("div", class_="step")
    if not step_elements:
        step_elements = soup.find_all("section", class_="step")
    for step in step_elements:
        step_title_el = step.find(["h2", "h3"])
        step_title = step_title_el.get_text().strip() if step_title_el else ""
        step_body = step.find("div", class_="step-body")
        step_text = step_body.get_text().strip() if step_body else step.get_text().strip()
        steps.append({"title": step_title, "text": step_text[:2000]})

    # Download file attachments
    artifacts_dir = site_dir / "artifacts"
    artifacts_dir.mkdir(exist_ok=True)
    for link in soup.find_all("a", href=True):
        href = link["href"]
        if not href.startswith("http"):
            href = urljoin(url, href)
        parsed = urlparse(href)
        ext = Path(parsed.path).suffix.lower()
        if ext in ARTIFACT_EXTS:
            fname = _sanitize_filename(Path(parsed.path).name)
            if _download_file(client, href, artifacts_dir / fname):
                meta.artifacts.append(_artifact_entry(fname, artifacts_dir))
            time.sleep(0.3)

    # Save steps
    if steps:
        (site_dir / "steps.json").write_text(json.dumps(steps, ensure_ascii=False, indent=2))

    meta.title = meta.title or "Instructables Project"


INSTRUCTABLES_CONFIG = SiteConfig(
    name="instructables",
    domain="instructables.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # Images handled in post_parse (CDN regex)
    artifact_strategies=[],
    post_parse=_instructables_post_parse,
)


# ── Cults3D ──────────────────────────────────────────────────────────────


class Cults3DJsonLdMetadata(MetadataStrategy):
    """Cults3D-specific JSON-LD extraction for MediaObject type with license parsing."""
    name = "cults3d_jsonld"

    def extract(self, soup: BeautifulSoup, url: str, meta: ProjectMeta) -> None:
        for script in soup.find_all("script", type="application/ld+json"):
            try:
                data = json.loads(script.string)
                if isinstance(data, dict) and data.get("@type") in ("MediaObject", "3DModel", "Product"):
                    meta.title = meta.title or data.get("name", "")
                    meta.description = meta.description or (data.get("description") or "")[:2000]
                    creator = data.get("creator") or {}
                    meta.author = meta.author or creator.get("name", "")
                    license_url = data.get("license", "")
                    if "creativecommons.org" in license_url:
                        m = re.search(r"/licenses/([^/]+)/", license_url)
                        meta.license = f"CC {m.group(1).upper()}" if m else license_url
                    else:
                        meta.license = meta.license or license_url
                    break
            except (json.JSONDecodeError, TypeError):
                continue


class Cults3DImages(ImageStrategy):
    """Cults3D image extraction: data-src lazy images + source data-srcset."""
    name = "cults3d_images"

    def collect(self, soup, url, images_dir, client, existing, **kwargs):
        new_images = []
        img_count = len(existing)
        seen_bases: set[str] = set()

        # Collect all image sources: eager src + lazy data-src
        for img in soup.find_all("img"):
            src = img.get("data-src") or img.get("src") or ""
            if not src.startswith("http") or not any(d in src for d in ("images.cults3d.com", "static.cults3d.com", "files.cults3d.com")):
                continue
            base = src.split("?")[0]
            if base in seen_bases:
                continue
            seen_bases.add(base)
            if img_count >= 12:
                break
            fname = f"img_{img_count:02d}.jpg"
            dl_url = base + "?width=1024"
            if _download_file(client, dl_url, images_dir / fname):
                new_images.append(f"images/{fname}")
                img_count += 1
            time.sleep(0.3)

        # Also try <source> tags with data-srcset (WebP gallery)
        for source in soup.find_all("source", attrs={"data-srcset": True}):
            if img_count >= 12:
                break
            srcset = source["data-srcset"]
            src = srcset.split(",")[0].strip().split(" ")[0]
            if not src.startswith("http"):
                continue
            base = src.split("?")[0]
            if base in seen_bases:
                continue
            seen_bases.add(base)
            fname = f"img_{img_count:02d}.jpg"
            dl_url = base + "?width=1024"
            if _download_file(client, dl_url, images_dir / fname):
                new_images.append(f"images/{fname}")
                img_count += 1
            time.sleep(0.3)

        return new_images


def _cults3d_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Cults3D post-parse: extract file names from information-table (auth required for downloads)."""
    info_table = soup.find("table", class_="information-table")
    if info_table:
        for row in info_table.find_all("tr"):
            cells = row.find_all(["th", "td"])
            if len(cells) >= 2:
                header = cells[0].get_text(strip=True).lower()
                if "file" in header or "fichier" in header:
                    file_names = cells[1].get_text(strip=True)
                    for fname in re.split(r'\s+', file_names):
                        if any(fname.lower().endswith(ext) for ext in [".stl", ".3mf", ".step", ".obj"]):
                            meta.artifacts.append({
                                "filename": f"artifacts/{fname}",
                                "type": Path(fname).suffix.lstrip("."),
                                "size_bytes": 0,
                                "download_requires_auth": True,
                            })

    meta.title = meta.title or "Cults3D Project"


CULTS3D_CONFIG = SiteConfig(
    name="cults3d",
    domain="cults3d.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[Cults3DJsonLdMetadata(), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[Cults3DImages()],
    artifact_strategies=[],  # File names extracted in post_parse (downloads require auth)
    post_parse=_cults3d_post_parse,
    rate_limit=2.0,
)


# ── Ana-White ────────────────────────────────────────────────────────────


def _anawhite_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Ana-White post-parse: images from entry-content, author, PDF plan links."""
    # Author from entry meta
    author_el = soup.find("span", class_="entry-author") or soup.find("a", attrs={"rel": "author"})
    if author_el:
        meta.author = meta.author or author_el.get_text(strip=True)

    # Images from main content area
    content = soup.find("div", class_="entry-content") or soup.find("article")
    images_dir = site_dir / "images"
    images_dir.mkdir(exist_ok=True)
    img_count = 0
    seen: set[str] = set()
    if content:
        for img in content.find_all("img"):
            if img_count >= 15:
                break
            src = img.get("data-src") or img.get("src") or ""
            if not src.startswith("http"):
                continue
            base = src.split("?")[0]
            if base in seen:
                continue
            seen.add(base)
            # Skip tiny icons and ads
            w = img.get("width", "")
            if w and w.isdigit() and int(w) < 80:
                continue
            ext = Path(urlparse(base).path).suffix.lower() or ".jpg"
            if ext not in {".jpg", ".jpeg", ".png", ".gif", ".webp"}:
                continue
            fname = f"img_{img_count:02d}{ext}"
            if _download_file(client, src, images_dir / fname):
                meta.images.append(f"images/{fname}")
                img_count += 1
            time.sleep(0.2)

    # Fallback OG image
    if not meta.images:
        og_img = soup.find("meta", property="og:image")
        if og_img and og_img.get("content", "").startswith("http"):
            if _download_file(client, og_img["content"], images_dir / "og.jpg"):
                meta.images.append("images/og.jpg")

    # PDF plan links
    artifacts_dir = site_dir / "artifacts"
    artifacts_dir.mkdir(exist_ok=True)
    for link in soup.find_all("a", href=True):
        href = link["href"]
        if not href.startswith("http"):
            href = urljoin(meta.url, href)
        ext = Path(urlparse(href).path).suffix.lower()
        if ext in ARTIFACT_EXTS:
            fname = _sanitize_filename(Path(urlparse(href).path).name)
            if _download_file(client, href, artifacts_dir / fname):
                meta.artifacts.append(_artifact_entry(fname, artifacts_dir))
            time.sleep(0.3)

    meta.title = meta.title or "Ana White Project"


ANAWHITE_CONFIG = SiteConfig(
    name="anawhite",
    domain="ana-white.com",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # Images handled in post_parse
    artifact_strategies=[],
    post_parse=_anawhite_post_parse,
)


# ── Hackaday.io ──────────────────────────────────────────────────────────


def _hackaday_post_parse(
    soup: BeautifulSoup, meta: ProjectMeta, site_dir: Path, client: httpx.Client, **kw
) -> None:
    """Hackaday.io post-parse: project images, description, author, files."""
    # Author
    creator = soup.find("a", class_="project-creator") or soup.find("a", class_="hacker-link")
    if creator:
        meta.author = meta.author or creator.get_text(strip=True)

    # Better description from project detail
    desc_el = soup.find("div", class_="project-description") or soup.find("section", id="project-description")
    if desc_el:
        text = desc_el.get_text(separator=" ", strip=True)
        if len(text) > len(meta.description or ""):
            meta.description = text[:2000]

    # Images — project gallery and inline images
    images_dir = site_dir / "images"
    images_dir.mkdir(exist_ok=True)
    img_count = 0
    seen: set[str] = set()

    # Main project image
    for img in soup.find_all("img"):
        if img_count >= 15:
            break
        src = img.get("data-src") or img.get("src") or ""
        if not src.startswith("http"):
            continue
        # Accept hackaday CDN and common image hosts
        if not any(d in src for d in ("cdn.hackaday.io", "hackaday.io", "cloudfront.net", "imgur.com")):
            continue
        base = src.split("?")[0]
        if base in seen:
            continue
        seen.add(base)
        ext = Path(urlparse(base).path).suffix.lower() or ".jpg"
        if ext not in {".jpg", ".jpeg", ".png", ".gif", ".webp"}:
            continue
        fname = f"img_{img_count:02d}{ext}"
        if _download_file(client, src, images_dir / fname):
            meta.images.append(f"images/{fname}")
            img_count += 1
        time.sleep(0.2)

    # Fallback OG image
    if not meta.images:
        og_img = soup.find("meta", property="og:image")
        if og_img and og_img.get("content", "").startswith("http"):
            if _download_file(client, og_img["content"], images_dir / "og.jpg"):
                meta.images.append("images/og.jpg")

    # Downloadable files
    artifacts_dir = site_dir / "artifacts"
    artifacts_dir.mkdir(exist_ok=True)
    for link in soup.find_all("a", href=True):
        href = link["href"]
        if not href.startswith("http"):
            href = urljoin(meta.url, href)
        ext = Path(urlparse(href).path).suffix.lower()
        if ext in ARTIFACT_EXTS:
            fname = _sanitize_filename(Path(urlparse(href).path).name)
            if _download_file(client, href, artifacts_dir / fname):
                meta.artifacts.append(_artifact_entry(fname, artifacts_dir))
            time.sleep(0.3)

    meta.title = meta.title or "Hackaday.io Project"


HACKADAY_CONFIG = SiteConfig(
    name="hackaday",
    domain="hackaday.io",
    fetch_chain=_DEFAULT_FETCH_CHAIN,
    metadata_strategies=[JsonLdMetadata(), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[],  # Images handled in post_parse
    artifact_strategies=[],
    post_parse=_hackaday_post_parse,
)


# ── Generic ──────────────────────────────────────────────────────────────

GENERIC_CONFIG = SiteConfig(
    name="generic",
    domain="",
    fetch_chain=[*_DEFAULT_FETCH_CHAIN, WaybackFetcher()],
    metadata_strategies=[JsonLdMetadata(), OpenGraphMetadata(), HtmlMetadata()],
    image_strategies=[ImgTagImages(max_images=20), OgImages()],
    artifact_strategies=[LinkArtifacts()],
)


# ── Site Config Registry ─────────────────────────────────────────────────

SITE_CONFIGS: dict[str, SiteConfig] = {
    "printables": PRINTABLES_CONFIG,
    "thingiverse": THINGIVERSE_CONFIG,
    "github": GITHUB_CONFIG,
    "instructables": INSTRUCTABLES_CONFIG,
    "cults3d": CULTS3D_CONFIG,
    "anawhite": ANAWHITE_CONFIG,
    "hackaday": HACKADAY_CONFIG,
    "generic": GENERIC_CONFIG,
}

DOMAIN_CONCURRENCY: dict[str, int] = {
    name: 1 if cfg.rate_limit >= 2.0 else (4 if name == "generic" else 2)
    for name, cfg in SITE_CONFIGS.items()
}


# ── Quality check & Playwright fallback ────────────────────────────────────


def _quality_score(site_dir: Path) -> dict:
    """Evaluate crawl quality for a project. Returns dict with score + issues."""
    meta_path = site_dir / "meta.json"
    if not meta_path.exists():
        return {"score": 0, "issues": ["no_meta"]}

    d = json.loads(meta_path.read_text())
    issues = []
    score = 0

    # Images
    img_dir = site_dir / "images"
    actual_imgs = [f for f in img_dir.glob("*") if f.is_file() and f.stat().st_size > 1000] if img_dir.exists() else []
    if actual_imgs:
        score += 3
    else:
        issues.append("no_images")

    # Description
    desc = d.get("description", "")
    if len(desc.strip()) > 100:
        score += 2
    elif len(desc.strip()) > 30:
        score += 1
    else:
        issues.append("no_description")

    # Artifacts
    real_arts = [a for a in d.get("artifacts", []) if not a.get("download_failed")]
    if real_arts:
        score += 2

    # Raw content
    raw = site_dir / "raw.html"
    readme = site_dir / "README.md"
    if (raw.exists() and raw.stat().st_size > 1000) or (readme.exists() and readme.stat().st_size > 100):
        score += 1
    else:
        issues.append("no_content")

    return {"score": score, "issues": issues, "has_images": bool(actual_imgs)}


def _playwright_recrawl(url: str, site_dir: Path, meta: ProjectMeta) -> bool:
    """Re-crawl a page with Playwright headless browser. Returns True if improved."""
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        return False

    log.debug("    Playwright re-crawl: %s", url[:60])
    is_thingiverse = "thingiverse.com" in url
    initial_images = len(meta.images)
    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            ctx = browser.new_context(viewport={"width": 1280, "height": 800})
            page = ctx.new_page()
            page.goto(url, wait_until="networkidle", timeout=30000)

            # Wait for images to lazy-load
            page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
            page.wait_for_timeout(2000)
            page.evaluate("window.scrollTo(0, 0)")
            page.wait_for_timeout(1000)

            # Thingiverse SPA: click through gallery to trigger image loads
            if is_thingiverse:
                try:
                    page.wait_for_selector("img[class*='gallery'], img[class*='thumb'], .thing-image img", timeout=5000)
                except Exception:
                    pass

            # Save rendered HTML
            html = page.content()
            (site_dir / "raw.html").write_text(html)

            # Extract images from rendered page
            images_dir = site_dir / "images"
            images_dir.mkdir(exist_ok=True)
            img_count = len(meta.images)

            img_urls = page.evaluate("""
                () => Array.from(document.querySelectorAll('img'))
                    .map(img => img.src || img.dataset.src)
                    .filter(src => src && src.startsWith('http') && !src.startsWith('data:'))
            """)

            seen = set(meta.images)
            for img_url in img_urls[:20]:
                if img_count >= 20:
                    break
                # Skip tiny images (icons, spacers)
                try:
                    dims = page.evaluate("""
                        (url) => {
                            const img = document.querySelector(`img[src="${url}"]`);
                            return img ? {w: img.naturalWidth, h: img.naturalHeight} : null;
                        }
                    """, img_url)
                    if dims and (dims["w"] < 50 or dims["h"] < 50):
                        continue
                except Exception:
                    pass

                ext = Path(urlparse(img_url).path).suffix.lower() or ".jpg"
                fname = f"img_{img_count:02d}{ext}"
                rel_path = f"images/{fname}"
                if rel_path in seen:
                    continue
                try:
                    # page.request uses the browser's cookies/session — bypasses CDN 403s
                    resp = page.request.get(img_url)
                    if resp.ok:
                        (images_dir / fname).write_bytes(resp.body())
                        if (images_dir / fname).stat().st_size > 1000:
                            meta.images.append(rel_path)
                            seen.add(rel_path)
                            img_count += 1
                except Exception:
                    pass

            # Thingiverse: also try to grab og:image and gallery data from DOM
            if is_thingiverse and img_count == initial_images:
                og_urls = page.evaluate("""
                    () => {
                        const metas = document.querySelectorAll('meta[property="og:image"]');
                        return Array.from(metas).map(m => m.content).filter(Boolean);
                    }
                """)
                for og_url in og_urls[:5]:
                    if img_count >= 20:
                        break
                    fname = f"img_{img_count:02d}.jpg"
                    rel_path = f"images/{fname}"
                    try:
                        resp = page.request.get(og_url)
                        if resp.ok:
                            (images_dir / fname).write_bytes(resp.body())
                            if (images_dir / fname).stat().st_size > 1000:
                                meta.images.append(rel_path)
                                img_count += 1
                    except Exception:
                        pass

            # Extract description if missing
            if len(meta.description.strip()) < 30:
                desc = page.evaluate("""
                    () => {
                        const p = document.querySelector('article p, main p, .content p, #content p');
                        return p ? p.textContent.trim() : '';
                    }
                """)
                if len(desc) > len(meta.description):
                    meta.description = desc[:2000]

            browser.close()
            return len(meta.images) > initial_images or len(meta.description) > 30
    except Exception as e:
        log.debug("    Playwright failed: %s", str(e)[:100])
        return False


_PLAYWRIGHT_AVAILABLE: bool | None = None


def _has_playwright() -> bool:
    """Check if Playwright is available (cached)."""
    global _PLAYWRIGHT_AVAILABLE
    if _PLAYWRIGHT_AVAILABLE is None:
        try:
            from playwright.sync_api import sync_playwright
            # Quick check that browser is installed
            with sync_playwright() as p:
                p.chromium.launch(headless=True).close()
            _PLAYWRIGHT_AVAILABLE = True
        except Exception:
            _PLAYWRIGHT_AVAILABLE = False
    return _PLAYWRIGHT_AVAILABLE


# ── Stage 3: Crawl — Dispatcher ───────────────────────────────────────────


CRAWL_WORKERS = 6  # total concurrent crawl threads


def stage_crawl(
    state: PipelineState,
    verified: list[VerifiedLink],
    retry_failed: bool = False,
    printables_token: str = "",
    thingiverse_token: str = "",
    thingiverse_cookies: dict[str, str] | None = None,
    optimize: bool = True,
) -> list[CrawlResult]:
    """Crawl all verified links using appropriate extractors (parallel)."""
    import threading
    from concurrent.futures import ThreadPoolExecutor, as_completed

    log.info("Stage 3: Crawling %d links...", len(verified))

    # Load existing progress
    progress: dict[str, dict] = {}
    if state.progress_file.exists():
        for entry in _read_jsonl(state.progress_file):
            progress[entry["url"]] = entry

    # Determine what to crawl
    to_crawl = []
    for v in verified:
        if v.status == "dead":
            continue
        existing = progress.get(v.url)
        if existing:
            if existing["status"] == "completed":
                continue  # never re-crawl completed items
            if existing["status"] == "failed":
                if not retry_failed and existing.get("attempts", 0) >= MAX_RETRIES:
                    continue
                # retry_failed=True → re-attempt failed items regardless of attempts
        to_crawl.append(v)

    if not to_crawl:
        log.info("  All links already crawled")
        return [CrawlResult(**p) for p in progress.values()]

    log.info("  %d links to crawl (%d already done)", len(to_crawl), len(progress))

    # Group by extractor for stats
    by_ext: dict[str, list] = {}
    for v in to_crawl:
        by_ext.setdefault(v.extractor or "generic", []).append(v)
    for ext, items in sorted(by_ext.items()):
        log.info("    %s: %d", ext, len(items))

    state.sites_dir.mkdir(parents=True, exist_ok=True)

    # Populate runtime config for hooks
    _RUNTIME_CONFIG["printables_token"] = printables_token
    _RUNTIME_CONFIG["thingiverse_token"] = thingiverse_token
    _RUNTIME_CONFIG["thingiverse_cookies"] = thingiverse_cookies

    # Per-extractor semaphores for rate limiting
    ext_sems: dict[str, threading.Semaphore] = {
        name: threading.Semaphore(DOMAIN_CONCURRENCY.get(name, 2))
        for name in SITE_CONFIGS
    }
    progress_lock = threading.Lock()
    counter = {"done": 0}

    with httpx.Client(timeout=60, follow_redirects=True) as client:
        scrapers: dict[str, SiteScraper] = {
            name: SiteScraper(config, client, state.sites_dir)
            for name, config in SITE_CONFIGS.items()
        }

        def _crawl_one(v: VerifiedLink) -> CrawlResult:
            # Re-classify at crawl time (handles fixes to classifier)
            ext_name = _classify_extractor(v.url)
            # Wayback-recovered links should use generic scraper (wayback serves HTML)
            if v.status == "wayback" and v.wayback_url:
                ext_name = "generic"
            scraper = scrapers.get(ext_name, scrapers["generic"])
            sem = ext_sems.get(ext_name, ext_sems["generic"])
            attempts = progress.get(v.url, {}).get("attempts", 0) + 1

            with sem:  # per-domain rate limit
                try:
                    result = scraper.extract(v)
                    result.attempts = attempts
                except Exception as e:
                    log.error("    FAILED: %s", e)
                    result = CrawlResult(
                        url=v.url, status="failed", extractor=ext_name,
                        error=str(e)[:200], attempts=attempts,
                        ts=datetime.now(timezone.utc).isoformat(),
                    )

            # Post-crawl: quality check, Playwright fallback, optimization
            if result.status == "completed":
                site_dir = scraper.site_dir_for(v.final_url or v.url)
                meta_path = site_dir / "meta.json"
                if meta_path.exists():
                    try:
                        meta_obj = _meta_from_dict(json.loads(meta_path.read_text()))
                        scraper.post_extract(site_dir, meta_obj, v.final_url or v.url)
                        if optimize:
                            bytes_saved = _optimize_project(site_dir, meta_obj)
                            if bytes_saved > 0:
                                result.size_bytes -= bytes_saved
                        _save_meta(site_dir, meta_obj)
                        result.files = len(meta_obj.artifacts) + len(meta_obj.images)
                    except Exception as e:
                        log.debug("    post-crawl error: %s", e)

            with progress_lock:
                counter["done"] += 1
                n = counter["done"]
                progress[v.url] = asdict(result)
                _append_jsonl(state.progress_file, result)

            if result.status == "completed":
                log.info("  [%d/%d] %s OK: %s (%d files, %.1f KB)",
                         n, len(to_crawl), ext_name, result.title[:50],
                         result.files, result.size_bytes / 1024)
            else:
                log.info("  [%d/%d] %s FAIL: %s — %s",
                         n, len(to_crawl), ext_name, v.url[:60], result.error[:60])
            return result

        log.info("  Starting %d workers...", CRAWL_WORKERS)
        with ThreadPoolExecutor(max_workers=CRAWL_WORKERS) as pool:
            futures = {pool.submit(_crawl_one, v): v for v in to_crawl}
            for future in as_completed(futures):
                try:
                    future.result()
                except Exception as e:
                    v = futures[future]
                    log.error("  Unhandled error for %s: %s", v.url[:60], e)

    state.stage = "crawl"
    state.save()

    results = [CrawlResult(**p) for p in progress.values()]
    completed = sum(1 for r in results if r.status == "completed")
    failed = sum(1 for r in results if r.status == "failed")
    log.info("  Crawl done: %d completed, %d failed", completed, failed)
    return results


# ── Stage 3 helper: Generate project page HTML ────────────────────────────


def _strip_html(html: str) -> str:
    """Strip HTML tags, keeping text content."""
    soup = BeautifulSoup(html, "html.parser")
    return soup.get_text(separator=" ", strip=True)


def _clean_description_html(html: str) -> str:
    """Clean description HTML: keep safe tags, strip scripts/styles."""
    soup = BeautifulSoup(html, "html.parser")
    # Remove scripts, styles, iframes
    for tag in soup.find_all(["script", "style", "iframe", "oembed"]):
        tag.decompose()
    # Keep only safe tags
    safe_tags = {"p", "br", "strong", "em", "b", "i", "ul", "ol", "li", "a", "h2", "h3", "h4", "figure", "img"}
    for tag in soup.find_all(True):
        if tag.name not in safe_tags:
            tag.unwrap()
    return str(soup).strip()


def _generate_project_page(site_dir: Path, meta: ProjectMeta, steps: list[dict] | None = None) -> None:
    """Generate a clean HTML page for a crawled project."""

    # Images — clickable to open full size
    images_html = ""
    for img_path in meta.images[:12]:
        images_html += f'<a href="{escape(img_path)}" target="_blank"><img src="{escape(img_path)}" alt="" loading="lazy"></a>\n'

    # Artifacts
    artifacts_html = ""
    real_artifacts = [a for a in meta.artifacts if not a.get("download_failed")]
    if real_artifacts:
        artifacts_html = '<h2>Downloads</h2><ul class="artifacts">\n'
        for art in real_artifacts:
            fname = Path(art["filename"]).name
            ftype = art.get("type", "").upper()
            fsize = art.get("size_bytes", 0)
            size_str = f"{fsize / 1024:.0f} KB" if fsize < 1_000_000 else f"{fsize / 1_000_000:.1f} MB"
            artifacts_html += f'<li><a href="{escape(art["filename"])}">{escape(fname)}</a> <span class="badge">{ftype}</span> <span class="size">{size_str}</span></li>\n'
        artifacts_html += "</ul>\n"

    # Steps (Instructables)
    steps_html = ""
    if steps:
        steps_html = '<h2>Steps</h2>\n'
        for i, step in enumerate(steps, 1):
            title = step.get("title", f"Step {i}")
            text = step.get("text", "")
            steps_html += f'<div class="step"><h3>{escape(title)}</h3><p>{escape(text[:2000])}</p></div>\n'

    desc = meta.description
    better = _extract_desc_from_raw(site_dir, desc)
    if better:
        desc = better
    desc = unescape(desc)

    if "<p>" in desc or "<br" in desc or "<strong>" in desc:
        desc_html = f'<div class="description">{_clean_description_html(desc)}</div>'
    else:
        desc_html = f'<p class="description">{escape(desc)}</p>' if desc else ""

    license_html = ""
    if meta.license:
        license_html = f'<p class="license">License: {escape(meta.license)}</p>'

    # Include cleaned raw page content if available
    raw_content_html = ""
    raw_path = site_dir / "raw.html"
    # For GitHub projects, use README.md as content source if no raw.html
    readme_path = site_dir / "README.md"
    if not (raw_path.exists() and raw_path.stat().st_size > 500) and readme_path.exists():
        try:
            readme_text = readme_path.read_text()
            # Simple markdown-to-HTML: wrap in pre or convert basic formatting
            # Convert headers, bold, links, lists
            lines = []
            for line in readme_text.split("\n"):
                if line.startswith("# "):
                    lines.append(f"<h2>{escape(line[2:])}</h2>")
                elif line.startswith("## "):
                    lines.append(f"<h3>{escape(line[3:])}</h3>")
                elif line.startswith("### "):
                    lines.append(f"<h4>{escape(line[4:])}</h4>")
                elif line.startswith("- ") or line.startswith("* "):
                    lines.append(f"<li>{escape(line[2:])}</li>")
                elif line.strip():
                    lines.append(f"<p>{escape(line)}</p>")
            readme_html = "\n".join(lines)
            if len(readme_text) > 100:
                has_content = len(meta.images) > 2 and len(_strip_html(meta.description)) > 100
                open_attr = "" if has_content else " open"
                raw_content_html = f'<details class="raw-content"{open_attr}><summary>README</summary><div class="raw-body">{readme_html}</div></details>'
        except Exception:
            pass
    if raw_path.exists() and raw_path.stat().st_size > 500:
        try:
            raw_soup = BeautifulSoup(raw_path.read_text(), "html.parser")
            # Try to extract main content area — broadest set of selectors
            main = (
                raw_soup.find("main")
                or raw_soup.find("article")
                or raw_soup.find("div", class_="content")
                or raw_soup.find("div", id="content")
                or raw_soup.find("div", class_="entry-content")
                or raw_soup.find("div", class_="post-content")
                or raw_soup.find("div", class_="step-container")
                or raw_soup.find("div", class_="article-content")
                or raw_soup.find("div", class_="page-content")
                or raw_soup.find("div", class_="site-content")
                or raw_soup.find("div", class_="main-content")
                or raw_soup.find("div", id="main")
                or raw_soup.find("div", role="main")
                or raw_soup.find("section", class_="content")
            )
            # Last resort: use body, but strip header/footer/nav
            if not main:
                main = raw_soup.find("body")
                if main:
                    for tag in main.find_all(["header", "footer", "nav"]):
                        tag.decompose()
            if main:
                # Remove scripts, styles, navs, footers, ads
                for tag in main.find_all(["script", "style", "nav", "footer", "iframe",
                                          "noscript", "svg", "form", "input", "button"]):
                    tag.decompose()
                for tag in main.find_all(class_=lambda c: c and any(
                    x in str(c).lower() for x in ["ad-", "sidebar", "newsletter", "popup", "cookie", "social"]
                )):
                    tag.decompose()
                # Fix image paths — make relative to site_dir
                for img in main.find_all("img"):
                    src = img.get("src") or img.get("data-src")
                    if src:
                        img["src"] = src
                        img["loading"] = "lazy"
                        img["style"] = "max-width:100%;height:auto;"
                content_text = main.get_text(strip=True)
                if len(content_text) > 100:  # only include if substantial
                    # Auto-open if project has no images or short description
                    has_content = len(meta.images) > 2 and len(_strip_html(meta.description)) > 100
                    open_attr = "" if has_content else " open"
                    raw_content_html = f'<details class="raw-content"{open_attr}><summary>Full page content</summary><div class="raw-body">{str(main)}</div></details>'
        except Exception:
            pass

    # Category breadcrumb for navigation + FTS indexing
    cat_slug = _slugify(meta.category) if meta.category else ""
    cat_html = ""
    if meta.category:
        cat_link = f'<a href="../../category/{cat_slug}/index.html">{escape(meta.category)}</a>'
        if meta.subcategory:
            cat_html = f'<p class="breadcrumb">{cat_link} / {escape(meta.subcategory)}</p>'
        else:
            cat_html = f'<p class="breadcrumb">{cat_link}</p>'

    # Artifact types as searchable keywords (e.g. "STL PDF" so FTS picks them up)
    artifact_types = sorted({a.get("type", "").upper() for a in real_artifacts if a.get("type")})
    keywords = " ".join(artifact_types)

    html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{escape(meta.title)}</title>
<meta name="keywords" content="{escape(meta.category)}, {escape(meta.subcategory)}, {escape(keywords)}">
<style>{PROJECT_CSS}</style>
</head>
<body>
<nav><a href="../../index.html">Home</a></nav>
{cat_html}
<h1>{escape(meta.title)}</h1>
<p class="meta">
  <span class="author">{escape(meta.author)}</span>
  <span class="domain">{escape(meta.source_domain)}</span>
  <span class="source"><a href="{escape(meta.url)}">Original</a></span>
</p>
{license_html}
{desc_html}
<div class="gallery">{images_html}</div>
{artifacts_html}
{steps_html}
{raw_content_html}
</body>
</html>"""

    (site_dir / "index.html").write_text(html)


# ── Post-crawl optimization ────────────────────────────────────────────────

# Extensions to always drop (compiler output, installers, redundant formats)
DROP_ALWAYS = {".o", ".crf", ".exe", ".wrl", ".ttf", ".iges", ".igs"}

# Extensions to drop when a better alternative exists in the same project
DROP_IF_REDUNDANT = {
    ".gcode": {".stl", ".3mf"},       # gcode is printer-specific; STL is universal
}

# Non-essential CAD source formats (proprietary, or redundant when .step exists)
DROP_CAD_NON_ESSENTIAL = {".blend", ".skp"}

# Max image dimension (pixels) for resized images
IMAGE_MAX_DIM = 800
IMAGE_JPEG_QUALITY = 80


def _optimize_project(site_dir: Path, meta: ProjectMeta) -> int:
    """Optimize a crawled project's files in-place. Returns bytes saved."""
    saved = 0

    # Single pass: collect all files with their extensions and sizes
    all_files: list[Path] = []
    has_exts: set[str] = set()
    for f in site_dir.rglob("*"):
        if f.is_file():
            all_files.append(f)
            has_exts.add(f.suffix.lower())

    # Partition by what we need to do with them
    to_drop: list[Path] = []
    large_gifs: list[Path] = []
    resizable: list[Path] = []

    drop_exts = DROP_ALWAYS | DROP_CAD_NON_ESSENTIAL
    # Add redundant extensions where better alternatives exist
    for ext, alternatives in DROP_IF_REDUNDANT.items():
        if has_exts & alternatives:
            drop_exts.add(ext)

    for f in all_files:
        ext = f.suffix.lower()
        if ext in drop_exts:
            to_drop.append(f)
        elif ext == ".gif" and f.stat().st_size > 500_000:
            large_gifs.append(f)
        elif ext in {".jpg", ".jpeg", ".png"} and f.stat().st_size >= 50_000:
            resizable.append(f)

    # 1. Drop unwanted files
    for f in to_drop:
        size = f.stat().st_size
        f.unlink()
        saved += size
        log.debug("    drop %s (%d KB)", f.name, size // 1024)

    # 2. GIFs > 500KB → extract first frame as JPG
    try:
        from PIL import Image as PILImage
        for f in large_gifs:
            try:
                old_size = f.stat().st_size
                img = PILImage.open(f)
                img = img.convert("RGB")
                jpg_path = f.with_suffix(".jpg")
                img.save(str(jpg_path), "JPEG", quality=IMAGE_JPEG_QUALITY)
                new_size = jpg_path.stat().st_size
                f.unlink()
                saved += old_size - new_size
                old_rel = str(f.relative_to(site_dir))
                new_rel = str(jpg_path.relative_to(site_dir))
                meta.images = [new_rel if i == old_rel else i for i in meta.images]
            except Exception:
                pass

        # 3. Resize large images
        for f in resizable:
            try:
                old_size = f.stat().st_size
                img = PILImage.open(f)
                w, h = img.size
                if max(w, h) <= IMAGE_MAX_DIM:
                    continue
                ratio = IMAGE_MAX_DIM / max(w, h)
                new_size_tuple = (int(w * ratio), int(h * ratio))
                img = img.resize(new_size_tuple, PILImage.LANCZOS)
                if f.suffix.lower() == ".png":
                    img.save(str(f), "PNG", optimize=True)
                else:
                    if img.mode in ("RGBA", "P"):
                        img = img.convert("RGB")
                    img.save(str(f), "JPEG", quality=IMAGE_JPEG_QUALITY)
                new_size = f.stat().st_size
                saved += old_size - new_size
            except Exception:
                pass
    except ImportError:
        log.debug("    Pillow not installed, skipping image optimization")

    # 4. Update meta.json artifacts list (remove deleted files)
    meta.artifacts = [
        a for a in meta.artifacts
        if not a.get("download_failed")
        and (site_dir / a["filename"]).exists()
    ]
    for a in meta.artifacts:
        p = site_dir / a["filename"]
        if p.exists():
            a["size_bytes"] = p.stat().st_size

    return saved


# ── Stage 4: Package ──────────────────────────────────────────────────────


def _collect_projects(state: PipelineState) -> list[dict]:
    """Collect all meta.json files from crawled sites."""
    projects = []
    if not state.sites_dir.exists():
        return projects
    for meta_file in state.sites_dir.rglob("meta.json"):
        try:
            data = json.loads(meta_file.read_text())
            data["_dir"] = str(meta_file.parent.relative_to(state.sites_dir))
            projects.append(data)
        except Exception:
            continue
    return projects


def _refetch_truncated_descriptions(projects: list[dict], state: PipelineState) -> None:
    """Re-fetch full descriptions for Printables projects truncated during crawl."""
    truncated = [
        p for p in projects
        if p.get("source_domain") == "printables.com"
        and len(p.get("description", "")) >= 490  # likely truncated at old 500-char limit
    ]
    if not truncated:
        return

    log.info("  Re-fetching %d truncated Printables descriptions...", len(truncated))
    with httpx.Client(timeout=30) as client:
        for proj in truncated:
            model_id = None
            m = re.search(r"/model/(\d+)", proj.get("url", ""))
            if m:
                model_id = m.group(1)
            if not model_id:
                continue
            try:
                r = client.post(
                    "https://api.printables.com/graphql/",
                    json={
                        "query": "{ print(id: \"%s\") { description } }" % model_id,
                    },
                    headers={**HEADERS, "Content-Type": "application/json",
                             "Origin": "https://www.printables.com"},
                    timeout=15,
                )
                if r.status_code == 200:
                    data = r.json()
                    full_desc = (data.get("data", {}).get("print", {}).get("description") or "")
                    if len(full_desc) > len(proj.get("description", "")):
                        proj["description"] = full_desc
                        _patch_meta(state.sites_dir / proj["_dir"], description=full_desc)
                time.sleep(0.3)
            except Exception as e:
                log.debug("  Failed to refetch desc for %s: %s", model_id, e)


def _enrich_descriptions_from_raw_html(projects: list[dict], state: PipelineState) -> None:
    """Extract better descriptions from raw HTML for projects with weak/missing descriptions.

    Persists improvements back to meta.json so category pages also benefit.
    """
    enriched = 0
    for proj in projects:
        desc = proj.get("description", "")
        needs_better = (
            not desc
            or len(desc.strip()) < 30
            or desc.rstrip().endswith("…")
            or desc.rstrip().endswith("...")
        )
        if not needs_better:
            continue

        proj_dir = state.sites_dir / proj["_dir"]
        new_desc = _extract_desc_from_raw(proj_dir, desc)
        if new_desc:
            proj["description"] = new_desc
            _patch_meta(proj_dir, description=new_desc)
            enriched += 1

    if enriched:
        log.info("  Enriched %d project descriptions from raw HTML/README", enriched)


def stage_package(
    state: PipelineState,
    verified: list[VerifiedLink],
    crawl_results: list[CrawlResult],
    output_path: Path,
) -> None:
    """Generate HTML frontpage and package everything into a ZIM."""
    log.info("Stage 4: Packaging ZIM...")

    projects = _collect_projects(state)
    log.info("  Found %d crawled projects", len(projects))

    # Enrich descriptions: re-fetch truncated Printables, extract from raw HTML
    log.info("  Enriching descriptions...")
    _refetch_truncated_descriptions(projects, state)
    _enrich_descriptions_from_raw_html(projects, state)

    # Optimize existing crawl data (drop redundant files, resize large images)
    # Runs before image validation since it may rename/delete image files.
    log.info("  Optimizing project assets...")
    total_saved = 0
    optimized_count = 0
    for proj in projects:
        proj_dir = state.sites_dir / proj["_dir"]
        marker = proj_dir / ".optimized"
        if marker.exists():
            continue
        try:
            meta_obj = _meta_from_dict(proj)
            saved = _optimize_project(proj_dir, meta_obj)
            if saved > 0:
                total_saved += saved
                optimized_count += 1
            _save_meta(proj_dir, meta_obj)
            proj["artifacts"] = meta_obj.artifacts
            proj["images"] = meta_obj.images
            marker.touch()
        except Exception as e:
            log.debug("  Could not optimize %s: %s", proj.get("title", "?")[:40], e)
    if total_saved > 0:
        log.info("  Optimized %d projects, saved %.1f MB", optimized_count, total_saved / 1e6)

    # Validate image paths: drop references to missing files, discover actual images on disk
    log.info("  Validating image paths...")
    images_fixed = 0
    for proj in projects:
        proj_dir = state.sites_dir / proj["_dir"]
        images = proj.get("images", [])
        valid = [img for img in images if (proj_dir / img).exists()]
        if len(valid) < len(images):
            img_dir = proj_dir / "images"
            if img_dir.is_dir():
                on_disk = sorted(
                    f"images/{f.name}" for f in img_dir.iterdir()
                    if f.is_file() and f.suffix.lower() in {".jpg", ".jpeg", ".png", ".gif", ".webp"}
                )
                seen = set(valid)
                for img in on_disk:
                    if img not in seen:
                        valid.append(img)
                        seen.add(img)
            if valid != images:
                proj["images"] = valid
                _patch_meta(proj_dir, images=valid)
                images_fixed += 1
    if images_fixed:
        log.info("  Fixed image references for %d projects", images_fixed)

    # Compute quality scores for badges
    for proj in projects:
        proj_dir = state.sites_dir / proj["_dir"]
        proj["_quality"] = _quality_score(proj_dir)

    # Regenerate all project pages with latest template
    log.info("  Regenerating project pages...")
    for proj in projects:
        proj_dir = state.sites_dir / proj["_dir"]
        try:
            meta_obj = _meta_from_dict(proj)
            steps = None
            steps_file = proj_dir / "steps.json"
            if steps_file.exists():
                steps = json.loads(steps_file.read_text())
            _generate_project_page(proj_dir, meta_obj, steps=steps)
        except Exception as e:
            log.debug("  Could not regenerate %s: %s", proj.get("title", "?")[:40], e)

    # Group by category
    by_category: dict[str, list[dict]] = {}
    for proj in projects:
        cat = proj.get("category", "Other") or "Other"
        by_category.setdefault(cat, []).append(proj)

    # Count dead links
    dead = [v for v in verified if v.status == "dead"]

    # Stats
    total_artifacts = sum(len(p.get("artifacts", [])) for p in projects)
    total_images = sum(len(p.get("images", [])) for p in projects)

    # Build ZIM
    log.info("  Creating ZIM: %s", output_path)
    zim = Creator(str(output_path))
    zim.config_indexing(True, "eng")
    zim.config_clustersize(2048)
    zim.set_mainpath("index.html")

    with zim:
        zim.add_metadata("Title", "Make It Yourself")
        zim.add_metadata("Description", f"{len(projects)} curated DIY projects archived for offline use")
        zim.add_metadata("Language", "eng")
        zim.add_metadata("Creator", "NODE / makeityourself.org")
        zim.add_metadata("Publisher", "svalbard")
        zim.add_metadata("Date", datetime.now().strftime("%Y-%m-%d"))
        zim.add_metadata("Tags", "diy;maker;3d-printing;woodworking;sewing;electronics")

        # Illustration (cover image for Kiwix library view)
        cover_path = state.workdir / "cover.png"
        if not cover_path.exists():
            log.info("  Downloading cover image...")
            try:
                _download_file_sync(
                    "https://makeityourself.org/img/book.png",
                    cover_path,
                )
            except Exception:
                log.warning("  Could not download cover image")
        if cover_path.exists():
            try:
                from PIL import Image as PILImage
                img = PILImage.open(cover_path)
                # 48x48 required by Kiwix spec
                for size in (48,):
                    thumb = img.copy()
                    thumb.thumbnail((size, size), PILImage.LANCZOS)
                    import io
                    buf = io.BytesIO()
                    thumb.save(buf, "PNG")
                    zim.add_metadata(f"Illustration_{size}x{size}@1", buf.getvalue())
            except ImportError:
                log.warning("  Pillow not installed, skipping cover")

        # CSS
        zim.add_item(StaticItem("style.css", "text/css", FRONTPAGE_CSS))

        # Index page
        index_html = _make_index_page(by_category, len(projects), total_artifacts, len(dead))
        zim.add_item(HtmlItem("index.html", "Make It Yourself", index_html, is_front=True))

        # Category pages
        for cat_name, cat_projects in sorted(by_category.items()):
            cat_slug = _slugify(cat_name)
            cat_html = _make_category_page(cat_name, cat_projects)
            zim.add_item(HtmlItem(
                f"category/{cat_slug}/index.html",
                cat_name,
                cat_html,
                is_front=True,
            ))

        # Dead links page
        if dead:
            dead_html = _make_dead_page(dead)
            zim.add_item(HtmlItem("dead/index.html", "Dead Links", dead_html))

        # Project pages and assets — track paths to avoid duplicates
        added_paths: set[str] = set()

        for proj in projects:
            proj_dir = state.sites_dir / proj["_dir"]
            zim_prefix = f"sites/{proj['_dir']}"

            # index.html
            index_file = proj_dir / "index.html"
            zim_path = f"{zim_prefix}/index.html"
            if index_file.exists() and zim_path not in added_paths:
                added_paths.add(zim_path)
                zim.add_item(HtmlItem(
                    zim_path,
                    proj.get("title", ""),
                    index_file.read_text(),
                    is_front=True,
                ))

            # Images
            for img_path in proj.get("images", []):
                full_path = proj_dir / img_path
                zim_path = f"{zim_prefix}/{img_path}"
                if full_path.exists() and zim_path not in added_paths:
                    added_paths.add(zim_path)
                    mime = mimetypes.guess_type(str(full_path))[0] or "image/jpeg"
                    zim.add_item(FileItem(zim_path, mime, full_path))

            # Artifacts
            for art in proj.get("artifacts", []):
                full_path = proj_dir / art["filename"]
                zim_path = f"{zim_prefix}/{art['filename']}"
                if full_path.exists() and zim_path not in added_paths:
                    added_paths.add(zim_path)
                    mime = mimetypes.guess_type(str(full_path))[0] or "application/octet-stream"
                    zim.add_item(FileItem(zim_path, mime, full_path))

    size_mb = output_path.stat().st_size / 1e6
    log.info("  Done: %s (%.1f MB)", output_path, size_mb)
    log.info("  %d projects, %d artifacts, %d images", len(projects), total_artifacts, total_images)


# ── HTML Templates ─────────────────────────────────────────────────────────

PROJECT_CSS = """
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  max-width: 900px; margin: 0 auto; padding: 16px; background: #fafafa; color: #222; }
h1 { font-size: 22px; margin-bottom: 4px; }
h2 { font-size: 18px; color: #444; margin-top: 24px; }
h3 { font-size: 15px; color: #666; margin-top: 16px; }
nav { margin-bottom: 16px; }
nav a { color: #2563eb; text-decoration: none; }
.meta { font-size: 14px; color: #666; }
.meta span { margin-right: 12px; }
.description { line-height: 1.6; margin: 12px 0; }
.gallery { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; }
.gallery img { max-width: 280px; max-height: 220px; border-radius: 4px; object-fit: cover; }
.artifacts { list-style: none; padding: 0; }
.artifacts li { padding: 6px 0; border-bottom: 1px solid #eee; }
.artifacts a { color: #2563eb; text-decoration: none; font-weight: 500; }
.badge { font-size: 11px; background: #e0e7ff; color: #3730a3; padding: 2px 6px;
  border-radius: 3px; font-weight: 600; }
.size { font-size: 12px; color: #999; }
.license { font-size: 13px; color: #666; font-style: italic; }
.step { margin: 16px 0; padding: 12px; background: #fff; border-radius: 6px;
  border: 1px solid #e5e7eb; }
.breadcrumb { font-size: 13px; color: #666; margin-bottom: 4px; }
.breadcrumb a { color: #2563eb; text-decoration: none; }
.gallery a { display: inline-block; }
.raw-content { margin-top: 24px; }
.raw-content summary { cursor: pointer; font-weight: 600; color: #2563eb; padding: 8px 0; }
.raw-body { margin-top: 12px; line-height: 1.6; overflow-wrap: break-word; }
.raw-body img { max-width: 100%; height: auto; border-radius: 4px; margin: 8px 0; }
"""

FRONTPAGE_CSS = """
* { box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  max-width: 1100px; margin: 0 auto; padding: 16px; background: #fafafa; color: #222; }
h1 { font-size: 28px; margin-bottom: 4px; }
h2 { font-size: 20px; color: #444; margin-top: 32px; }
.subtitle { font-size: 16px; color: #666; margin-bottom: 24px; }
.stats { display: flex; gap: 24px; margin: 16px 0; flex-wrap: wrap; }
.stat { text-align: center; padding: 12px 20px; background: #fff; border-radius: 8px;
  border: 1px solid #e5e7eb; }
.stat-num { font-size: 24px; font-weight: 700; color: #2563eb; display: block; }
.stat-label { font-size: 13px; color: #666; }
.cat-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px; margin: 16px 0; }
.cat-card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px;
  padding: 16px; text-decoration: none; color: inherit; transition: border-color 0.15s; }
.cat-card:hover { border-color: #2563eb; }
.cat-card h3 { margin: 0 0 4px; font-size: 16px; color: #111; }
.cat-card .count { font-size: 13px; color: #666; }
.project-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px; margin: 16px 0; }
.project-card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px;
  overflow: hidden; text-decoration: none; color: inherit; }
.project-card:hover { border-color: #2563eb; }
.project-card img { width: 100%; height: 140px; object-fit: cover; }
.project-card .body { padding: 12px; }
.project-card h3 { margin: 0 0 4px; font-size: 14px; color: #111; }
.project-card .desc { font-size: 12px; color: #666; line-height: 1.4;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.project-card .badges { margin-top: 8px; display: flex; gap: 6px; flex-wrap: wrap; }
.badge { font-size: 11px; padding: 2px 6px; border-radius: 3px; font-weight: 600; }
.badge-domain { background: #e0e7ff; color: #3730a3; }
.badge-artifacts { background: #dcfce7; color: #166534; }
.badge-wayback { background: #fef3c7; color: #92400e; }
.badge-dead { background: #fee2e2; color: #991b1b; }
.badge-quality-high { background: #fef3c7; color: #92400e; }
.badge-quality-good { background: #e0f2fe; color: #075985; }
.badge-quality-basic { background: #f3f4f6; color: #6b7280; }
.credit { margin-top: 40px; padding: 16px; background: #f9fafb; border-radius: 8px;
  font-size: 13px; color: #666; line-height: 1.6; }
.credit a { color: #2563eb; }
a { color: #2563eb; text-decoration: none; }
nav { margin-bottom: 16px; }
.muted { color: #999; }
table { border-collapse: collapse; width: 100%; }
th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid #e5e7eb; font-size: 14px; }
th { font-weight: 600; color: #666; }
"""


def _make_index_page(
    by_category: dict[str, list[dict]],
    total_projects: int,
    total_artifacts: int,
    dead_count: int,
) -> str:
    dead_label = '<a href="dead/index.html">Dead Links</a>' if dead_count > 0 else "Dead Links"
    stats = f"""
    <div class="stats">
      <div class="stat"><span class="stat-num">{total_projects}</span><span class="stat-label">Projects Archived</span></div>
      <div class="stat"><span class="stat-num">{total_artifacts}</span><span class="stat-label">Downloadable Files</span></div>
      <div class="stat"><span class="stat-num">{len(by_category)}</span><span class="stat-label">Categories</span></div>
      <div class="stat"><span class="stat-num">{dead_count}</span><span class="stat-label">{dead_label}</span></div>
    </div>"""

    cat_cards = ""
    for cat_name in sorted(by_category.keys()):
        cat_projects = by_category[cat_name]
        slug = _slugify(cat_name)
        cat_cards += f"""
        <a href="category/{slug}/index.html" class="cat-card">
          <h3>{escape(cat_name)}</h3>
          <span class="count">{len(cat_projects)} projects</span>
        </a>"""

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Make It Yourself — 1000+ DIY Projects</title>
<link rel="stylesheet" href="style.css">
</head>
<body>
<h1>Make It Yourself</h1>
<p class="subtitle">1000+ useful DIY projects archived for offline use</p>
{stats}
<h2>Categories</h2>
<div class="cat-grid">{cat_cards}</div>
<div class="credit">
  Curated by <a href="https://makeityourself.org">NODE</a> (makeityourself.org).
  Each project belongs to its original creator — licenses vary per project.
  Archived by <a href="https://github.com/pkronstrom/svalbard">svalbard</a>
  on {datetime.now().strftime("%Y-%m-%d")}.
</div>
</body>
</html>"""


def _make_category_page(cat_name: str, projects: list[dict]) -> str:
    cards = ""
    for proj in sorted(projects, key=lambda p: p.get("title", "").lower()):
        proj_dir = proj.get("_dir", "")
        title = proj.get("title", "Untitled")
        desc = proj.get("description", "")[:100]
        domain = proj.get("source_domain", "")
        n_artifacts = len(proj.get("artifacts", []))
        status = proj.get("source_status", "")

        # Image
        images = proj.get("images", [])
        img_html = ""
        if images:
            img_html = f'<img src="../../sites/{proj_dir}/{images[0]}" alt="" loading="lazy">'
        else:
            img_html = '<div style="height:140px;background:#f3f4f6;display:flex;align-items:center;justify-content:center;color:#9ca3af;">No image</div>'

        # Badges
        badges = f'<span class="badge badge-domain">{escape(domain)}</span>'
        if n_artifacts > 0:
            badges += f' <span class="badge badge-artifacts">{n_artifacts} files</span>'
        if status == "wayback":
            badges += ' <span class="badge badge-wayback">Wayback</span>'
        qscore = proj.get("_quality", {}).get("score", 0)
        if qscore >= 6:
            badges += ' <span class="badge badge-quality-high">★★★</span>'
        elif qscore >= 4:
            badges += ' <span class="badge badge-quality-good">★★</span>'
        elif qscore >= 1:
            badges += ' <span class="badge badge-quality-basic">★</span>'

        cards += f"""
        <a href="../../sites/{proj_dir}/index.html" class="project-card">
          {img_html}
          <div class="body">
            <h3>{escape(title[:60])}</h3>
            <p class="desc">{escape(desc)}</p>
            <div class="badges">{badges}</div>
          </div>
        </a>"""

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{escape(cat_name)} — Make It Yourself</title>
<link rel="stylesheet" href="../../style.css">
</head>
<body>
<nav><a href="../../index.html">Home</a></nav>
<h1>{escape(cat_name)}</h1>
<p class="subtitle">{len(projects)} projects</p>
<div class="project-grid">{cards}</div>
</body>
</html>"""


def _make_dead_page(dead: list[VerifiedLink]) -> str:
    rows = ""
    for v in sorted(dead, key=lambda x: (x.category or "zzz", x.title or x.url)):
        wb = ""
        if v.wayback_url:
            wb = f'<a href="{escape(v.wayback_url)}">Wayback</a>'
        title_cell = escape(v.title) if v.title else f'<span class="muted">{escape(v.url)}</span>'
        cat = escape(v.category) if v.category else ""
        rows += f"<tr><td>{title_cell}</td><td>{cat}</td><td><a href=\"{escape(v.url)}\">{escape(urlparse(v.url).netloc)}</a></td><td>{v.http_status}</td><td>{wb}</td></tr>\n"

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Dead Links — Make It Yourself</title>
<link rel="stylesheet" href="../style.css">
</head>
<body>
<nav><a href="../index.html">Home</a></nav>
<h1>Dead Links</h1>
<p>{len(dead)} links were not reachable at crawl time.</p>
<table>
<tr><th>Project</th><th>Category</th><th>Domain</th><th>Status</th><th>Wayback</th></tr>
{rows}
</table>
</body>
</html>"""


# ── ZIM Item classes ───────────────────────────────────────────────────────


class HtmlItem(Item):
    def __init__(self, path: str, title: str, content: str, is_front: bool = False):
        super().__init__()
        self._path = path
        self._title = title
        self._content = content
        self._is_front = is_front

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return self._title

    def get_mimetype(self) -> str:
        return "text/html"

    def get_contentprovider(self):
        return StringProvider(self._content)

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: self._is_front}


class FileItem(Item):
    def __init__(self, path: str, mimetype: str, filepath: Path):
        super().__init__()
        self._path = path
        self._mimetype = mimetype
        self._filepath = filepath

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return ""

    def get_mimetype(self) -> str:
        return self._mimetype

    def get_contentprovider(self):
        return FileProvider(str(self._filepath))

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: False}


class StaticItem(Item):
    def __init__(self, path: str, mimetype: str, content: str):
        super().__init__()
        self._path = path
        self._mimetype = mimetype
        self._content = content

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return ""

    def get_mimetype(self) -> str:
        return self._mimetype

    def get_contentprovider(self):
        return StringProvider(self._content)

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: False}


# ── CLI ────────────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(
        description="Archive makeityourself.org projects into a ZIM file",
    )
    parser.add_argument(
        "--workdir", "-w", default=DEFAULT_WORKDIR,
        help=f"Working directory for intermediate files and state (default: {DEFAULT_WORKDIR})",
    )
    parser.add_argument(
        "--output", "-o", default=DEFAULT_OUTPUT,
        help=f"Output ZIM path (default: {DEFAULT_OUTPUT})",
    )
    parser.add_argument(
        "--pdf", default=None,
        help="Path to local MIY.pdf (downloads if not provided)",
    )
    parser.add_argument(
        "--force", action="store_true",
        help="Wipe workdir and start from scratch",
    )
    parser.add_argument(
        "--force-stage",
        choices=["extract", "verify", "crawl", "package"],
        help="Force re-run of a specific stage and everything downstream",
    )
    parser.add_argument(
        "--retry-failed", action="store_true",
        help="Re-attempt only previously failed crawls",
    )
    parser.add_argument(
        "--retry-dead", action="store_true",
        help="Re-check Wayback Machine for links currently marked dead",
    )
    parser.add_argument(
        "--do-not-optimize", action="store_true",
        help="Skip post-crawl optimization (keep all files at original size)",
    )
    parser.add_argument(
        "--verbose", "-v", action="store_true",
        help="Enable debug logging",
    )
    parser.add_argument(
        "--skip-thingiverse-check", action="store_true",
        help="Skip the Thingiverse token check (archive without STLs)",
    )
    parser.add_argument(
        "--thingiverse-browser", action="store_true",
        help="Open a browser to capture Thingiverse Cloudflare cookies for CDN downloads",
    )
    args = parser.parse_args()

    # Logging
    level = logging.DEBUG if args.verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s %(levelname)-5s %(message)s",
        datefmt="%H:%M:%S",
    )

    import os

    workdir = Path(args.workdir)

    # Load .env from project root (or workdir) if present
    for env_path in [Path.cwd() / ".env", workdir / ".env"]:
        if env_path.exists():
            log.info("Loading env from %s", env_path)
            with open(env_path) as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith("#") and "=" in line:
                        key, _, val = line.partition("=")
                        os.environ.setdefault(key.strip(), val.strip())
            break

    printables_token = os.environ.get("MIY_PRINTABLES_TOKEN", "") or os.environ.get("PRINTABLES_TOKEN", "")
    thingiverse_token = os.environ.get("MIY_THINGIVERSE_TOKEN", "") or os.environ.get("THINGIVERSE_TOKEN", "")

    # Warn / block if Thingiverse token is missing
    if not thingiverse_token and not args.skip_thingiverse_check:
        log.warning("=" * 70)
        log.warning("MIY_THINGIVERSE_TOKEN is not set!")
        log.warning("")
        log.warning("  124 Thingiverse links (14%% of all projects) will be archived")
        log.warning("  WITHOUT STL files — only metadata and thumbnails.")
        log.warning("")
        log.warning("  To fix: register a free app at https://www.thingiverse.com/developers")
        log.warning("  then:   export MIY_THINGIVERSE_TOKEN=your_token_here")
        log.warning("")
        log.warning("  To proceed without Thingiverse STLs, re-run with:")
        log.warning("    --skip-thingiverse-check")
        log.warning("=" * 70)
        sys.exit(1)

    if printables_token:
        log.info("Printables token: set (will use for auth)")
    if thingiverse_token:
        log.info("Thingiverse token: set (STL downloads enabled)")
    else:
        log.info("Thingiverse token: not set (metadata only)")

    output_path = Path(args.output)

    # Handle --force
    if args.force:
        import shutil
        if workdir.exists():
            log.warning("Force mode: wiping %s", workdir)
            shutil.rmtree(workdir)

    # Handle --force-stage
    state = PipelineState.load(workdir)
    if args.force_stage:
        log.warning("Force stage: invalidating from '%s'", args.force_stage)
        state.invalidate_from(args.force_stage)

    workdir.mkdir(parents=True, exist_ok=True)
    state.started_at = datetime.now(timezone.utc).isoformat()

    # Pipeline
    pdf_path = Path(args.pdf) if args.pdf else None
    links = stage_extract(state, pdf_path)
    verified = stage_verify(state, links, retry_dead=args.retry_dead)

    # Thingiverse CDN cookies (Cloudflare bypass)
    tv_cookies = None
    if thingiverse_token:
        tv_cookies = _load_thingiverse_cookies(workdir)
        if not tv_cookies and args.thingiverse_browser:
            tv_cookies = _capture_thingiverse_cookies(workdir)
        if not tv_cookies:
            log.info("No Thingiverse CDN cookies — images/STLs will be skipped (metadata only)")
            log.info("  Run with --thingiverse-browser to capture Cloudflare cookies")

    crawl_results = stage_crawl(
        state, verified,
        retry_failed=args.retry_failed,
        printables_token=printables_token,
        thingiverse_token=thingiverse_token,
        thingiverse_cookies=tv_cookies,
        optimize=not args.do_not_optimize,
    )
    stage_package(state, verified, crawl_results, output_path)

    state.stage = "done"
    state.save()
    log.info("All done! ZIM at %s", output_path)


if __name__ == "__main__":
    main()
