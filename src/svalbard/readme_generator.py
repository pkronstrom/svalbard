"""Generate a README.md for the drive explaining its contents and usage."""

from collections import defaultdict
from pathlib import Path

from svalbard.manifest import Manifest
from svalbard.models import Source
from svalbard.presets import load_preset


def _generate_license_section(sources: list[Source]) -> list[str]:
    """Build a Licenses & Attribution section grouped by license id."""
    # Group sources by license id
    by_license: dict[str, list[Source]] = defaultdict(list)
    unlicensed: list[Source] = []

    for source in sources:
        if source.license and source.license.id:
            by_license[source.license.id].append(source)
        else:
            unlicensed.append(source)

    if not by_license and not unlicensed:
        return []

    lines = [
        "## Licenses & Attribution",
        "",
        "All content on this drive is redistributed under the terms of its",
        "original license. This drive was assembled using Svalbard",
        "(AGPL-3.0 with Commons Clause — not for commercial redistribution).",
        "",
    ]

    # Sort license groups: NC licenses last, then alphabetical
    def _sort_key(license_id: str) -> tuple[int, str]:
        nc = any(s.license and s.license.noncommercial for s in by_license[license_id])
        return (1 if nc else 0, license_id)

    for license_id in sorted(by_license, key=_sort_key):
        group = by_license[license_id]
        nc = any(s.license and s.license.noncommercial for s in group)
        header = f"### {license_id}"
        if nc:
            header += " (NonCommercial)"
        lines.append(header)
        lines.append("")
        for source in sorted(group, key=lambda s: s.id):
            attribution = source.license.attribution if source.license else ""
            line = f"- **{source.description or source.id}**"
            if attribution:
                line += f" — {attribution}"
            lines.append(line)
        lines.append("")

    if unlicensed:
        lines.append("### Other")
        lines.append("")
        for source in sorted(unlicensed, key=lambda s: s.id):
            lines.append(f"- {source.description or source.id}")
        lines.append("")

    return lines


def generate_drive_readme(drive_path: Path) -> Path:
    """Write README.md to the drive root based on the manifest."""
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    lines = [
        f"# Svalbard Drive — {manifest.preset}",
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
        "| `maps/`    | PMTiles — offline map tiles when a preset includes them |",
        "| `books/`   | PDF and EPUB — books and documents              |",
        "| `models/`  | GGUF — local language models                    |",
        "| `apps/`    | Standalone web apps (CyberChef, etc.)           |",
        "| `bin/`     | Server binaries in platform folders such as `bin/macos-arm64/` |",
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
    ]

    lines.extend(_generate_license_section(preset.sources))

    lines.extend([
        "---",
        "",
        "**No internet is required.** All content on this drive works fully",
        "offline. Just plug in and go.",
        "",
    ])

    dest = drive_path / "README.md"
    dest.write_text("\n".join(lines))
    return dest
