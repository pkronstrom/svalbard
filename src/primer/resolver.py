import re
import httpx
from primer.models import Source


def resolve_url(source: Source) -> str:
    """Resolve a source's URL pattern to a concrete URL by finding the latest date."""
    if source.url and not source.url_pattern:
        return source.url
    if not source.url_pattern:
        raise ValueError(f"Source {source.id} has no url or url_pattern")

    pattern = source.url_pattern
    base_url = pattern.rsplit("/", 1)[0] + "/"
    filename_pattern = pattern.rsplit("/", 1)[1]

    # Build regex: replace {date} with capturing group for YYYY-MM
    regex_str = re.escape(filename_pattern).replace(r"\{date\}", r"(\d{4}-\d{2})")
    regex = re.compile(regex_str)

    response = httpx.get(base_url, follow_redirects=True, timeout=30)
    response.raise_for_status()

    matches = []
    for match in regex.finditer(response.text):
        date = match.group(1)
        filename = match.group(0)
        matches.append((date, filename))

    if not matches:
        raise ValueError(f"No matching files found for {source.id} at {base_url}")

    matches.sort(key=lambda x: x[0])
    latest_date, latest_filename = matches[-1]
    return base_url + latest_filename


def resolve_all(sources: list[Source]) -> dict[str, str]:
    """Resolve all sources to concrete URLs. Returns {source_id: url}."""
    resolved = {}
    for s in sources:
        try:
            resolved[s.id] = resolve_url(s)
        except Exception as e:
            resolved[s.id] = f"ERROR: {e}"
    return resolved
