# Custom Site Crawling

Place YAML configs here to crawl websites into ZIM files.

**One YAML file = one output ZIM file** in `zim/custom/`.

## Quick start

1. Copy an example: `cp nordic-emergency.yaml.example my-sites.yaml`
2. Edit the sites list and crawl rules
3. Run: `svalbard crawl my-sites`

## Requirements

- Docker (for Zimit)
- Internet connection

## Manual alternative

You can skip crawling entirely and drop pre-made `.zim` files directly into `zim/custom/` on your drive. They will be auto-discovered by kiwix-serve.

## Config reference

```yaml
name: Human-readable name
description: What this bundle contains
tags: [domain-tag-1, domain-tag-2]    # From svalbard taxonomy
depth: comprehensive                   # comprehensive | overview | reference-only

sites:
  - url: https://example.com/docs      # Seed URL
    scope: prefix                       # prefix | domain | host | page
    page_limit: 500                     # Max pages to crawl
    size_limit_mb: 256                  # Override default size limit
    exclude: "\\?action=|/old/"         # Regex to exclude URLs

defaults:
  size_limit_mb: 512                   # Per-site size limit
  timeout_minutes: 60                  # Per-site timeout
```

## Disclaimer

You are responsible for respecting each site's `robots.txt`, terms of service,
and applicable copyright/licensing. Crawl configs in this directory are
references only — they do not grant permission to redistribute crawled content.
Public sector and open-licensed sites are generally safe to archive for personal use.

### Scope types

- `page` — single page only
- `prefix` — URLs under the same path prefix as seed
- `host` — same hostname
- `domain` — same domain including subdomains
