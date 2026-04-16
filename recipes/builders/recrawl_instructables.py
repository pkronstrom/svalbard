#!/usr/bin/env python3
"""Targeted Playwright re-crawl for Instructables projects.

Re-fetches raw.html with a headless browser to get JS-rendered step content,
then re-extracts steps, author, and any new CDN images.

Usage (Docker):
    docker run --rm -v $(pwd)/library/workspace/miy:/data \
        mcr.microsoft.com/playwright/python:v1.52.0-noble \
        bash -c "pip install beautifulsoup4 httpx && python3 /script/recrawl_instructables.py /data"
"""

from __future__ import annotations

import json
import re
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

from bs4 import BeautifulSoup


def recrawl_project(project_dir: Path) -> dict:
    """Re-crawl a single Instructables project with Playwright."""
    meta_path = project_dir / "meta.json"
    if not meta_path.exists():
        return {"status": "skip", "reason": "no meta.json"}

    meta = json.loads(meta_path.read_text())
    url = meta.get("url", "")
    if not url:
        return {"status": "skip", "reason": "no url"}

    print(f"  Fetching: {url[:70]}...", flush=True)

    from playwright.sync_api import sync_playwright

    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            ctx = browser.new_context(
                viewport={"width": 1280, "height": 800},
                ignore_https_errors=True,
            )
            page = ctx.new_page()
            page.goto(url, wait_until="domcontentloaded", timeout=30000)
            # Wait for JS to render step content
            page.wait_for_timeout(3000)
            # Scroll to trigger lazy-loaded content
            page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
            page.wait_for_timeout(2000)
            page.evaluate("window.scrollTo(0, 0)")
            page.wait_for_timeout(1000)

            html = page.content()
            browser.close()
    except Exception as e:
        return {"status": "error", "reason": str(e)[:200]}

    # Save rendered HTML
    (project_dir / "raw.html").write_text(html)

    # Parse with BeautifulSoup
    soup = BeautifulSoup(html, "html.parser")

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

    if steps:
        (project_dir / "steps.json").write_text(
            json.dumps(steps, ensure_ascii=False, indent=2)
        )

    # Extract author if missing
    if not meta.get("author"):
        author_tag = soup.find("a", class_="member-header-display-name") or soup.find(
            "a", attrs={"rel": "author"}
        )
        if author_tag:
            meta["author"] = author_tag.get_text().strip()

    # Extract description if too short
    if len(meta.get("description", "").strip()) < 50:
        desc_el = soup.find("meta", property="og:description")
        if desc_el and desc_el.get("content"):
            meta["description"] = desc_el["content"][:2000]

    # Check for new CDN images not already downloaded
    images_dir = project_dir / "images"
    existing_images = set(meta.get("images", []))
    img_count = len(existing_images)
    img_urls = re.findall(
        r'https?://content\.instructables\.com/[A-Z0-9/]+\.[a-z]+\?[^"\'&\s<>]+',
        html,
    )
    seen_urls: set[str] = set()
    new_images = 0
    for src in img_urls:
        if img_count >= 20:
            break
        src = re.sub(r"&amp;", "&", src)
        base_path = src.split("?")[0]
        if base_path in seen_urls:
            continue
        seen_urls.add(base_path)
        if "height=620&width=620" in src or "width=320" in src:
            continue
        # Check if we already have this image by checking existing filenames
        ext = Path(urlparse(base_path).path).suffix or ".jpg"
        if ext == ".webp":
            ext = ".jpg"
        fname = f"img_{img_count:02d}{ext}"
        rel_path = f"images/{fname}"
        if rel_path in existing_images:
            continue
        # Download
        try:
            import httpx

            with httpx.Client(timeout=15, follow_redirects=True) as client:
                dl_url = base_path + "?frame=1&width=1024"
                r = client.get(dl_url)
                if r.status_code == 200 and len(r.content) > 1000:
                    images_dir.mkdir(exist_ok=True)
                    (images_dir / fname).write_bytes(r.content)
                    meta.setdefault("images", []).append(rel_path)
                    img_count += 1
                    new_images += 1
        except Exception:
            pass

    # Save updated meta
    meta_path.write_text(json.dumps(meta, ensure_ascii=False, indent=2))

    return {
        "status": "ok",
        "steps": len(steps),
        "new_images": new_images,
        "author": meta.get("author", ""),
    }


def main():
    if len(sys.argv) < 2:
        print("Usage: recrawl_instructables.py <workdir>")
        sys.exit(1)

    workdir = Path(sys.argv[1])
    inst_dir = workdir / "sites" / "instructables.com"
    if not inst_dir.exists():
        print(f"No instructables.com directory at {inst_dir}")
        sys.exit(1)

    projects = sorted(d for d in inst_dir.iterdir() if d.is_dir())
    print(f"Re-crawling {len(projects)} Instructables projects with Playwright...\n")

    results = {"ok": 0, "skip": 0, "error": 0, "total_steps": 0}
    for i, proj in enumerate(projects, 1):
        print(f"[{i}/{len(projects)}] {proj.name}")
        r = recrawl_project(proj)
        results[r["status"]] = results.get(r["status"], 0) + 1
        if r["status"] == "ok":
            results["total_steps"] += r.get("steps", 0)
            print(f"    OK — {r['steps']} steps, {r.get('new_images', 0)} new images")
        else:
            print(f"    {r['status'].upper()}: {r.get('reason', '')}")
        time.sleep(0.5)  # Be polite

    print(f"\nDone: {results['ok']} OK, {results['skip']} skipped, {results['error']} errors")
    print(f"Total steps extracted: {results['total_steps']}")


if __name__ == "__main__":
    main()
