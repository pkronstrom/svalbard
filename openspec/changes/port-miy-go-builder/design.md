## Context

`makeityourself-zim.py` is a mature but monolithic implementation. A clean port must preserve its verified-site state machine and fallback behavior rather than translating functions mechanically.

## Goals / Non-Goals

**Goals:** Go control plane, persistent vault staging, equivalent project coverage, quality checks, and verified ZIM output.

**Non-Goals:** rewriting Chromium or libzim, removing the Python implementation before parity, or generalizing site strategies into a plugin framework.

## Decisions

- Inventory responsibilities and golden fixtures first.
- Keep site strategies, shared crawl state, PDF extraction, optimization, and packaging physically colocated under one MIY builder area.
- Use Go HTTP/HTML/PDF libraries where maintained support exists.
- Use tools-browser only for proven JS-heavy recrawls and tools-base for ZIM writing/checking.
- Compare old/new output counts, metadata, links, assets, and representative pages before cutover.

## Risks / Trade-offs

The existing script is large and site behavior changes externally. Golden fixtures and staged cutover are mandatory; this is not a line-for-line rewrite.
