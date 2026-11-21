# Offline PlatformIO implementation plan

> For agentic workers: execute task by task using subagent-driven-development, with spec compliance and code quality review. Preserve shared-tree edits; no staging, commits, Git metadata mutations or worktrees.

Status: executing; design and plan reviewed 2026-10-07.

**Goal:** A normally provisioned ESP32 pack builds its included ESP-IDF projects from the stick on a clean host with networking disabled.
**Architecture:** Complete pinned closure acquired by existing apply/builders; generated inventory consumed by native embedded action. Bundled Core installs local archives into content-addressed host cache generations, with separately bundled IDF/esptool Python dependencies and a truthful readiness marker.
**Tech stack:** Existing Go modules, standalone Python 3.12/uv builder, PlatformIO Core 6.2.0, espressif32 7.1.3, ESP-IDF 6.1.
**Design:** [offline PlatformIO](2026-10-07-offline-platformio-design.md).

## Workspace policy

All pre-existing Phase 2–4 and catalog refresh work is retained. Coordinator owns docs/state/final acceptance. Workers are not alone and must accommodate others without reverting their edits. Capture baseline before source edits at `/private/tmp/svalbard-phase5-baseline`. Local network fixture/Docker calls may need sandbox escalation. Use `GOCACHE=/private/tmp/svalbard-go-cache`.

## Shared interface

`.svalbard/platformio.json` JSON: `{"version":1,"packages":[{"path":"tools/platformio/packages/...","sha256":"...","owner":"platformio","name":"tool-esptoolpy","kind":"tool","platform":""}]}`. Host derives owner/kind/name from official registry URL and associates each realized archive by basename; platform is the matching normalized recipe platform or empty. Do not record requested recipe version over the installed archive. Required Python IDs are `platformio-core` and `espidf-python`, installed beneath `runtime/python/<platform>/tools/`; wrappers remain existing `bin/<platform>/pio` and `platformio`. Native `Prepare(ctx, stdout, driveRoot)` returns the ready generation directory for the shell and acceptance path; tests may call it without starting an interactive shell.

## Chunk 1 — Complete acquisition and generated inventory

Ownership: `recipes/tools/` PIO recipes, `presets/packs/embedded/esp32-dev.yaml`, `host-cli/internal/apply/`, `host-cli/internal/toolkit/` inventory generation only, `host-cli/internal/builder/pythonvenv.go` and its tests, catalog embedded copies through sync script. Do not edit runtime or downloader, or unrelated recipes.

- [ ] Query official registry version metadata for every required tool and all four target archives; pin available exact compatible versions/checksums, add missing Intel Mac variants. Add optional catalog.Item.PlatformSHA256 (`platform_sha256`) parallel to Platforms; fetch uses the matched candidate key checksum when present, retaining shared SHA defaults for old recipes. Cover parsing and different per-host digests in fixtures. Keep the corrected OpenOCD pin. Include espressif32 platform, SCons, CMake, Ninja, both GDBs, ROM ELF and RISC-V ULP dependency.
- [ ] Add `platformio-core` python-package with pinned `platformio==6.2.0`, pio/platformio entrypoints, explicit dependency on svalbard-python. Add `espidf-python` separate package tree with exact compatible top-level pins and their required transitive closure, including pip and esptool's cryptography/ecdsa/bitstring/reedsolo/intelhex. Resolve/test against the pinned PlatformIO builder dictionary and actual example builds before finalizing pins; do not mix incompatible rolling standalone IDF constraints; Core/IDF urllib3 are separate.
- [ ] Write failing apply fixtures asserting shared archives land under tools/platformio/packages and per-platform archives under the normalized target subdirectory; missing required target fails rather than silently completing. Implement a destination-aware fetch helper without changing other recipe types.
- [ ] Write inventory generation fixtures for shared + multi-platform packages, owner espressif, realized legacy root paths, missing/ambiguous URL association (the same official universal Mac archive may map to both targets; distinguish using normalized destination path or intentionally emit one record per compatible target for a known legacy shared archive) and non-PIO recipes. Add `GenerateOpts.PlatformIORecipes` map of ID→catalog.Item collected by apply. Emit only recognized realized registry archives with their actual recorded paths and checksums; missing expected closure is detected runtime. Inventory is regenerated atomically with toolkit config. Do not remove ordinary menu generation.
- [ ] Schedule an already realized svalbard-python builder when pending python-package IDs require new trees, using complete desired IDs to retain existing selected tools. Add integration fixture for selecting platformio-core on a vault already containing svalbard-python; ordinary apply must install it instead of reporting unrealized. Validate requested PIO target coverage before inventory publication; old/incomplete incompatible closure fails with concrete reprovision guidance rather than success. Keep recipe-version drift updates deferred.
- [ ] Add strict Python target behavior only for the two PIO package IDs: missing wheels/interpreter/entrypoints on any requested supported target is a failure, with completed artifacts preserved for retry. Test this through existing builder fixtures if available; avoid rewriting general Python builder behavior.
- [ ] Run `scripts/sync-catalog.sh`; `cd host-cli && go test ./internal/catalog ./internal/apply ./internal/toolkit ./internal/builder`; inspect exact recipe registry availability. Write handoff `.work/handoffs/pio-acquisition.md` with pins and unresolved target limitations.

## Chunk 2 — Native offline cache preparation

Ownership: `drive-runtime/internal/embedded/` production/helper/tests and `drive-runtime/internal/shell/` narrow shared environment-after-rc helper/tests. Dependency: shared interface settled; can implement against fixtures while Chunk 1 acquisition verification runs.

