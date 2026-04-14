# Preset Pack Tier Composition Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the `default-*` and `finland-*` preset families with the pack philosophy by defining which capability packs enter at which size tiers, while allowing overlap between packs and deduping repeated `source.id` values at composition time.

**Architecture:** Treat `recipes/` as canonical artifact definitions, `presets/packs/` as capability catalogs, and the canonical preset families as intentional tiered compositions. Do not blindly inherit whole packs into each tier; instead, promote specific pack slices at the tier where they become defensible on size and usefulness grounds.

**Tech Stack:** YAML presets, `src/svalbard/presets.py` composition logic, `src/tests/test_presets.py`

---

## Current Baseline

- `packs/core`: `0.005 GB`
- `packs/tools-base`: `0.397 GB`
- `packs/survival-medical`: `0.262 GB`
- `packs/homesteading`: `3.961 GB`
- `packs/engineering`: `3.525 GB`
- `packs/communications`: `3.274 GB`
- `packs/computing`: `6.488 GB`
- `packs/sciences`: `15.730 GB`
- `packs/fi-survival`: `14.742 GB`
- `packs/fi-reference`: `62.850 GB`
- `packs/fi-maps`: `3.563 GB`

The pack totals are already large enough that the default and Finland families
should continue to compose slices rather than extending every pack wholesale.

## Decision Rules

1. Packs are standalone capability bundles, not strict taxonomy buckets.
2. Overlap between packs is acceptable when it improves standalone usefulness.
3. Final preset composition must dedupe by `source.id`.
4. Lower tiers prefer generalist usefulness and small, high-value references.
5. Higher tiers can absorb larger and more specialized sources.
6. Finland tiers should add locality, language, and geodata rather than
   re-bundling the universal default capability set.

## Pack Entry Points

### `core`

- Enter: all general presets
- Role: tiny universal additions only
- Leave out: heavy domain or regional content

### `tools-base`

- Enter: all interactive presets
- Role: serving, search, inspection, lightweight drive apps
- Leave out: domain content and hardware-specific toolchains

### `survival-medical`

- Enter: `default-32`
- Role: emergency care, field medicine, disease/drug reference
- Promote early because the pack is compact and high-value
- Leave out from lower tiers only if the source is large or not operationally useful

### `homesteading`

- Enter: tiny essentials at `default-32`, practical slice at `default-64`, broad slice at `default-128`
- Role: agriculture, low-tech systems, food, repairable practical living
- Full pack is reasonable by `default-128`/`default-256`
- Leave out pure maker/electronics references

### `engineering`

- Enter: language/tooling references at `default-128`, maker/hardware Q&A at `default-256`, deeper hardware references at `default-512`
- Role: embedded systems, fabrication, electronics, applied making
- Leave out general software/sysadmin references

### `communications`

- Enter: optional intro slice at `default-128`, meaningful core at `default-256`, broader comms reference at `default-512`
- Role: ham radio, networking, RF, mesh, comms-supporting theory
- Leave out general software reference and broad fabrication material unless directly comms-relevant

### `computing`

- Enter: doc core at `default-128`, sysadmin/security core at `default-256`, broader Q&A and distro references at `default-512`, largest archives at `default-1tb+`
- Role: programming, Linux, troubleshooting, security, software reference
- This pack has the highest risk of runaway size and needs strict tier gating

### `sciences`

- Enter: intro textbook slice at `default-128`, full LibreTexts core at `default-256`, heavier science Q&A at `default-512`, largest math/science archives at `default-1tb+`
- Role: science learning and reference
- Leave out regional field guides and clinical medicine from the core slice

### `fi-survival`

- Enter: do not wholesale-extend in the Finland family
- Role: Finland-oriented capability catalog for emergency and practical living
- Use as a source pool for `finland-2` and for selective promotions into larger Finland tiers

### `fi-reference`

- Enter: do not wholesale-extend at low tiers; selectively promote language and local-reference sources
- Role: Finnish-language and Finland-oriented broad reference
- The full pack is too large for direct inheritance below the largest tiers

### `fi-maps`

