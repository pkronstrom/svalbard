# Packed library trees: design

Status: parked, 2026-11-26. Owner decision: keep library trees as loose files for now and recommend small exFAT clusters (32 KB or 4 KB) when preparing a stick. Packing and unpacking on the host may come back later only as an option the end user is asked about. The measurements below stay valid.

Status: proposal, 2026-11-26. Nothing in the repository was changed. Scratch installs under `/Users/pkronstrom/.cache/svalbard-scratch/trees/` are removed (the directory is empty again).

## Summary

- **Recommendation:** store each Python tree as one zip per recipe per platform and unpack it once into a cache on the computer, keyed by archive content. Open WebUI on one platform goes from 3.20 GB to 0.21 GB at 128 KB clusters.
- **Hybrid (d) is not worth it:** on the real Open WebUI set it removes 22–28% of the files; 14,081–15,137 files stay loose.
- **Three things found on the way make the loose cost worse than the 2.94 GB you measured:**
  - Directories take a cluster each: +0.32 GB for Open WebUI (confirmed on a real exFAT image).
  - The builder's uv cache sits on the vault root in `.staging/uv-cache` and is never removed (read from `pythonvenv.go:85`, not run). A uv cache holds about one file per installed file (measured 1,158 against 1,132), so the stick carries a second loose copy.
  - Open WebUI's bytecode cache is on the stick (`PYTHONPYCACHEPREFIX` in `openwebui.go:103`): up to 8,311 files, 1.10 GB at 128 KB if every module were imported.
- **Datasets must stay zipped.** The open item "extract dataset zips" would cost 6.30 GB for Exploit-DB and 1.94 GB for nuclei-templates at 128 KB.

## 1. Inventory

Allocation counts files only; empty files take no cluster. Your 2.94 GB counts Open WebUI's 462 empty files as one cluster each; without them it is 2.88 GB. Directories are a separate column because each takes one cluster.

### Python trees (measured by installing with uv for macOS arm64, Python 3.12)

| Recipe | Type | On the stick today | Files / dirs | Content | 128 KB | 32 KB | 4 KB | Dirs at 128 KB | Packed |
|---|---|---|---|---|---|---|---|---|---|
| open-webui 0.13.5 slim | python-package | loose tree | 19,532 / 2,445 | 553.5 MB | 2.88 GB | 1.08 GB | 0.60 GB | 0.32 GB | 210 MB zip |
| same, linux-x86_64 | python-package | loose tree | 19,548 / 2,449 | 619.3 MB | 2.95 GB | 1.15 GB | 0.67 GB | 0.32 GB | 227 MB zip |
| pwntools | python-package | loose tree | 10,560 / 295 | 116.9 MB | 1.44 GB | 0.42 GB | 0.13 GB | 0.04 GB | 25 MB tar.gz |
| angr | python-package | loose tree | 5,409 / 582 | 460.4 MB | 1.09 GB | 0.60 GB | 0.47 GB | 0.08 GB | 95 MB tar.gz |
| espidf-python | python-package | loose tree, copied into the PlatformIO generation | 2,181 / 278 | 51.5 MB | 0.30 GB | 0.11 GB | 0.06 GB | 0.04 GB | 15 MB tar.gz |
| svalbard-python (interpreter) | python-venv | loose tree, 10 symlinks | 1,659 / 156 | 65.3 MB | 0.26 GB | 0.11 GB | 0.07 GB | 0.02 GB | 24 MB tar.gz |
| platformio-core | python-package | loose tree | 1,417 / 234 | 15.0 MB | 0.18 GB | 0.05 GB | 0.02 GB | 0.03 GB | 3 MB tar.gz |
| sqlmap | python-package | loose tree | 902 / 158 | 15.2 MB | 0.12 GB | 0.04 GB | 0.02 GB | 0.02 GB | 8 MB tar.gz |
| volatility3 | python-package | loose tree | 460 / 54 | 9.8 MB | 0.06 GB | 0.02 GB | 0.01 GB | 0.01 GB | 1 MB tar.gz |
| document-library set (not a recipe on `main`) | — | — | 1,132 / 112 | 44.7 MB | 0.17 GB | 0.07 GB | 0.05 GB | 0.01 GB | 17 MB tar.gz |

binwalk (declared 1 MB) was not measured. Every one of these is installed once per target platform, so a four-platform vault holds about 12.9 GB of Open WebUI at 128 KB (two platforms measured, two extrapolated).

### Apps and datasets (from each archive's listing; downloaded, not extracted)

