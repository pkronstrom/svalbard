# Drive Runtime Go Phase 2 Menu And Native Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat phase-1 runtime menu with a richer generic menu model sourced from inline YAML metadata, then port the easiest shell-backed actions (`inspect`, `verify`, `share`) to native Go.

**Architecture:** Menu metadata lives inline on existing content/tool/app definitions under a dedicated `menu:` key with sane defaults. The Python provisioner merges those definitions into a generic runtime menu JSON with groups, items, descriptions, optional subheaders, and action payloads. The Go launcher renders that generic model with one submenu level max and dispatches either to native Go actions or the existing shell-backed adapters while the migration is in progress.

**Tech Stack:** Python 3.12, pytest, Go 1.25, Bubble Tea, Lip Gloss

---

## File Map

### Python provisioner and metadata

- Modify: `src/svalbard/toolkit_generator.py`
  - Replace the flat `actions[]` runtime contract with grouped menu JSON
  - Merge inline `menu:` metadata from definitions
  - Apply sane defaults for definitions without explicit menu metadata
- Modify: `src/svalbard/presets.py`
  - Ensure menu metadata is available on loaded definitions if extra parsing is needed
- Modify: `recipes/**/*.yaml`
  - Add inline `menu:` metadata to initial built-in definitions needed for the first grouped menu
- Modify: `src/tests/test_toolkit_generator.py`
  - Assert grouped runtime menu output, descriptions, group ordering, and subheaders

### Go runtime menu model

- Modify: `drive-runtime/internal/config/config.go`
  - Replace the flat runtime config model with generic groups/items
- Modify: `drive-runtime/internal/config/config_test.go`
  - Cover grouped menu parsing
- Modify: `drive-runtime/internal/menu/model.go`
  - Add top-level group navigation and one-level submenu navigation
- Modify: `drive-runtime/internal/menu/view.go`
  - Render group descriptions, item descriptions, and subheaders
- Modify: `drive-runtime/internal/menu/model_test.go`
  - Cover group navigation, submenu behavior, and output-screen behavior

### Native Go actions

- Create: `drive-runtime/internal/runtimeinspect/inspect.go`
- Create: `drive-runtime/internal/runtimeverify/verify.go`
- Create: `drive-runtime/internal/runtimeshare/share.go`
- Modify: `drive-runtime/internal/actions/actions.go`
  - Add native action mode selection
- Modify: `drive-runtime/internal/actions/actions_test.go`
  - Verify native actions resolve without shell scripts
- Create: `drive-runtime/internal/runtimeinspect/inspect_test.go`
- Create: `drive-runtime/internal/runtimeverify/verify_test.go`
- Create: `drive-runtime/internal/runtimeshare/share_test.go`

### Docs

- Modify: `README.md`
  - Update the menu description to reflect grouped navigation
- Modify: `src/svalbard/readme_generator.py`
  - Update generated drive README text to reflect grouped menu navigation

---

### Task 1: Introduce the generic grouped runtime menu contract

**Files:**
- Modify: `src/svalbard/toolkit_generator.py`
- Modify: `src/tests/test_toolkit_generator.py`
- Modify: `recipes/**/*.yaml` for initial menu metadata

- [ ] **Step 1: Write failing Python tests for grouped runtime JSON**

Add tests like:

```python
def test_runtime_config_groups_top_level_menu(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    runtime = _read_runtime_config(tmp_path)
    assert [group["id"] for group in runtime["groups"]] == ["search", "library", "maps", "local-ai", "tools"]
```

```python
def test_library_group_contains_format_subheaders(tmp_path):
    runtime = _read_runtime_config(tmp_path)
    library = next(group for group in runtime["groups"] if group["id"] == "library")
    assert any(item.get("subheader") == "Archives" for item in library["items"])
```

- [ ] **Step 2: Run the focused toolkit tests to verify RED**

Run:

```bash
uv run pytest -q src/tests/test_toolkit_generator.py -k "groups_top_level_menu or library_group_contains_format_subheaders"
```

Expected:

- FAIL because runtime JSON still uses a flat `actions[]` shape

