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
from html import escape
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

IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp", ".gif"}

MAX_RETRIES = 3
VERIFY_CONCURRENCY = 10
VERIFY_DELAY = 0.2
CRAWL_DELAY_PER_DOMAIN = 1.5  # seconds between requests to same domain


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
    try:
        r = await client.get(WAYBACK_API, params={"url": url}, timeout=15)
        if r.status_code == 200:
            data = r.json()
            snap = data.get("archived_snapshots", {}).get("closest", {})
            if snap.get("available"):
                return snap["url"]
    except Exception:
        pass
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


def stage_verify(state: PipelineState, links: list[LinkEntry]) -> list[VerifiedLink]:
    """Verify all links and classify by extractor type."""
    existing = {}
    if state.verified_file.exists():
        for entry in _read_jsonl(state.verified_file):
            existing[entry["url"]] = entry

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


def _download_file(client: httpx.Client, url: str, dest: Path, *, timeout: float = 120) -> bool:
    """Download a file to dest. Returns True on success."""
    if dest.exists() and dest.stat().st_size > 0:
        return True
    dest.parent.mkdir(parents=True, exist_ok=True)
    try:
        with client.stream("GET", url, follow_redirects=True, timeout=timeout, headers=HEADERS) as r:
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


def _sanitize_filename(name: str) -> str:
    """Make a filename safe for all platforms."""
    name = re.sub(r'[<>:"/\\|?*]', "_", name)
    name = name.strip(". ")
    return name[:200] or "file"


class BaseExtractor:
    name: str = "base"

    def __init__(self, client: httpx.Client, sites_dir: Path):
        self.client = client
        self.sites_dir = sites_dir

    def site_dir_for(self, url: str) -> Path:
        parsed = urlparse(url)
        domain = parsed.netloc.replace("www.", "")
        path_slug = _slugify(parsed.path.strip("/"))
        d = self.sites_dir / domain / path_slug
        d.mkdir(parents=True, exist_ok=True)
        return d

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        raise NotImplementedError


