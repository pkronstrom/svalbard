## Context

MIY already contains the evidence for a reusable design:

- `PipelineState` and JSONL state;
- extract, verify, crawl, and package stages;
- fetch, metadata, image, and artifact strategies;
- declarative `SiteConfig` values;
- a `SiteScraper` orchestrator;
- generic cleanup/render/index/package logic;
- a smaller set of truly site-specific API/auth enrichers.

The goal is to extract that model, not translate 3,700 lines mechanically.

## Goals / Non-Goals

**Goals:** reusable Go blocks, inspectable directory edges, content caching, per-item fan-out, declarative site rules, explicit exceptional enrichers, and parity-proven MIY output.

**Non-Goals:** a general DAG, arbitrary scripts, arbitrary payload objects, a full web-scraping DSL, or eliminating browser/ZIM tools that have no maintained Go equivalent.

## Pipeline shape

```text
source document
  -> extract links          output: links/manifest.jsonl
  -> verify/classify        output: verified/manifest.jsonl
  -> map(project pipeline)  output: projects/<stable-id>/
  -> collect                output: site/
  -> package ZIM            output: archive.zim
```

The project sub-pipeline is linear:

```text
fetch -> metadata -> assets -> transform -> render -> verify
```

`map` is one specialized block that enumerates manifest records, runs a sub-pipeline per stable item ID, and collects output directories. Each item/block is content-addressed independently. Changing templates reruns transform/render/collect/package but not fetch.

## Directory contracts

Every block follows:

```text
(input directory, typed params) -> output directory + manifest.json
```

Large content never passes through JSON. JSON/JSONL describes records, artifacts, provenance, status, and digests.

## Declarative rules

Common site behavior uses typed rule data:

- domain matching;
- HTTP/Wayback/browser fetch chain;
- JSON-LD, OpenGraph, and CSS-selector metadata;
- image selectors and limits;
- attachment link patterns;
- safe HTML remove/keep/rewrite rules;
- category/template selection.

Rules are data, not a general expression language. Missing primitives are added only when at least two real sites need them.

## Exceptional enrichers

Named Go enrichers remain for behavior not honestly representable as rules:

- Printables GraphQL and signed downloads;
- Thingiverse authentication/API/CDN behavior;
- GitHub repository/API/archive behavior;
- Instructables step/attachment semantics.

An enricher receives typed project state and returns typed metadata/artifacts. It cannot control pipeline scheduling or container invocation.

## Heavy effects

Go owns control flow. Browser fetch uses the pinned browser appliance. ZIM writing/checking uses the pinned base appliance. All other HTTP, HTML, JSON, template, filesystem, and state work uses Go.

## Cache behavior

Block keys include rule/config version and input manifest digests. Per-project stable IDs isolate cache invalidation. Template changes do not invalidate fetch/metadata/assets. Rule changes invalidate only affected downstream blocks.

## Test strategy

Golden MIY fixtures cover source extraction, generic HTTP pages, each exceptional enricher, failed/Wayback links, browser fallback, assets, safe HTML, project rendering, collection, and ZIM inventory. The old and new builders run against the same bounded fixture set and compare manifests, metadata, links, assets, pages, and archive verification before cutover.
