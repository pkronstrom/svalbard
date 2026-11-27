# Workspace HTML packing implementation plan

The workspace capability packs a page and returns a link to its sibling artifact. The shared packer handles both the browser workflow and the terminal command.

1. Add workspace packing tests in `drive-runtime/internal/htmlpack/workspace_test.go`. Start with nested modules and CSS, an installed library, relative paths that stay in the workspace, absolute and escaping paths, symlinks, implicit config, dynamic imports and preview-incompatible script sources.
2. Reuse the existing document and library packer. Route workspace reads through confined roots and route every esbuild dependency through a workspace or library plugin. Give esbuild no real resolution directory, no source maps and an explicit empty TypeScript config. Reject filesystem glob imports before bundling.
3. Bound source, archive and decompressed reads, and return cancellation before publishing. Render up to the workspace write limit and use its atomic writer. Test that failed work preserves an existing output.
4. Expose `file_pack_html` through `drive-runtime/internal/mcp/files.go` when the launcher enables it. Test discovery and the Markdown link format in `drive-runtime/internal/mcp/files_test.go`.
5. From `drive-runtime`, run `go test ./internal/htmlpack ./internal/workspace ./internal/mcp`. On a minimal vault with the installed Chart.js library, check the packed link through the workspace preview and open a copy of the artifact with networking disabled.
