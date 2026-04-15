import platform as host_platform
from pathlib import Path

import pytest

import svalbard.toolkit_generator as toolkit_generator


@pytest.fixture(autouse=True)
def fake_drive_runtime_binaries(monkeypatch, tmp_path_factory):
    root = tmp_path_factory.mktemp("drive-runtime-binaries")
    binaries = {}
    for platform in ("macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64"):
        binary = root / platform / "svalbard-drive"
        binary.parent.mkdir(parents=True, exist_ok=True)
        binary.write_text("#!/usr/bin/env sh\nexit 0\n")
        binary.chmod(0o755)
        binaries[platform] = binary

    def _filter_binaries(platform_filter=None):
        if not platform_filter:
            return binaries

        normalized = platform_filter
        if platform_filter == "host":
            system = host_platform.system().lower()
            machine = host_platform.machine().lower()
            if system == "darwin":
                normalized = "macos-arm64" if machine in {"arm64", "aarch64"} else "macos-x86_64"
            else:
                normalized = "linux-arm64" if machine in {"arm64", "aarch64"} else "linux-x86_64"

        if normalized in {"arm64", "x86_64"}:
            return {
                platform: binary
                for platform, binary in binaries.items()
                if platform.endswith(f"-{normalized}")
            }

        return {normalized: binaries[normalized]}

    monkeypatch.setattr(toolkit_generator, "_build_drive_runtime_binaries", _filter_binaries, raising=False)
    return binaries
