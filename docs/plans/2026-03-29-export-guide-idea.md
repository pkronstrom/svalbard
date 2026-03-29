# Export Guide — Idea (v2)

## Problem

Someone with the Svalbard drive wants to extract specific articles and share them as a single file (HTML/PDF/EPUB) with someone who doesn't have the kit or Kiwix.

## Key constraint

This runs **from the drive**, not from the provisioner. No Python, no package manager. Must work with what's on the drive.

## UX — needs more thought

### Option A: CLI with URLs
Browse kiwix-serve in browser, copy URLs of articles you want, pass them to an export tool:

```bash
./export http://localhost:8080/wikihow/Build-a-Snow-Shelter \
         http://localhost:8080/wikipedia/Hypothermia \
         --title "Winter Survival" --format html
```

Simple but clunky — copying URLs one by one.

### Option B: Browser-based picker
A thin web UI on top of kiwix-serve with a "shopping cart" for articles. Browse, click "add to guide", then export. Best UX but needs a custom web frontend.

### Option C: Hybrid
Start with CLI (Option A), add a browser bookmarklet that sends the current page to the export tool. Middle ground.

## Technical approach

A custom **Go static binary** (`svalbard-export`) included in `bin/`:
- Fetches pages from running kiwix-serve via HTTP
- Strips navigation/chrome, keeps article content + images
- Concatenates into single file with table of contents
- Output formats: self-contained HTML (inline base64 images), PDF (via embedded renderer or wkhtmltopdf-style), EPUB
- Small binary, cross-platform, no dependencies

## Formats

| Format | Pros | Cons |
|--------|------|------|
| Single HTML | Zero deps, any browser, any device | Images bloat via base64 |
| PDF | Printable, universal | Needs rendering engine in binary |
| EPUB | Reflowable, small, great on phones | Needs reader app |

## Open questions

- What's the best UX for selecting articles? Browsing + clicking feels right but needs frontend work.
- Should it support offline search across ZIMs to find relevant articles?
- How to handle images — inline base64, or strip them for smaller output?
- Should exported guides be saveable/reusable as templates?
