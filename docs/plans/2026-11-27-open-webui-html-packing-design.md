# Pack offline HTML from Open WebUI

Draft design · 2026-11-27

## User path

With file tools enabled and a JS library installed, ask Open WebUI to make an interactive chart that opens offline. On a vault with library quickstarts indexed, it finds the quickstart, writes the source in the drive's workspace, packs it, and links to the packed HTML. When the index is off, packing can still use a library named in the request. The same output opens through the workspace preview or by double-clicking the file with the network disconnected. [Existing HTML packing checkpoint](2026-11-27-skill-vault-stage3-checkpoint.md) · [Library index availability](../../drive-runtime/internal/mcp/server.go#L182)

```text
┌────────────┐     ┌────────────────┐     ┌────────────────┐
│ Open WebUI │ ──→ │ Workspace MCP  │ ──→ │ Offline packer │
└────────────┘     └────────────────┘     └────────────────┘
                          ↑                   ↑         │
                    packed HTML link      JS archives   │
                          └─────────────────────────────┘
```

Add a single `file_pack_html` action to the existing workspace MCP server. It takes a workspace-relative HTML filename and emits a sibling `*.packed.html`. Use the existing workspace link format and report errors with the local name of the missing asset or library. Capability discovery follows the workspace file-tools switch and the presence of an installed JS library. Describe the browser tool in the workspace instructions so a model can translate the terminal packing step in the library quickstarts. Terminal clients continue to call `html pack` directly. Directory output and libraries that need a running server stay with the terminal command in this slice.

## Workspace access and publication

Reuse the same `htmlpack` checks and library resolver as the command. Introduce a rooted packing path that reads the input and every transitive workspace asset through the already opened workspace filesystem. Current packer paths include native absolute paths and esbuild's default file loader, so checking only the initial filename would still let a page bundle host files by reference. In workspace mode, esbuild's resolver and loader must take assets from the rooted filesystem, inline page source or recognized installed library archives. Use virtual module paths, an explicit empty TypeScript config and no source map inputs. Esbuild expands computed import globs through its own filesystem resolver, so the browser mode refuses computed import paths before bundling. Resolve relative JavaScript, data and CSS references within the workspace and library imports through a separate root for the installed library store. [Workspace packer](../../drive-runtime/internal/htmlpack/workspace.go) · [Workspace filesystem](../../drive-runtime/internal/workspace/workspace.go)

Build the result in memory and publish it atomically through the workspace file writer after checks pass. Cap output at the current 2 MiB workspace write limit. Bound dependency operations and the bytes read from workspace files, compressed archives and archive members. Use Chart.js as the first measured example. A failed rebuild keeps the previous complete output. A repeat build may replace the sibling output and leaves its source available for editing. Check compatibility with the current workspace browser policy before publishing. The terminal packer may emit a script data URL that works from disk but the preview blocks. Refuse that output with a diagnostic in workspace mode unless the packer can emit a compatible representation. [Script encoding](../../drive-runtime/internal/htmlpack/pack.go) · [Workspace browser policy](../../drive-runtime/internal/workspace/serve.go#L20)

## Acceptance

- In Open WebUI, write an interactive chart with a bundled library and one local asset, receive a clickable link and verify the chart renders over the preview origin. Copy only the packed HTML to an empty folder and double-click it with networking unavailable. Edit the source, repack and verify the new chart.
- Test missing assets, CDN URLs, a server-only library, the script data-URL case and oversized output. A refusal preserves existing packed output and gives the model enough detail to repair the source.
- Verify absolute and traversing references, outward symlinks and indirect JavaScript and CSS imports cannot read host files. Confirm a config above the workspace cannot influence the bundle. Check that cancellation stops the work, unsuccessful packaging never publishes a partial artifact and the existing terminal command still passes its packing tests.
- Confirm file tools off removes the browser action. Measure a small model at 8K and 32K to see whether it can discover the library, write the source, pack and link the result.