| Recipe | Type | On the stick today | Files / dirs | Content | 128 KB | 32 KB | 4 KB | Archive |
|---|---|---|---|---|---|---|---|---|
| ghidra | app | loose tree, no launcher | 5,324 / 1,499 | 908.6 MB | 1.49 GB (+0.20 dirs) | 1.03 GB | 0.92 GB | 570 MB zip |
| hashcat | app | loose tree, no launcher | 3,100 / 56 | 389.4 MB | 0.66 GB | 0.45 GB | 0.40 GB | 20 MB 7z |
| drawio | app | loose tree | 3,174 / 163 | 132.3 MB | 0.52 GB | 0.22 GB | 0.14 GB | 53 MB war |
| cyberchef | app | one zip, never extracted, does not open | 132 / 7 | 83.4 MB | 0.10 GB | 0.09 GB | 0.08 GB | 83 MB zip |
| seclists | dataset | one zip | 6,277 / 156 if extracted | 1,970.8 MB | 2.74 GB | 2.15 GB | 1.99 GB | 713 MB zip |
| exploitdb | dataset | one zip | 47,783 / 273 if extracted | 183.4 MB | 6.30 GB | 1.62 GB | 0.31 GB | 70 MB zip |
| nuclei-templates | dataset | one zip | 14,604 / 922 if extracted | 46.6 MB | 1.94 GB | 0.50 GB | 0.08 GB | 20 MB zip |

Not measured: minipaint, jscad-web, sqliteviz, duckdb-wasm, maplibre-vendor, mml-map-sheets (loose, declared 4–39 MB each).

### Types that are not a file-count problem

- **toolchain (13 recipes):** archives stay on the stick (17 files, 1.5 GB for macOS arm64) and unpack to a 5.3 GB host generation. Figures are from the PlatformIO checkpoint, not re-measured.
- **binary (36 recipes, 29 of them archives):** the archive sits in `bin/<platform>/` and `runtimebinary.go` unpacks it onto the stick at first use. The largest listed are pi (208 files, 111 MB) and llama-server (43 files, 18 symlinks); the others checked hold 1–3 files. They write to the stick on first use, so they fail on a read-only stick.
- **zim, pmtiles, gguf, pdf, sqlite, gpkg:** single files.

### Real exFAT check

On a 1 GB exFAT image formatted with 128 KB clusters, the document-library set took 0.351 GB: 0.173 GB of files, 0.015 GB of directories, and 0.163 GB for 1,244 `._` companion files that macOS wrote beside every file and directory my process created. sqlmap took 0.284 GB against 0.124 GB files-only. Whether a real `svalbard apply` gets the companions depends on the writing process; `runtimebinary.go` already skips `._` files, so they do occur on real sticks. For Open WebUI they would add up to 2.88 GB.

## 2. Options for trees that must be real files

Figures are Open WebUI, one platform, 128 KB clusters.

| | (a) loose, small clusters, warning | (b) zip, unpacked to a host cache | (c) zip, unpacked per run | (d) hybrid |
|---|---|---|---|---|
| Stick space | 3.20 GB (1.16 GB at 32 KB, 0.61 GB at 4 KB), plus uv cache and bytecode unless fixed | 0.21 GB at any cluster size | 0.21 GB | 2.5–2.6 GB (2.2–2.3 GB of files plus directories) |
| First start | reads each imported file from the stick; not measured (no stick here) | zip read + 1.0–1.3 s unpack + first load of new libraries | same as (b) | as (a) |
| Later starts | as first, less what the OS still has cached | nothing read from the stick for the tree | every start is a first start | as (a), plus recompiling the zipped modules each start |
| Left on a borrowed computer | nothing | 0.55 GB tree and up to 0.1 GB bytecode in the user cache dir | nothing after a clean exit; 0.55 GB in temp after a kill, power loss or pulled stick | nothing |
| Read-only stick | tree works | tree works | tree works | tree works |
| Little host space | not affected | checked before unpack, fails with the amount needed | fails on every start | not affected |
| Other mount path | works | same generation reused | not affected | works |
| Update | rewrites 19,532 files over USB | one file; new content selects a new generation, the old one is orphaned | one file | both of the above |
| Removal | deletes 19,532 files | one file; other computers keep their generation until cleaned | one file | both |
| Contracts touched | none | manifest entry gains a file checksum; PlatformIO when its Python moves | manifest entry only | none, but a per-package allowlist to maintain |

Notes that apply across the table:

