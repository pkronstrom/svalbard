"""Embedding client for llama-server.

Manages a llama-server subprocess running in embedding mode and provides
helpers to batch-embed text and convert vectors to compact binary blobs.
"""

from __future__ import annotations

import platform as _platform
import shutil
import struct
import subprocess
import time
from pathlib import Path

import httpx


def _find_llama_server(drive_path: str | Path | None = None) -> str:
    """Find llama-server binary: drive bin/, then system PATH."""
    if drive_path:
        os_name = "macos" if _platform.system() == "Darwin" else "linux"
        arch = "arm64" if _platform.machine() in ("aarch64", "arm64") else "x86_64"
        bin_dir = Path(drive_path) / "bin" / f"{os_name}-{arch}"
        candidate = bin_dir / "llama-server"
        if candidate.exists() and candidate.stat().st_mode & 0o111:
            return str(candidate)
        # Try extracting archives if binary not found
        if bin_dir.exists():
            import tarfile
            import zipfile
            for archive in bin_dir.iterdir():
                name = archive.name.lower()
                if not any(name.endswith(s) for s in (".tar.gz", ".tar.xz", ".zip")):
                    continue
                import tempfile
                with tempfile.TemporaryDirectory() as tmp:
                    if name.endswith(".zip"):
                        with zipfile.ZipFile(archive) as zf:
                            zf.extractall(tmp)
                    else:
                        with tarfile.open(archive) as tf:
                            tf.extractall(tmp, filter="data")
                    # Search for llama-server in extracted content
                    for f in Path(tmp).rglob("llama-server"):
                        if f.is_file():
                            dest = bin_dir / "llama-server"
                            shutil.copy2(f, dest)
                            dest.chmod(dest.stat().st_mode | 0o755)
                            return str(dest)
    path = shutil.which("llama-server")
    if path:
        return path
    raise FileNotFoundError(
        "llama-server not found. Ensure it is on your drive (bin/) or system PATH."
    )


def start_embedding_server(
    model_path: str,
    port: int = 8085,
    host: str = "127.0.0.1",
    llama_server_path: str | None = None,
) -> subprocess.Popen:
    """Start llama-server in embedding mode.

    Waits up to 30 seconds for the server's ``/health`` endpoint to return
    HTTP 200 before returning.  Raises :class:`RuntimeError` if the server
    fails to become healthy in time.
    """
    binary = llama_server_path or _find_llama_server()
    proc = subprocess.Popen(
        [
            binary,
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
