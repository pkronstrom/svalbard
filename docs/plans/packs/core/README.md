# Pack: Core

Base layer included on every Svalbard drive regardless of size or specialization.
Everything here must justify its bytes — if a 2 GB stick gets only one pack, this is it.

## Audience

Everyone — this is the universal foundation.

## Status

Idea — collecting sources

## Source candidates

### Encyclopedia / General reference

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| Wikipedia EN Top-100 Mini | `wikipedia-en-100-mini` | ZIM | 5 MB | 2 GB | ~5 000 core articles, text only |
| Wikipedia EN no-pic | `wikipedia-en-nopic` | ZIM | 25 GB | 32 GB | All 6M+ articles, no images |
| Wikipedia EN maxi | `wikipedia-en-maxi` | ZIM | 100 GB | 128+ GB | All articles with images |
| Wiktionary EN | `wiktionary-en` | ZIM | 5 GB | 32 GB | Definitions, etymologies, translations |
| Wikibooks EN | `wikibooks-en` | ZIM | 3 GB | 128+ GB | Community textbooks across subjects |

### Medical

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| WikEM | `wikem` | ZIM | 42 MB | 2 GB | Emergency medicine quick-ref, tiny footprint |
| zimgit-medicine | `zimgit-medicine` | ZIM | 67 MB | 2 GB | Field medicine supplement |
| WHO Basic Emergency Care | `who-basic-emergency-care` | PDF | 4 MB | 2 GB | Triage and acute care for limited-resource settings |
| WikiMed | `wikimed` | ZIM | 2 GB | 32 GB | Full medical encyclopedia with images |
| MedlinePlus | `medlineplus` | ZIM | 1.8 GB | 128+ GB | NIH consumer health — conditions, drugs, lab tests |

### Practical / How-to

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| zimgit-water | `zimgit-water` | ZIM | ~50 MB | 2 GB | Water purification and sourcing guides |
| zimgit-knots | `zimgit-knots` | ZIM | ~30 MB | 2 GB | Knot tying reference |
| zimgit-food-preparation | `zimgit-food-preparation` | ZIM | ~40 MB | 2 GB | Basic food prep and preservation |
| Practical Action | `practical-action` | ZIM | 1 GB | 32 GB | Appropriate tech: water, energy, construction |
| iFixit | `ifixit` | ZIM | 5 GB | 128+ GB | Repair guides for electronics and appliances |
| CD3WD | `cd3wd` | ZIM | 550 MB | 32 GB | Agriculture, water, energy, construction library |
| zimgit-post-disaster | `zimgit-post-disaster` | ZIM | 600 MB | 32 GB | Shelter, sanitation, infrastructure recovery |

### Books

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| Project Gutenberg | `gutenberg` | ZIM | 10 GB | 128+ GB | 70 000+ public domain books |

### Maps

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| Natural Earth | `natural-earth` | GPKG | 300 MB | 2 GB | Global basemap: borders, coastlines, cities, roads |

### Tools (drive infrastructure)

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| Kiwix serve | `kiwix-serve` | binary | 200 MB | 2 GB | Serves all ZIM content via browser; multi-platform |
| SQLite CLI | `sqlite3` | binary | 2 MB | 2 GB | Required for cross-ZIM FTS5 search index |
| fzf | `fzf` | binary | 3 MB | 2 GB | Fuzzy finder for interactive terminal menus |
| 7-Zip | `7z` | binary | 1 MB | 2 GB | Universal archive extraction |
| age | `age` | binary | 5 MB | 2 GB | File encryption/decryption |
| Toybox | `toybox` | binary | 3 MB | 2 GB | 200+ POSIX coreutils in one binary (Linux only) |
| dufs | `dufs` | binary | 3 MB | 2 GB | HTTP file server with upload and directory UI |
| CyberChef | `cyberchef` | app | 15 MB | 32 GB | Browser-based data encoding/analysis |
| SQLiteViz | `sqliteviz` | app | 5 MB | 32 GB | Browser-based SQLite explorer |
| DuckDB WASM | `duckdb-wasm` | app | 20 MB | 128+ GB | Browser-based analytical SQL engine |

### Models (local AI)

| Source | Recipe | Type | Size est. | Tier | Notes |
|--------|--------|------|-----------|------|-------|
| Nomic Embed v1.5 | `nomic-embed-text-v1.5` | GGUF | 140 MB | 2 GB | Embedding model for semantic search |
| llama-server | `llama-server` | binary | 40 MB | 2 GB | llama.cpp inference server |
| Qwen 3.5 0.8B (Q4) | `qwen-0.8b` | GGUF | 560 MB | 2 GB | Tiny Q&A model, runs on anything with 1 GB RAM |
| Qwen 3.5 9B (Q4) | `qwen-9b` | GGUF | 5.9 GB | 32 GB | Solid general-purpose model, needs ~8 GB RAM |
| Qwen 3.5 35B-A3B (Q4) | `qwen-35b-a3b` | GGUF | 20.7 GB | 128+ GB | Strong MoE model, needs ~24 GB RAM |

### Not yet in recipes (needs new recipes)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Wikipedia Simple English | ZIM | ~400 MB | 2 GB | Simpler prose, good for the smallest tier |
| run.sh launcher | shell | ~20 KB | 2 GB | Drive-side launcher script (menu, search, serve) |
| Offline search index | SQLite | varies | 2 GB | Pre-built FTS5 + embedding index for bundled ZIMs |
| OSM planet extract (low-zoom) | PMTiles | ~500 MB | 32 GB | z0-z8 global overview tiles for basic world map |
| Protomaps basemap styles | JSON | <1 MB | 32 GB | Default styles for PMTiles map viewer |

## Tiering notes

- **2 GB** (~1.5 GB usable after tools): Wikipedia mini + WikEM + WHO BEC + zimgit trio (water, knots, food-prep, medicine) + Natural Earth + Qwen 0.8B + Nomic embeddings + all CLI tools. Absolute bare minimum that is still genuinely useful.
- **32 GB** (~28 GB usable): Swap Wikipedia mini for full nopic (25 GB). Add WikiMed, Wiktionary, Practical Action, CD3WD, post-disaster guides, CyberChef, SQLiteViz, and Qwen 9B. This is the sweet spot for a USB stick.
- **128+ GB**: Add Wikipedia maxi (images), iFixit, Gutenberg, Wikibooks, MedlinePlus, DuckDB WASM, and Qwen 35B-A3B. Room left over for regional packs (Finnish, Nordic, etc.) and specialty content.

## Relationship to presets

The core pack is not a preset itself — it is a building block that every preset includes.
Regional presets (e.g. `finland-32`) layer region-specific packs on top of core.
The existing `presets/packs/tools-base.yaml` overlaps heavily with the tools section here and should be reconciled or absorbed into core.

## Open questions

- Should Wikipedia Simple English replace the Top-100 Mini at the 2 GB tier? It covers far more ground (~230 000 articles) but is roughly 80x larger.
- Is Qwen 0.8B useful enough at 2 GB to justify 560 MB, or should the smallest tier skip generative AI and keep only the embedding model for search?
- How to handle the run.sh launcher — does it live in a recipe or is it part of the provisioning system itself?
- Should we ship a pre-built FTS5 search index per tier, or always build it on first run?
- Natural Earth is GPKG; do we also want a low-zoom PMTiles global basemap at 32 GB+ to enable the MapLibre viewer without a regional OSM extract?
- CyberChef at 32 GB vs 2 GB — it is only 15 MB; should it be in the smallest tier too?
