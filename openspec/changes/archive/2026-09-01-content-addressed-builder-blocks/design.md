## Context

Linear procedures and persistent recipe staging exist. Current fingerprints hash procedure configuration only. The first cache refinement should support the expensive shape already expected from crawling and packaging without introducing a scheduler.

## Goals / Non-Goals

**Goals:** directory edges, content-derived keys, inspectable manifests, atomic cache publication, and a narrow container workspace.

**Non-Goals:** DAG execution, arbitrary payloads, map/fan-out, distributed cache, eviction policy, or making every legacy procedure cacheable immediately.

## Decisions

A cacheable procedure declares input and output paths. Its key is:

```text
sha256(procedure kind + normalized params + ordered input tree digests)
```

Cached layout:

```text
<vault>/.staging/cache/<key>/
  manifest.json
  output/
```

`manifest.json` records format version, key, procedure, input digests, output tree digest, and files. Downstream hashing trusts a valid manifest when the directory matches its recorded identity; otherwise it computes the tree digest.

V1 copies or hard-links cache data into the requested destination. Final drive artifacts never remain symlinked to staging. A future map combinator may symlink immutable intermediate outputs, but is deferred.

New tool procedures receive:

```text
/input   read-only declared inputs
/output  writable empty output
/work    persistent procedure scratch
```

Legacy procedures lacking declared outputs remain uncached and retain their existing execution path. This permits incremental recipe migration.

## Risks / Trade-offs

Initial tree hashing is proportional to input size. Producer manifests prevent repeated downstream rehashing. Cache eviction is deliberately deferred; users can remove `.staging/cache` safely.