class PrintablesExtractor(BaseExtractor):
    name = "printables"

    def __init__(self, client: httpx.Client, sites_dir: Path, auth_token: str = ""):
        super().__init__(client, sites_dir)
        self.auth_token = auth_token

    def _extract_model_id(self, url: str) -> str | None:
        m = re.search(r"/model/(\d+)", url)
        return m.group(1) if m else None

    def _parse_ssr_data(self, html: str) -> dict | None:
        """Try to extract SvelteKit SSR data from the page HTML."""
        soup = BeautifulSoup(html, "html.parser")

        # Try JSON-LD first
        for script in soup.find_all("script", type="application/ld+json"):
            try:
                data = json.loads(script.string)
                if isinstance(data, dict) and data.get("@type") in ("Product", "Thing", "CreativeWork"):
                    return data
            except (json.JSONDecodeError, TypeError):
                continue

        # Try extracting from meta tags
        meta = {}
        for tag in soup.find_all("meta"):
            prop = tag.get("property", "") or tag.get("name", "")
            content = tag.get("content", "")
            if prop and content:
                meta[prop] = content

        if meta.get("og:title"):
            return {
                "title": meta.get("og:title", ""),
                "description": meta.get("og:description", ""),
                "image": meta.get("og:image", ""),
                "url": meta.get("og:url", ""),
                "_meta": meta,
            }

        return None

    def _fetch_via_graphql(self, model_id: str) -> dict | None:
        """Try the Printables GraphQL API to get file metadata."""
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
            r = self.client.post(
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

    def _get_download_links(self, model_id: str, file_ids: list[dict]) -> dict[str, str]:
        """Get signed download URLs via mutation. No auth needed with file IDs.

        Args:
            model_id: The Printables model ID
            file_ids: List of dicts with 'id' and 'type' keys (stl/gcode/other)

        Returns:
            Dict mapping file ID to download URL.
        """
        # Group files by type
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
        if self.auth_token:
            headers["Authorization"] = f"Bearer {self.auth_token}"

        result_map: dict[str, str] = {}
        try:
            r = self.client.post(
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

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        model_id = self._extract_model_id(url)
        site_dir = self.site_dir_for(url)
        result = CrawlResult(url=verified.url, extractor=self.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain="printables.com",
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        # Try GraphQL first (richest data)
        gql_data = None
        if model_id:
            gql_data = self._fetch_via_graphql(model_id)
            time.sleep(CRAWL_DELAY_PER_DOMAIN)

        if gql_data:
            meta.title = gql_data.get("name", "")
            meta.description = (gql_data.get("description") or "")[:500]
            meta.author = (gql_data.get("user") or {}).get("publicUsername", "")
            lic = gql_data.get("license") or {}
            meta.license = lic.get("name", "")

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
                if _download_file(self.client, img_url, images_dir / fname):
                    meta.images.append(f"images/{fname}")
                    total_size += (images_dir / fname).stat().st_size
                time.sleep(0.3)

            # Download PDF if available (publicly accessible)
            artifacts_dir = site_dir / "artifacts"
            artifacts_dir.mkdir(exist_ok=True)
            pdf_path = gql_data.get("pdfFilePath")
            if pdf_path:
                pdf_url = f"https://files.printables.com/{pdf_path}"
                if _download_file(self.client, pdf_url, artifacts_dir / "description.pdf"):
                    fsize = (artifacts_dir / "description.pdf").stat().st_size
                    meta.artifacts.append({
                        "filename": "artifacts/description.pdf",
                        "type": "pdf",
                        "size_bytes": fsize,
                    })
                    total_size += fsize
                time.sleep(0.3)

            # Download STLs and other files via signed URLs (no auth needed)
            stls = gql_data.get("stls") or []
            gcodes = gql_data.get("gcodes") or []
            other = gql_data.get("otherFiles") or []
            all_files = [
                *[{**f, "_type": "stl"} for f in stls],
                *[{**f, "_type": "gcode"} for f in gcodes],
                *[{**f, "_type": "other"} for f in other],
            ]
            if all_files:
                # Get signed download URLs for all files at once
                file_ids = [{"id": f["id"], "type": f["_type"]} for f in all_files]
                dl_urls = self._get_download_links(model_id, file_ids)
                time.sleep(CRAWL_DELAY_PER_DOMAIN)

                for f_info in all_files:
                    fid = f_info["id"]
                    dl_url = dl_urls.get(fid)
                    fname = _sanitize_filename(f_info.get("name", f"file_{fid}"))
                    if dl_url:
                        if _download_file(self.client, dl_url, artifacts_dir / fname):
                            fsize = (artifacts_dir / fname).stat().st_size
                            meta.artifacts.append({
                                "filename": f"artifacts/{fname}",
                                "type": Path(fname).suffix.lstrip("."),
                                "size_bytes": fsize,
                            })
                            total_size += fsize
                        time.sleep(0.5)
                    else:
                        # Record metadata for files we couldn't get URLs for
                        meta.artifacts.append({
                            "filename": fname,
                            "type": Path(fname).suffix.lstrip(".") or f_info["_type"],
                            "size_bytes": f_info.get("fileSize", 0),
                            "download_failed": True,
                        })

        else:
            # Fallback: fetch page HTML and parse meta tags
            try:
                r = self.client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
                r.raise_for_status()
                ssr = self._parse_ssr_data(r.text)
                if ssr:
                    meta.title = ssr.get("title", "") or ssr.get("name", "")
                    meta.description = (ssr.get("description") or "")[:500]
                    # Save the raw HTML as fallback
                    (site_dir / "raw.html").write_text(r.text)
                time.sleep(CRAWL_DELAY_PER_DOMAIN)
            except Exception as e:
                result.status = "failed"
                result.error = str(e)[:200]
                return result

        # Generate clean project page
        meta.title = meta.title or verified.title or f"Printables Model {model_id}"
        _generate_project_page(site_dir, meta)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result


class ThingiverseExtractor(BaseExtractor):
    name = "thingiverse"

    def __init__(self, client: httpx.Client, sites_dir: Path, api_token: str = ""):
        super().__init__(client, sites_dir)
        self.api_token = api_token

    def _extract_thing_id(self, url: str) -> str | None:
        m = re.search(r"thing:(\d+)", url)
        if m:
            return m.group(1)
        # Handle make: URLs — these are user prints, not things.
        # Fall back to generic extractor by returning None.
        return None

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        thing_id = self._extract_thing_id(url)
        site_dir = self.site_dir_for(url)
        result = CrawlResult(url=verified.url, extractor=self.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain="thingiverse.com",
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        if not thing_id:
            result.status = "failed"
            result.error = "Could not extract thing ID"
            return result

        # Strategy: fetch HTML and parse JSON-LD (no auth needed), then try API
        html = None
        try:
            r = self.client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
            r.raise_for_status()
            html = r.text
            (site_dir / "raw.html").write_text(html)
            time.sleep(CRAWL_DELAY_PER_DOMAIN)
        except Exception as e:
            result.status = "failed"
            result.error = str(e)[:200]
            return result

        soup = BeautifulSoup(html, "html.parser")

        # Extract JSON-LD (richest no-auth data source)
        jsonld = None
        for script in soup.find_all("script", type="application/ld+json"):
            try:
                data = json.loads(script.string)
                if isinstance(data, dict) and data.get("@type") == "Product":
                    jsonld = data
                    break
            except (json.JSONDecodeError, TypeError):
                continue

        if jsonld:
            meta.title = jsonld.get("name", "")
            meta.description = (jsonld.get("description") or "")[:500]
            brand = jsonld.get("brand") or {}
            meta.author = brand.get("name", "")
            # Author is also in mainEntityOfPage
            meop = jsonld.get("mainEntityOfPage") or {}
            author_obj = meop.get("author") or {}
            if author_obj.get("name"):
                meta.author = author_obj["name"]
            meta.license = meop.get("license", "")
            # Thumbnail
            img_obj = jsonld.get("image") or jsonld.get("thumbnailUrl")
            thumb_url = None
            if isinstance(img_obj, dict):
                thumb_url = img_obj.get("url", "")
            elif isinstance(img_obj, str):
                thumb_url = img_obj
        else:
            # Fallback to title tag
            title_tag = soup.find("title")
            if title_tag:
                t = title_tag.get_text()
                meta.title = re.sub(r"\s*[-–]\s*Thingiverse\s*$", "", t).strip()
                # Title format: "Name by Author - Thingiverse"
                m = re.match(r"(.+?)\s+by\s+(.+)", meta.title)
                if m:
                    meta.title, meta.author = m.group(1), m.group(2)
            thumb_url = None

        # Download thumbnail/image
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        if thumb_url and thumb_url.startswith("http"):
            if _download_file(self.client, thumb_url, images_dir / "thumb.jpg"):
                meta.images.append("images/thumb.jpg")
                total_size += (images_dir / "thumb.jpg").stat().st_size
            time.sleep(0.3)

        # Try Thingiverse API for files (requires app token)
        if self.api_token:
            api_base = "https://api.thingiverse.com"
            api_headers = {**HEADERS, "Authorization": f"Bearer {self.api_token}"}
            try:
                r = self.client.get(
                    f"{api_base}/things/{thing_id}/files",
                    headers=api_headers, timeout=30,
                )
                if r.status_code == 200:
                    files_data = r.json()
                    artifacts_dir = site_dir / "artifacts"
                    artifacts_dir.mkdir(exist_ok=True)
                    for f_info in files_data:
                        # direct_url is CDN (no auth needed once we have it)
                        dl_url = f_info.get("direct_url") or f_info.get("public_url")
                        if not dl_url:
                            continue
                        fname = _sanitize_filename(f_info.get("name", "file"))
                        if _download_file(self.client, dl_url, artifacts_dir / fname):
                            fsize = (artifacts_dir / fname).stat().st_size
                            meta.artifacts.append({
                                "filename": f"artifacts/{fname}",
                                "type": Path(fname).suffix.lstrip("."),
                                "size_bytes": fsize,
                            })
                            total_size += fsize
                        time.sleep(0.5)
                elif r.status_code == 401:
                    log.warning("  Thingiverse API returned 401 — token may be invalid")
                time.sleep(CRAWL_DELAY_PER_DOMAIN)
            except Exception as e:
                log.debug("  Thingiverse API error for thing:%s: %s", thing_id, e)

        meta.title = meta.title or verified.title or f"Thingiverse Thing {thing_id}"
        _generate_project_page(site_dir, meta)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result


class GitHubExtractor(BaseExtractor):
    name = "github"

    def _parse_repo(self, url: str) -> tuple[str, str] | None:
        parsed = urlparse(url)
        parts = parsed.path.strip("/").split("/")
        if len(parts) >= 2:
            return parts[0], parts[1]
        return None

    def _is_org_url(self, url: str) -> str | None:
        """Check if URL is a GitHub org/user (single path segment)."""
        parsed = urlparse(url)
        parts = [p for p in parsed.path.strip("/").split("/") if p]
        if len(parts) == 1:
            return parts[0]
        return None

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        repo_info = self._parse_repo(url)
        site_dir = self.site_dir_for(url)
        result = CrawlResult(url=verified.url, extractor=self.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain="github.com",
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        # Handle org/user URLs (no repo) — find main repo or crawl generically
        org_name = self._is_org_url(url)
        if org_name and not repo_info:
            try:
                r = self.client.get(
                    f"https://api.github.com/orgs/{org_name}/repos?sort=stars&per_page=5",
                    headers={**HEADERS, "Accept": "application/vnd.github.v3+json"},
                    timeout=30,
                )
                if r.status_code == 200:
                    repos = r.json()
                    if repos:
                        # Use the top-starred repo
                        top = repos[0]
                        repo_info = (org_name, top["name"])
                        log.info("    GitHub org %s → using top repo: %s", org_name, top["name"])
                if not repo_info:
                    # Try as user
                    r = self.client.get(
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
            result.status = "failed"
            result.error = "Could not parse GitHub repo from URL"
            return result

        owner, repo = repo_info

        # Fetch repo metadata via API
        try:
            r = self.client.get(
                f"https://api.github.com/repos/{owner}/{repo}",
                headers={**HEADERS, "Accept": "application/vnd.github.v3+json"},
                timeout=30,
            )
            if r.status_code == 200:
                repo_data = r.json()
                meta.title = repo_data.get("name", "")
                meta.description = (repo_data.get("description") or "")[:500]
                meta.author = (repo_data.get("owner") or {}).get("login", "")
                lic = repo_data.get("license") or {}
                meta.license = lic.get("spdx_id", "")
                default_branch = repo_data.get("default_branch", "main")
            else:
                meta.title = repo
                meta.author = owner
                default_branch = "main"
            time.sleep(0.5)
        except Exception:
            meta.title = repo
            meta.author = owner
            default_branch = "main"

        # Download repo as ZIP
        artifacts_dir = site_dir / "artifacts"
        artifacts_dir.mkdir(exist_ok=True)
        zip_url = f"https://github.com/{owner}/{repo}/archive/refs/heads/{default_branch}.zip"
        zip_path = artifacts_dir / f"{repo}-{default_branch}.zip"

        if _download_file(self.client, zip_url, zip_path):
            fsize = zip_path.stat().st_size
            meta.artifacts.append({
                "filename": f"artifacts/{zip_path.name}",
                "type": "zip",
                "size_bytes": fsize,
            })
            total_size += fsize
        time.sleep(0.5)

        # Fetch README for display
        for readme_name in ("README.md", "readme.md", "README.rst", "README"):
            try:
                r = self.client.get(
                    f"https://raw.githubusercontent.com/{owner}/{repo}/{default_branch}/{readme_name}",
                    headers=HEADERS, timeout=15,
                )
                if r.status_code == 200:
                    (site_dir / "README.md").write_text(r.text)
                    break
                time.sleep(0.2)
            except Exception:
                continue

        # Download OG image if available
        try:
            r = self.client.get(url, headers=HEADERS, timeout=15, follow_redirects=True)
            if r.status_code == 200:
                soup = BeautifulSoup(r.text, "html.parser")
                og_img = soup.find("meta", property="og:image")
                if og_img:
                    img_url = og_img.get("content", "")
                    if img_url:
                        images_dir = site_dir / "images"
                        images_dir.mkdir(exist_ok=True)
                        if _download_file(self.client, img_url, images_dir / "social.png"):
                            meta.images.append("images/social.png")
        except Exception:
            pass

        meta.title = meta.title or verified.title or f"{owner}/{repo}"
        _generate_project_page(site_dir, meta)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result


class InstructablesExtractor(BaseExtractor):
    name = "instructables"

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        site_dir = self.site_dir_for(url)
        result = CrawlResult(url=verified.url, extractor=self.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain="instructables.com",
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        try:
            r = self.client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
            r.raise_for_status()
            html = r.text
            soup = BeautifulSoup(html, "html.parser")
        except Exception as e:
            result.status = "failed"
            result.error = str(e)[:200]
            return result

        # Extract metadata
        title_tag = soup.find("title")
        meta.title = title_tag.get_text().replace(" : ", " - ").strip() if title_tag else ""
        og_desc = soup.find("meta", property="og:description")
        if og_desc:
            meta.description = og_desc.get("content", "")[:500]

        # Author
        author_tag = soup.find("a", class_="member-header-display-name") or soup.find("a", attrs={"rel": "author"})
        if author_tag:
            meta.author = author_tag.get_text().strip()

        # Download images
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        img_count = 0
        for img in soup.find_all("img"):
            src = img.get("src", "") or img.get("data-src", "")
            if not src or not src.startswith("http"):
                continue
            if "instructables.com" not in src and "content.instructables.com" not in src:
                continue
            if img_count >= 20:
                break
            ext = Path(urlparse(src).path).suffix or ".jpg"
            fname = f"img_{img_count:02d}{ext}"
            if _download_file(self.client, src, images_dir / fname):
                meta.images.append(f"images/{fname}")
                total_size += (images_dir / fname).stat().st_size
                img_count += 1
            time.sleep(0.2)

        # Extract steps
        steps = []
        step_elements = soup.find_all("div", class_="step")
        if not step_elements:
            # Alternative layout
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
                if _download_file(self.client, href, artifacts_dir / fname):
                    fsize = (artifacts_dir / fname).stat().st_size
                    meta.artifacts.append({
                        "filename": f"artifacts/{fname}",
                        "type": ext.lstrip("."),
                        "size_bytes": fsize,
                    })
                    total_size += fsize
                time.sleep(0.3)

        # Save raw HTML and steps
        (site_dir / "raw.html").write_text(html)
        if steps:
            (site_dir / "steps.json").write_text(json.dumps(steps, ensure_ascii=False, indent=2))

        meta.title = meta.title or verified.title or "Instructables Project"
        _generate_project_page(site_dir, meta, steps=steps)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result


class GenericExtractor(BaseExtractor):
    name = "generic"

    def extract(self, verified: VerifiedLink) -> CrawlResult:
        url = verified.final_url or verified.url
        if verified.status == "wayback" and verified.wayback_url:
            url = verified.wayback_url

        site_dir = self.site_dir_for(verified.url)  # Use original URL for dir
        result = CrawlResult(url=verified.url, extractor=self.name)
        total_size = 0

        meta = ProjectMeta(
            url=verified.url,
            category=verified.category,
            subcategory=verified.subcategory,
            source_domain=urlparse(verified.url).netloc.replace("www.", ""),
            crawled_at=datetime.now(timezone.utc).isoformat(),
            source_status=verified.status,
        )

        try:
            r = self.client.get(url, headers=HEADERS, timeout=30, follow_redirects=True)
            r.raise_for_status()
            html = r.text
        except Exception as e:
            result.status = "failed"
            result.error = str(e)[:200]
            return result

        soup = BeautifulSoup(html, "html.parser")

        # Extract metadata from HTML
        title_tag = soup.find("title")
        meta.title = title_tag.get_text().strip()[:200] if title_tag else ""
        og_desc = soup.find("meta", property="og:description") or soup.find("meta", attrs={"name": "description"})
        if og_desc:
            meta.description = og_desc.get("content", "")[:500]
        author_meta = soup.find("meta", attrs={"name": "author"})
        if author_meta:
            meta.author = author_meta.get("content", "")

        # Download images (limit to same domain, max 15)
        images_dir = site_dir / "images"
        images_dir.mkdir(exist_ok=True)
        base_domain = urlparse(url).netloc
        img_count = 0
        for img in soup.find_all("img"):
            if img_count >= 15:
                break
            src = img.get("src", "") or img.get("data-src", "")
            if not src:
                continue
            if not src.startswith("http"):
                src = urljoin(url, src)
            # Only download from same domain or CDN
            src_domain = urlparse(src).netloc
            if base_domain.replace("www.", "") not in src_domain.replace("www.", ""):
                continue
            ext = Path(urlparse(src).path).suffix.lower()
            if ext not in IMAGE_EXTS:
                continue
            fname = f"img_{img_count:02d}{ext}"
            if _download_file(self.client, src, images_dir / fname):
                meta.images.append(f"images/{fname}")
                total_size += (images_dir / fname).stat().st_size
                img_count += 1
            time.sleep(0.2)

        # Download artifact links from page
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
                if _download_file(self.client, href, artifacts_dir / fname):
                    fsize = (artifacts_dir / fname).stat().st_size
                    meta.artifacts.append({
                        "filename": f"artifacts/{fname}",
                        "type": ext.lstrip("."),
                        "size_bytes": fsize,
                    })
                    total_size += fsize
                time.sleep(0.3)

        # Save raw HTML
        (site_dir / "raw.html").write_text(html)

        meta.title = meta.title or verified.title or urlparse(verified.url).path.strip("/")
        _generate_project_page(site_dir, meta)
        _save_meta(site_dir, meta)

        result.status = "completed"
        result.title = meta.title
        result.files = len(meta.artifacts) + len(meta.images)
        result.size_bytes = total_size
        result.ts = datetime.now(timezone.utc).isoformat()
        return result


# ── Stage 3: Crawl — Dispatcher ───────────────────────────────────────────


EXTRACTORS: dict[str, type[BaseExtractor]] = {
    "printables": PrintablesExtractor,
    "thingiverse": ThingiverseExtractor,
    "github": GitHubExtractor,
    "instructables": InstructablesExtractor,
    "generic": GenericExtractor,
}


CRAWL_WORKERS = 6  # total concurrent crawl threads

# Max concurrent crawls per domain — rate-limited sites get 1
DOMAIN_CONCURRENCY: dict[str, int] = {
    "printables": 2,
    "thingiverse": 1,
    "github": 2,
    "instructables": 2,
    "generic": 4,  # spread across many different domains
}


def stage_crawl(
    state: PipelineState,
    verified: list[VerifiedLink],
    retry_failed: bool = False,
    printables_token: str = "",
    thingiverse_token: str = "",
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

    # Per-extractor semaphores for rate limiting
    ext_sems: dict[str, threading.Semaphore] = {
        name: threading.Semaphore(DOMAIN_CONCURRENCY.get(name, 2))
        for name in EXTRACTORS
    }
    progress_lock = threading.Lock()
    counter = {"done": 0}

    with httpx.Client(timeout=60, follow_redirects=True) as client:
        extractors: dict[str, BaseExtractor] = {}
        for name, cls in EXTRACTORS.items():
            if name == "printables":
                extractors[name] = cls(client, state.sites_dir, auth_token=printables_token)
            elif name == "thingiverse":
                extractors[name] = cls(client, state.sites_dir, api_token=thingiverse_token)
            else:
                extractors[name] = cls(client, state.sites_dir)

        def _crawl_one(v: VerifiedLink) -> CrawlResult:
            # Re-classify at crawl time (handles fixes to classifier)
            ext_name = _classify_extractor(v.url)
            extractor = extractors.get(ext_name, extractors["generic"])
            sem = ext_sems.get(ext_name, ext_sems["generic"])
            attempts = progress.get(v.url, {}).get("attempts", 0) + 1

            with sem:  # per-domain rate limit
                try:
                    result = extractor.extract(v)
                    result.attempts = attempts
                except Exception as e:
                    log.error("    FAILED: %s", e)
                    result = CrawlResult(
                        url=v.url, status="failed", extractor=ext_name,
                        error=str(e)[:200], attempts=attempts,
                        ts=datetime.now(timezone.utc).isoformat(),
                    )

            # Post-crawl optimization
            if optimize and result.status == "completed":
                site_dir = extractor.site_dir_for(v.final_url or v.url)
                meta_path = site_dir / "meta.json"
                if meta_path.exists():
                    try:
                        meta_data = json.loads(meta_path.read_text())
                        meta_obj = ProjectMeta(**{
                            k: v for k, v in meta_data.items()
                            if k in ProjectMeta.__dataclass_fields__
                        })
                        bytes_saved = _optimize_project(site_dir, meta_obj)
                        if bytes_saved > 0:
                            _save_meta(site_dir, meta_obj)
                            result.size_bytes -= bytes_saved
                            result.files = len(meta_obj.artifacts) + len(meta_obj.images)
                    except Exception as e:
                        log.debug("    optimize error: %s", e)

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


def _generate_project_page(site_dir: Path, meta: ProjectMeta, steps: list[dict] | None = None) -> None:
    """Generate a clean HTML page for a crawled project."""

    images_html = ""
    for img_path in meta.images[:8]:
        images_html += f'<img src="{escape(img_path)}" alt="" loading="lazy">\n'

    artifacts_html = ""
    if meta.artifacts:
        artifacts_html = '<h2>Downloads</h2><ul class="artifacts">\n'
        for art in meta.artifacts:
            fname = Path(art["filename"]).name
            ftype = art.get("type", "").upper()
            fsize = art.get("size_bytes", 0)
            size_str = f"{fsize / 1024:.0f} KB" if fsize < 1_000_000 else f"{fsize / 1_000_000:.1f} MB"
            artifacts_html += f'<li><a href="{escape(art["filename"])}">{escape(fname)}</a> <span class="badge">{ftype}</span> <span class="size">{size_str}</span></li>\n'
        artifacts_html += "</ul>\n"

    steps_html = ""
    if steps:
        steps_html = '<h2>Steps</h2>\n'
        for i, step in enumerate(steps, 1):
            title = step.get("title", f"Step {i}")
            text = step.get("text", "")
            steps_html += f'<div class="step"><h3>{escape(title)}</h3><p>{escape(text[:1000])}</p></div>\n'

    license_html = ""
    if meta.license:
        license_html = f'<p class="license">License: {escape(meta.license)}</p>'

    html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{escape(meta.title)}</title>
<style>{PROJECT_CSS}</style>
</head>
<body>
<nav><a href="../../index.html">Home</a></nav>
<h1>{escape(meta.title)}</h1>
<p class="meta">
  <span class="author">{escape(meta.author)}</span>
  <span class="domain">{escape(meta.source_domain)}</span>
  <span class="source"><a href="{escape(meta.url)}">Original</a></span>
</p>
{license_html}
<p class="description">{escape(meta.description)}</p>
<div class="gallery">{images_html}</div>
{artifacts_html}
{steps_html}
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
IMAGE_MAX_DIM = 1200
IMAGE_JPEG_QUALITY = 85


def _optimize_project(site_dir: Path, meta: ProjectMeta) -> int:
    """Optimize a crawled project's files in-place. Returns bytes saved."""
    saved = 0
    has_exts = {p.suffix.lower() for p in site_dir.rglob("*") if p.is_file()}

    # 1. Drop always-unwanted files
    for f in list(site_dir.rglob("*")):
        if f.is_file() and f.suffix.lower() in DROP_ALWAYS:
            size = f.stat().st_size
            f.unlink()
            saved += size
            log.debug("    drop %s (%d KB)", f.name, size // 1024)

    # 2. Drop gcode when STL/3MF exists
    for ext, alternatives in DROP_IF_REDUNDANT.items():
        if has_exts & alternatives:
            for f in list(site_dir.rglob(f"*{ext}")):
                size = f.stat().st_size
                f.unlink()
                saved += size
                log.debug("    drop redundant %s (%d KB)", f.name, size // 1024)

    # 3. Drop non-essential CAD formats
    for f in list(site_dir.rglob("*")):
        if f.is_file() and f.suffix.lower() in DROP_CAD_NON_ESSENTIAL:
            size = f.stat().st_size
            f.unlink()
            saved += size
            log.debug("    drop CAD %s (%d KB)", f.name, size // 1024)

    # 4. GIFs > 500KB → extract first frame as JPG
    try:
        from PIL import Image as PILImage
        for f in list(site_dir.rglob("*.gif")):
            if f.stat().st_size > 500_000:
                try:
                    old_size = f.stat().st_size
                    img = PILImage.open(f)
                    img = img.convert("RGB")
                    jpg_path = f.with_suffix(".jpg")
                    img.save(str(jpg_path), "JPEG", quality=IMAGE_JPEG_QUALITY)
                    new_size = jpg_path.stat().st_size
                    f.unlink()
                    saved += old_size - new_size
                    # Update meta references
                    old_rel = str(f.relative_to(site_dir))
                    new_rel = str(jpg_path.relative_to(site_dir))
                    meta.images = [new_rel if i == old_rel else i for i in meta.images]
                except Exception:
                    pass

        # 5. Resize large images
        for f in list(site_dir.rglob("*")):
            if f.suffix.lower() not in {".jpg", ".jpeg", ".png"}:
                continue
            if f.stat().st_size < 50_000:  # skip tiny images
                continue
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

    # 6. Update meta.json artifacts list (remove deleted files)
    meta.artifacts = [
        a for a in meta.artifacts
        if not a.get("download_failed")
        and (site_dir / a["filename"]).exists()
    ]
    # Update sizes
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
.credit { margin-top: 40px; padding: 16px; background: #f9fafb; border-radius: 8px;
  font-size: 13px; color: #666; line-height: 1.6; }
.credit a { color: #2563eb; }
a { color: #2563eb; text-decoration: none; }
nav { margin-bottom: 16px; }
"""


def _make_index_page(
    by_category: dict[str, list[dict]],
    total_projects: int,
    total_artifacts: int,
    dead_count: int,
) -> str:
    stats = f"""
    <div class="stats">
      <div class="stat"><span class="stat-num">{total_projects}</span><span class="stat-label">Projects Archived</span></div>
      <div class="stat"><span class="stat-num">{total_artifacts}</span><span class="stat-label">Downloadable Files</span></div>
      <div class="stat"><span class="stat-num">{len(by_category)}</span><span class="stat-label">Categories</span></div>
      <div class="stat"><span class="stat-num">{dead_count}</span><span class="stat-label">Dead Links</span></div>
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
    for v in sorted(dead, key=lambda x: x.url):
        wb = ""
        if v.wayback_url:
            wb = f'<a href="{escape(v.wayback_url)}">Wayback</a>'
        rows += f"<tr><td>{escape(v.url)}</td><td>{v.http_status}</td><td>{wb}</td></tr>\n"

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
<tr><th>URL</th><th>Status</th><th>Wayback</th></tr>
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
    verified = stage_verify(state, links)
    crawl_results = stage_crawl(
        state, verified,
        retry_failed=args.retry_failed,
        printables_token=printables_token,
        thingiverse_token=thingiverse_token,
        optimize=not args.do_not_optimize,
    )
    stage_package(state, verified, crawl_results, output_path)

    state.stage = "done"
    state.save()
    log.info("All done! ZIM at %s", output_path)


if __name__ == "__main__":
    main()
