# Recipe acquisition tracking

Status: agreed scope 2026-09-30; queued as Phase 7 in MASTERPLAN.md; not implemented.

## Goal

Detect when an installed recipe has changed even if its ID stays the same. The current Go planner and status compare IDs only; the manifest records artifact checksums but no acquisition identity.

## Simple design

- Compute SHA-256 when loading/resolving a recipe, using deterministic JSON of an explicit acquisition-field projection. Share this helper between installation, planning, and status.
- Include source URLs/patterns/platforms, destination filename/type, acquisition strategy, build parameters/steps, and Python/package/entry-point installation parameters. Include resolved defaults used by acquisition. Preserve meaningful execution order; serialization must not depend on YAML or map-key order.
- Exclude descriptions, tags, menu/display fields, license prose, and size estimates. New fields that affect acquisition join the projection; presentation-only fields do not.
- Store the result as `acquisition_hash` on existing realized manifest entries only after successful installation. Multiple artifacts from a recipe receive the same recipe hash. Do not add a generated hash field to recipe YAML.
- Compare the computed recipe hash with the recorded installed hash: equal means acquisition current; different means update available; absent means version unknown. Keep recipe update detection distinct from file-integrity verification.
- Make plan/status report changed recipes and let the existing apply path reacquire/rebuild them. Do not advance the recorded hash on a failed installation. Check existing cache behavior so an update cannot merely reuse the previous artifact.

Use a simple hash format such as `acq-v1:sha256:<hex>`. The prefix versions the projection, not the YAML syntax. Recipe format changes that parse into the same effective acquisition values leave the hash unchanged. If a future incompatible projection change requires a new version, report needs reconciliation; no migration framework is needed upfront.

Older manifests remain readable. An absent hash is unknown, not evidence that installed files match today's recipe; record a hash after a successful refresh rather than stamping existing files as current.

## Deliberate limits

Start with one hash per recipe and the existing manifest structure. A change to any platform source can conservatively flag the recipe for update. Do not introduce acquisition-unit schemas, dependency hash graphs, provenance snapshots, field-by-field explanations, separate metadata hashes, or automatic adoption/migration machinery in this feature.

A stable URL whose remote contents change cannot be detected by this hash. Prefer immutable release/model revision pins; remote freshness remains separate. The hash also does not replace existing artifact checksums or builder cache fingerprints.

## Implementation outline

1. Add the acquisition projection/hash helper and behavioral tests in the catalog layer.
2. Add the optional manifest field and record it through successful installation paths.
3. Compare hashes in plan/status and connect changed recipes to apply, checking cache invalidation. Keep unknown versions explicit.

## Acceptance criteria

- Formatting, map-key order, descriptions, and size edits leave the hash unchanged.
- Source pins, package pins, filenames, and meaningful build parameters/steps change it.
- Equivalent parsed acquisition values produce identical hashes despite recipe syntax changes.
- A newly supported acquisition parameter participates in hashing, with a test alongside its support.
- Old manifests load and report version unknown; hashes are saved only after successful installation.
- Plan/status agree on current versus update available, and applying an update installs the changed recipe rather than a stale cached result.

This is the agreed feature scope, not a completed implementation. The historical Python integrity plan is background; implement against the current Go code.
