import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path
from rich.console import Console

console = Console()

@dataclass
class DownloadResult:
    source_id: str
    success: bool
    filepath: Path | None = None
    error: str = ""

def find_downloader() -> str | None:
    """Find available download tool: aria2c preferred, fallback to wget, then curl."""
    for tool in ["aria2c", "wget", "curl"]:
        if shutil.which(tool):
            return tool
    return None

def download_file(url: str, dest_dir: Path, tool: str | None = None) -> Path:
    """Download a file to dest_dir using the best available tool. Returns filepath."""
    dest_dir.mkdir(parents=True, exist_ok=True)
    filename = url.rsplit("/", 1)[-1]
    dest_path = dest_dir / filename

    if dest_path.exists():
        console.print(f"  [dim]Already exists: {filename}[/dim]")
        return dest_path

    if tool is None:
        tool = find_downloader()
    if tool is None:
        raise RuntimeError("No download tool found. Install aria2c, wget, or curl.")

    console.print(f"  [bold]Downloading:[/bold] {filename}")

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

def download_sources(sources: list[tuple[str, str, Path]]) -> list[DownloadResult]:
    """Download multiple sources. Input: [(source_id, url, dest_dir), ...]."""
    tool = find_downloader()
    results = []
    for source_id, url, dest_dir in sources:
        try:
            path = download_file(url, dest_dir, tool)
            results.append(DownloadResult(source_id=source_id, success=True, filepath=path))
        except Exception as e:
            results.append(DownloadResult(source_id=source_id, success=False, error=str(e)))
            console.print(f"  [red]Failed: {source_id}: {e}[/red]")
    return results
