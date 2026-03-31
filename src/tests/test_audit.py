from pathlib import Path

from svalbard.audit import generate_audit
from svalbard.commands import init_drive
from svalbard.manifest import Manifest, ManifestEntry


def test_generate_audit_has_sections(tmp_path):
    """Audit report should contain all expected sections."""
    init_drive(str(tmp_path), "finland-128")

    # Add a fake entry so inventory isn't empty
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    manifest.entries.append(ManifestEntry(
        id="wikimed",
        type="zim",
        filename="wikimed.zim",
        size_bytes=2_000_000_000,
        tags=["medicine"],
        depth="comprehensive",
        downloaded="2026-01-01T00:00:00",
        url="https://example.com/wikimed.zim",
    ))
    manifest.save(tmp_path / "manifest.yaml")

    report = generate_audit(tmp_path)
    assert "# Svalbard Audit Report" in report
    assert "## Inventory" in report
    assert "## Coverage Matrix" in report
    assert "## Format Accessibility Matrix" in report
    assert "wikimed" in report


def test_generate_audit_empty_drive(tmp_path):
    """Audit should work even with no downloaded content."""
    init_drive(str(tmp_path), "finland-128")
    report = generate_audit(tmp_path)
    assert "# Svalbard Audit Report" in report
    assert "## Coverage Matrix" in report