- Enter: `finland-64+`
- Role: Finland geodata, basemap, and viewer support
- This is the cleanest regional pack to inherit directly

## Recommended Default Family Matrix

### `default-32`

Include:
- `tools-base`
- `core`
- current survival/medical spine
- tiny homesteading essentials

Representative sources:
- `wikiciv`, `permacomputing`
- `wikem`, `who-basic-emergency-care`, `zimgit-medicine`
- `zimgit-water`, `zimgit-food-preparation`, `based-cooking`, `usda-nutrition`
- `practical-action`, `zimgit-knots`

Leave out:
- specialist computing, engineering, communications, and science packs
- large video/media-heavy preparedness material

### `default-64`

Include:
- recovery/repair additions
- broader practical and education slice
- world basemap

Representative sources:
- `ifixit`, `zimgit-post-disaster`
- `stackexchange-diy`, `stackexchange-sustainability`
- `wikibooks-en`, `natural-earth`

Leave out:
- most specialist engineering/computing/comms material
- large science/reference expansions

### `default-128`

Include:
- broad practical slice from `homesteading`
- intro textbook/reference slice from `sciences`
- first lightweight capability-specific docs from `computing` and `engineering`

Recommended additions beyond the current profile:
- computing intro: `devdocs-python`, `devdocs-rust`, `devdocs-go`, `arch-wiki`, `man-pages`
- engineering intro: `devdocs-c`, `devdocs-cpp`

Keep at this tier:
- `lowtech-magazine`, `cd3wd`, `appropedia`, `stackexchange-gardening`
- `libretexts-engineering`, `libretexts-medicine`, `libretexts-geosciences`

Leave out:
- heavy Stack Exchange specialist archives
- full `sciences`, `communications`, and `computing` pack content

### `default-256`

Include:
- full practical/homesteading core
- meaningful science textbook core
- specialist intro slices from `communications`, `engineering`, and `computing`

Recommended additions:
- communications core: `stackexchange-networkengineering`
- engineering core: `stackexchange-arduino`, `stackexchange-raspberrypi`, `stackexchange-robotics`, `stackexchange-3dprinting`
- computing core: `stackexchange-unix`, `stackexchange-security`

Keep at this tier:
- `stackexchange-amateur-radio`, `stackexchange-electronics`, `stackexchange-physics`
- `libretexts-chemistry`, `libretexts-biology`, `libretexts-physics`, `libretexts-math`, `libretexts-statistics`

Leave out:
- largest math/computing specialist archives
- media-heavy preparedness expansion

### `default-512`

Include:
- heavy specialist Q&A where the size is justified
- deeper communications/computing/engineering references
- preparedness media and first serious local-AI tier

Recommended additions:
- computing deep slice: `stackexchange-askubuntu`, `stackexchange-dba`, `stackexchange-softwareengineering`, `stackexchange-codereview`, `stackexchange-cryptography`, `stackexchange-reverseengineering`, `stackexchange-devops`, `alpinelinux-wiki`, `gentoo-wiki`, `termux-wiki`
- engineering deep slice: `stackexchange-engineering`, `ted-3d-printing`
- communications deep slice: `libretexts-engineering`

Keep at this tier:
- `stackexchange-math`
- preparedness media set
- `qwen-9b`, `llama-server`

Leave out:
- only the very largest admin/generalist archives such as `serverfault`, `superuser`, `stack-overflow`

### `default-1tb`

Include:
- remaining large high-value specialist Q&A
- stronger long-form preparedness and learning material
- large-model local AI tier

Keep at this tier:
- `stackexchange-chemistry`, `stackexchange-biology`, `stackexchange-engineering`
- `lrn-self-reliance`
- `qwen-35b-a3b`

### `default-2tb`

Include:
- largest region-neutral generalist archives
- redundant model tiers where justified

Keep at this tier:
- `wikipedia-en-all`
- `stackexchange-serverfault`, `stackexchange-superuser`
- re-add `qwen-9b`

Future candidates for this tier:
- `stack-overflow`

## Recommended Finland Family Matrix

### `finland-2`

Stay bespoke.

Reason:
- It is a true emergency field kit, not a simple slice of `default-32`
- It should remain aggressively practical and size-constrained

