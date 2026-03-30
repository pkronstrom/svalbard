"""Embedding client for llama-server.

Manages a llama-server subprocess running in embedding mode and provides
helpers to batch-embed text and convert vectors to compact binary blobs.
"""

from __future__ import annotations

import struct
import subprocess
import time

import httpx


def start_embedding_server(
    model_path: str,
    port: int = 8085,
    host: str = "127.0.0.1",
) -> subprocess.Popen:
    """Start llama-server in embedding mode.

    Waits up to 30 seconds for the server's ``/health`` endpoint to return
    HTTP 200 before returning.  Raises :class:`RuntimeError` if the server
    fails to become healthy in time.
    """
    proc = subprocess.Popen(
        [
            "llama-server",
            "--model", model_path,
            "--port", str(port),
            "--host", host,
            "--embedding",
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )

    health_url = f"http://{host}:{port}/health"
    deadline = time.monotonic() + 30

    while time.monotonic() < deadline:
        try:
            resp = httpx.get(health_url, timeout=2)
            if resp.status_code == 200:
                return proc
        except httpx.ConnectError:
            pass
        time.sleep(0.5)

    proc.kill()
    raise RuntimeError(
        f"llama-server failed to become healthy within 30 s "
        f"(http://{host}:{port}/health)"
    )


def embed_batch(
    texts: list[str],
    port: int = 8085,
    host: str = "127.0.0.1",
) -> list[list[float]]:
    """POST texts to the llama-server ``/embedding`` endpoint.

    Returns a list of float vectors, one per input text.
    """
    url = f"http://{host}:{port}/embedding"
    payload = {"content": texts}
    resp = httpx.post(url, json=payload, timeout=120)
    resp.raise_for_status()
    data = resp.json()
    return [item["embedding"] for item in data]


def vectors_to_blob(vectors: list[list[float]]) -> list[bytes]:
    """Pack float vectors as little-endian float32 blobs.

    Each vector is stored as ``struct.pack('<Nf', *vec)`` where *N* is the
    dimensionality.  This is compact and trivial to unpack later.
    """
    blobs: list[bytes] = []
    for vec in vectors:
        blobs.append(struct.pack(f"<{len(vec)}f", *vec))
    return blobs
