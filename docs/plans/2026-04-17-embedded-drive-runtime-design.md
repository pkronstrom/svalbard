# Embedded Drive-Runtime & Release Pipeline Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make svalbard installable as a single pre-built binary (no Go toolchain required) by embedding cross-compiled svalbard-drive binaries and setting up a goreleaser-based release pipeline.

**Architecture:** The release pipeline cross-compiles svalbard-drive for all 4 platforms, then embeds them into the svalbard binary via `//go:embed`. At apply-time, `toolkit.Generate()` extracts the embedded binaries instead of compiling from source. The existing `runtimeBinarySources` function pointer makes this a clean swap.

**Tech Stack:** goreleaser, `//go:embed`, Makefile/shell for pre-build step

---

## Current State

Toolkit generation calls `buildDriveRuntimeBinaries()` which:
1. Resolves `drive-runtime/` source via `runtime.Caller()` (fragile)
2. Runs `go build -o <tmp>/<platform>/svalbard-drive ./cmd/svalbard-drive` for all 4 platforms
3. Copies binaries to `<vault>/.svalbard/runtime/<platform>/svalbard-drive`

This requires Go on the user's machine and the repo source tree.

## New Design

### Pre-build step (Makefile / goreleaser hook)

Before `go build` of svalbard itself, a script cross-compiles svalbard-drive:

```
host-cli/internal/toolkit/embedded/
├── macos-arm64/svalbard-drive
├── macos-x86_64/svalbard-drive
├── linux-arm64/svalbard-drive
└── linux-x86_64/svalbard-drive
```

These are real binaries, built with `CGO_ENABLED=0` and `-ldflags="-s -w"` (stripped).

### Embedding

New file `host-cli/internal/toolkit/embed_runtime.go`:

```go
//go:embed embedded/macos-arm64/svalbard-drive
//go:embed embedded/macos-x86_64/svalbard-drive
//go:embed embedded/linux-arm64/svalbard-drive
//go:embed embedded/linux-x86_64/svalbard-drive
var embeddedRuntime embed.FS
```

New function `extractEmbeddedBinaries()` that writes the embedded binaries to a temp dir and returns the same `map[string]string` that `buildDriveRuntimeBinaries()` returns.

### Swap in toolkit.go

The `runtimeBinarySources` function pointer switches from `buildDriveRuntimeBinaries` to `extractEmbeddedBinaries`. The rest of `installRuntimeBinaries()` is unchanged — it already just copies from source paths to vault.

Fallback: if embedded binaries are missing (development mode, running from `go run`), fall back to the current `buildDriveRuntimeBinaries()`.

### Release pipeline (goreleaser)

`.goreleaser.yaml` at repo root:

1. **Pre-hook:** `make build-drive-runtime` — cross-compiles svalbard-drive for 4 platforms into `host-cli/internal/toolkit/embedded/`
2. **Builds:** svalbard binary for 4 platforms (each now contains all 4 drive-runtime variants)
3. **Archives:** tar.gz per platform with just the `svalbard` binary
4. **Release:** uploads to GitHub Releases

### .gitignore

`host-cli/internal/toolkit/embedded/` is gitignored — binaries are built fresh in CI. Development uses the fallback path (compile from source).

### Development workflow

Developers with Go installed see no change:
- `go run ./cmd/svalbard` → embedded dir empty → falls back to `buildDriveRuntimeBinaries()` → compiles from source as before
- `make build` → pre-build step populates embedded/ → binary is self-contained

---

## Size impact

| Component | Size (stripped) |
|-----------|----------------|
| svalbard-drive per platform | ~7 MB |
| 4 platforms embedded | ~28 MB |
| svalbard binary today | ~21 MB |
| svalbard binary after | ~49 MB |

Acceptable for a tool that provisions multi-gigabyte vaults.

---

## What this does NOT cover

- **Docker builder scripts** — still requires repo checkout for Docker-based builds (ZIM scraping, geodata). These are advanced/rare recipes. Can be addressed separately by embedding the Python scripts.
- **Homebrew tap** — nice to have but not required. goreleaser can auto-generate it.
- **Auto-update** — out of scope. Users re-download or `brew upgrade`.

---

## Task breakdown

### Task 1: Pre-build script
Create `scripts/build-drive-runtime.sh` that cross-compiles svalbard-drive for 4 platforms into `host-cli/internal/toolkit/embedded/`.

### Task 2: Embed and extract
Create `host-cli/internal/toolkit/embed_runtime.go` with `//go:embed` and `extractEmbeddedBinaries()`. Wire it as the primary `runtimeBinarySources` with fallback to compile-from-source.

### Task 3: Gitignore embedded binaries
Add `host-cli/internal/toolkit/embedded/` to `.gitignore`. Add placeholder file so the directory exists for `go:embed`.

### Task 4: Makefile targets
Add `make build` (includes pre-build step) and `make build-dev` (skips pre-build, uses fallback).

### Task 5: goreleaser config
Create `.goreleaser.yaml` with pre-hooks, multi-platform builds, and GitHub Release publishing.

### Task 6: Verify end-to-end
Build with `make build`, run from outside repo, confirm apply works without Go toolchain.
