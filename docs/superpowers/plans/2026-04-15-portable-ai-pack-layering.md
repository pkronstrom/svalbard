# Portable AI Pack Layering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a RAM-tiered portable AI pack system to Svalbard, wire it into the stock `default-*` presets, and expose the bundled local AI stack through the drive toolkit.

**Architecture:** Keep the current preset loader and drive sync flow unchanged. Implement the feature as a catalog-and-launcher expansion: add concrete AI recipes, group them into nested `presets/packs/ai/*` packs, recompose the large default presets through those packs, and add one shell action that launches terminal AI clients against a locally started `llama-server` with sane defaults.

**Tech Stack:** Python 3.12, PyYAML preset files, Bash drive actions, pytest, Click/Rich-adjacent existing CLI code, llama.cpp

---

## File Map

**Create:**
- `docs/superpowers/plans/2026-04-15-portable-ai-pack-layering.md`
- `recipes/apps/opencode.yaml`
- `recipes/apps/crush.yaml`
- `recipes/apps/goose.yaml`
- `recipes/models/gemma-4-e2b-it.yaml`
- `recipes/models/gemma-4-e4b-it.yaml`
- `recipes/models/gemma-4-26b-a4b-it.yaml`
- `recipes/models/gemma-4-31b-it.yaml`
- `presets/packs/ai/harnesses.yaml`
- `presets/packs/ai/models-8gb-ram.yaml`
- `presets/packs/ai/models-16gb-ram.yaml`
- `presets/packs/ai/models-24gb-ram.yaml`
- `presets/packs/ai/models-64gb-ram.yaml`
- `recipes/actions/agent.sh`

**Modify:**
- `presets/default-512.yaml`
- `presets/default-1tb.yaml`
- `presets/default-2tb.yaml`
- `presets/packs/README.md`
- `src/svalbard/toolkit_generator.py`
- `src/tests/test_presets.py`
- `src/tests/test_toolkit_generator.py`
- `README.md`
- `docs/usage.md`

## V1 Model Matrix

Use this exact initial mapping unless a failing local validation step forces a same-tier substitution:

| Pack | General model | Coding model | Quant target |
| --- | --- | --- | --- |
| `ai/models-8gb-ram` | `gemma-4-e2b-it` | `qwen-9b` | `Q4_K_M` |
| `ai/models-16gb-ram` | `gemma-4-e4b-it` | `qwen-9b` | `Q4_K_M` |
| `ai/models-24gb-ram` | `gemma-4-26b-a4b-it` | `qwen-35b-a3b` | `Q4_K_M` |
| `ai/models-64gb-ram` | `gemma-4-31b-it` | `qwen-35b-a3b` | `Q4_K_M` |

Quant rule for stock presets:

- prefer `Q4_K_M` or `Q5_K_M`
- do not ship full-precision weights in stock packs
- prefer a bigger model at a sane lower quant over a smaller model at a heavier quant

## Curated Stock Preset Matrix

Implement the stock presets with this exact model set:

- `default-512`
  - `gemma-4-e2b-it`
  - `qwen-9b`
- `default-1tb`
  - `gemma-4-e4b-it`
  - `gemma-4-26b-a4b-it`
  - `qwen-9b`
  - `qwen-35b-a3b`
- `default-2tb`
  - `gemma-4-e4b-it`
  - `gemma-4-26b-a4b-it`
  - `gemma-4-31b-it`
  - `qwen-9b`
  - `qwen-35b-a3b`

The stock presets are curated, not fully cumulative. Use RAM-tier packs as the reusable catalog, but compose the built-in presets so they match this exact matrix.

## Task 1: Lock the AI Pack Contract With Failing Tests

**Files:**
- Modify: `src/tests/test_presets.py`

- [ ] **Step 1: Add failing tests for the new AI packs and additive preset behavior**