- [ ] **Step 3: Add inline `menu:` metadata to initial built-in definitions**

Add inline menu metadata to the built-ins needed for the stock grouped menu. Use a dedicated `menu:` block with sane defaults, for example:

```yaml
id: wikipedia-en-nopic
type: zim
description: English Wikipedia without images
menu:
  group: library
  subheader: Archives
  label: Wikipedia (text only)
  description: Browse the image-free English Wikipedia archive
  order: 100
```

```yaml
id: opencode
type: binary
description: OpenCode terminal coding assistant
menu:
  group: local-ai
  subheader: AI Clients
  label: OpenCode
  description: Launch the OpenCode terminal client against local models
  order: 200
```

- [ ] **Step 4: Implement grouped runtime JSON generation with defaults**

Refactor the runtime generator to emit a structure like:

```python
{
    "version": 2,
    "groups": [
        {
            "id": "library",
            "label": "Library",
            "description": "Browse packaged offline archives and documents",
            "order": 200,
            "items": [
                {
                    "id": "wikipedia-en-nopic",
                    "label": "Wikipedia (text only)",
                    "description": "Browse the image-free English Wikipedia archive",
                    "subheader": "Archives",
                    "order": 100,
                    "action": "open-archive",
                    "args": {"source_id": "wikipedia-en-nopic"},
                }
            ],
        }
    ],
}
```

Apply sane defaults when `menu:` is omitted:

- derive `label` from `description` or `id`
- derive `description` from the definition `description`
- derive `group` from `type`
- no `subheader` by default
- default `order` from type-specific buckets, then declaration order as tiebreaker

- [ ] **Step 5: Run the full toolkit test files to verify GREEN**

Run:

```bash
uv run pytest -q src/tests/test_toolkit_generator.py src/tests/test_commands.py
```

Expected:

- PASS with grouped runtime JSON and updated assertions

- [ ] **Step 6: Commit the grouped runtime contract**

```bash
git add src/svalbard/toolkit_generator.py src/tests/test_toolkit_generator.py src/tests/test_commands.py recipes
git commit -m "feat: add grouped drive runtime menu model"
```

---

### Task 2: Teach the Go launcher to render grouped menus with descriptions

**Files:**
- Modify: `drive-runtime/internal/config/config.go`
- Modify: `drive-runtime/internal/config/config_test.go`
- Modify: `drive-runtime/internal/menu/model.go`
- Modify: `drive-runtime/internal/menu/view.go`
- Modify: `drive-runtime/internal/menu/model_test.go`

- [ ] **Step 1: Write failing Go tests for grouped config and submenu navigation**

Add tests like:

```go
func TestLoadGroupedRuntimeConfig(t *testing.T) {
    cfg, err := config.Load(pathToGroupedFixture(t))
    if err != nil {
        t.Fatalf("Load() error = %v", err)
    }
    if got, want := cfg.Groups[0].ID, "search"; got != want {
        t.Fatalf("Groups[0].ID = %q, want %q", got, want)
    }
}
```

```go
func TestEnterOpensGroupScreen(t *testing.T) {
    m := NewModel(sampleGroupedConfig(), "/tmp/drive")
    updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    got := updated.(Model)
    if !got.inGroup {
        t.Fatal("inGroup = false, want true")
    }
}
```

- [ ] **Step 2: Run the Go tests to verify RED**

Run:

```bash
cd drive-runtime && go test ./...
```

Expected:

- FAIL because config/model types still assume a flat action list

- [ ] **Step 3: Replace the flat config model with groups and items**

Update the config package to model:

```go
type RuntimeConfig struct {
    Version int         `json:"version"`
    Groups  []MenuGroup `json:"groups"`
}

type MenuGroup struct {
    ID          string     `json:"id"`
    Label       string     `json:"label"`
    Description string     `json:"description"`
    Order       int        `json:"order"`
    Items       []MenuItem `json:"items"`
}

type MenuItem struct {
    ID          string            `json:"id"`
    Label       string            `json:"label"`
    Description string            `json:"description"`
    Subheader   string            `json:"subheader"`
    Order       int               `json:"order"`
    Action      string            `json:"action"`
    Args        map[string]string `json:"args"`
}
```

