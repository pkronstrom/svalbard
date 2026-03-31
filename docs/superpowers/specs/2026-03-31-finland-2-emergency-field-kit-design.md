# Finland-2 Emergency Field Kit Design

## Summary

Svalbard should add a new visible preset named `finland-2`.

This preset should be a compact emergency field kit for Finland use, sized for a nominal `2 GB` tier but intentionally kept below that ceiling so users still have room for later additions. It should optimize for evacuation, short off-grid disruption, and phone-friendly offline reading rather than broad general reference.

The preset should be text-first, include one compact medical video reference, and avoid heavyweight components such as maps, local AI models, rescue boot environments, and large encyclopedic archives.

## Goals

- Provide a real low-end preset that is useful in emergency and bugout scenarios, not just a smoke-test bundle.
- Keep the bundle small enough to copy partially or fully onto a phone or a very small USB stick.
- Prioritize survival value per GB: emergency care, water, food, shelter-adjacent practical skills, and low-tech recovery knowledge.
- Leave visible headroom inside the nominal `2 GB` tier for user-added content and future map additions.
- Fit naturally into the visible wizard preset selection.

## Non-Goals

- Broad offline general knowledge coverage such as Wikipedia.
- Bundled maps or routing in the first version of `finland-2`.
- Local LLMs, embedding models, or semantic search.
- Desktop-heavy rescue tooling such as Ventoy, SystemRescue, ddrescue, TestDisk, or QGIS.
- Rich Finland-language coverage in the first version.

## Current Context

Current visible preset tiers start much higher and assume significantly more storage. The existing `test-1gb` preset is hidden from the wizard and optimized for smoke testing rather than real emergency use.

That leaves a product gap for a genuinely useful ultra-small preset. The smallest real field-ready bundle should not try to be a miniature encyclopedia. It should instead behave like an offline emergency handbook library with a minimal runtime.

## User Model

`finland-2` should be understood as:

- a compact Finland-oriented emergency field kit
- appropriate for evacuation, outages, and short off-grid scenarios
- mostly English in v1, with selective Finland-specific additions where they are small and high-value
- usable primarily through Kiwix and basic file viewing

The "Finland" part of the preset should come from operational relevance, not from trying to mirror a full Finnish-language archive in only `2 GB`.

## Size Budget

The nominal preset size is:

- `target_size_gb: 2`

The actual target should be lower:

- preferred realized size: about `1.5–1.7 GB`
- desired headroom: about `0.3–0.5 GB`

This headroom is intentional. It leaves space for:

- small user-added local sources
- future tiny map additions
- normal source-size drift between snapshots

## Content Principles

`finland-2` should follow these rules:

- medical knowledge gets the strongest weight
- text beats video unless the task is highly procedural
- one compact medical video source is acceptable and desirable
- practical survival content should focus on water, food, shelter-adjacent skills, and field repair knowledge
- runtime/tooling should stay minimal
- every included source should justify its space cost

The preset should not include broad but low-priority material simply because it is well known.

## Proposed Source Set

The first version of `finland-2` should include:

### Medical Core

- `wikem`
- `who-basic-emergency-care`
- `zimgit-medicine`
- `fas-military-medicine`
- `quick-guides-medicine`
- `fimea`

Rationale:

- `wikem` is a compact high-value emergency medicine reference
- `who-basic-emergency-care` is tiny and directly aligned with first-contact emergency treatment
- `zimgit-medicine` adds practical field-oriented medical content
- `fas-military-medicine` adds procedural and field manual depth at low cost
- `quick-guides-medicine` is the one allowed video slice because procedure demonstrations are worth the space here
- `fimea` is a compact Finland-specific medicine dataset with real local value

### Water, Food, and Practical Basics

- `zimgit-water`
- `zimgit-food-preparation`
- `based-cooking`
- `grimgrains`
- `usda-nutrition`
- `zimgit-knots`
- `cd3wd`

Rationale:

- `zimgit-water` and `zimgit-food-preparation` directly target basic survival tasks
- `based-cooking`, `grimgrains`, and `usda-nutrition` provide compact food-preparation and nutrition support
- `zimgit-knots` covers a small but important field skill area relevant to shelter, carrying, and repair
- `cd3wd` is the main broad low-tech reference library in the preset because it offers strong practical value per GB

### Minimal Runtime and Tooling

- `kiwix-serve`
- `7z`
- `sqlite3`
- `toybox`

Rationale:

- `kiwix-serve` is the core offline reading runtime
- `7z` and `toybox` are cheap, general-purpose support tools
- `sqlite3` is small enough that it is reasonable to keep for inspecting bundled SQLite datasets

## Proposed YAML

```yaml
name: finland-2
description: Emergency field kit for Finland use on a 2GB stick or phone-copyable archive
target_size_gb: 2
region: finland
sources:
- wikem
- who-basic-emergency-care
- zimgit-medicine
- fas-military-medicine
- quick-guides-medicine
- zimgit-water
- zimgit-food-preparation
- based-cooking
- grimgrains
- usda-nutrition
- zimgit-knots
- cd3wd
- fimea
- kiwix-serve
- 7z
- sqlite3
- toybox
```

## Estimated Size

Using the declared recipe sizes currently in the repo, this bundle is about:

- `1.65 GB`

This estimate is close enough to the target range to satisfy the product goal while still preserving some headroom.

## Explicit Exclusions

The first version of `finland-2` should not include:

- `joukahainen`
- any Wikipedia variant
- `wiktionary-fi`
- `wiktionary-en`
- any map basemap or overlay
- any local LLM or embedding model
- `stackexchange-survival`
- `practical-action`
- `ready-gov`
- heavy rescue or provisioning tools

Rationale:

- `joukahainen` is a lexical backend, not a practical emergency aid
- Wikipedia and Wiktionary consume too much space for this tier
- maps are deferred to preserve the emergency knowledge core and leave future headroom
- AI/runtime additions are a poor trade at `2 GB`
- `stackexchange-survival` is useful but too expensive in this tier
- `practical-action` and `ready-gov` are both valuable, but too large to justify in the first `2 GB` cut

## Known Gaps

The first version of `finland-2` will intentionally leave these gaps:

- no dedicated navigation or map-reading guide
- no interactive map coverage
- limited Finland-language support
- no desktop recovery environment

These are acceptable tradeoffs for the `2 GB` budget. They should be treated as future expansion candidates rather than reasons to weaken the current emergency-care core.

## Wizard And UX

`finland-2` should be visible in the normal wizard preset list.

It should be presented as the smallest Finland preset and described as a compact emergency or field kit rather than as a general offline library.

## Testing Expectations

Implementation should verify:

- the preset loads and resolves all source ids
- the total estimated size remains within the intended range
- the wizard includes `finland-2` in visible preset choices
- existing tests that assume the smallest Finland preset starts at `32` are updated accordingly

## Rollout

This design only adds `finland-2`.

Future low-end tiers such as `finland-4`, `finland-8`, `default-2`, and similar ladders should be designed separately. This avoids prematurely locking the rest of the small-tier ladder before the first real emergency preset is validated in use.
