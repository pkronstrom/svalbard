#!/usr/bin/env python3
"""Fetch the Helsinki Pinkka mushroom identification course and build a ZIM file.

Pulls 240 species cards from the Pinkka API (University of Helsinki / LUOMUS),
downloads images from image.laji.fi, generates static HTML pages, and packages
everything into a single ZIM file browsable in Kiwix.

Descriptions: CC0 (public domain)
Images: CC-BY-NC-4.0 (Jouko Rikkinen and others, University of Helsinki)

Requirements: pip install libzim httpx
Usage: python scripts/build-pinkka-sienet-zim.py [--output pinkka-sienet.zim]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import time
from datetime import datetime
from pathlib import Path

import httpx
from libzim.writer import Creator, Item, StringProvider, FileProvider, Hint

API_BASE = "https://fmnh-ws-prod3.it.helsinki.fi/pinkka/api"
COURSE_ID = 6  # BIO-519 Sienituntemus
IMAGE_SIZE = "large"  # large (~800px) is a good balance for offline use

# License IDs from FinBIF mapped to human-readable
LICENSE_MAP = {
    "MZ.intellectualRightsCC-BY-NC-4.0": "CC BY-NC 4.0",
    "MZ.intellectualRightsCC-BY-4.0": "CC BY 4.0",
    "MZ.intellectualRightsCC-BY-SA-4.0": "CC BY-SA 4.0",
    "MZ.intellectualRightsCC0-4.0": "CC0",
}


def fetch_json(path: str) -> dict | list:
    url = f"{API_BASE}/{path}"
    r = httpx.get(url, timeout=30, follow_redirects=True)
    r.raise_for_status()
    return r.json()


def download_image(url: str, dest: Path) -> bool:
    if dest.exists():
        return True
    try:
        with httpx.stream("GET", url, timeout=60, follow_redirects=True) as r:
            r.raise_for_status()
            dest.parent.mkdir(parents=True, exist_ok=True)
            with open(dest, "wb") as f:
                for chunk in r.iter_bytes(65536):
                    f.write(chunk)
        return True
    except Exception as e:
        print(f"  WARN: failed to download {url}: {e}", file=sys.stderr)
        return False


# ── HTML templates ──────────────────────────────────────────────────────────

CSS = """
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  max-width: 900px; margin: 0 auto; padding: 16px; background: #fafafa; color: #222; }
h1 { font-size: 22px; margin-bottom: 4px; }
h2 { font-size: 18px; color: #444; margin-top: 24px; }
h3 { font-size: 15px; color: #666; margin-top: 20px; }
.subtitle { font-size: 16px; color: #666; font-style: italic; margin-bottom: 16px; }
.gallery { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; }
.gallery img { max-width: 280px; max-height: 220px; border-radius: 4px; object-fit: cover; }
.attribution { font-size: 12px; color: #888; }
.section { margin-bottom: 16px; }
.section-body { line-height: 1.6; }
.nav { margin: 16px 0; padding: 12px; background: #f0f0f0; border-radius: 6px; }
.nav a { color: #2563eb; text-decoration: none; margin-right: 16px; }
a { color: #2563eb; }
.species-list { columns: 2; column-gap: 24px; }
.species-list li { margin-bottom: 4px; break-inside: avoid; }
.cat-header { margin-top: 28px; margin-bottom: 8px; font-size: 16px; color: #444;
  border-bottom: 1px solid #ddd; padding-bottom: 4px; }
"""


def make_species_page(card: dict, images_downloaded: dict[str, str], category: str) -> str:
    sci_name = card["scientificName"]
    fi_name = card.get("vernacularName", {}).get("fi", "")
    sv_name = card.get("vernacularName", {}).get("sv", "")

    title = fi_name.capitalize() if fi_name else sci_name
    subtitle_parts = [f"<em>{sci_name}</em>"]
    if sv_name:
        subtitle_parts.append(f"sv: {sv_name}")

    # Images
    gallery_html = ""
    attr_parts = []
    for img in card.get("images", []):
        img_id = img["id"]
        if img_id not in images_downloaded:
            continue
        img_path = images_downloaded[img_id]
        meta = img.get("meta", {})
        owner = meta.get("rightsOwner", "")
        lic = LICENSE_MAP.get(meta.get("license", ""), "")
        gallery_html += f'<img src="../{img_path}" alt="{sci_name}">\n'
        if owner and owner not in [a.split(" (")[0] for a in attr_parts]:
            attr_parts.append(f"{owner} ({lic})" if lic else owner)

    attribution = ""
    if attr_parts:
        attribution = f'<p class="attribution">Photos: {", ".join(attr_parts)}</p>'

    # Description sections
    sections_html = ""
    for section in card.get("description", []):
        section_title = section.get("title", {}).get("fi", "")
        body = section.get("body", {}).get("fi", "")
        if not body:
            body = section.get("body", {}).get("sv", "")
        if body:
            sections_html += f'<div class="section"><h3>{section_title}</h3><div class="section-body">{body}</div></div>\n'

    return f"""<!DOCTYPE html>
<html lang="fi">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title} — Pinkka Sienituntemus</title>
<style>{CSS}</style></head>
<body>
<div class="nav"><a href="../index.html">Etusivu</a><a href="../{_cat_slug(category)}.html">{category}</a></div>
<h1>{title}</h1>
<p class="subtitle">{" · ".join(subtitle_parts)}</p>
<div class="gallery">{gallery_html}</div>
{attribution}
{sections_html}
</body></html>"""


def _cat_slug(name: str) -> str:
    return name.lower().replace(" ", "-").replace("–", "").replace("ä", "a").replace("ö", "o").strip("-")


def make_category_page(name: str, cards: list[dict]) -> str:
    items = ""
    for card in sorted(cards, key=lambda c: c.get("vernacularName", {}).get("fi", c["scientificName"]).lower()):
        fi = card.get("vernacularName", {}).get("fi", "")
        sci = card["scientificName"]
        card_id = card["id"]
        label = f"{fi.capitalize()} (<em>{sci}</em>)" if fi else f"<em>{sci}</em>"
        items += f'<li><a href="species/{card_id}.html">{label}</a></li>\n'

    return f"""<!DOCTYPE html>
<html lang="fi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{name} — Pinkka Sienituntemus</title>
<style>{CSS}</style></head>
<body>
<div class="nav"><a href="index.html">Etusivu</a></div>
<h1>{name}</h1>
<p>{len(cards)} lajia</p>
<ul class="species-list">{items}</ul>
</body></html>"""


def make_index_page(categories: list[tuple[str, str, int]]) -> str:
    total = sum(c[2] for c in categories)
    cat_html = ""
    for name, slug, count in categories:
        cat_html += f'<li><a href="{slug}.html">{name}</a> ({count} lajia)</li>\n'

    return f"""<!DOCTYPE html>
<html lang="fi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pinkka Sienituntemus — Suomen sienet</title>
<style>{CSS}</style></head>
<body>
<h1>Pinkka Sienituntemus</h1>
<p class="subtitle">BIO-519 · Helsingin yliopisto · {total} lajia</p>
<p>Suomen tavallisimmat suursienet — tunnistus, ekologia ja käyttö.</p>
<ul>{cat_html}</ul>
<hr>
<p class="attribution">
Kuvaustekstit: CC0 (vapaa käyttö).<br>
Valokuvat: CC BY-NC 4.0 — tekijänoikeudet kuvaajilla (Jouko Rikkinen ym.).<br>
Lähde: <a href="https://pinkka.laji.fi">Pinkka</a>, Luonnontieteellinen keskusmuseo LUOMUS, Helsingin yliopisto.
</p>
</body></html>"""


def make_license_page() -> str:
    return f"""<!DOCTYPE html>
<html lang="fi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Lisenssi — Pinkka Sienituntemus</title>
<style>{CSS}</style></head>
<body>
<div class="nav"><a href="index.html">Etusivu</a></div>
<h1>Lisenssi ja tekijänoikeudet</h1>

<h2>Kuvaustekstit</h2>
<p>Lajikuvaustekstit ovat vapaasti käytettävissä (CC0, public domain).</p>

<h2>Valokuvat</h2>
<p>Valokuvat on lisensoitu <strong>CC BY-NC 4.0</strong> (Creative Commons
Nimeä-EiKaupallinen 4.0 Kansainvälinen) -lisenssillä. Tekijänoikeudet
kuuluvat kuvaajille. Pääasiallinen kuvaaja: Jouko Rikkinen.</p>
<p>Lisenssin ehdot: <a href="https://creativecommons.org/licenses/by-nc/4.0/deed.fi">
creativecommons.org/licenses/by-nc/4.0</a></p>

<h2>Lähde</h2>
<p>Aineisto on peräisin <a href="https://pinkka.laji.fi">Pinkka</a>-oppimisympäristöstä
(BIO-519 Sienituntemus), Luonnontieteellinen keskusmuseo LUOMUS, Helsingin yliopisto.</p>
<p>Rikkinen J., Virtanen V., Enroth J. & Åström H.: Basic mushroom identification
course — ca. 120 species of Finnish macrofungi.</p>

<h2>Tämä ZIM-tiedosto</h2>
<p>Koottu {datetime.now().strftime("%Y-%m-%d")} svalbard-projektissa offline-käyttöä varten.</p>
</body></html>"""


# ── ZIM writer ──────────────────────────────────────────────────────────────

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
        hints = {Hint.FRONT_ARTICLE: self._is_front}
        return hints


class ImageItem(Item):
    def __init__(self, path: str, filepath: Path, mimetype: str = "image/jpeg"):
        super().__init__()
        self._path = path
        self._filepath = filepath
        self._mimetype = mimetype

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


# ── Main ────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Build Pinkka mushroom ZIM")
    parser.add_argument("--output", "-o", default="pinkka-sienet.zim", help="Output ZIM path")
    parser.add_argument("--cache", default=".cache/pinkka", help="Cache directory for API data and images")
    args = parser.parse_args()

    cache_dir = Path(args.cache)
    img_cache = cache_dir / "images"
    api_cache = cache_dir / "api"
    api_cache.mkdir(parents=True, exist_ok=True)
    img_cache.mkdir(parents=True, exist_ok=True)

    # 1. Fetch course structure
    print("Fetching course structure...")
    course_cache = api_cache / "course.json"
    if course_cache.exists():
        course = json.loads(course_cache.read_text())
    else:
        course = fetch_json(f"pinkkas/{COURSE_ID}")
        course_cache.write_text(json.dumps(course, ensure_ascii=False))

    subpinkkas = course["subPinkkas"]
    print(f"  {len(subpinkkas)} categories")

    # 2. Fetch all species cards
    all_cards: list[tuple[str, dict]] = []  # (category_name, card_data)
    for sub in subpinkkas:
        sub_id = sub["id"]
        sub_name = sub["name"].get("fi", sub["name"].get("en", f"Sub {sub_id}"))
        print(f"  Fetching {sub_name}...")

        sub_cache = api_cache / f"sub_{sub_id}.json"
        if sub_cache.exists():
            sub_data = json.loads(sub_cache.read_text())
        else:
            sub_data = fetch_json(f"subpinkkas/{sub_id}")
            sub_cache.write_text(json.dumps(sub_data, ensure_ascii=False))
            time.sleep(0.2)

        for card_ref in sub_data.get("speciesCards", []):
            card_id = card_ref["id"]
            card_cache = api_cache / f"card_{card_id}.json"
            if card_cache.exists():
                card = json.loads(card_cache.read_text())
            else:
                card = fetch_json(f"speciescards/{card_id}")
                card_cache.write_text(json.dumps(card, ensure_ascii=False))
                time.sleep(0.1)
            all_cards.append((sub_name, card))

    print(f"  {len(all_cards)} species cards total")

    # 3. Download images
    print("Downloading images...")
    images_downloaded: dict[str, str] = {}  # img_id -> relative path in ZIM
    total_images = sum(len(c[1].get("images", [])) for c in all_cards)
    downloaded = 0

    for _, card in all_cards:
        for img in card.get("images", []):
            img_id = img["id"]
            urls = img.get("urls", {})
            url = urls.get(IMAGE_SIZE) or urls.get("full")
            if not url:
                continue

            ext = Path(url).suffix.lower() or ".jpg"
            local_path = img_cache / f"{img_id}{ext}"
            zim_path = f"images/{img_id}{ext}"

            if download_image(url, local_path):
                images_downloaded[img_id] = zim_path
            downloaded += 1
            if downloaded % 50 == 0:
                print(f"  {downloaded}/{total_images} images...")

    print(f"  {len(images_downloaded)} images downloaded")

    # 4. Generate HTML and build ZIM
    print(f"Building ZIM: {args.output}")
    output_path = Path(args.output)

    zim = Creator(str(output_path))
    zim.config_indexing(True, "fi")
    zim.config_clustersize(2048)
    zim.set_mainpath("index.html")

    # Group cards by category
    by_category: dict[str, list[dict]] = {}
    for cat_name, card in all_cards:
        by_category.setdefault(cat_name, []).append(card)

    with zim:
        zim.add_metadata("Title", "Pinkka Sienituntemus — Suomen sienet")
        zim.add_metadata("Description", "240 Finnish mushroom species — identification, ecology, and use (University of Helsinki)")
        zim.add_metadata("Language", "fin")
        zim.add_metadata("Creator", "LUOMUS, University of Helsinki")
        zim.add_metadata("Publisher", "svalbard")
        zim.add_metadata("Date", datetime.now().strftime("%Y-%m-%d"))
        zim.add_metadata("License", "Text: CC0; Images: CC-BY-NC-4.0")
        zim.add_metadata("Tags", "mushrooms;fungi;finland;identification;pinkka")

        # Index page
        categories_info = [
            (name, _cat_slug(name), len(cards))
            for name, cards in by_category.items()
        ]
        zim.add_item(HtmlItem("index.html", "Pinkka Sienituntemus", make_index_page(categories_info), is_front=True))
        zim.add_item(HtmlItem("license.html", "Lisenssi", make_license_page()))

        # Category pages
        for cat_name, cards in by_category.items():
            slug = _cat_slug(cat_name)
            zim.add_item(HtmlItem(f"{slug}.html", cat_name, make_category_page(cat_name, cards), is_front=True))

        # Species pages
        seen_ids = set()
        for cat_name, card in all_cards:
            card_id = card["id"]
            if card_id in seen_ids:
                continue
            seen_ids.add(card_id)

            fi_name = card.get("vernacularName", {}).get("fi", "")
            title = fi_name.capitalize() if fi_name else card["scientificName"]
            html = make_species_page(card, images_downloaded, cat_name)
            zim.add_item(HtmlItem(f"species/{card_id}.html", title, html, is_front=True))

        # Images
        for img_id, zim_path in images_downloaded.items():
            ext = Path(zim_path).suffix.lower()
            mime = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif"}.get(ext, "image/jpeg")
            local_path = img_cache / f"{img_id}{ext}"
            if local_path.exists():
                zim.add_item(ImageItem(zim_path, local_path, mime))

    size_mb = output_path.stat().st_size / 1e6
    print(f"\nDone: {output_path} ({size_mb:.1f} MB)")
    print(f"  {len(seen_ids)} species, {len(images_downloaded)} images")


if __name__ == "__main__":
    main()
