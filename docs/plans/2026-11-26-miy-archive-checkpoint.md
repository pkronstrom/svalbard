# Make It Yourself archive: checkpoint 2026-11-26

The `makeityourself` recipe (family `content-archive`, `host-cli/internal/builder/`) archives the 917 links in the Make It Yourself PDF into a ZIM. This records where the work stopped, what was measured, and what the next session should know. The build staging, the audit data and the built ZIM were deleted at this point, so the next build starts from scratch.

## Where it stands

| Measure | Value |
|---|---|
| Projects archived | 914 of 917 (3 unavailable: dead links and a bot-check page with no clean Wayback capture) |
| Buildable from the ZIM alone (audit) | 541 of 913 (59%), up from 476 at the start of the subpage work |
| ZIM size | 6.05 GB, of which about 2 GB is packed GitHub repositories |
| Full build from scratch | several hours (Thingiverse API pacing, 3 s a request, and Wayback 429s dominate) |

"Buildable" is a strict bar: a Sonnet reviewer, reading a text dump of each archived page plus its zip listings, judged instructions `complete` and files `present` or `not-needed`. About 60–80 projects can never pass, because the source itself has no build steps (parts lists only, video-only builds, pattern generators).

## What the builder does now

- **Site parsers:** Printables (GraphQL API), Thingiverse (API, `THINGIVERSE_TOKEN`), Instructables, Hackaday, YouTube (yt-dlp), and generic HTML with per-domain rules in the recipe. Failing pages fall back to the Wayback Machine, retrying an older capture (`timestamp=2023`) when the newest is a bot check.
- **Subpages** (`archivesubpages.go`): same-site continuation pages become sections: guide chapters below the project's path, Adafruit chapters, Hackaday instructions, files and logs, GitHub Markdown docs and wikis. At most 30.
- **GitHub repositories** (`archivegithub.go`): the project's own repo and up to two it links are packed as zips; linked repos' READMEs, docs and wikis become sections. Under 25 MB the repo goes in whole (codeload zip); larger ones get a ranked selection up to 50 MB: docs, then design files, PCB and firmware source (with fabrication-named zips), then firmware builds and archives, then pictures under 2 MB, with old, test and component-library folders last. Skipped: hidden folders, vendored code, STM32 HAL and CMSIS. Everything left out is listed in `OMITTED.txt`. Git LFS stubs are replaced by the real file from `media.githubusercontent.com`. Needs `GITHUB_TOKEN` (60 API requests an hour without one); `gh auth token` works.
- **Linked guides** (`archivelinked.go`): up to two pages on other sites a project links, followed one hop: project pages on Adafruit Learn, Instructables, Hackaday, Printables, Thingiverse and Hackster, and any link whose text names a build guide, instructions, tutorial or plans. YouTube, shops, social sites, Discord, Google and GitHub are never followed. Built and unit-tested, but **not yet run in a full build or audited**. A scan of all 917 projects found linked guides on 108 (81 with one, 27 with two). Most went to Thingiverse (37), Adafruit Learn (16) and Printables (12). It also showed junk, now excluded: an Ana White footer link to the kregtool.com home page (34 projects), Thingiverse remix lists, Linktree and Shopify stores. Site home pages are never followed.
- **Files:** more than three attachments are bundled into one zip (the recipe depends on `7z` so the stick can open it). Downloads are sniffed: a zip, 3MF, F3D or FCStd must start with `PK`, a PDF must contain `%PDF`, and nothing may be HTML.
- **Resume:** each project caches `project.json`; `archiveProjectVersion` (now 8) forces a refetch when the archiving logic changes. Failure records are cached too.

## How the work was done, and what to reuse

- **Live builds** ran through a temporary test that loads `recipes/content/makeityourself.yaml`, resets the endpoint variables `TestMain` points at `127.0.0.1:1` (Wayback, Printables, Thingiverse, `githubWeb`, `githubAPI`, `githubRaw`, `githubCodeload`, `githubMedia`), and calls `buildContentArchive(root, item, nil, Options{})`. Run it under `caffeinate -i`: a sleeping Mac broke one full build and left 64 network failures cached.
- **Targeted refetch** instead of a version bump: scan every project with the parser plus the new logic only (no images or files, about 20 minutes), move the affected project folders aside, mark the rest current in their `project.json`, and resume the build. A full refetch is risky, because a source can lose files between runs; that is how Thingiverse files went from 44 projects to 1.
- **Audit:** a dump script printed each project's source, picture and video counts, file and zip-entry lists, subpage section names and page text (cut at 15 000 characters). Sonnet agents judged 30–35 projects each into JSONL with `instructions`, `files`, `missing`, `where`, `cause` and `note`. Re-audit only the projects a change touched, and expect a few verdicts to flip from reviewer judgement alone.

## Remaining failures, by cause (after the repo work)

| Cause | About | Route |
|---|---|---|
| Files need a login or the CDN refuses them | 140 | Thingiverse now returns 403 for every download, even with the token. Instructables and MyMiniFactory attachments need an account. A logged-in cookie route needs the owner's approval. |
| Guide on another site | 67 | The linked-guide code above targets roughly 35 of these. GitLab and sourcehut repos need their own route (GitLab pages need JavaScript). |
| Video-only builds | 38 | Embedded video download exists behind `video_embeds`; about 5–8 GB at 480p. |
| Paywall, registration or shop page | 40 | Not fixable. |
| Page needs JavaScript | 14 | Needs a headless render. |
| Source has no build steps | 60–80 | Not fixable. |

## What to do next

1. **Full build from scratch** with `THINGIVERSE_TOKEN` and `GITHUB_TOKEN` set, under `caffeinate -i`. This is the first run of the linked-guide code against real pages. Expect several hours.
2. **Spot-check linked-guide sections** in about 20 projects before auditing: rule C (links by text) is the one most likely to pull in junk pages. Add a host denylist if it does.
3. **Re-audit all projects** with the method above and compare with 541. The estimate for the linked-guide work is +20–30.
4. **Keep files a refetch loses:** before more refetches, make the builder keep a project's previous files when a new fetch returns fewer, so a flaky source cannot shrink the archive.
5. **Then, by expected gain:** the GitLab and sourcehut repo route (about 5 projects), embedded videos (about 15–25, at 5–8 GB), the login-cookie route for Thingiverse, Instructables and MyMiniFactory (about 40–60, needs approval), a JavaScript render (about 10).

## Gotchas

- Test site access with Go through the builder's fetcher, not curl: Printables and Thingiverse serve curl a bot check.
- GitHub repo sizes from the API include history; use the recursive tree's blob sizes. A truncated tree means a framework repo (QMK, CircuitPython); skip it unless the link names a folder (`tree/<ref>/<dir>`, fetched as `git/trees/<ref>:<dir>`).
- Chrome pruning must happen before the 200-character "real text" check, or an empty Hackaday tab passes on its dialog text.
- A CSS class name containing `sidebar` made the chrome pruner drop the whole Adafruit guide; the recipe now names the content element.
- `awk '$2>0'` is true for the string `ERR`; filter scan output with a numeric pattern.
