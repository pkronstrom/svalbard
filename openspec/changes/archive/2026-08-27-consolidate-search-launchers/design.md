## Context

Session and MCP have different ownership models but may still duplicate embedding and Kiwix binary/model resolution, flags, port allocation, readiness polling, timeout cleanup, and process waiting after terminal search moves onto Session.

## Goals / Non-Goals

**Goals:**
- Reinspect post-search-consolidation code before extracting anything.
- Share only concrete launch protocols that remain duplicated.
- Keep process ownership explicit at each caller.

**Non-Goals:**
- A generic process supervisor or configurable launcher framework.
- Consolidating Browse, Maps, Share, Apps, or Serve-all.
- Removing MCP caching or packaged-directory resolution.

## Decisions

- Implement after `consolidate-search-session`.
- Keep separate concrete embedding and Kiwix launchers.
- A launcher returns a process and bound port only after readiness succeeds.
- Startup failure and timeout kill and wait for the child before returning.
- Session closes only per-session processes; MCP retains its `sync.Once` reuse and closes cached processes from its existing `Close`.
- Skip an extraction when post-refactor code no longer has meaningful duplication.

## Risks / Trade-offs

- Sharing ownership policy would couple unrelated lifetimes; only launch protocol is shared.
- Packaged MCP binary resolution may legitimately remain caller-specific.
- Process tests need deterministic fake binaries or existing package conventions to avoid timing flakes.

## Reassessment

After `consolidate-search-session`, Session and MCP still duplicated llama flags,
model discovery, Kiwix arguments, HTTP readiness polling, startup timeouts, and
timeout cleanup. `internal/search/server` now owns those concrete protocols.
Session retains its per-session process fields and closes them per `Close`;
MCP retains `sync.Once` caching, packaged Kiwix-binary resolution, and closes
only its cached processes.
