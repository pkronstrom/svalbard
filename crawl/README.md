# Custom Site Crawling

Place YAML configs here to crawl websites into generated ZIM files.

**Current limitation:** config-driven crawl supports one seed site per YAML file and writes the generated artifact under `generated/`, then registers it as a local source in `local/`.

## Quick start

1. Copy an example: `cp nordic-emergency.yaml.example my-site.yaml`
2. Edit the sites list and crawl rules
3. Run: `svalbard crawl config my-site.yaml`

For one-off crawls without a config file:

```bash
svalbard crawl url https://example.com/docs -o example-docs.zim
```

## Requirements

- Docker (for Zimit)
- Internet connection

## Manual alternative

You can also register an existing local file or directory as a reusable local source:

```bash
svalbard local add /path/to/file.zim
```

The generated or added source can then be selected into a drive and copied there during `svalbard sync`.

## Config reference

```yaml
name: Human-readable name
description: What this bundle contains
tags: [domain-tag-1, domain-tag-2]    # From svalbard taxonomy
depth: comprehensive                   # comprehensive | overview | reference-only

sites:
  - url: https://example.com/docs      # Seed URL (currently exactly one site)
    scope: prefix                       # prefix | domain | host | page
    page_limit: 500                     # Max pages to crawl

defaults:
  size_limit_mb: 512                    # Crawl size limit
  timeout_minutes: 60                   # Crawl timeout
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
