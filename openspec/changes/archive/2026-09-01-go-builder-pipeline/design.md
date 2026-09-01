## Context

Svalbard provisions a standalone offline drive. Docker may exist on the provisioning host for build recipes, but the completed stick must run on supported hosts without Docker or installed dependencies.

Current build behavior has false surfaces:

- `BuildSpec.Builder` is parsed but ignored.
- asset and table arrays are not represented by `BuildSpec` and are discarded.
- fallback derives `<family>.py` and mounts `<vault>/../recipes/builders`, which depends on a source-checkout layout.
- several declared map, dataset, compact-ZIM, and custom families have no matching executable handler.
- `python-venv` is Go-orchestrated but falls back to Docker when `uv` races or is unavailable.

## Goals / Non-Goals

**Goals:**

- Make every retained build family executable through Go orchestration.
- Preserve simple YAML authoring and existing linear step behavior.
- Isolate all provisioning-time non-Go dependencies from the host.
- Show network, image, download, and staging cost before apply.
- Produce one verified manifest artifact with resumable state and typed progress.

**Non-Goals:**

- Starlark, Lua, WASM, arbitrary scripts, recipe-selected images, or a plugin SDK.
- A generic object-payload workflow framework.
- Installing packages into the host system.
- Requiring Docker on the machine consuming the finished drive.
- Shipping DAG scheduling in v1.

## Target architecture

```text
recipe YAML
    |
    v
CompileProcedures([]BuildStep) -> []Procedure
    |
    v
RunLinear(procedures)
    |                         |
    v                         v
Go actions                ToolRunner
HTTP/archive/files         tools-base or tools-browser
    |                         |
    +-----------+-------------+
                v
      verified artifact + manifest
```

`Procedure` is the durable seam:

```go
type Procedure struct {
    ID       string
    Action   Action
    Inputs   []ArtifactRef
    Outputs  []ArtifactRef
    Resource ResourceEstimate
}
```

V1 executes the slice in order. IDs derive deterministically from list position and action when omitted. A future `Needs []string` and `RunDAG` can consume the same procedure/artifact/event types, but `needs` is not exposed until implemented.

Procedures exchange declared files under `<vault>/.staging/build/<recipe-id>/`, plus small typed metadata. There is no arbitrary payload bus.

## Catalog contract

Simple recipes remain standalone YAML. `BuildSpec` gains typed fields only for current catalog needs:

- `Assets []BuildAsset`
- `Tables []BuildTable`
- `Requires []string`
- `EstimatedDownloadGB float64`
- `EstimatedWorkGB float64`
- backward-compatible `Steps []BuildStep`

Unknown nested arrays must not silently disappear. Family handlers validate their required fields before apply.

Runtime tools such as pwntools, angr, sqlmap, and volatility remain portable drive payload. Their build may use the tools image, but using them later does not require Docker.

## Modules and seams

```text
host-cli/internal/catalog/
  buildspec.go          typed build YAML and validation data

host-cli/internal/builder/
  builder.go            family dispatch only
  procedure.go          procedure/action/artifact/resource types
  compile.go            YAML steps -> []Procedure
  linear.go             ordered execution and resume
  events.go             JSONL event schema
  steps.go              Go HTTP/archive/file/verify actions
  tools.go              sole container boundary
  appbundle.go
  pythonvenv.go
  reference_static.go
  zimit.go
  zimcompact.go
  geodata.go
  buildertest/scenario.go

host-cli/internal/apply/
  recipe dependency ordering and worker scheduling
```

`apply` owns dependencies between recipes. The builder package owns procedures within one recipe. Family handlers know recipe semantics but not Docker flags. `ToolRunner` knows image execution but not recipe semantics.

## Tools appliance

One Dockerfile produces two final targets:

```text
tools-base
  uv, build Python, libzim/zim-tools, GDAL, tippecanoe,
  ffmpeg, pmtiles, Go build helpers

tools-browser
  pinned official zimit/Browsertrix base
  Chromium, WARC tooling, zimit
```

Both publish to GHCR with semver, SHA, and digest references. Recipes select a named tool capability; Go maps that capability to base or browser. Recipes cannot supply arbitrary image names or commands.

Final images use multi-stage builds, contain no compilers or package caches, and pin binary/Python dependencies. The existing `latest` constant is replaced by a release-controlled version or digest.

## Resources and progress

Build recipes declare:

- whether network access is required;
- estimated source download GB;
- estimated staging/work GB;
- required tool capabilities.

Plan/apply surfaces these before execution.

All procedure executions emit JSON Lines-compatible events:

```text
queued, started, progress, log, completed, failed, skipped
```

CLI and TUI consume the same event fields. Logs remain representable as events rather than a second output channel.

## Implementation order

1. Build-family inventory and scenario test DSL.
2. Typed BuildSpec arrays/resources with catalog round-trip tests.
3. Procedure compilation and linear executor with resumable fingerprints.
4. Recipe dependency ordering, first proving `uv -> python-venv -> python-package`.
5. JSONL event contract shared by CLI and TUI.
6. Go-only app-bundle and reference-static handlers.
7. Pinned base/browser ToolRunner targets and python-venv migration.
8. ZIM, zimit, and geodata family handlers.
9. Make It Yourself Go-controller spike after PDF/HTML/ZIM seams exist.
10. Consider DAG execution only if a concrete recipe demonstrates branching or parallelism value.

## Test strategy

The test DSL is implementation unit one. Each scenario reads as:

```text
Given: recipe YAML, fake source server, available tool capabilities
When:  apply runs
Then:  one verified artifact, manifest entry, event sequence, and resumable state
```

Happy paths cross `apply`; they do not stop at helper methods. Unit tests cluster around the pihvi: schema parsing, procedure compilation, deterministic IDs, path containment, fingerprints, resource estimates, dependency ordering, and family-specific tool arguments.

A fake `ToolRunner` keeps the default suite Docker-free. One opt-in integration suite pulls pinned base/browser images and exercises representative uv, zimwriterfs, zimit, and geodata operations.

Failure coverage includes malformed procedures, missing tools, cycles in recipe dependencies, cancellation, retry after partial completion, stale fingerprints, insufficient staging space, malformed JSONL events, and artifact verification failure.

## Risks / Trade-offs

- The browser target uses the maintained upstream zimit/Browsertrix base rather than inheriting the Alpine tools-base runtime; both are built and published from one workflow.
- Resource sizes are estimates and must be presented as such.
- Porting MIY orchestration to Go is significant; it must not block fixing simpler families.
- Linear execution may leave parallelism unused initially; this is intentional until measurements justify a scheduler.
