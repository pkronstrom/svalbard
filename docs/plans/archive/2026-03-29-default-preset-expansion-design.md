# Default Preset Expansion Design

Expand the `default-*` family into a full mirrored size ladder so region-neutral
presets exist at the same major tiers as the `finland-*` family.

## Status

**Design phase** — approved direction, not yet implemented.

---

## Goal

Make `default-*` the region-neutral baseline for every major drive size, instead
of treating larger tiers as Finland-specific by default.

This gives the preset families a clear meaning:

- `default-*` = region-neutral, English-first baseline
- `finland-*` = `default-*` plus Finnish-language and Finland-specific content

The size ladder itself should not be Finland-only.

---

## Preset Family Model

After this change, both families should exist across the same tier ladder:

### Default presets

- `default-32`
- `default-64`
- `default-128`
- `default-256`
- `default-512`
- `default-1tb`
- `default-2tb`

### Finland presets

- `finland-32`
- `finland-64`
- `finland-128`
- `finland-256`
- `finland-512`
- `finland-1tb`
- `finland-2tb`

The wizard should present both families symmetrically.

---

## Content Rules

### 1. What belongs in `default-*`

`default-*` presets should include content that is broadly useful regardless of
country:

- English Wikipedia and related English reference sources
- Practical guides like WikiHow, iFixit, Practical Action, and relevant Stack Exchanges
- Education sources like Wikibooks and Khan Academy
- General tools like Kiwix and CyberChef
- LLM models and their serving binaries at sufficiently large tiers

### 2. What does not belong in `default-*`

`default-*` presets should not include content whose main purpose is Finnish or
Nordic localization:

- Finnish Wikipedia / Wiktionary
- Swedish / Norwegian / Danish / German language additions that only exist today
  because of the old Nordic preset identity
- Finland-specific regional datasets
- Finland-specific legal, cultural, or geographic material

### 3. What belongs in `finland-*`

`finland-*` presets should be the localized overlay:

- Finnish-language reference sources
- Finland-specific datasets
- Any Nordic or nearby-language additions that are intentionally part of the
  Finland-focused package

This means larger tier features like GGUF downloads are not Finland-specific.
They should exist in both families once the default ladder is expanded.

---

## Tier Behavior

The simplest rule is to mirror the current growth pattern from the Finland
family, but strip out localization-specific additions from `default-*`.

### Proposed default ladder

| Tier | Direction |
|------|-----------|
| `default-32` | Minimal survival/reference starter |
| `default-64` | Broader practical/reference set |
| `default-128` | Full practical + education baseline |
| `default-256` | Expanded reference depth and more technical/practical sources |
| `default-512` | Add first local LLM tier |
| `default-1tb` | Add larger LLM tier and broader reference coverage |
| `default-2tb` | Maximum region-neutral archive |

### LLM policy

LLM inclusion should follow tier size, not region:

- `default-32` / `64` / `128` / `256`: no GGUF models
- `default-512`: small GGUF + `llama-server`
- `default-1tb`: larger GGUF + `llama-server`
- `default-2tb`: multiple GGUF models + `llama-server`

The same tier thresholds should continue to apply to `finland-*`.

---

## Implementation Approach

Keep the implementation simple:

1. Add the missing `default-256.yaml`, `default-512.yaml`, `default-1tb.yaml`,
   and `default-2tb.yaml`
2. Populate them by deriving from the current `finland-*` tiers
3. Remove Finland/Nordic-localized sources from the default variants
4. Keep shared tools and large-tier features like GGUF models in both families
5. Update README tables to show the full default ladder and which tiers include
   LLMs

Do not change schema again for this work. This is a content/catalog expansion,
not another preset-structure rewrite.

---

## Validation

Implementation should verify:

- `list_presets()` includes the full default ladder
- wizard region selection shows both families with matching tier depth
- default presets remain region-neutral
- GGUF/llama sources appear in `default-512+`
- README accurately reflects the expanded default family

---

## Open Decision

This design assumes that region-neutral larger presets should exist even if they
overlap heavily with Finland tiers.

That is the recommended direction because it makes the product model easier to
understand:

- size determines depth
- family determines localization
