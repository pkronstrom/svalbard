import hashlib
import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path

import httpx
from rich.console import Console
from rich.progress import (
    BarColumn,
    DownloadColumn,
    Progress,
    SpinnerColumn,
    TextColumn,
    TimeRemainingColumn,
    TransferSpeedColumn,
)

console = Console()


@dataclass
class DownloadResult:
    source_id: str
    success: bool
    filepath: Path | None = None
    error: str = ""
    sha256: str = ""


def find_downloader() -> str | None:
    """Find available download tool: aria2c preferred, fallback to wget, then curl."""
    for tool in ["aria2c", "wget", "curl"]:
        if shutil.which(tool):
            return tool
    return None


def fetch_sha256_sidecar(url: str) -> str | None:
    """Try to fetch a .sha256 sidecar file for the given URL."""
    sidecar_url = url + ".sha256"
    try:
        resp = httpx.get(sidecar_url, follow_redirects=True, timeout=10)
        if resp.status_code == 200:
            # Format is usually: "hash  filename" or just "hash"
            return resp.text.strip().split()[0].lower()
    except Exception:
        pass
    return None


def compute_sha256(filepath: Path) -> str:
    """Compute SHA-256 hash of a file."""
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        while chunk := f.read(1 << 20):  # 1 MB chunks
            h.update(chunk)
    return h.hexdigest()


def download_file_httpx(url: str, dest_path: Path, progress: Progress, task_id) -> Path:
    """Download via httpx with Rich progress bar and resume support."""
    existing_size = dest_path.stat().st_size if dest_path.exists() else 0
    headers = {}
    if existing_size > 0:
        headers["Range"] = f"bytes={existing_size}-"

    with httpx.stream("GET", url, follow_redirects=True, timeout=60, headers=headers) as r:
        if r.status_code == 416:
            # Range not satisfiable — file is already complete
            return dest_path

        if r.status_code == 206:
            # Partial content — resuming
            total = existing_size + int(r.headers.get("content-length", 0))
            progress.update(task_id, total=total, completed=existing_size)
            mode = "ab"
        else:
            # Full download
            total = int(r.headers.get("content-length", 0)) or None
            progress.update(task_id, total=total)
            mode = "wb"
            existing_size = 0

        r.raise_for_status()

        with open(dest_path, mode) as f:
            for chunk in r.iter_bytes(chunk_size=65536):
                f.write(chunk)
                progress.advance(task_id, len(chunk))

    return dest_path


def download_file_cli(url: str, dest_dir: Path, tool: str) -> Path:
    """Download using CLI tools (aria2c/wget/curl) as fallback."""
    filename = url.rsplit("/", 1)[-1]
    dest_path = dest_dir / filename

    if tool == "aria2c":
        cmd = ["aria2c", "-x", "4", "-d", str(dest_dir), "-o", filename, "-c", url]
    elif tool == "wget":
        cmd = ["wget", "-c", "-q", "--show-progress", "-O", str(dest_path), url]
    else:
        cmd = ["curl", "-L", "-C", "-", "-o", str(dest_path), url]

    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"Download failed: {url}")

    return dest_path


def download_file(url: str, dest_dir: Path, expected_sha256: str = "",
                  progress: Progress | None = None, task_id=None,
                  use_cli: str | None = None) -> tuple[Path, str]:
    """Download a file. Returns (filepath, sha256).

    Uses httpx with Rich progress by default. Falls back to CLI tools
    if use_cli is set or httpx fails.
    """
    dest_dir.mkdir(parents=True, exist_ok=True)
    filename = url.rsplit("/", 1)[-1]
    dest_path = dest_dir / filename

    if dest_path.exists() and dest_path.stat().st_size > 0:
        # Verify existing file if we have a checksum
        if expected_sha256:
            actual = compute_sha256(dest_path)
            if actual == expected_sha256:
                return dest_path, actual
            else:
                # Corrupted — re-download
                dest_path.unlink()
        else:
            return dest_path, ""

    if use_cli:
        download_file_cli(url, dest_dir, use_cli)
    elif progress and task_id is not None:
        download_file_httpx(url, dest_path, progress, task_id)
    else:
        # No progress context — use CLI fallback
        cli_tool = find_downloader()
        if cli_tool:
            download_file_cli(url, dest_dir, cli_tool)
        else:
            # Last resort: plain httpx without progress
            with httpx.stream("GET", url, follow_redirects=True, timeout=60) as r:
                r.raise_for_status()
                with open(dest_path, "wb") as f:
                    for chunk in r.iter_bytes(chunk_size=65536):
                        f.write(chunk)

    # Checksum verification
    file_hash = ""
    if dest_path.exists():
        if expected_sha256:
            file_hash = compute_sha256(dest_path)
            if file_hash != expected_sha256:
                dest_path.unlink()
                raise RuntimeError(
                    f"Checksum mismatch for {filename}: "
                    f"expected {expected_sha256[:16]}..., got {file_hash[:16]}..."
                )

    return dest_path, file_hash


def download_sources(
    sources: list[tuple[str, str, Path]],
    checksums: dict[str, str] | None = None,
    use_cli: str | None = None,
    parallel: int = 4,
) -> list[DownloadResult]:
    """Download multiple sources with Rich progress bars.

    Input: [(source_id, url, dest_dir), ...]
    checksums: optional {source_id: expected_sha256}
    parallel: number of concurrent downloads (default 1 = sequential)
    """
    if checksums is None:
        checksums = {}

    results: list[DownloadResult] = []

    with Progress(
        SpinnerColumn(),
        TextColumn("[bold]{task.fields[filename]}"),
        BarColumn(),
        DownloadColumn(),
        TransferSpeedColumn(),
        TimeRemainingColumn(),
        console=console,
    ) as progress:

        def _download_one(source_id: str, url: str, dest_dir: Path) -> DownloadResult:
            filename = url.rsplit("/", 1)[-1]
            task_id = progress.add_task("dl", filename=filename, total=None)
            try:
                expected = checksums.get(source_id, "")
                filepath, sha256 = download_file(
                    url, dest_dir,
                    expected_sha256=expected,
                    progress=progress,
                    task_id=task_id,
                    use_cli=use_cli,
                )
                progress.update(task_id, completed=progress.tasks[task_id].total or 0)
                return DownloadResult(
                    source_id=source_id, success=True,
                    filepath=filepath, sha256=sha256,
                )
            except Exception as e:
                console.print(f"  [red]Failed: {source_id}: {e}[/red]")
                return DownloadResult(
                    source_id=source_id, success=False, error=str(e),
                )

        if parallel <= 1:
            for source_id, url, dest_dir in sources:
                results.append(_download_one(source_id, url, dest_dir))
        else:
            from concurrent.futures import ThreadPoolExecutor, as_completed

            with ThreadPoolExecutor(max_workers=parallel) as executor:
                futures = {
                    executor.submit(_download_one, sid, url, dest): sid
                    for sid, url, dest in sources
                }
                for future in as_completed(futures):
                    results.append(future.result())

    return results
