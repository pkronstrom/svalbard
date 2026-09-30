# Concurrent Claude session — coordination snapshot

Session: `e58429bd-4762-4c52-80d1-dcd7df2b59d1`, in the same Svalbard working tree. Read-only session inspection on 2026-09-30, through the lead's 14:16:43 UTC / 17:16:43 Helsinki final report. This is a session snapshot, not a new implementation plan or an independent rerun of its benchmarks.

## Completed or reported complete

| Area | Work and evidence |
| --- | --- |
| Catalog fixes | Corrected broken tool/content pins, renamed ZIM patterns, measured sizes, removed unavailable content; SYKE multi-asset vector support and resolver suffix handling. |
| Content/presets | Added 32 ZIM recipes, rebalanced default/Finnish tiers, moved oversized Gutenberg/Khan archives upward, updated specialization packs and README. The lead reports 41 new/changed ZIM URLs resolving. |
| Custom presets | XDG user preset directory plus dev `local/presets/`; shared extends/removal resolution, strict validation, wizard listing, copy stubs/flatten option, pack names for add/remove. Agent reports host CLI/TUI tests passing. This is the file-based foundation, not completion of the Phase 3 TUI customization journey. |
| Live local AI | Model agent reports real Qwen3.8 tool tasks passing in Pi (15 s), OpenCode (23 s), Goose (42 s), and Gemma 12B tasks passing in Pi/OpenCode. These are its M4 Max results, not universal comparisons. Retained llama.cpp b11146. |
| AI changes after our checkpoint | Added embedded patched Qwen chat template for mid-conversation system messages and minimal/max thinking; renamed Pi provider to `svalbard`; changed 8 GB tier to E4B + Bartowski Qwen4B; sizes now GiB. Dropped Gemma MTP after reported slowdown, retained optional Qwen drafter pack. |
| Python-package provisioning | Python agent reports replacing nonexistent `uv pip download` with cross-target Python acquisition + `uv pip install --target`, preinstalled package directories and relocatable wrappers. Real builds for four platforms, runtime checks on macOS arm64 and both Linux architectures. sqlmap/pwntools/volatility/binwalk install on all four; angr skips Linux arm64 and macOS x86_64 due compatible-wheel gaps. Binwalk now uses the OSPG v2.4.3 source fork because upstream v2.3.4 crashes under Python 3.12. |

The lead reports `make verify` passing (52 packages, catalog check included). Its final link check reports all download URLs resolving except the known unavailable MML topo source, plus nine dead license links repaired. It counts 57 completed and 47 open audit items. Nothing is committed. macOS x86_64 execution, large Q6 inference, long thinking sessions and actual Svalbard MCP tool journeys remain unverified in the agent reports.

## Latest state / proposed next work

- The lead now says all approved work is done and is waiting for decisions, rather than continuing implementation automatically.
- It proposes approximately four commits separating pins/links, content/presets, models/runtime, and Python/custom presets. No commit was made.
- Its script assessment proposes deleting unused `buildItem()`, `wikipedia-compact-zim.py` and `media-zim.py`, and deciding whether Pinkka gets a recipe or is removed. No completed Go port or authorized deletion is evidenced.
- Other decisions: Qwen separate drafter versus built-in-layer toggle; fail-fast versus warn-and-skip for malformed user presets; whether Python recipe sizes count one platform or all installed platforms.
- The 47 open audit items are not all promised next implementation. Reconcile those with the masterplan before taking them on.

Open decisions reported there: retain separate Qwen drafter file versus an on/off switch using the layer already in the main GGUF; fail all catalog loads on a malformed custom preset versus warn-and-skip. Its MTP measurements on Metal showed no Qwen speedup and a Gemma slowdown, so preserve optionality and hardware-specific attribution.

## Coordination

That session handled the Phase 1b work: Python builder/package recipes, content/preset curation, custom-preset interfaces, model/template follow-ups and integration. Avoid duplicating or overwriting it. Our next distinct focus is Phase 2, the initial deployment/cancel/restart/resume path. Phase 3 still needs the TUI journey; Phase 7 acquisition hashes are still unimplemented.

Our earlier [refresh results](2026-09-30-catalog-refresh-results.md) and [continuation checkpoint](2026-09-30-continuation.md) describe the earlier verified snapshot. Their Gemma MTP, 8 GB lineup, Pi provider, model-size units and unverified live-chat/Python statements are superseded by the work reported above. Do not restore those older choices or repeat the already completed work.

## Session records

- Lead: `/Users/pkronstrom/.claude/projects/-Users-pkronstrom-Projects-own-svalbard/e58429bd-4762-4c52-80d1-dcd7df2b59d1.jsonl`; authorization around line 680, latest verification around line 1168.
- Models: same project/session subdirectory, `subagents/agent-amodels-874e35bd42f6697b.jsonl`; final report around line 820.
- Python: `subagents/agent-apybuilder-771306cc528f64db.jsonl`; detailed final report around line 341.
- Custom presets: `subagents/agent-acustompresets-48123685f150b0b2.jsonl`; lead received completion around line 898.

The active lead is updating [catalog audit](2026-09-30-catalog-update-audit.md). Re-read its newest messages before starting work in overlapping files.
