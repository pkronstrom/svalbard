"""Audit report generator with baked-in LLM prompt for gap analysis."""

import shutil
from datetime import datetime
from pathlib import Path

from svalbard.drive_config import load_snapshot_preset
from svalbard.manifest import Manifest
from svalbard.models import Source
from svalbard.presets import load_preset
from svalbard.taxonomy import compute_coverage, load_taxonomy

FORMAT_ACCESSIBILITY = """| Format | macOS | iOS | Android | Linux | Viewer on drive? |
|--------|-------|-----|---------|-------|-------------------|
| ZIM    | ✓     | ✓   | ✓       | ✓     | ✓ kiwix-serve     |
| PMTiles| ✓     | ✓   | ✓       | ✓     | ✓ go-pmtiles      |
| PDF    | ✓     | ✓   | ✓       | ✓     | ✗ OS built-in     |
| EPUB   | ✓     | ✓   | ✓       | ~     | ✗ OS built-in     |
| GGUF   | ✓     | ✗   | ✗       | ✓     | ✓ llama-server    |
| HTML   | ✓     | ✓   | ✓       | ✓     | ✓ native          |
| WebM   | ✓     | ✓   | ✓       | ✓     | ✓ via kiwix-serve |"""


def generate_audit(drive_path: Path) -> str:
    """Generate a markdown audit report for AI analysis."""
    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_snapshot_preset(drive_path) or load_preset(
        manifest.preset,
        workspace=manifest.workspace_root or None,
    )
    taxonomy = load_taxonomy()

    try:
        usage = shutil.disk_usage(drive_path)
        total_gb = usage.total / 1e9
        used_gb = usage.used / 1e9
        free_gb = usage.free / 1e9
    except OSError:
        total_gb = used_gb = free_gb = 0

    sources = [
        Source(
            id=e.id,
            type=e.type,
            tags=e.tags,
            depth=e.depth,
            size_gb=e.size_bytes / 1e9,
        )
        for e in manifest.entries
    ]
    coverage = compute_coverage(sources, taxonomy)

    lines = []
    lines.append("# Svalbard Audit Report")
    lines.append(f"Generated: {datetime.now().strftime('%Y-%m-%d')}")
    lines.append(f"Preset: {manifest.preset}")
    lines.append(
        f"Drive: {drive_path} ({total_gb:.0f}GB total, {used_gb:.0f}GB used, {free_gb:.0f}GB free)"
    )
    lines.append("")
    lines.append("## System Prompt for AI Analysis")
    lines.append("")
    lines.append("You are analyzing an offline knowledge kit designed for survival and")
    lines.append("civilization rebuilding scenarios, with a Nordic/Finnish focus.")
    lines.append("The kit must be usable with:")
    lines.append("- MacBook (M4, macOS, 128GB RAM)")
    lines.append("- iPhone/iPad (iOS, Kiwix reader)")
    lines.append("- Android phone (Kiwix, OsmAnd)")
    lines.append("- Any x86/ARM Linux machine")
    lines.append("")
    lines.append("Analyze the inventory below and identify:")
    lines.append(
        "1. Critical knowledge gaps for survival (Nordic climate, -30\u00b0C winters)"
    )
    lines.append(
        "2. Knowledge gaps for rebuilding (agriculture, manufacturing, governance)"
    )
    lines.append("3. Missing practical formats (theory but no step-by-step guides)")
    lines.append(
        "4. Accessibility gaps (content that can't be opened without specific software)"
    )
    lines.append("5. Redundancies worth eliminating to free space")
    lines.append(
        "6. Specific freely-available resources that would fill the top 10 gaps"
    )
    lines.append("7. Regional blind spots (Nordic flora, fauna, building codes, law)")
    lines.append(f"\nAvailable free space: {free_gb:.0f} GB")
    lines.append("")
    lines.append("## Inventory")
    lines.append("")
    lines.append("| ID | Type | Size | Tags | Depth | Downloaded |")
    lines.append("|----|------|------|------|-------|------------|")
    for e in sorted(manifest.entries, key=lambda x: x.id):
        size = (
            f"{e.size_bytes / 1e9:.1f} GB"
            if e.size_bytes > 1e9
            else f"{e.size_bytes / 1e6:.0f} MB"
        )
        tags = ", ".join(e.tags[:5])
        date = e.downloaded[:10] if e.downloaded else "\u2014"
        lines.append(
            f"| {e.id} | {e.type} | {size} | {tags} | {e.depth} | {date} |"
        )
    lines.append("")
    lines.append("## Coverage Matrix")
    lines.append("")
    lines.append("| Domain | Group | Score | Sources | Gaps |")
    lines.append("|--------|-------|-------|---------|------|")
    for c in sorted(coverage, key=lambda x: x.score):
        bar = "\u2588" * (c.score // 10) + "\u2591" * (10 - c.score // 10)
        gap = ""
        if c.score == 0:
            gap = "\u2717 No sources"
        elif c.score < 30:
            gap = "\u26a0 Weak coverage"
        lines.append(
            f"| {c.domain} | {c.group} | {bar} {c.score}% | {len(c.sources)} | {gap} |"
        )
    lines.append("")
    lines.append("## Format Accessibility Matrix")
    lines.append("")
    lines.append(FORMAT_ACCESSIBILITY)
    lines.append("")
    return "\n".join(lines)
