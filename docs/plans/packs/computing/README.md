# Pack: Computing

Programming references, system administration, networking, and security resources for offline use.

## Audience

Software developers, sysadmins, security professionals, and anyone who needs to code or manage systems offline.

## Status

Idea -- collecting sources

## Scope boundary

This pack covers **reference content**: documentation, Q&A archives, guides, and protocol specifications. Actual compilers, interpreters, and development tools are covered separately in the [Programming Toolkit Preset](../../2026-03-30-programming-toolkit-preset.md), which defines composable toolchain tiers (Zig, Go, Rust, embedded-dev, etc.). The two are complementary -- this pack gives you the docs to read, the toolkit gives you the binaries to run.

## Subsections

### Programming languages & frameworks

API references and language documentation for offline development. The DevDocs ZIM collection on Kiwix is the primary source -- each language ships as a small, self-contained ZIM (typically under 10 MB). Stack Overflow is the heavyweight companion for worked examples and troubleshooting.

### System administration & networking

Linux documentation (man pages, Arch Wiki, distro references), networking protocol references, and infrastructure tooling docs. The man-pages DevDocs ZIM alone covers the full Linux/POSIX man page corpus.

### Security & cryptography

OWASP methodology, applied cryptography Q&A, reverse engineering, and defensive security references.

## Source candidates

### Stack Exchange ZIM archives

These are large, high-value Q&A archives. Stack Overflow is by far the biggest single item. Sizes from the Kiwix mirror (2026-02 builds).

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Stack Overflow | ZIM | 75 GB | 512+ | The big one. All English SO Q&A with images. Only for large drives. |
| Super User | ZIM | 3.7 GB | 128 | Desktop/laptop troubleshooting, power-user Q&A |
| Server Fault | ZIM | 1.5 GB | 128 | Already a recipe (`stackexchange-serverfault`). Sysadmin Q&A |
| Ask Ubuntu | ZIM | 2.6 GB | 128 | Ubuntu/Linux desktop and server Q&A |
| Unix & Linux | ZIM | 1.2 GB | 128 | Shell, POSIX, Linux internals, CLI workflows |
| DBA | ZIM | 670 MB | 128 | Database administration (PostgreSQL, MySQL, SQL Server, etc.) |
| Software Engineering | ZIM | 457 MB | 128 | Architecture, design patterns, methodology |
| Code Review | ZIM | 525 MB | 256 | Peer code review, best practices |
| Information Security | ZIM | 420 MB | 128 | Defensive security, penetration testing, policy |
| Computer Science | ZIM | 264 MB | 128 | Algorithms, complexity theory, formal methods |
| Cryptography | ZIM | 176 MB | 128 | Applied and theoretical cryptography |
| Network Engineering | ZIM | 124 MB | 128 | Routing, switching, protocol design |
| Reverse Engineering | ZIM | 110 MB | 256 | Binary analysis, malware, disassembly |
| DevOps | ZIM | 33 MB | 128 | CI/CD, infrastructure-as-code, SRE |

**Note:** Super User and Server Fault already have recipes in `/recipes/content/`. The others need new recipe YAML files.

### DevDocs ZIM collection (aggregated API docs)

DevDocs ZIMs are tiny -- the entire collection of 220+ docs totals well under 1 GB. They can be included generously even on smaller drives. Grouped by relevance tier.

#### Core languages (ship at 32 GB+)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| devdocs: Python | ZIM | 4.1 MB | 32 | Standard library + language reference |
| devdocs: C | ZIM | 1.2 MB | 32 | C standard library (cppreference) |
| devdocs: C++ | ZIM | 6.9 MB | 32 | C++ standard library (cppreference) |
| devdocs: Rust | ZIM | 5.7 MB | 32 | Rust std, language reference |
| devdocs: Go | ZIM | 1.5 MB | 32 | Standard library |
| devdocs: Bash | ZIM | 546 KB | 32 | Bash builtins and syntax |
| devdocs: JavaScript | ZIM | 2.6 MB | 32 | MDN JS reference |
| devdocs: TypeScript | ZIM | 1.1 MB | 32 | TypeScript handbook + API |
| devdocs: Git | ZIM | 1.5 MB | 32 | Git command reference |
| devdocs: Zig | ZIM | 476 KB | 32 | Language reference (pairs with toolkit binary) |
| devdocs: Lua | ZIM | 418 KB | 32 | Reference manual |
| devdocs: GNU Make | ZIM | 600 KB | 32 | Build system reference |
| devdocs: jq | ZIM | 367 KB | 32 | JSON processor manual |

