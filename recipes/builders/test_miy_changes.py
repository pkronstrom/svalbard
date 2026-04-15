#!/usr/bin/env python3
"""Smoke-test the changed MIY extractors and optimization without a full build.

Tests:
  1. Cults3D extractor (fixed JSON-LD + CDN domains)
  2. Ana-White extractor (new)
  3. Hackaday.io extractor (new)
  4. PNG→WebP optimization
  5. Quality badges + logging in HTML generation
  6. Dead links link on index page

Usage:
    python3 recipes/builders/test_miy_changes.py
"""
import json
import sys
import tempfile
import types
from pathlib import Path

# Stub libzim (C extension) so the builder module can import without it
if "libzim" not in sys.modules:
    _lz = types.ModuleType("libzim")
    _lzw = types.ModuleType("libzim.writer")
    class _StubBase:
        def __init_subclass__(cls, **kw): super().__init_subclass__(**kw)
    _lzw.Creator = type("Creator", (), {"__init__": lambda *a, **k: None})
    _lzw.Item = _StubBase
    _lzw.StringProvider = _StubBase
    _lzw.FileProvider = _StubBase
    _lzw.Hint = type("Hint", (), {"FRONT_ARTICLE": 1})
    sys.modules["libzim"] = _lz
    sys.modules["libzim.writer"] = _lzw

# Add the builder to path
sys.path.insert(0, str(Path(__file__).parent))

import importlib
_mod = importlib.import_module("makeityourself-zim")
SITE_CONFIGS = _mod.SITE_CONFIGS
VerifiedLink = _mod.VerifiedLink
SiteScraper = _mod.SiteScraper
_quality_score = _mod._quality_score
_optimize_project = _mod._optimize_project
_generate_project_page = _mod._generate_project_page
_make_index_page = _mod._make_index_page
_make_category_page = _mod._make_category_page
ProjectMeta = _mod.ProjectMeta
IMAGE_MAX_DIM = _mod.IMAGE_MAX_DIM
IMAGE_JPEG_QUALITY = _mod.IMAGE_JPEG_QUALITY
EXTRACTOR_MAP = _mod.EXTRACTOR_MAP
import httpx

# One test URL per changed extractor
TEST_URLS = {
    "cults3d":  "https://cults3d.com/en/3d-model/home/simple-wall-hook",
    "anawhite": "https://www.ana-white.com/community-projects/diy-set-firepit-chairs",
    "hackaday": "https://hackaday.io/project/12876-chipwhisperer-security-research",
}

PASS = "\033[32m✓\033[0m"
FAIL = "\033[31m✗\033[0m"


def test_extractor(name: str, url: str) -> bool:
    """Test a single extractor against a real URL."""
    config = SITE_CONFIGS.get(name)
    if not config:
        print(f"  {FAIL} {name}: config not found in SITE_CONFIGS")
        return False

    with tempfile.TemporaryDirectory() as tmpdir:
        sites_dir = Path(tmpdir) / "sites"
        sites_dir.mkdir()

        with httpx.Client(timeout=30, follow_redirects=True) as client:
            scraper = SiteScraper(config, client, sites_dir)
            verified = VerifiedLink(url=url, page=0, status="alive", extractor=name)

            try:
                result = scraper.extract(verified)
            except Exception as e:
                print(f"  {FAIL} {name}: extract raised {e}")
                return False

        if result.status != "completed":
            print(f"  {FAIL} {name}: status={result.status} error={result.error}")
            return False

        site_dir = list(sites_dir.rglob("meta.json"))
        if not site_dir:
            print(f"  {FAIL} {name}: no meta.json produced")
            return False

        meta = json.loads(site_dir[0].read_text())
        title = meta.get("title", "")
        n_images = len(meta.get("images", []))
        n_arts = len(meta.get("artifacts", []))
        desc_len = len(meta.get("description", ""))

        print(f"  {PASS} {name}: \"{title[:50]}\" — {n_images} images, {n_arts} artifacts, desc {desc_len} chars")

        # Quality score
        quality = _quality_score(site_dir[0].parent)
        print(f"    score={quality['score']}/8, issues={quality.get('issues', [])}")
        return True


