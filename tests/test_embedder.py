"""Tests for svalbard.embedder — pure-function tests only.

start_embedding_server and embed_batch require a running llama-server
and are NOT tested here.
"""

import struct

from svalbard.embedder import vectors_to_blob


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