#### Infrastructure & ops docs (ship at 128 GB+)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| devdocs: PostgreSQL | ZIM | 2.5 MB | 128 | Full PostgreSQL manual |
| devdocs: SQLite | ZIM | 3.4 MB | 128 | SQLite reference (pairs with bundled sqlite3 binary) |
| devdocs: DuckDB | ZIM | 1.9 MB | 128 | Analytical SQL engine (pairs with bundled duckdb-wasm) |
| devdocs: Redis | ZIM | 853 KB | 128 | In-memory store commands |
| devdocs: Docker | ZIM | 1.7 MB | 128 | Dockerfile, CLI, Compose |
| devdocs: Kubernetes | ZIM | 571 KB | 128 | k8s API and kubectl |
| devdocs: Nginx | ZIM | 797 KB | 128 | HTTP server config |
| devdocs: Apache HTTP Server | ZIM | 1.3 MB | 128 | httpd reference |
| devdocs: Terraform | ZIM | 3.0 MB | 128 | IaC provider docs |
| devdocs: Ansible | ZIM | 30 MB | 128 | Automation playbooks (largest DevDocs ZIM) |
| devdocs: GCC | ZIM | 1.6 MB | 128 | Compiler flags and extensions |
| devdocs: CMake | ZIM | 2.5 MB | 128 | Build system |
| devdocs: HTTP | ZIM | 1.9 MB | 128 | HTTP protocol reference (MDN) |

#### Extended languages (ship at 256 GB+)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| devdocs: Kotlin | ZIM | 7.8 MB | 256 | JVM language |
| devdocs: OpenJDK | ZIM | 14 MB | 256 | Java standard library |
| devdocs: Elixir | ZIM | 1.2 MB | 256 | Erlang VM functional language |
| devdocs: Erlang | ZIM | 3.9 MB | 256 | OTP + standard library |
| devdocs: Haskell | ZIM | 4.1 MB | 256 | GHC + base library |
| devdocs: Scala | ZIM | 3.8 MB | 256 | JVM functional/OO |
| devdocs: OCaml | ZIM | 1.7 MB | 256 | ML family |
| devdocs: Julia | ZIM | 1.5 MB | 256 | Scientific computing |
| devdocs: Nim | ZIM | 2.0 MB | 256 | Systems language |
| devdocs: Crystal | ZIM | 2.0 MB | 256 | Ruby-like compiled language |
| devdocs: D | ZIM | 1.7 MB | 256 | Systems language |
| devdocs: Perl | ZIM | 4.8 MB | 256 | Text processing / sysadmin classic |
| devdocs: Clojure | ZIM | 425 KB | 256 | Lisp on JVM |
| devdocs: PHP | ZIM | 6.8 MB | 256 | Web scripting |
| devdocs: Ruby | ZIM | 2.9 MB | 256 | Scripting language |
| devdocs: Deno | ZIM | 2.6 MB | 256 | JS/TS runtime |
| devdocs: Node.js | ZIM | 1.3 MB | 256 | JS runtime |
| devdocs: LaTeX | ZIM | 767 KB | 256 | Typesetting |
| devdocs: Nix | ZIM | 391 KB | 256 | Package manager / NixOS |
| devdocs: Fish | ZIM | 575 KB | 256 | Friendly shell |
| devdocs: Nushell | ZIM | 674 KB | 256 | Structured data shell |
| devdocs: Emacs Lisp | ZIM | 1.9 MB | 256 | Editor scripting |
| devdocs: GNU Fortran | ZIM | 772 KB | 256 | Scientific computing |

#### Framework docs (ship at 256-512 GB+)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| devdocs: Django | ZIM | 1.9 MB | 256 | Python web framework |
| devdocs: Flask | ZIM | 603 KB | 256 | Python micro-framework |
| devdocs: FastAPI | ZIM | varies | 256 | Python async API framework |
| devdocs: Rails | ZIM | 2.6 MB | 256 | Ruby web framework |
| devdocs: React | ZIM | 2.6 MB | 256 | UI library |
| devdocs: Vue | ZIM | 951 KB | 256 | UI framework |
| devdocs: Angular | ZIM | 2.1 MB | 256 | Google web framework |
| devdocs: Spring Boot | ZIM | 813 KB | 256 | Java web framework |
| devdocs: HTML | ZIM | 1.6 MB | 256 | MDN HTML reference |
| devdocs: CSS | ZIM | 4.6 MB | 256 | MDN CSS reference |
| devdocs: SVG | ZIM | varies | 256 | MDN SVG reference |
| devdocs: DOM | ZIM | varies | 256 | MDN DOM API |