### `finland-64`

Include:
- `default-64`
- `fi-maps`
- Finland language essentials
- `fimea`

Keep:
- `wikipedia-fi-nopic`, `wiktionary-fi`
- early practical source promotions if they materially improve Finnish use cases

### `finland-128`

Include:
- `default-128`
- `fi-maps`
- Finnish language/reference spine
- `fimea`

Allow earlier Finland-specific promotion of sources that remain low-cost and highly useful, such as:
- `stackexchange-amateur-radio`

### `finland-256`

Include:
- `default-256`
- `fi-maps`
- Finnish plus close-neighbor language coverage

Keep:
- `wikipedia-sv-nopic`, `wikipedia-no-nopic`

Do not yet wholesale-extend `fi-reference`; keep selecting specific local-language and local-reference sources.

### `finland-512`

Include:
- `default-512`
- `fi-maps`
- broader Nordic language support

Keep:
- `wikipedia-da-nopic`

### `finland-1tb`

Include:
- `default-1tb`
- `fi-maps`
- wider neighboring-language support

Keep:
- `wikipedia-de-nopic`

### `finland-2tb`

Include:
- `default-2tb`
- `fi-maps`
- full Finnish encyclopedia tier
- full neighboring-language set where useful

Keep:
- `wikipedia-fi-all`
- `wikipedia-sv-all`, `wikipedia-no-all`, `wikipedia-da-all`
- `wikipedia-ru-nopic`, `wikipedia-et-nopic`

## Implementation Priorities

### Task 1: Freeze the tier entry points in documentation

**Files:**
- Create: `docs/plans/2026-04-02-pack-tier-composition-plan.md`
- Reference: `presets/packs/README.md`

- [ ] Confirm the pack-to-tier entry points in this document before refactoring presets.
- [ ] Use this document as the source of truth for later preset changes.

### Task 2: Refactor the default family to follow the matrix

**Files:**
- Modify: `presets/default-32.yaml`
- Modify: `presets/default-64.yaml`
- Modify: `presets/default-128.yaml`
- Modify: `presets/default-256.yaml`
- Modify: `presets/default-512.yaml`
- Modify: `presets/default-1tb.yaml`
- Modify: `presets/default-2tb.yaml`
- Test: `src/tests/test_presets.py`

- [ ] Introduce missing low-tier `core` and `tools-base` intent explicitly where useful.
- [ ] Add the recommended `computing`, `engineering`, and `communications` intro slices at the tiers defined above.
- [ ] Keep large or niche sources at `512+` and `1tb+`.
- [ ] Re-run preset size calculations after each tier change.

### Task 3: Refactor the Finland family to follow the matrix

**Files:**
- Modify: `presets/finland-2.yaml`
- Modify: `presets/finland-64.yaml`
- Modify: `presets/finland-128.yaml`
- Modify: `presets/finland-256.yaml`
- Modify: `presets/finland-512.yaml`
- Modify: `presets/finland-1tb.yaml`
- Modify: `presets/finland-2tb.yaml`
- Test: `src/tests/test_presets.py`

- [ ] Keep `finland-2` bespoke and emergency-focused.
- [ ] Continue to inherit `fi-maps` directly at `64+`.
- [ ] Promote local-language and Finland-specific reference sources intentionally instead of inheriting `fi-reference` wholesale at low tiers.

### Task 4: Add regression tests for tier intent

**Files:**
- Modify: `src/tests/test_presets.py`

- [ ] Add tests for the intended pack/tier entry points, such as:
  - computing intro sources enter by `default-128`
  - maker/comms specialist sources enter by `default-256`
  - deep specialist archives remain `512+`
  - Finland tiers add local language and maps without duplicating universal defaults conceptually

### Task 5: Decide whether packs should become first-class preset building blocks

**Files:**
- Reference: `src/svalbard/presets.py`
- Reference: `presets/packs/README.md`

- [ ] Decide whether the canonical presets should continue to list source IDs directly or grow a higher-level pack-slice composition mechanism.
- [ ] Do not add new composition syntax until the source-by-tier matrix is stable.