```python
from svalbard.presets import list_presets, load_preset, resolve_preset_path


def test_list_presets_includes_ai_pack_family():
    presets = list_presets()
    assert "ai/harnesses" in presets
    assert "ai/models-8gb-ram" in presets
    assert "ai/models-16gb-ram" in presets
    assert "ai/models-24gb-ram" in presets
    assert "ai/models-64gb-ram" in presets


def test_resolve_nested_ai_pack():
    path = resolve_preset_path("ai/models-8gb-ram")
    assert path.exists()
    assert path.name == "models-8gb-ram.yaml"
    assert "ai" in path.parts


def test_ai_harnesses_pack_contains_local_ai_clients():
    preset = load_preset("ai/harnesses")
    ids = {source.id for source in preset.sources}
    assert "llama-server" in ids
    assert "opencode" in ids
    assert "crush" in ids
    assert "goose" in ids


def test_ai_models_8gb_pack_contains_general_and_coding_models():
    preset = load_preset("ai/models-8gb-ram")
    ids = {source.id for source in preset.sources}
    assert "gemma-4-e2b-it" in ids
    assert "qwen-9b" in ids


def test_default_1tb_uses_curated_mainstream_and_high_tier_ai_set():
    preset = load_preset("default-1tb")
    ids = {source.id for source in preset.sources}
    assert "gemma-4-e4b-it" in ids
    assert "gemma-4-26b-a4b-it" in ids
    assert "qwen-9b" in ids
    assert "qwen-35b-a3b" in ids


def test_default_2tb_uses_full_curated_ai_set():
    preset = load_preset("default-2tb")
    ids = {source.id for source in preset.sources}
    assert "gemma-4-e4b-it" in ids
    assert "gemma-4-26b-a4b-it" in ids
    assert "gemma-4-31b-it" in ids
    assert "qwen-9b" in ids
    assert "qwen-35b-a3b" in ids
```

- [ ] **Step 2: Run the targeted preset tests to verify they fail**

Run: `uv run pytest -q src/tests/test_presets.py -k "ai_pack_family or nested_ai_pack or ai_harnesses_pack or ai_models_8gb_pack or default_512_uses_curated_ai_pair or default_1tb_uses_curated_mainstream_and_high_tier_ai_set or default_2tb_uses_full_curated_ai_set"`

Expected: FAIL because the nested AI pack files and recipe ids do not exist yet.

- [ ] **Step 3: Commit the test changes after the implementation tasks pass**

```bash
git add src/tests/test_presets.py
git commit -m "test: cover ai pack hierarchy"
```

## Task 2: Add Concrete AI Recipes and RAM-Tier Packs

**Files:**
- Create: `recipes/apps/opencode.yaml`
- Create: `recipes/apps/crush.yaml`
- Create: `recipes/apps/goose.yaml`
- Create: `recipes/models/gemma-4-e2b-it.yaml`
- Create: `recipes/models/gemma-4-e4b-it.yaml`
- Create: `recipes/models/gemma-4-26b-a4b-it.yaml`
- Create: `recipes/models/gemma-4-31b-it.yaml`
- Create: `presets/packs/ai/harnesses.yaml`
- Create: `presets/packs/ai/models-8gb-ram.yaml`
- Create: `presets/packs/ai/models-16gb-ram.yaml`
- Create: `presets/packs/ai/models-24gb-ram.yaml`
- Create: `presets/packs/ai/models-64gb-ram.yaml`
- Modify: `presets/packs/README.md`

- [ ] **Step 1: Create the harness recipes and the first two model recipes**

Use this schema for the first batch:

```yaml
# recipes/models/gemma-4-e4b-it.yaml
id: gemma-4-e4b-it
type: gguf
display_group: models
tags:
- computing
- general-reference
- tool-calling
depth: overview
size_gb: 5.34
url: https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-e4b-it-Q4_K_M.gguf
description: Gemma 4 E4B IT (Q4_K_M) — general/tool-call model for 16 GB RAM hosts
license:
  id: Apache-2.0
  attribution: Google DeepMind / ggml-org
```