### Linux & system documentation

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| devdocs: man pages | ZIM | 28 MB | 32 | Full Linux/POSIX man page corpus. Essential. |
| Arch Wiki | ZIM | 30 MB | 128 | Best single Linux reference wiki. Practical, distro-agnostic at its core. |
| Alpine Linux Wiki | ZIM | 2.9 MB | 256 | Minimal Linux / container base |
| Gentoo Wiki | ZIM | 69 MB | 512 | Deep Linux internals, compilation guides |
| Termux Wiki | ZIM | 2.2 MB | 128 | Android terminal emulator -- relevant for phone-based offline dev |

### Security references

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| OWASP Web Security Testing Guide | custom | ~5-10 MB | 128 | No Kiwix ZIM available. Would need to be scraped/packaged as a static site or PDF bundle. |
| OWASP Cheat Sheet Series | custom | ~3-5 MB | 128 | Markdown source available on GitHub. Could package as a ZIM or static HTML. |
| OWASP Top Ten | custom | ~1 MB | 32 | Small enough to always include. GitHub source. |
| CyberChef | app | 15 MB | 32 | Already a recipe (`cyberchef`). Data encoding/decoding/analysis. |

### Content already in recipes

These are already defined as Svalbard recipes and included in various presets. Listed here for completeness -- they count toward this pack's coverage.

| Source | Recipe ID | Size | Notes |
|--------|-----------|------|-------|
| Server Fault | `stackexchange-serverfault` | 5 GB (est. in recipe; actual ~1.5 GB) | Sysadmin Q&A |
| Super User | `stackexchange-superuser` | 5 GB (est. in recipe; actual ~3.7 GB) | End-user computing Q&A |
| Electronics SE | `stackexchange-electronics` | 2 GB | Hardware / embedded (also relevant to toolkit preset) |
| CyberChef | `cyberchef` | 15 MB | Data analysis tool |
| Wikibooks | `wikibooks-en` | 3 GB | Contains programming textbooks among other topics |
| Security SE | `stackexchange-security` | 420 MB | InfoSec, crypto, vulnerabilities Q&A |
| Unix SE | `stackexchange-unix` | 1.2 GB | Linux/UNIX sysadmin Q&A |
| Arch Wiki | `arch-wiki` | 30 MB | Best single-source Linux reference |
| Man pages | `man-pages` | 180 MB | Full Linux/POSIX man page corpus |
| DevDocs C | `devdocs-c` | 1 MB | C standard library reference |
| DevDocs C++ | `devdocs-cpp` | 7 MB | C++ standard library reference |
| DevDocs Python | `devdocs-python` | 4 MB | Python standard library reference |
| DevDocs Rust | `devdocs-rust` | 6 MB | Rust std reference |
| DevDocs Go | `devdocs-go` | 2 MB | Go standard library reference |
| DevDocs JavaScript | `devdocs-javascript` | 3 MB | MDN JavaScript reference |

## Tiering notes

### 32 GB -- developer field reference

Carry the essentials in your pocket. Focused on the languages you can actually run offline with the programming-toolkit-preset binaries.

- DevDocs core languages: Python, C, C++, Rust, Go, Bash, JS/TS, Git, Zig, Lua, Make, jq (~30 MB total)
- Man pages ZIM (~28 MB)
- OWASP Top Ten (~1 MB)
- CyberChef (already included)
- **Total computing-specific addition: ~60 MB** (negligible at this tier -- easily fits)

### 128 GB -- working developer library

Enough to seriously develop, debug, and administer systems offline.

- Everything from 32 GB tier
- Stack Exchange: Unix & Linux, Ask Ubuntu, DBA, Information Security, Cryptography, Network Engineering, Computer Science, Software Engineering, DevOps (~6.5 GB total)
- DevDocs infrastructure: PostgreSQL, SQLite, DuckDB, Redis, Docker, Kubernetes, Nginx, Apache, Terraform, Ansible, GCC, CMake, HTTP (~50 MB total)
- Arch Wiki (~30 MB)
- Termux Wiki (~2 MB)
- OWASP Web Security Testing Guide + Cheat Sheets (~10-15 MB, custom packaging)
- Server Fault and Super User (already in presets at this tier)
- **Total new addition: ~7 GB**

### 256 GB -- comprehensive reference

Broad language coverage for polyglot teams and deep security work.

- Everything from 128 GB tier
- Stack Exchange: Code Review, Reverse Engineering (~635 MB)
- DevDocs extended languages: Kotlin, Java, Elixir, Erlang, Haskell, Scala, OCaml, Julia, Nim, Crystal, D, Perl, Clojure, PHP, Ruby, Deno, Node.js, LaTeX, Nix, Fish, Nushell, Emacs Lisp, Fortran (~60 MB total)
- DevDocs frameworks: Django, Flask, FastAPI, Rails, React, Vue, Angular, Spring Boot, HTML, CSS, SVG, DOM (~20 MB total)
- Alpine Linux Wiki (~3 MB)
- **Total new addition: ~720 MB**

