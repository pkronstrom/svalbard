# Agent tools implementation plan

The three designs cover independent user paths. Each path has a focused regression test and a smoke check on a minimal vault. The shared Open WebUI launcher connects the separate pieces and owns their lifecycle.

## Live web search

- [x] Add the keyless Go client for DuckDuckGo HTML and an explicitly configured SearXNG JSON endpoint. Bound the request, response and citation results. Test provider failures and result shapes.
- [x] In `drive-runtime/internal/{settings,mcp,agent}` and the drive CLI, give the helper one opt-in decision for discovery, direct calls and stdio. Include the decision in oh-my-pi's tool allowlist. Check that off, invalid and legacy settings expose no tool.
- [x] On Open WebUI's existing MCP listener, mount the same capability as a named connection only when enabled. Keep the other HTTP and SSE entry points vault-only.
- [x] Verify direct CLI, pi and Open WebUI with a loopback SearXNG-compatible provider. Check the direct CLI and a stale stdio call with web search turned off.

## Small PDF uploads

- [x] In `drive-runtime/internal/openwebui`, implement the external loader contract and tests for authentication, file input limits, worker cancellation, physical page labels and clean failure.
- [x] In the Open WebUI recipe, make the pinned PDF worker available on minimal Open WebUI installs. Verify that the external route preserves the pinned slim loader's text, CSV and HTML behavior.
- [x] Wire the local loader into the launcher with the smallest model context in the launch. Verify ingestion and paginated reading in the pinned browser UI with a local model.

## Workspace HTML packing

- [x] In `drive-runtime/internal/htmlpack` and `internal/workspace`, add a bounded rooted mode with explicit dependency resolution and atomic output. Test transitive reads, paths outside the workspace, cancellation, preview compatibility and output that works by itself.
- [x] Add `file_pack_html` to the Open WebUI workspace capability when file tools and libraries are available. Verify an interactive chart through preview and from a copy of the packed file alone.

## Integration

- [x] Run catalog sync and `make verify` from the isolated worktree. The full suite, including the HTML packer race tests, passed. The Linux ARM64 drive runtime cross-build passed.

## Smoke results

- macOS ARM64. A clone of the minimal vault reused Qwen3.5 4B and the cached Open WebUI 0.13.5 and pypdf 7.2.0 packages. At 8K, the pinned browser UI accepted a mixed PDF and the model cited physical page 3 for the fuse information. The attachment tool path stayed paginated for ordinary text.
- The browser and pi both cited the Go release notes URL returned by the loopback SearXNG-compatible fixture. The direct CLI and HTTP and stdio MCP smoke calls returned the same result. Live DuckDuckGo HTML and Lite requests returned HTTP 202 challenges on this host. A real SearXNG installation and the other harnesses' model turns remain unverified.
- A browser chat returned the packed chart link. Chromium rendered the chart through the workspace preview and from only the packed file in an empty folder under offline network emulation.
- PDF extraction and its memory check were also exercised in a Linux ARM64 container. Full Open WebUI startup on Linux, Intel Mac, and a physical exFAT stick remain unverified.