- [ ] **Step 4: Implement one-level grouped navigation**

Update the menu model so:

- top-level shows groups with descriptions
- `Enter` on a group opens its item screen
- `Esc` returns from a group screen to top-level
- item screens render subheaders inline, not nested submenus
- captured output screens still override the menu until dismissed

Keep navigation one level deep only.

- [ ] **Step 5: Run the full Go suite to verify GREEN**

Run:

```bash
cd drive-runtime && go test ./...
```

Expected:

- PASS for grouped config parsing, top-level navigation, group screens, and output behavior

- [ ] **Step 6: Commit grouped launcher rendering**

```bash
git add drive-runtime
git commit -m "feat: render grouped drive runtime menu"
```

---

### Task 3: Port `inspect` to native Go

**Files:**
- Create: `drive-runtime/internal/runtimeinspect/inspect.go`
- Create: `drive-runtime/internal/runtimeinspect/inspect_test.go`
- Modify: `drive-runtime/internal/actions/actions.go`
- Modify: `drive-runtime/internal/actions/actions_test.go`

- [ ] **Step 1: Write failing tests for native `inspect` output**

Add tests like:

```go
func TestInspectListsDriveContents(t *testing.T) {
    drive := buildTempDrive(t, map[string]string{
        "manifest.yaml": "preset: default-32\nregion: default\ncreated: 2026-01-01T00:00:00\n",
        "zim/example.zim": "",
    })

    output, err := runtimeinspect.Render(drive)
    if err != nil {
        t.Fatalf("Render() error = %v", err)
    }
    if !strings.Contains(output, "Drive contents") {
        t.Fatalf("output = %q, want drive header", output)
    }
    if !strings.Contains(output, "zim/") {
        t.Fatalf("output = %q, want zim listing", output)
    }
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run:

```bash
cd drive-runtime && go test ./internal/runtimeinspect
```

Expected:

- FAIL because the package does not exist yet

- [ ] **Step 3: Implement native `inspect` rendering**

Port the shell behavior into a pure Go renderer:

- read manifest metadata from `manifest.yaml`
- count files and sizes under `zim`, `maps`, `models`, `data`, `apps`, `books`, `bin`
- render file listings for ZIMs, GGUFs, SQLite files, and PMTiles

Expose:

```go
func Render(driveRoot string) (string, error)
```

- [ ] **Step 4: Switch `inspect` action resolution to native capture mode**

In the action registry, make `inspect` resolve to a native Go implementation that returns captured output for the menu to display instead of shelling out to `inspect.sh`.

- [ ] **Step 5: Run the full Go suite to verify GREEN**

Run:

```bash
cd drive-runtime && go test ./...
```

Expected:

- PASS with `inspect` no longer depending on the shell script

- [ ] **Step 6: Commit native `inspect`**

```bash
git add drive-runtime
git commit -m "feat: port drive inspect action to go"
```

---

### Task 4: Port `verify` to native Go

**Files:**
- Create: `drive-runtime/internal/runtimeverify/verify.go`
- Create: `drive-runtime/internal/runtimeverify/verify_test.go`
- Modify: `drive-runtime/internal/actions/actions.go`
- Modify: `drive-runtime/internal/actions/actions_test.go`

- [ ] **Step 1: Write failing tests for checksum verification output**

Add tests like:

```go
func TestVerifyReportsOkAndMissingFiles(t *testing.T) {
    drive := buildChecksumDrive(t)

    output, err := runtimeverify.Render(drive)
    if err != nil {
        t.Fatalf("Render() error = %v", err)
    }
    if !strings.Contains(output, "OK") {
        t.Fatalf("output = %q, want OK", output)
    }
    if !strings.Contains(output, "MISSING") {
        t.Fatalf("output = %q, want MISSING", output)
    }
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run:

```bash
cd drive-runtime && go test ./internal/runtimeverify
```

Expected:

- FAIL because the package does not exist yet

- [ ] **Step 3: Implement native checksum verification**

Port the shell behavior into Go:

- parse `.svalbard/checksums.sha256`
- compute SHA-256 for existing files
- report `OK`, `FAIL`, `MISSING`
- emit a summary line for passed/failed/missing counts

Expose:

```go
func Render(driveRoot string) (string, error)
```

- [ ] **Step 4: Switch `verify` action resolution to native capture mode**

Make the action registry use the native Go verifier instead of `verify.sh`.

- [ ] **Step 5: Run the full Go suite to verify GREEN**

Run:

```bash
cd drive-runtime && go test ./...
```

Expected:

- PASS with native checksum verification

- [ ] **Step 6: Commit native `verify`**

```bash
git add drive-runtime
git commit -m "feat: port drive verify action to go"
```

---

### Task 5: Port `share` to native Go

**Files:**
- Create: `drive-runtime/internal/runtimeshare/share.go`
- Create: `drive-runtime/internal/runtimeshare/share_test.go`
- Modify: `drive-runtime/internal/actions/actions.go`
- Modify: `drive-runtime/internal/actions/actions_test.go`

- [ ] **Step 1: Write failing tests for native sharing behavior**

Add tests like:

```go
func TestShareBuildsLanBindingMessage(t *testing.T) {
    result, err := runtimeshare.Prepare("/tmp/drive")
    if err != nil {
        t.Fatalf("Prepare() error = %v", err)
    }
    if result.BindAddr != "0.0.0.0" {
        t.Fatalf("BindAddr = %q, want 0.0.0.0", result.BindAddr)
    }
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run:

```bash
cd drive-runtime && go test ./internal/runtimeshare
```

Expected:

- FAIL because the package does not exist yet

- [ ] **Step 3: Implement native `share` orchestration**

Port the orchestration logic to Go while still relying on bundled or system file serving binaries if needed:

- determine LAN IP
- choose a free port
- start a file server process bound to `0.0.0.0`
- return/display the LAN URL
- keep the process attached until user interruption

Expose:

```go
func Run(driveRoot string) error
```

If phase-2 native serving is not practical in one step, it is acceptable for this first cut to use Go for port/IP/process orchestration while still execing `dufs`.

- [ ] **Step 4: Switch `share` action resolution to native run mode**

Update the action registry so `share` no longer shells through `share.sh`.

- [ ] **Step 5: Run the full Go suite to verify GREEN**

Run:

```bash
cd drive-runtime && go test ./...
```

Expected:

- PASS with `share` routed through native Go orchestration

- [ ] **Step 6: Commit native `share`**

```bash
git add drive-runtime
git commit -m "feat: port drive share action to go"
```

---

### Task 6: Update documentation and run final verification

**Files:**
- Modify: `README.md`
- Modify: `src/svalbard/readme_generator.py`

- [ ] **Step 1: Update menu descriptions in docs**

Update docs to describe:

- grouped top-level menu
- item descriptions
- submenu behavior for `Library`, `Local AI`, and any built-in groups used in the new runtime config

- [ ] **Step 2: Run the full verification commands**

Run:

```bash
uv run pytest -q src/tests/test_toolkit_generator.py src/tests/test_commands.py
cd drive-runtime && go test ./...
```

Expected:

- PASS for the Python provisioning/runtime-config suite
- PASS for the Go runtime module suite

- [ ] **Step 3: Commit docs and final verification**

```bash
git add README.md src/svalbard/readme_generator.py
git commit -m "docs: describe grouped drive runtime menus"
```

---

## Self-Review

### Spec coverage

- Generic-ish menu model sourced from inline definitions: covered by Task 1
- One submenu level max with subheaders and descriptions: covered by Task 2
- No pack-level override layer for now: reflected in Task 1 design
- First native Go ports for easier actions: covered by Tasks 3-5

### Placeholder scan

- No `TBD` or `TODO` placeholders remain
- Each task includes concrete commands, expected results, and implementation direction

### Type consistency

- Runtime menu model consistently uses `groups[]` and `items[]`
- Items consistently use `label`, `description`, `subheader`, `action`, and `args`
- Native action packages consistently expose either `Render()` for captured-output actions or `Run()` for long-lived actions
