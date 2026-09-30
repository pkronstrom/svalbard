# Dodo → masterplan triage

Status: reconciled 2026-09-30; the masterplan is ordered around the user paths specified on 2026-09-30; marked scope questions await user answers.

The source export contained **39 pending tasks** (the earlier audit count of 38 was incorrect). Four clearly completed tasks were closed. All remaining entries retain their IDs and pending status, with a masterplan destination and triage metadata; pending no longer means no implementation exists. No task was deleted.

`MASTERPLAN.md` owns the queue and grouped wishlist. This ledger preserves the original task wording and explains partial completion; it is not a second implementation plan. Future features receive their own discovery and plans.

## Decisions requested

- Wishlist versus implementation queue for bootable/Kali/Ventoy and optional apps/map enhancements.
- Guides/recommendations versus bundled installers/APKs for mobile and KiCad.
- Queue order follows initial/resumable deployment, customization, pack operability, updates, and add/remove; the current catalog refresh remains in progress.
- Identify the NODE PDF source and clarify AutoDoc offline-content intent.

## Source tasks

| Dodo ID | Original task | Destination / disposition | Remaining scope or evidence |
|---|---|---|---|
| `2148bd8f` | Research more useful tools | catalog-refresh (partial) | Audit useful bundled tools and record supported acquisition/runtime coverage; align with the catalog refresh. |
| `7e3b264a` | Kali linux included somewhere somehow | wishlist-boot (wishlist) | Preserved for discovery in the named masterplan group. |
| `bda1b4c7` | A sane set of llms? | catalog-refresh (partial) | Refresh local GGUF model lineup, RAM-tier presets, sampling settings, and harness defaults. |
| `90bd69e0` | iPhone / Android app suggestions in a separate Markdown file -- perhaps add that to wizard as an extra step | provisioning-mobile (partial) | Refresh existing iOS/Android recommendation guides and decide wizard integration; installer/APK scope awaits clarification. |
| `deac56d3` | dataset licensing | content-curation (queued) | Recipe license fields exist; dataset coverage and attribution need auditing. |
| `6564a263` | Offline Code library documentation | content-curation (queued) | Language reference recipes exist; library ecosystem documentation remains to curate. |
| `d051c59e` | Bootable sys partition support at some point? | wishlist-boot (wishlist) | Preserved for discovery in the named masterplan group. |
| `dbddc735` | what is ventoy? | wishlist-boot (wishlist) | Preserved for discovery in the named masterplan group. |
| `30a7f508` | iOS / app viewer recs? | completed (completed) | provisioning/ios.md already recommends ZIM, map, PDF and EPUB viewers; recommendation existence verified, freshness not claimed. |
| `8830ce48` | offline routing | offline-routing (queued) | Preserved for discovery in the named masterplan group. |
| `645b4677` | provisioning / installing the tools with a shell script | provisioning-mobile (partial) | Finish host-tool provisioning workflow; a macOS Brewfile exists, portable installation flow remains. |
| `9607eecd` | a universal run.sh type of script that can be used to view the data / provision stuff / check updates etc? | online-freshness (partial) | Finish update/provisioning access from the universal drive workflow; run launcher, browsing and serving already exist. |
| `411b3688` | brewfile type thing for linux | provisioning-mobile (queued) | Preserved for discovery in the named masterplan group. |
| `a9724d6e` | This should be an opinionated, easy to use script | ux-principle (needs-scope) | Keep opinionated, easy-to-use defaults as a cross-cutting acceptance criterion; define a concrete UX improvement before scheduling a feature. |
| `71a96d38` | Serve endpoint for the run.sh script to serve the archive on local network | completed (completed) | Native share/serve-all implemented; share and serve-all tests passed 2026-09-30. |
| `5298a470` | open hardware documentation, such as microcontroller datasheets, open‑source hardware designs, and electronics textbooks, to support repair and fabrication | content-curation (queued) | Preserved for discovery in the named masterplan group. |
| `f672ebe6` | perhaps download the NODE pdf links as zim or similar | needs-source (needs-scope) | Source URL and meaning of NODE requested from user; do not guess. |
| `547ac676` | Builders of doomsday libraries emphasize including local‑specific information: national and regional maps, local flora and fauna identification guides, and regionally appropriate crop and building practices. Some archive municipal zoning maps, local regulations, and infrastructure diagrams where available, arguing that local knowledge is more actionable than global reference in a crisis. Community records, family documents, and scanned papers are often added as well, turning the drive into both a survival toolkit and a cultural archive. | content-curation (queued) | Regional sources already exist; audit coverage and reliability rather than restart the regional pack. |
| `9bc975ab` | search index on svalbard build time, to an sqlite3 file | search-reliability (partial) | Wire the selected indexing strategy into provisioning/build workflow; SQLite ZIM indexing exists but apply does not invoke it. |
| `7048c570` | programming presets | content-curation (partial) | Curate and verify programming preset coverage and tiers; the computing reference pack already exists. |
| `5df2fd3c` | inculde adb app packages -- with provisioning | provisioning-mobile (needs-scope) | Actual APK bundling is not implemented; keep scope pending user answer. |
| `7d14e95a` | move to go/cobra with the bundled CLI scripts + search | completed (completed) | Go/Cobra host CLI and Go drive runtime implemented; see README and module command entrypoints. |
| `626c497d` | use go for custom scripts to serve stuff / serve zim search mcp etc. | completed (completed) | Native Go browsing/search/MCP/services implemented; targeted runtime/MCP tests passed. |
| `e8cdf806` | semantic search / rag with a small embed model + zim indexing | document-indexing-rag (partial) | Finish reliable local RAG over indexed content; semantic retrieval exists, but model/index compatibility and answer integration need verification. |
| `e480132a` | index also other than ZIM files | document-indexing-rag (queued) | Preserved for discovery in the named masterplan group. |
| `7551554e` | kicad! | provisioning-mobile (needs-scope) | Clarify KiCad scope: already recommended in the macOS Brewfile; bundled offline installer/documentation scope undecided. |
| `c55ae5fb` | Sane autodoc defaults (or choosable by user) — ref: https://www.youtube.com/@autodocuk/playlists | needs-source (needs-scope) | Existing channel reference retained; intent requested from user, no media ingestion undertaken. |
| `4bf6d0f3` | Local AI multi-model routing for OpenCode: expose only local GGUF models and switch/restart backend cleanly from /models (possibly via llama-swap or Svalbard-managed restart) | local-model-routing-mcp (queued) | Preserved for discovery in the named masterplan group. |
| `19bc4258` | Expose our Go-based actions as MCP tools | local-model-routing-mcp (partial) | Expose additional native Go drive actions through MCP; inventory, SQLite query, and search already exist. |
| `bd4b4208` | Investigate smarter handling for diagrams and text-heavy images in compact Wikipedia ZIMs so labels remain readable, possibly by keeping them larger or using different image rules than photos. | content-curation (queued) | Preserved for discovery in the named masterplan group. |
| `4c46278d` | RAG pipeline — let users upload docs (PDF/HTML) and ask questions against them offline via llama.cpp + vector search | document-indexing-rag (queued) | Preserved for discovery in the named masterplan group. |
| `bd378ea6` | Kolibri education platform — interactive Khan Academy with progress tracking instead of static ZIM | wishlist-apps (wishlist) | Preserved for discovery in the named masterplan group. |
| `18773c41` | Note-taking app — lightweight Markdown notes on the drive (e.g. FlatNotes or similar) | wishlist-apps (wishlist) | Preserved for discovery in the named masterplan group. |
| `cc2fe3f3` | Web UI dashboard — browser-based alternative to TUI for less technical users | wishlist-apps (wishlist) | Preserved for discovery in the named masterplan group. |
| `3c878804` | Auto-updates — check for recipe/tool updates when online | online-freshness (queued) | Preserved for discovery in the named masterplan group. |
| `82ebbb50` | Satellite imagery layer — Sentinel photos alongside vector PMTiles maps | wishlist-maps (wishlist) | Preserved for discovery in the named masterplan group. |
| `78a95cc0` | Map annotations — let users mark locations, add photos, export GeoJSON on offline maps | wishlist-maps (wishlist) | Preserved for discovery in the named masterplan group. |
| `e75b2cb2` | JupyterHub — interactive coding/data-science environment on the drive | wishlist-apps (wishlist) | Preserved for discovery in the named masterplan group. |
| `e9fbc61d` | Content creation tools — lightweight wiki or note-taking on the drive | wishlist-apps (wishlist) | Overlap with 18773c41: one notes/wiki wishlist discovery, not two separate implementations. |