- [ ] Add failing fixtures for missing inventory/archive/Python tree, wrong checksum, unchanged reuse, content change, relocation, lock cancellation, incomplete retry, and absence of false ready messages. Use tiny fixtures where possible; actual Core acceptance belongs Chunk 4.
- [ ] Replace the lossy native extractor with embedded Python bootstrap. Read/version-check inventory; select shared/current-platform entries; validate paths, unique package identities, complete pinned ESP-IDF dependency list, SHA-256. Resolve bundled Python by existing standalone layout; require Core and IDF package trees and wrapper.
- [ ] Content-hash archive bytes and required Python tree contents with caller-context checks. Scope default cache using os.UserCacheDir + vault identity, retain SVALBARD_PIO_CACHE as a root. Use cancellable flock and fingerprint generation final paths, with atomic ready marker; never rename a venv after creation. Preserve previous ready generations and remove only the failed generation after children are reaped.
- [ ] Bootstrap uses local typed PackageSpec owner/name/uri and skip_dependencies; audit hooks, replace known esptool pip hook with prebundled _contrib and reject unknown hooks. Keep Core extraction structure/symlinks and .piopm. Validate actual versions against pinned platform requirements, including SCons.
- [ ] Create cached IDF venv with bundled Python, copy espidf-python into real site-packages, validate imports/distribution compatibility, write exact IDF_ENV_VERSION/interpreter marker. Ensure esptool works with its copied dependencies. Disable telemetry/update checks and PIP_NO_INDEX for preparation/use; no host Python fallback or .pio_skip_pypackages.
- [ ] Wire Prepare into Run; reuse internal/shell temporary rc to preserve user rc and reapply environment/PATH afterward (test a user rc overwriting PATH). Merge environment keys uniquely. Use bundled bin path and selected cache, preserve ordinary per-project build directories and explicit user's overrides. Terminate/reap bootstrap process group on cancellation before lock release/cleanup. Verify lock and cache callers are race safe.
- [ ] Run `cd drive-runtime && go test -race ./internal/embedded`; write `.work/handoffs/pio-runtime.md` with exact tested boundaries and bootstrap command for acceptance.

## Chunk 3 — Mirror reliability and user path

Ownership: coordinator `host-cli/internal/commands/index_configured.go` and its tests (minimal fresh tools-only post-apply no-op while preserving missing-source protection for existing indexes), `host-cli/internal/downloader/`, `recipes/actions/pio-setup.sh`, `recipes/actions/docs/embedded-getting-started.md`, example files under toolkit generation (coordinate with Chunk1 worker), menu alias.

- [ ] Reproduce official redirect failure then write controlled HTTP fixtures for canonical failure, official mirror HEAD Location/X-PIO-Mirror/X-PIO-Content-SHA256 + bypass, checksum mismatch, cancellation and unchanged partial identity. Keep existing resumable sidecar tests.
- [ ] Add narrowly scoped official registry retry via server-advertised mirrors; retain canonical URL as acquisition identity. Different mirror validators must not splice partial bytes; pinned checksum verifies final bytes. Limit retries and don't retry caller cancellation or a completed checksum rejection through arbitrary hosts.
- [ ] Ship a small ESP-IDF blink example with pinned platform, esp32dev, S2 and S3 environments, CMake files and a no-external-component source. Generate to tools/platformio/examples; do not mutate user working projects. Add an embedded alias; replace legacy setup script with current native action dispatch.
- [ ] Rewrite getting-started guide for normal provisioning, copying example to a chosen host workdir, first-use/retry cache, build/upload/monitor, supported targets and explicit limitations. Remove host brew/pip prerequisite and unsupported full-offline claims.
- [ ] Run `cd host-cli && go test -race ./internal/downloader ./internal/toolkit ./internal/apply`; verify guide commands against bundled action and example.

The real CLI gate exposed that default indexing attempted to scan a nonexistent ZIM directory in a fresh tool-only vault. Skip this automatic stage only when a valid manifest records no ZIM, no archive directory exists and no existing search database exists. Retain errors for missing source directories with existing indexes or recorded archives.

## Chunk 4 — Actual offline acceptance and final review

Ownership: coordinator docs/state/MASTERPLAN, acceptance scratch scripts; fresh reviewers read all feature deltas.

- [ ] Build/provision a disposable native macOS arm64 esp32-dev vault through normal CLI/apply, isolated Docker/uv acquisition as needed; no manual final artifact layout. Verified official cache sources may accelerate downloads, but separately exercise fresh registry mirror fallback. Record actual sizes/version outputs.
- [ ] Run actual packaged action preparation/build with fresh isolated cache, restricted PATH excluding host Python/PIO, and OS networking denied (`sandbox-exec` deny network on macOS, verify a test socket is blocked). Compile included ESP32/S2/S3 environments, inspect nonempty ELF/bin and run bundled esptool version. Rerun same generation, relocate vault, and cancel/retry preparation; retain working generation and observe no partial ready markers.
- [ ] Spec compliance review first, then correctness/code quality review; verify findings before fixing. Run fresh `make verify`, affected race suites, `make build-drive-runtime`, `make build` and `git diff --check` once final changes settle.
- [ ] Record checkpoint with exact evidence, hardware/unexecuted host limitations and remaining items. Mark Phase5 complete only if real offline acceptance succeeds; leave Phase6 untouched. Update `.work/state.yaml` and handoff.

## Done condition

Fresh host cache + bundled commands + blocked networking produces the example firmware through the normal provisioned pack, with truthful cancellation/cache freshness and all required checks passing. A recipe list, warm-cache build or mocked extraction alone does not satisfy completion.