```yaml
# recipes/models/qwen-9b.yaml
id: qwen-9b
type: gguf
display_group: models
tags:
- computing
- coding
- agentic
depth: overview
size_gb: 5.9
url: https://huggingface.co/bartowski/Qwen_Qwen3.5-9B-GGUF/resolve/main/Qwen3.5-9B-Q4_K_M.gguf
description: Qwen3.5 9B Instruct (Q4_K_M) — coding-capable model for 16 GB RAM hosts
license:
  id: Apache-2.0
  attribution: Alibaba Cloud / Qwen Team
```

```yaml
# recipes/apps/opencode.yaml
id: opencode
type: binary
display_group: tools
tags:
- computing
- agentic
- coding
depth: reference-only
size_gb: 0.05
platforms:
  linux-x86_64: https://github.com/sst/opencode/releases/latest/download/opencode-linux-x64.tar.gz
  linux-arm64: https://github.com/sst/opencode/releases/latest/download/opencode-linux-arm64.tar.gz
  macos-arm64: https://github.com/sst/opencode/releases/latest/download/opencode-darwin-arm64.tar.gz
  macos-x86_64: https://github.com/sst/opencode/releases/latest/download/opencode-darwin-x64.tar.gz
description: OpenCode terminal coding assistant
license:
  id: MIT
  attribution: OpenCode contributors
```

Mirror this schema for the remaining new recipes using the v1 model matrix above, keeping:

- general models tagged with `tool-calling`
- coding models tagged with `coding`
- stock quant choices in the `description`
- `display_group: models` for GGUFs and `display_group: tools` for bundled binaries
- existing `qwen-9b` and `qwen-35b-a3b` recipes should be reused instead of adding duplicate older-family Qwen recipes

- [ ] **Step 2: Create the remaining model recipes with the exact stock ids**

Use these concrete files and URLs:

```text
recipes/models/gemma-4-e2b-it.yaml
  id: gemma-4-e2b-it
  url: https://huggingface.co/gguf-org/gemma-4-e2b-it-gguf/resolve/main/gemma-4-e2b-it-Q4_0.gguf
  size_gb: 3.04

recipes/models/gemma-4-26b-a4b-it.yaml
  id: gemma-4-26b-a4b-it
  url: https://huggingface.co/ggml-org/gemma-4-26B-A4B-it-GGUF/resolve/main/gemma-4-26B-A4B-it-Q4_K_M.gguf
  size_gb: 16.8

recipes/models/gemma-4-31b-it.yaml
  id: gemma-4-31b-it
  url: https://huggingface.co/ggml-org/gemma-4-31B-it-GGUF/resolve/main/gemma-4-31B-it-Q4_K_M.gguf
  size_gb: 18.7

recipes/models/qwen-9b.yaml
  id: qwen-9b
  url: https://huggingface.co/bartowski/Qwen_Qwen3.5-9B-GGUF/resolve/main/Qwen3.5-9B-Q4_K_M.gguf
  size_gb: 5.9

recipes/models/qwen-35b-a3b.yaml
  id: qwen-35b-a3b
  url: https://huggingface.co/bartowski/Qwen_Qwen3.5-35B-A3B-GGUF/resolve/main/Qwen3.5-35B-A3B-Q4_K_S.gguf
  size_gb: 20.7
```

- [ ] **Step 3: Create the AI pack files as reusable capability bundles**

```yaml
# presets/packs/ai/harnesses.yaml
name: ai/harnesses
kind: pack
display_group: AI
description: Shared local AI runtimes and terminal clients
target_size_gb: 1
sources:
- llama-server
- opencode
- crush
- goose
```

```yaml
# presets/packs/ai/models-8gb-ram.yaml
name: ai/models-8gb-ram
kind: pack
display_group: AI
description: General and coding models sized for 8 GB RAM class hosts
target_size_gb: 8
sources:
- gemma-4-e2b-it
- qwen-9b
```

```yaml
# presets/packs/ai/models-16gb-ram.yaml
name: ai/models-16gb-ram
kind: pack
display_group: AI
description: General and coding models sized for 16 GB RAM class hosts
target_size_gb: 16
sources:
- gemma-4-e4b-it
- qwen-9b
```

