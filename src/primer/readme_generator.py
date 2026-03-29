"""Generate a README.md for the drive explaining its contents and usage."""

from pathlib import Path

from primer.manifest import Manifest


def generate_drive_readme(drive_path: Path) -> Path:
    """Write README.md to the drive root based on the manifest."""
    manifest = Manifest.load(drive_path / "manifest.yaml")

    lines = [
        f"# Primer Drive — {manifest.preset}",
        "",
        f"**Region:** {manifest.region or 'general'}",
        f"**Created:** {manifest.created}",
        "",
        "This drive contains offline reference content that works without any",
        "internet connection. All content is self-contained.",
        "",
        "---",
        "",
        "## Quick Start",
        "",
        "### On a computer (Mac or Linux)",
        "",
        "```bash",
        "cd /path/to/this/drive",
        "./serve.sh",
        "```",
        "",
        "The script will detect your platform, find available content, and let you",
        "choose which services to start. Then open your browser to the URL shown.",
        "",
        "### On iPhone / iPad",
        "",
        "1. Install **Kiwix** from the App Store (it is free)",
        "2. Connect this drive to your device",
        "3. Open Kiwix and tap the folder icon to browse for ZIM files",
        "4. Navigate to the `zim/` folder on this drive",
        "5. Open any `.zim` file to browse its content offline",
        "",
        "### On Android",
        "",
        "1. Install **Kiwix** from Google Play or F-Droid",
        "2. Connect this drive via USB-C or use an OTG adapter",
        "3. Open Kiwix, go to Library, and use the local file picker",
        "4. Select `.zim` files from the `zim/` folder on this drive",
        "",
        "---",
        "",
        "## Directory Contents",
        "",
        "| Folder     | Contents                                      |",
        "| ---------- | --------------------------------------------- |",
        "| `zim/`     | ZIM files — Wikipedia, Wiktionary, guides etc. |",
        "| `maps/`    | PMTiles — offline OpenStreetMap map tiles       |",
        "| `books/`   | PDF and EPUB — books and documents              |",
        "| `models/`  | GGUF — local language models                    |",
        "| `apps/`    | Standalone web apps (CyberChef, etc.)           |",
        "| `bin/`     | Server binaries (kiwix-serve, go-pmtiles)       |",
        "",
        "---",
        "",
        "## Format Reference",
        "",
        "| Format   | What it is                                           |",
        "| -------- | ---------------------------------------------------- |",
        "| ZIM      | Compressed website archive (used by Kiwix)           |",
        "| PMTiles  | Single-file tile archive for maps                    |",
        "| PDF      | Portable Document Format — books and documents       |",
        "| EPUB     | E-book format — readable on most devices             |",
        "| GGUF     | Quantized language model format (for llama.cpp)      |",
        "| HTML     | Web pages — open directly in any browser             |",
        "",
        "---",
        "",
        "**No internet is required.** All content on this drive works fully",
        "offline. Just plug in and go.",
        "",
    ]

    dest = drive_path / "README.md"
    dest.write_text("\n".join(lines))
    return dest
