# Pack Philosophy

Packs are standalone capability bundles, not strict taxonomy buckets.

They answer the question: "What sources make this capability feel genuinely
useful offline?" The answer may overlap with other packs. That is acceptable.

## Core Rules

- Overlap between packs is allowed when it improves standalone usefulness.
- Duplicate artifacts must not be materialized twice during preset composition
  or sync.
- Canonical identity is `source.id`. If two packs reference the same source
  ID, the composed preset should include it once.
- `recipes/` define artifacts, `packs/` define capability bundles, and
  `default-*` / `finland-*` presets define size-tiered compositions.

## Inclusion Test

Include a source in a pack when leaving it out would make the pack feel
obviously incomplete for its intended use.

Signals that a source belongs:

- It is central to the pack's practical use case.
- It is one of the first offline references a user in that domain would expect.
- It completes a coherent workflow with the other sources in the pack.
- It has a strong size-to-value ratio for the capability it supports.

## Exclusion Test

Leave a source out when:

- It is only loosely related to the pack's main capability.
- It is better treated as regional rather than universal.
- It is too large for the value it adds at the target tier.
- It should be built later as a custom package, builder, or optional add-on.

## Tiering Guidance

Do not blindly inherit every pack into every size tier.

Instead:

1. Decide which capabilities each preset tier should provide.
2. Pull in the specific sources that represent those capabilities.
3. Let composition dedupe repeated `source.id` entries.
4. Prune low-value or oversized sources for that tier.

This means packs are the capability catalog, while the canonical presets are
the opinionated tiered builds.

## Current Pack Roles

- `core`: tiny universal additions that almost every drive benefits from
- `tools-base`: foundational tooling for search, serving, inspection, and ops
- `survival-medical`: emergency care, austere medicine, preparedness
- `homesteading`: agriculture, low-tech systems, construction, practical living
- `engineering`: embedded, fabrication, electronics, applied making
- `communications`: radio, networking, RF, mesh, comms support theory
- `computing`: programming, Linux/sysadmin, security, software reference
- `sciences`: science learning and reference across major disciplines
- `fi-*`: Finland-specific language, geodata, and regional capability packs
- `embedded/*`: hardware-specific offline development packs
- `ai/*`: RAM-tiered local AI bundles. `ai/harnesses` carries shared runtimes and terminal clients. `ai/models-*-ram` packs each include one general/tool-call model and one coding model.