```yaml
# presets/packs/ai/models-24gb-ram.yaml
name: ai/models-24gb-ram
kind: pack
display_group: AI
description: General and coding models sized for 24 GB RAM class hosts
target_size_gb: 24
sources:
- gemma-4-26b-a4b-it
- qwen-35b-a3b
```

```yaml
# presets/packs/ai/models-64gb-ram.yaml
name: ai/models-64gb-ram
kind: pack
display_group: AI
description: Highest-capability portable general and coding models for 64 GB RAM class hosts
target_size_gb: 64
sources:
- gemma-4-31b-it
- qwen-35b-a3b
```

Add one paragraph to `presets/packs/README.md`:

```markdown
- `ai/*`: RAM-tiered local AI bundles. `ai/harnesses` carries shared runtimes and terminal clients. `ai/models-*-ram` packs each include one general/tool-call model and one coding model.
```

- [ ] **Step 4: Run the targeted preset tests to verify the pack tree passes**

Run: `uv run pytest -q src/tests/test_presets.py -k "ai_pack_family or nested_ai_pack or ai_harnesses_pack or ai_models_8gb_pack"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add recipes/apps recipes/models presets/packs/ai presets/packs/README.md src/tests/test_presets.py
git commit -m "feat: add portable ai recipes and packs"
```

## Task 3: Recompose the Large Default Presets Through AI Packs

**Files:**
- Modify: `presets/default-512.yaml`
- Modify: `presets/default-1tb.yaml`
- Modify: `presets/default-2tb.yaml`
- Modify: `src/tests/test_presets.py`

- [ ] **Step 1: Replace direct AI recipe references in `default-512` with pack-based composition**

```yaml
name: default-512
description: Region-neutral large reference archive with the first portable AI tier
target_size_gb: 512
region: default
extends:
  - default-256
  - ai/harnesses
  - ai/models-8gb-ram
sources:
# ── Q&A ──
- stackexchange-math
# ── Preparedness media ──
- canadian-prepper-bugout-concepts
- canadian-prepper-bugout-roll
- canadian-prepper-prepping-food
- canadian-prepper-winter-prepping
- urban-prepper
- s2-underground
```

- [ ] **Step 2: Make `default-1tb` additive instead of replacing the smaller AI tier**

```yaml
name: default-1tb
description: Region-neutral full reference archive with additive portable AI tiers
target_size_gb: 1024
region: default
extends:
  - default-512
  - ai/models-16gb-ram
sources:
# ── Wikipedia upgrade ──
- -wikipedia-en-nopic
- wikipedia-en-maxi
# ── Q&A ──
- stackexchange-chemistry
- stackexchange-biology
- stackexchange-engineering
# ── Preparedness ──
- lrn-self-reliance
```

- [ ] **Step 3: Make `default-2tb` additive and attach the 24 GB and 64 GB packs**

```yaml
name: default-2tb
description: Maximum region-neutral archive with additive portable AI tiers
target_size_gb: 2048
region: default
extends:
  - default-1tb
  - ai/models-24gb-ram
  - ai/models-64gb-ram
sources:
# ── Wikipedia upgrade ──
- -wikipedia-en-maxi
- wikipedia-en-all
# ── Q&A ──
- stackexchange-serverfault
- stackexchange-superuser
```

- [ ] **Step 4: Replace the old qwen-specific assertions with additive pack assertions**

```python
def test_default_512_includes_ai_harnesses_and_8gb_pair():
    preset = load_preset("default-512")
    ids = {source.id for source in preset.sources}
    assert "llama-server" in ids
    assert "opencode" in ids
    assert "gemma-4-e2b-it" in ids
    assert "qwen-9b" in ids


def test_default_1tb_adds_16gb_models_without_removing_8gb_pair():
    preset = load_preset("default-1tb")
    ids = {source.id for source in preset.sources}
    assert "gemma-4-e4b-it" in ids
    assert "qwen-9b" in ids
    assert "gemma-4-26b-a4b-it" in ids
    assert "qwen-35b-a3b" in ids


def test_default_2tb_is_additive_across_all_ai_tiers():
    preset = load_preset("default-2tb")
    ids = {source.id for source in preset.sources}
    assert "gemma-4-e4b-it" in ids
    assert "gemma-4-26b-a4b-it" in ids
    assert "gemma-4-31b-it" in ids
    assert "qwen-9b" in ids
    assert "qwen-35b-a3b" in ids
    assert "gemma-4-31b-it" in ids
    assert "qwen-35b-a3b" in ids
```

- [ ] **Step 5: Run the preset test file**

Run: `uv run pytest -q src/tests/test_presets.py`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add presets/default-512.yaml presets/default-1tb.yaml presets/default-2tb.yaml src/tests/test_presets.py
git commit -m "feat: recompose default presets around ai packs"
```

## Task 4: Add a Drive-Side AI Client Launcher and Menu Entries

**Files:**
- Create: `recipes/actions/agent.sh`
- Modify: `src/svalbard/toolkit_generator.py`
- Modify: `src/tests/test_toolkit_generator.py`

- [ ] **Step 1: Add failing toolkit tests for AI client menu entries**

```python
def test_entries_tab_includes_ai_client_launchers_when_binaries_exist(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-512",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "gemma-4-e2b-it", "type": "gguf",
             "filename": "gemma-4-e2b-it-Q4_0.gguf",
             "size_bytes": 3_040_000_000, "tags": [], "depth": "overview"},
            {"id": "opencode", "type": "binary",
             "filename": "opencode-linux-x64.tar.gz",
             "size_bytes": 50_000_000, "tags": [], "depth": "reference-only"},
        ],
    })
    (tmp_path / "models").mkdir()
    (tmp_path / "models" / "gemma-4-e2b-it-Q4_0.gguf").touch()
    (tmp_path / "bin").mkdir()

    generate_toolkit(tmp_path, "default-512")

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "Open OpenCode with local model" in entries
    assert ".svalbard/actions/agent.sh" in entries
```

- [ ] **Step 2: Create `recipes/actions/agent.sh` as the AI client wrapper**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

client="${1:?Usage: agent.sh <client-id>}"
client_bin="$(find_binary "$client" 2>/dev/null || true)"
llama_bin="$(find_binary llama-server 2>/dev/null || true)"
[ -n "$client_bin" ] || { ui_error "Client not found: $client"; exit 1; }
[ -n "$llama_bin" ] || { ui_error "llama-server not found"; exit 1; }

mapfile -t models < <(find "$DRIVE_ROOT/models" -name "*.gguf" -not -name "._*" -type f | sort)
[ "${#models[@]}" -gt 0 ] || { ui_error "No GGUF model found in models/"; exit 1; }

echo "Select model:"
select model in "${models[@]}"; do
    [ -n "${model:-}" ] && break
done

trap_cleanup
port="$(find_free_port 8084)"
"$llama_bin" -m "$model" --jinja --port "$port" --host 127.0.0.1 &
SVALBARD_PIDS+=($!)
sleep 2

case "$client" in
  opencode)
    mkdir -p "$DRIVE_ROOT/.svalbard/config/opencode"
    cat > "$DRIVE_ROOT/.svalbard/config/opencode/opencode.json" <<JSON
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "llama.cpp": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "llama-server (local)",
      "options": { "baseURL": "http://127.0.0.1:${port}/v1" }
    }
  }
}
JSON
    OPENCODE_CONFIG="$DRIVE_ROOT/.svalbard/config/opencode/opencode.json" "$client_bin"
    ;;
  crush)
    mkdir -p "$DRIVE_ROOT/.svalbard/config/crush"
    cat > "$DRIVE_ROOT/.svalbard/config/crush/crush.json" <<JSON
{
  "providers": {
    "local": {
      "type": "openai",
      "api_key": "local",
      "base_url": "http://127.0.0.1:${port}/v1",
      "models": [{ "id": "local-model", "name": "Local llama.cpp model" }]
    }
  }
}
JSON
    CRUSH_CONFIG_DIR="$DRIVE_ROOT/.svalbard/config/crush" "$client_bin"
    ;;
  goose)
    OPENAI_API_KEY=local OPENAI_BASE_URL="http://127.0.0.1:${port}/v1" "$client_bin"
    ;;
esac
```

- [ ] **Step 3: Add AI client menu entries to `toolkit_generator.py`**

Insert this block just after the existing chat model entries in `_build_entries()`:

```python
    ai_client_ids = {"opencode": "OpenCode", "crush": "Crush", "goose": "Goose"}
    available_ai_clients = [
        source for source in preset.sources
        if source.id in ai_client_ids and source.type == "binary"
    ]
    if available_ai_clients:
        if "[ai]" not in lines:
            lines.append("[ai]")
        for source in available_ai_clients:
            lines.append(
                f"Open {ai_client_ids[source.id]} with local model"
                f"\t.svalbard/actions/agent.sh\t{source.id}"
            )
        lines.append("")
```

Keep the existing model-chat entries above this block. Do not remove the browser chat path in v1.

- [ ] **Step 4: Run the toolkit generator tests**

Run: `uv run pytest -q src/tests/test_toolkit_generator.py -k "ai_client_launchers or entries_tab_includes_maps or entries_tab_includes_search or entries_tab_includes_serve"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add recipes/actions/agent.sh src/svalbard/toolkit_generator.py src/tests/test_toolkit_generator.py
git commit -m "feat: add local ai client launcher"
```

## Task 5: Update User-Facing Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/usage.md`

- [ ] **Step 1: Update the roadmap and preset language in `README.md`**

Replace the roadmap line:

```markdown
- [ ] Offline coding assistant — bundled LLM + editor (opencode) as a self-contained dev environment
```

with:

```markdown
- [ ] Offline coding assistant — RAM-tiered local AI packs, bundled llama.cpp runtime, and terminal AI clients
```

Add one short note under the preset section:

```markdown
Portable AI begins at `default-512` and scales additively through `default-1tb` and `default-2tb`; larger presets keep smaller-model compatibility instead of replacing it.
```

- [ ] **Step 2: Add AI pack usage guidance to `docs/usage.md`**

Append a short section:

```markdown
## AI Packs

Large default presets compose nested AI packs:

- `ai/harnesses`: local AI runtimes and terminal clients
- `ai/models-8gb-ram`
- `ai/models-16gb-ram`
- `ai/models-24gb-ram`
- `ai/models-64gb-ram`

These packs are additive. Larger presets keep lower-RAM models available so the same drive remains useful on older or smaller host machines.

Custom presets may either extend the built-in AI packs or reference individual AI recipes directly.
```

- [ ] **Step 3: Run the targeted doc-adjacent regression tests**

Run: `uv run pytest -q src/tests/test_presets.py src/tests/test_toolkit_generator.py`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add README.md docs/usage.md
git commit -m "docs: explain ai pack tiers and launcher flow"
```

## Self-Review

### Spec Coverage

- Nested `presets/packs/ai/*` packs: covered by Tasks 1-2
- `recipe -> pack -> preset` composition: covered by Tasks 2-3
- additive `default-*` tiers: covered by Task 3
- `ai/harnesses` semantics: covered by Task 2
- terminal AI launcher/runtime behavior: covered by Task 4
- docs for stock vs custom composition: covered by Task 5
- stock quant guidance: encoded in Task 2 recipe descriptions and model matrix

### Placeholder Scan

- No `TODO`, `TBD`, or deferred “implement later” markers remain in the task steps.
- The one deliberate v1 simplification is explicit: keep the existing browser chat path and add a separate terminal AI launcher instead of replacing it.

### Type Consistency

- AI clients are treated as `binary` recipes and launched through a shell action, not through the static-app path.
- GGUF models remain `type: gguf` and land in `models/`.
- Nested AI packs are referenced through `extends:` just like existing nested packs.