- **Read-only stick:** Open WebUI itself needs a writable stick in every option, because its chats, settings, logs and secret live in `.svalbard/runtime/open-webui/`.
- **(a):** Svalbard has no cluster awareness today. The plan's space check uses `size_gb` (0.58 for Open WebUI), a fifth of the real cost at 128 KB. Smaller clusters mean the user reformats the stick, which wipes it.
- **(c):** on Linux `/tmp` is often RAM-backed, so 0.65 GB of RAM would go to the tree beside the model.

### Unpack time, measured on this machine

Local SSD, M4 Max, while other jobs kept the load average at 5–14. Archive already in the page cache.

| Archive of the Open WebUI tree | Size | Go, 1 writer | Go, 4 writers | Go, 8 writers | bsdtar |
|---|---|---|---|---|---|
| zip (deflate) | 209.5 MB | 3.25 s | 1.3 s | 1.0 s | — |
| tar.zst level 3 | 195.4 MB | 1.4–1.7 s | — | 1.2–2.4 s | 3.1–3.2 s |
| tar.zst level 19 | 164.7 MB | 2.4 s | — | 2.7 s | — |
| tar.gz | 202.5 MB | 3.4–3.8 s | — | 2.7 s | — |

- A tar made with macOS defaults carries 21,978 extra `._` entries and took bsdtar 7.9–8.7 s; make archives without Mac metadata.
- **USB scaling:** reading the 210 MB zip takes about 7 s at 30 MB/s (USB 2), 2.1 s at 100 MB/s and 0.7 s at 300 MB/s. Add the unpack time above; slower CPUs and disks were not measured.
- **First load after unpack:** importing 17 of the compiled packages (893 modules) from a freshly unpacked tree took 6.3 s the first time and 0.27 s the second. Without a bytecode cache it took 1.05 s, so most of the first-run cost is macOS checking new libraries, not compiling.

### (d) measured on the real Open WebUI set

177 distributions, classified from each `RECORD` plus a scan for `__file__` and `resource_filename`.

| Class | Distributions | Files | 128 KB |
|---|---|---|---|
| Pure Python, no data, no path reads | 96 | 2,845 | 0.35 GB |
| Data files but no path reads (openai, googleapis-common-protos, opentelemetry-sdk, urllib3, tqdm) | 7 | 1,550 | 0.20 GB |
| Reference `__file__`, ship no data; each needs checking (pygments, langchain_core, pydantic, psycopg, rich, anyio, mako, qrcode) | 12 | 1,056 | 0.14 GB |
| Read their own data by path; must stay loose | 17 | 11,301 | 1.68 GB |
| Compiled libraries; must stay loose | 45 | 2,779 | 0.52 GB |

- **Could go into an import archive:** 4,395 files (22%) with confidence, 5,451 (28%) at most. Saving: 0.55–0.69 GB of 2.88 GB.
- **Cannot, because they read data by path:** open_webui (6,407 files, 6,029 of them the built frontend served from a directory), botocore (1,972, service JSON), langchain_classic (1,335), pytz (618, zone files), langsmith (482), alembic (104), boto3, fastapi, jsonschema, jsonschema_specifications, certifi, python_dateutil, typer, validators, youtube_transcript_api, langchain_text_splitters, pycparser.
- **Cannot, because they hold compiled libraries:** numpy (915 files), sqlalchemy (276), lxml (176), pillow (140), cryptography (119), greenlet, black, aiohttp, grpcio, uvloop, websockets, wcwidth, protobuf, chardet, cffi, psycopg2_binary, psycopg_binary, pycrdt, pyyaml, charset_normalizer, httptools, wrapt, watchfiles, yarl, psutil, multidict, tiktoken, propcache, pytokens, pycares, markupsafe, orjson, uuid_utils, zstandard, frozenlist, hiredis, pydantic_core, regex, xxhash, bcrypt, jiter, rpds_py, brotli, google_re2, argon2_cffi_bindings.
- pwntools and angr come out the same way: 8,560 of pwntools' 10,560 files are its own data, and angr's bulk is angr_data and pypcode data.
- The classification is a heuristic and would need rechecking on every pin bump.

## 3. Recommendation

Option (b), with these choices:

- **Format: zip.** Both Go modules read and write it from the standard library, it unpacks in parallel (1.0 s against 3.25 s), it lists uncompressed sizes without reading the data (for the free-space check), and it is the format already decided for JS libraries. tar.zst at level 19 is 21% smaller but the drive runtime has no zstd library today.
- **Key: archive content, not vault path.** The key is the archive's recorded sha256 plus platform and a format version. I unpacked the tree at a different path and imported numpy, lxml, PIL, sqlalchemy, aiohttp, uvloop, grpc, cryptography, psycopg2, tiktoken and others from it, so the tree is relocatable. A stick mounted at another path reuses the generation. PlatformIO keeps its own vault-path key.
- **No re-hash at start.** The runtime reads the archive's checksum from `.svalbard/checksums.sha256`, which the toolkit already writes and `verify` already checks. Python trees have no checksum today, so `verify` starts covering them.
- **Publication:** unpack into `<id>-<key>.partial`, rename when complete, under the same file lock PlatformIO uses. Cancel removes the partial directory.
- **Bytecode** goes beside the generation on the computer, not on the stick.
- **Hash separation stays as AGENTS.md requires:** the archive sha256 is a file checksum on the realized entry, the `.packages` stamp stays the builder fingerprint, and the acquisition hash is unchanged. No manifest version change.
- **Rule for other types:** a packed tree is a zip. Trees a server can read are served from the zip in place (browser apps, as with JS libraries). Trees that must be real files unpack to the host cache. Datasets stay zipped on the stick.

### Where the bytes live

```text
 USB stick (exFAT, any cluster size)         This computer (user cache dir)
┌────────────────────────────────────────┐  ┌──────────────────────────────────┐
│ runtime/python/<platform>/             │  │ svalbard/trees/                  │
│   tools/open-webui.zip   210 MB  [1] ──┼─►│   open-webui-<key>/     553 MB   │
│   tools/<tool>.zip               [1] ──┼─►│   <tool>-<key>/                  │
│   python.zip              24 MB  [3] ──┼─►│   python-<key>/          65 MB   │
│ bin/<platform>/<entry>   wrapper       │  │   <id>-<key>.pyc/   bytecode     │
│ .svalbard/checksums.sha256  gives <key>│  │   locks/                         │
│ .svalbard/runtime/open-webui/          │  │ svalbard/platformio/             │
│   chats, settings, logs  (unchanged)   │  │   existing 5.3 GB generations    │
├────────────────────────────────────────┤  └──────────────────────────────────┘
│ apps/drawio.zip   53 MB  [4]           │   unpacked once per archive content,
│   served from the zip, never unpacked  │   reused at any mount path, removed
│ data/*.zip   datasets stay zipped      │   by "Clean this computer"
└────────────────────────────────────────┘   [n] = chunk that moves it
```

### Chunks

**1. Pack Python package trees and run them from a host cache — L.** This is the smallest step that delivers most of the saving: every python-package except the two PlatformIO trees, with the interpreter left loose so `embedded` is untouched.