def test_optimization() -> bool:
    """Test PNG→WebP conversion and image resizing."""
    try:
        from PIL import Image as PILImage
    except ImportError:
        print(f"  {FAIL} optimization: Pillow not installed")
        return False

    with tempfile.TemporaryDirectory() as tmpdir:
        site_dir = Path(tmpdir)
        images_dir = site_dir / "images"
        images_dir.mkdir()

        # Create a test PNG (200x200 red square, ~2KB)
        img = PILImage.new("RGB", (200, 200), color="red")
        png_path = images_dir / "test_00.png"
        img.save(str(png_path), "PNG")
        png_size = png_path.stat().st_size

        # Create a large JPEG (1600x1200, must be >= 50KB to trigger resize)
        import random
        big = PILImage.new("RGB", (1600, 1200))
        pixels = big.load()
        for y in range(1200):
            for x in range(1600):
                pixels[x, y] = (random.randint(0, 255), random.randint(0, 255), random.randint(0, 255))
        jpg_path = images_dir / "test_01.jpg"
        big.save(str(jpg_path), "JPEG", quality=95)

        meta = ProjectMeta(
            url="https://example.com/test",
            title="Test",
            images=["images/test_00.png", "images/test_01.jpg"],
        )

        # Write meta.json so _quality_score works
        (site_dir / "meta.json").write_text(json.dumps({
            "title": "Test", "description": "A test project", "images": meta.images, "artifacts": [],
        }))

        saved = _optimize_project(site_dir, meta)

        # Check PNG→WebP conversion
        webp_path = images_dir / "test_00.webp"
        if webp_path.exists() and not png_path.exists():
            print(f"  {PASS} PNG→WebP: {png_size}→{webp_path.stat().st_size} bytes")
        else:
            print(f"  {FAIL} PNG→WebP: webp={webp_path.exists()}, png_still={png_path.exists()}")
            return False

        # Check image references updated
        if "images/test_00.webp" in meta.images:
            print(f"  {PASS} meta.images updated to .webp reference")
        else:
            print(f"  {FAIL} meta.images not updated: {meta.images}")
            return False

        # Check JPEG was resized (max dim should be 800)
        if jpg_path.exists():
            resized = PILImage.open(jpg_path)
            w, h = resized.size
            if max(w, h) <= IMAGE_MAX_DIM:
                print(f"  {PASS} JPEG resized: 1600x1200 → {w}x{h} (max={IMAGE_MAX_DIM})")
            else:
                print(f"  {FAIL} JPEG not resized: {w}x{h}")
                return False
        print(f"  {PASS} optimization saved {saved} bytes total")
        return True


def test_html_generation() -> bool:
    """Test quality badges on cards and dead links link."""
    # Test dead links link
    html = _make_index_page({"Tools": [{"title": "T"}]}, 100, 50, 10)
    if 'href="dead/index.html"' in html:
        print(f"  {PASS} index page: dead links clickable when count > 0")
    else:
        print(f"  {FAIL} index page: no dead links link found")
        return False

    html_zero = _make_index_page({"Tools": [{"title": "T"}]}, 100, 50, 0)
    if 'href="dead/index.html"' not in html_zero:
        print(f"  {PASS} index page: dead links not clickable when count = 0")
    else:
        print(f"  {FAIL} index page: dead links link present with 0 dead")
        return False

    # Test quality badges
    projects = [
        {"title": "High", "description": "", "source_domain": "x.com",
         "images": [], "artifacts": [], "source_status": "alive", "_dir": "x/y",
         "_quality": {"score": 7, "issues": []}},
        {"title": "Good", "description": "", "source_domain": "x.com",
         "images": [], "artifacts": [], "source_status": "alive", "_dir": "x/z",
         "_quality": {"score": 4, "issues": ["no_images"]}},
        {"title": "None", "description": "", "source_domain": "x.com",
         "images": [], "artifacts": [], "source_status": "alive", "_dir": "x/w",
         "_quality": {"score": 0, "issues": ["no_meta"]}},
    ]
    cat_html = _make_category_page("Test", projects)
    if "★★★" in cat_html and "★★" in cat_html:
        print(f"  {PASS} category page: quality badges rendered")
    else:
        print(f"  {FAIL} category page: missing quality badges")
        return False

    # Score 0 should have no badge
    if cat_html.count("★") == 5:  # ★★★ (3) + ★★ (2) = 5 stars total
        print(f"  {PASS} category page: no badge for score=0")
    else:
        print(f"  {FAIL} category page: unexpected star count")
        return False

    return True


def test_constants() -> bool:
    """Verify the constant changes."""
    ok = True
    if IMAGE_MAX_DIM == 800:
        print(f"  {PASS} IMAGE_MAX_DIM = 800")
    else:
        print(f"  {FAIL} IMAGE_MAX_DIM = {IMAGE_MAX_DIM} (expected 800)")
        ok = False
    if IMAGE_JPEG_QUALITY == 80:
        print(f"  {PASS} IMAGE_JPEG_QUALITY = 80")
    else:
        print(f"  {FAIL} IMAGE_JPEG_QUALITY = {IMAGE_JPEG_QUALITY} (expected 80)")
        ok = False

    # EXTRACTOR_MAP already imported at module level
    for domain in ["ana-white.com", "www.ana-white.com", "hackaday.io", "www.hackaday.io"]:
        if domain in EXTRACTOR_MAP:
            print(f"  {PASS} {domain} in EXTRACTOR_MAP")
        else:
            print(f"  {FAIL} {domain} missing from EXTRACTOR_MAP")
            ok = False
    return ok


if __name__ == "__main__":
    print("\n=== MIY Changes Smoke Test ===\n")
    results = {}

    print("Constants:")
    results["constants"] = test_constants()

    print("\nHTML generation:")
    results["html"] = test_html_generation()

    print("\nImage optimization:")
    results["optimization"] = test_optimization()

    print("\nExtractors (live network):")
    for name, url in TEST_URLS.items():
        results[name] = test_extractor(name, url)

    print("\n--- Summary ---")
    passed = sum(1 for v in results.values() if v)
    total = len(results)
    for name, ok in results.items():
        print(f"  {PASS if ok else FAIL} {name}")
    print(f"\n{passed}/{total} passed\n")
    sys.exit(0 if passed == total else 1)
