# Local AI Serving Settings + pi Harness — Design

**Date:** 2026-06-09
**Status:** Approved, pending implementation plan
**Scope:** drive-runtime serving config (adaptive context + launch picker), harness client-config threading, and bundling the `pi` coding agent.

## Goal

Give svalbard's on-drive local-AI serving **sane, model-aware defaults** that **auto-scale context to the host** while letting the user **dial context down for speed** at launch — without a config-file subsystem. Bundle the `pi` coding agent as a third harness alongside OpenCode and Goose.

## Background / constraints

- drive-runtime has **no catalog access at serve time** — it globs `models/*.gguf`. Serving decisions are therefore derived from the model filename and the host environment, not recipe metadata.
- Three llama-server launch sites: `chat` (interactive), `agent` (interactive, per-harness), `serveall` (non-interactive LAN share).
- Bundled llama.cpp is **b9584** (post-MTP-merge), invoked via static per-platform binaries.

### Already implemented in the working tree (context for this design)

- Model recipes refreshed: Qwen3.5→3.6 (top tier + new dense `qwen-27b` on the MTP build), Gemma 4 → QAT GGUF.
- `llamaserve.ExtraFlags(modelPath)` emits, filename-gated: per-family sampling (Qwen `temp 0.6/top-k 20/min-p 0`; Gemma `temp 1.0/top-k 64/min-p 0`), Qwen3.6 q8-KV + flash-attn (Gated-DeltaNet correctness), and `--spec-type draft-mtp` for MTP builds. Wired into all three launch sites.
- A **hardcoded `--ctx-size 16384`** — **this design replaces it** with the adaptive value below.

## Design

### 1. Adaptive context — `llamaserve.ContextForHost`

`ContextForHost() int` returns an "Auto" context size by total-RAM tier:

| Total RAM | Context |
|---|---|
| ≤ 8 GB | 8192 |
| 8–16 GB | 32768 |
| 16–32 GB | 65536 |
| ≥ 32 GB | 131072 |

Rationale: weights are mmap'd, so the KV cache is the RAM lever we control; tiering on total RAM is robust and needs no GGUF parsing. RAM detection: Linux `/proc/meminfo` `MemTotal`, macOS `sysctl -n hw.memsize`. The RAM read sits behind a swappable function so tiers are table-testable. Detection failure → safe fallback of 8192.

### 2. `ExtraFlags` takes context as a parameter

`ExtraFlags(modelPath string, ctxSize int) []string` — emits `--ctx-size <ctxSize>` plus the existing sampling / q8-KV / MTP flags. The hardcoded `DefaultContext` constant is removed.

### 3. Launch-time context picker

Interactive paths (`chat`, `agent`) show a small Bubble Tea list (shared `tui` package) before llama-server starts:

```
Context window:
▸ Auto — 32K  (detected: 16 GB RAM)
  Fast — 8K   (snappier, less memory)
  Balanced — 32K
  Max — 128K  (host ceiling)
```

- **Enter accepts Auto** (default highlighted) — zero friction.
- Options are fixed presets: `Fast` 8K · `Balanced` 32K · `Max` 128K, plus `Auto` (the `ContextForHost()` tier, highlighted by default).
- Presets larger than the Auto tier are still selectable but labeled "(may be slow / high memory)" — pushing past the host's safe tier is the user's explicit choice, not hidden. Auto remains the safe recommendation.
- Non-TTY / piped stdin → skip the picker, use `ContextForHost()` silently.
- `serveall` never shows the picker — uses `ContextForHost()` directly.

### 4. Thread the resolved context to harness clients

The single resolved context value flows to both the server (`--ctx-size`) and the generated harness config so clients compact at the right point instead of being server-truncated:

- OpenCode `opencode.json` → model `contextWindow`/`context_length`.
- Goose → model context setting.
- pi `models.json` → model `contextWindow`.

(Exact key names verified against each tool before writing — see implementation plan.)

### 5. Bundle pi

- **Recipe** `recipes/apps/pi.yaml` (+ embedded copy): `type: binary`, four platform URLs at `https://github.com/earendil-works/pi/releases/latest/download/pi-{linux,darwin}-{x64,arm64}.tar.gz` (floating `latest`, matching OpenCode/Goose house style), `menu:` entry in the `local-ai` / "AI Clients" group.
- pi ships official Bun-compiled binaries as a **directory** (executable + `photon_rs_bg.wasm` + native sidecars + assets). svalbard's `runtimebinary.go` preserves directory structure / symlinks / exec bits on `.tar.gz` extraction, and `binary.Resolve` finds nested executables — so **no binary-subsystem changes are required**; sidecars stay adjacent to `pi` and resolve via `process.execPath`.
- **Agent integration** `agent.go` `case "pi"`: write pi's `models.json` (provider → local llama-server at `baseURL + /v1`, `api: openai-completions`, `apiKey: local`, model with the resolved `contextWindow`), point `PI_CODING_AGENT_DIR` at the sandboxed runtime dir (same `HOME`/`XDG` isolation as OpenCode/Goose), launch `pi --model llama.cpp/<model>`. Reference: turbollm `src/turbollm/harnesses/pi.py`.
- **MCP at parity with OpenCode/Goose:** pi supports MCP natively (no plugin) via `mcp.json` in the agent dir. Write `$PI_CODING_AGENT_DIR/mcp.json` with an `mcpServers.svalbard` stdio entry (`command`: the svalbard binary, `args`: `["mcp", "--drive", <driveRoot>]`) — mirroring the OpenCode `mcp` block. The optional third-party `pi-mcp-adapter` is not used.

## Error handling

- RAM detection failure → 8192 fallback.
- Picker on non-TTY → Auto, no prompt.
- pi `latest` asset-name drift (project mid-org-rename badlogic→earendil-works) → accepted risk; pin `v0.79.0` if it breaks.

## Testing

- `ContextForHost` tier boundaries (table test, injected RAM).
- `ExtraFlags(path, ctx)` for each family with explicit context.
- Picker default-selection + non-TTY fallback logic.
- pi recipe parses; catalog validation passes; `case "pi"` writes a well-formed `models.json` and `mcp.json` (svalbard stdio server) into the sandboxed agent dir.

## Out of scope (YAGNI)

- User-editable `serve.toml` registry (the full turbo-style config). Adaptive default + launch picker covers the stated need.
- Per-model sampling overrides beyond the two published family recipes.