### 512+ GB -- deep archive

Stack Overflow enters the picture.

- Everything from 256 GB tier
- Stack Overflow full dump (75 GB) -- only at 512 GB+ where budget allows
- Gentoo Wiki (~69 MB)
- **Total new addition: ~75 GB**

## Packaging considerations

### DevDocs ZIM bundling strategy

The 220+ individual DevDocs ZIMs are tiny (most under 5 MB). Two approaches:

1. **Individual recipes per ZIM** -- maximum flexibility, each language can be tiered independently. Verbose but consistent with how Stack Exchange recipes work today.
2. **Bundled recipe groups** -- e.g., `devdocs-core-languages`, `devdocs-infrastructure`, `devdocs-extended`. Simpler preset definitions. The recipe would list multiple URLs.

Recommendation: **per-language ecosystem bundles** that can be mixed into domain packs:

```
devdocs-c-eco:       C, C++, CMake, Make, GCC          → embedded pack + computing
devdocs-python-eco:  Python, Django, Flask, FastAPI     → computing + sciences
devdocs-js-eco:      JavaScript, TypeScript, Node, React, Vue → computing
devdocs-rust-eco:    Rust                               → computing
devdocs-go-eco:      Go                                 → computing
devdocs-ops:         Docker, Nginx, Terraform, Ansible, K8s → computing (sysadmin)
devdocs-data:        PostgreSQL, SQLite, DuckDB, Redis  → computing + sciences
devdocs-web:         HTML, CSS, SVG, DOM, HTTP          → computing
devdocs-general:     Git, Bash, jq, Lua, Zig, Make      → core or computing
```

This allows the embedded pack to pull in `devdocs-c-eco` without dragging in Django docs, and the sciences pack to pull in `devdocs-python-eco` + `devdocs-data` for data analysis use cases.

Each bundle is still tiny (under 50 MB) so they can ship at any tier.

### OWASP content

No official Kiwix ZIM exists for OWASP content. Options:

- Scrape the OWASP Web Security Testing Guide (WSTG) from its GitHub Pages site and package as a ZIM using `zim-tools` or `zimwriterfs`
- Bundle the Cheat Sheet Series Markdown source from GitHub as a static HTML site
- Package OWASP Top Ten as a single-page HTML reference

This is a small custom-packaging task but high value for the security subsection.

### Stack Overflow sizing

At 75 GB, Stack Overflow is a tier-defining item. It only makes sense at 512 GB+ where it can coexist with everything else. For smaller drives, the specialized Stack Exchange sites (Unix, Security, DBA, etc.) cover the most critical Q&A without the massive footprint.

## Relationship to other packs and presets

- **Programming Toolkit Preset** ([plan doc](../../2026-03-30-programming-toolkit-preset.md)): Provides actual compiler/interpreter binaries (Zig, Go, Rust, TCC, Lua, etc.) and embedded-dev toolchains. This computing pack provides the documentation those tools need. The two should be co-installable and cross-referenced.
- **Existing tool recipes**: CyberChef, sqlite3, DuckDB WASM, SQLiteViz, age (encryption) are already packaged as tools. This pack's DevDocs ZIMs for SQLite, DuckDB, and Git complement those tools with reference docs.
- **Wikibooks**: Already a recipe, contains some programming content (C, Python, etc.) but at textbook depth rather than API reference depth. Complementary, not duplicative.
- **Electronics SE**: Already a recipe, bridges into embedded development.

## Open questions

- Should DevDocs ZIMs be individual recipes or grouped bundles? (Leaning toward 3-4 groups aligned with tiers.)
- What is the best strategy for packaging OWASP content without an official Kiwix ZIM? Build from GitHub Markdown source?
- Should we carry a "Stack Overflow lite" -- e.g., top-voted questions only, or specific tag subsets (python, linux, git, etc.)? The full 75 GB dump is impractical below 512 GB, but a curated subset could fit at 128 GB.
- Are there other high-value computing references not on Kiwix? Candidates: The Rust Book (separate from DevDocs?), TLDP (The Linux Documentation Project), RFC archive, TCP/IP Illustrated-style references.
- Should protocol specifications (RFCs, HTTP/2, TLS 1.3, DNS, etc.) be included as a separate sub-collection, or is the devdocs HTTP ZIM + Wikipedia sufficient?
- How should this pack interact with the LLM models already in presets? A coding-focused model (e.g., a code-tuned Qwen or DeepSeek variant) would pair naturally with computing reference content.
