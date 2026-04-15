"""Tests for svalbard.embedder — pure-function tests only."""

import struct
from pathlib import Path

from svalbard.embedder import _find_llama_server, vectors_to_blob


def test_vectors_to_blob():
    """vectors_to_blob packs float vectors as float32 blobs that round-trip."""
    vectors = [
        [1.0, 2.0, 3.0],
        [0.0, -1.5, 0.25],
    ]
    blobs = vectors_to_blob(vectors)

    assert len(blobs) == 2

    # Each blob should be 3 floats * 4 bytes = 12 bytes
    for blob in blobs:
        assert len(blob) == 12

    # Unpack and verify values round-trip
    unpacked_0 = list(struct.unpack("<3f", blobs[0]))
    assert unpacked_0 == [1.0, 2.0, 3.0]

    unpacked_1 = list(struct.unpack("<3f", blobs[1]))
    assert unpacked_1[0] == 0.0
    assert unpacked_1[1] == -1.5
    assert unpacked_1[2] == 0.25


def test_vectors_to_blob_empty():
    """vectors_to_blob with an empty list returns an empty list."""
    assert vectors_to_blob([]) == []


def test_vectors_to_blob_single_dim():
    """vectors_to_blob handles single-dimensional vectors."""
    blobs = vectors_to_blob([[42.0]])
    assert len(blobs) == 1
    assert struct.unpack("<1f", blobs[0]) == (42.0,)


def test_find_llama_server_prefers_tool_specific_platform_dir(tmp_path, monkeypatch):
    """_find_llama_server should resolve the executable inside bin/{platform}/llama-server/."""
    tool_dir = tmp_path / "bin" / "macos-arm64" / "llama-server"
    tool_dir.mkdir(parents=True)
    binary = tool_dir / "llama-server"
    binary.write_text("#!/bin/sh\nexit 0\n")
    binary.chmod(0o755)

    monkeypatch.setattr("svalbard.embedder._platform.system", lambda: "Darwin")
    monkeypatch.setattr("svalbard.embedder._platform.machine", lambda: "arm64")
    monkeypatch.setattr("svalbard.embedder.shutil.which", lambda _: None)

    assert _find_llama_server(tmp_path) == str(binary)