- **Builder (`pythonvenv.go`):** install into a staging directory and uv cache on the host (the tool runner's `/work` mount already allows this), zip the tree to `runtime/python/<platform>/tools/<id>.zip`, record its sha256 and size on the realized entry. An existing loose tree with a matching stamp is zipped in place, so current vaults convert without network.
- **Runtime:** a new package that prepares a generation (lock, free-space check, parallel unpack, rename, cancel cleanup). `openwebui.Run` calls it and prints progress before starting; the `bin/<platform>/<entry>` wrappers call the drive runtime, which prepares and then runs the entry point.
- **Saving:** Open WebUI 3.20 → 0.21 GB per platform at 128 KB, plus the uv-cache copy and the bytecode cache leave the stick.
- **Risk:** an XL if a tool turns out to write into its own tree or need its on-stick path (angr and pwntools were not run from a moved tree), since that needs a second mechanism.
- **Verify through the user's path:**
  - Create a vault with `open-webui` on an exFAT image formatted with 128 KB clusters; the recipe takes under 0.25 GB.
  - `./run open-webui` unpacks, answers `/health`, and a chat persists; the second start does not unpack.
  - Remount at another path: no new generation. Ctrl+C during unpack: no generation left, retry works.
  - Bump the pin and `svalbard update`: new zip, new generation, the old chat history still opens.
  - Remove the recipe: zip, stamp and wrapper are gone and the menu entry disappears.
  - `source ./activate; sqlmap --version` and `pwn version` work from the stick.

**2. Show and clean what is on this computer — M.**

- First unpack prints the path and size. A `Clean this computer` menu entry and `./run cleanup` remove tree generations and PlatformIO generations.
- `svalbard remove` and `update` drop the provisioning computer's old generation.
- Generations of the same recipe that nothing is using are pruned; a running tool holds a shared lock so it is never pruned.
- A setting removes the unpacked tree on exit (decision 1).
- **Risk:** an L if pruning must also cover PlatformIO generations in use, because the PlatformIO design deferred leases.
- **Verify:** use the pack on a second user account, run cleanup, confirm the cache directory is empty and the next start unpacks again; update a recipe and confirm one generation remains.

**3. Pack the interpreter and the PlatformIO Python trees — M.**

- `python.zip`, `platformio-core.zip` and `espidf-python.zip`; `embedded.Prepare` gets its interpreter and trees from the tree cache, and its fingerprint uses the archive checksums in place of hashing 5,257 small files on every shell open (the checkpoint lists that hashing as slow on USB 2).
- The tree cache root follows the PlatformIO rule: canonical path, never under macOS `/tmp`. `platformio_inventory.go:108` and the format version change with it.
- **Saving:** 0.74 GB of files plus 0.09 GB of directories per platform at 128 KB.
- **Risk:** an L if espressif32 7.1.3's venv creation depends on where the interpreter lives.
- **Verify:** the checkpoint's acceptance run on a freshly applied `esp32-dev` vault: offline build for ESP32, S2 and S3, reopen reuses the generation, relocation prepares a new one, cancel leaves none.

**4. Serve browser apps from their zip in place — S.**

- The apps server maps `/apps/<id>/` to the app's zip when one exists. drawio goes from 0.52 GB to 0.05 GB at 128 KB, and CyberChef, which is a zip that is never extracted, starts opening.
- Ghidra and hashcat stay loose until they get launchers (they have none today); their launchers should use chunk 1's mechanism.
- **Risk:** an M if an app needs Range requests into a compressed entry, or reads other drive paths under its own prefix.
- **Verify:** create a vault with tools-base and drawio, open each app from the menu, update one, remove one.

Follow-ups to file as inbox tasks, not chunks: a cluster-aware space estimate in the plan for what stays loose; removing `._` companions after apply on macOS; recording "datasets stay zipped" against the open extraction item.

## 4. Decisions for the owner

**1. Files left on a borrowed computer.** Today Open WebUI leaves nothing on the computer (its code redirects HOME and every cache to the stick); PlatformIO already leaves 5.3 GB that nothing removes.

- A. Keep the unpacked tree, say where it is and how big at first unpack, offer `Clean this computer`, and add a setting that removes it on exit.
- B. Remove on exit by default. Every start pays the first-start cost, and a kill or pulled stick still leaves the files.
- C. Ask at exit each time.
- **Recommended: A.**

**2. One behaviour or cluster-dependent.**

- A. Always packed.
- B. Packed only when the stick's clusters are 32 KB or larger.
- C. Opt-in per recipe.
- **Recommended: A.** One code path, no format detection, and the zip is still a third of the loose size at 4 KB clusters (0.21 against 0.61 GB).

**3. Whether to touch the finished PlatformIO phase.**

- A. Do chunk 3. Each computer re-prepares its 5.3 GB generation once per vault, and the old generation stays until cleaned.
- B. Leave the interpreter and the two PlatformIO trees loose: 0.83 GB per platform stays at 128 KB.
- **Recommended: A, after chunk 2** so the orphaned generation can be cleaned.

## 5. What was run and what was read

**Run on this machine (macOS arm64):**

- uv installs of every tree in the Python table, the Linux x86_64 Open WebUI tree, and all counts and allocations.
- Archive listings for ghidra, hashcat, drawio, cyberchef, seclists, exploitdb, nuclei-templates and seven binary archives.
- All archive sizes and unpack timings; the relocated-tree import; compiling every module to count bytecode files; the uv-cache file count.
- The exFAT image with 128 KB clusters.
- The (d) classification.

**Read, not run:**

- The uv cache staying on the vault root.
- Other Python tools writing `__pycache__` into their tree on the stick (the wrapper sets no bytecode variable).
- Removal leaving the stamp and wrappers behind (`apply.go:130` removes only the entry's path).
- The apps server change in chunk 4.
- The toolchain figures (from the checkpoint).

**Not measured:**

- Anything from a physical USB stick; any start of Open WebUI.
- Linux or Intel execution; angr or pwntools running from an unpacked tree.
- How many modules Open WebUI imports at startup.

## 6. Owner decisions (2026-11-26)

1. **Contained in the stick.** The owner wants nothing to stay on the computer. Default: a packed tree is unpacked per run into a temporary location on the host and removed on exit; a stale copy left by a crash or a pulled stick is swept at the next start. A setting, off by default, keeps the unpacked tree on that computer for faster later starts (option A's behaviour, for the owner's own machines). The first-start cost is therefore paid on every start unless the setting is on; it must be measured on a real stick, and if it is too slow, mounting a read-only image from the stick (a disk image on macOS, squashfs on Linux) is the fallback to evaluate, not a second default.
2. **Always pack**, whatever the cluster size.
3. **Pack the interpreter and the PlatformIO Python trees too**, as chunk 3, after the cleanup chunk exists.
