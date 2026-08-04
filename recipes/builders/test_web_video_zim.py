#!/usr/bin/env python3
"""Self-check for web-video-zim discovery and rewriting.

Run: python3 test_web_video_zim.py   (no test framework needed)
"""

import importlib.util
import sys
import tempfile
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "wvz", Path(__file__).parent / "web-video-zim.py"
)
wvz = importlib.util.module_from_spec(spec)
sys.modules["wvz"] = wvz
spec.loader.exec_module(wvz)

VID = "ywljr9RKExQ"


def test_discovery():
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        (mirror / "a.html").write_text(
            f'<iframe width="560" src="https://www.youtube.com/embed/{VID}?rel=0"'
            f' frameborder="0"></iframe>'
        )
        (mirror / "sub").mkdir()
        # Same video via a watch link on another page — must dedupe to one id.
        (mirror / "sub" / "b.html").write_text(
            f'<a href="https://youtu.be/{VID}">watch</a>'
            f'<a href="https://www.youtube.com/watch?v=aaaaaaaaaaa">other</a>'
        )
        found = wvz.discover(mirror)
        assert set(found) == {VID, "aaaaaaaaaaa"}, found
        assert len(found[VID]) == 2, "video referenced on two pages"


def test_rewrite_replaces_embed_and_keeps_source():
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        page = mirror / "p.html"
        page.write_text(f'<div><iframe src="https://www.youtube.com/embed/{VID}"></iframe></div>')
        wvz.rewrite(page, mirror, {VID: Path("x.mp4")})
        out = page.read_text()
        assert "<video controls" in out, out
        assert f"_svalbard/media/videos/{VID}.mp4" in out, out
        assert "<iframe" not in out, out
        # Provenance must survive offline — assert the href itself, not just the
        # visible text, or a rewritten link slips through unnoticed.
        assert f'href="https://www.youtube.com/watch?v={VID}"' in out, out


def test_rewrite_leaves_failed_download_remote():
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        page = mirror / "p.html"
        original = f'<iframe src="https://www.youtube.com/embed/{VID}"></iframe>'
        page.write_text(original)
        wvz.rewrite(page, mirror, {})  # nothing downloaded
        assert page.read_text() == original


def test_rewrite_uses_relative_prefix_in_subdir():
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        (mirror / "deep").mkdir()
        page = mirror / "deep" / "p.html"
        page.write_text(f'<iframe src="https://www.youtube.com/embed/{VID}"></iframe>')
        wvz.rewrite(page, mirror, {VID: Path("x.mp4")})
        assert "../_svalbard/media/videos/" in page.read_text()


def test_iframe_and_watch_link_on_same_page():
    """Both rewrites must coexist: the link localises, the provenance stays remote."""
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        page = mirror / "p.html"
        page.write_text(
            f'<iframe src="https://www.youtube.com/embed/{VID}"></iframe>'
            f'<a href="https://www.youtube.com/watch?v={VID}">also watch</a>'
        )
        wvz.rewrite(page, mirror, {VID: Path("x.mp4")})
        out = page.read_text()
        assert f'href="https://www.youtube.com/watch?v={VID}"' in out, out
        assert f'href="_svalbard/media/videos/{VID}.mp4"' in out, out
        assert "<video controls" in out, out


def test_watch_link_rewritten_to_local_file():
    with tempfile.TemporaryDirectory() as tmp:
        mirror = Path(tmp)
        page = mirror / "p.html"
        page.write_text(f'<a href="https://www.youtube.com/watch?v={VID}">see</a>')
        wvz.rewrite(page, mirror, {VID: Path("x.mp4")})
        assert f'href="_svalbard/media/videos/{VID}.mp4"' in page.read_text()


if __name__ == "__main__":
    for name, fn in sorted(globals().items()):
        if name.startswith("test_"):
            fn()
            print(f"ok  {name}")
    print("all passed")
