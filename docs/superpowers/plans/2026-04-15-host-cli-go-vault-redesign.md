# Host CLI Go Hard Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current Python host CLI with a new Go CLI centered on vault-first desired-state workflows: `init`, `add`, `remove`, `plan`, `apply`, `import`, and `preset`.

**Architecture:** The new host CLI lives in its own Go module and treats `manifest.yaml` as the vault marker and single source of truth. Desired state lives only in `manifest.yaml`; `plan` computes reconciliation and `apply` executes it, updates realized state, and regenerates on-drive runtime assets consumed by the separate `drive-runtime/` Go program.

**Tech Stack:** Go 1.25, Cobra, `gopkg.in/yaml.v3`, Go `embed`, standard library HTTP/filesystem/process APIs, existing `drive-runtime/` contract as reference, pytest/Python code only as behavior reference during migration

---

## Scope

This plan covers the host-side hard reset only. It intentionally excludes:

- top-level read-only verbs like `inspect`, `show`, or `list`
- sharing code with `drive-runtime/`
- preserving the old Python command contract
- reconstructing desired state from vault disk contents

## File Map

### New Go host CLI module

- Create: `host-cli/go.mod`
- Create: `host-cli/go.sum`
- Create: `host-cli/cmd/svalbard/main.go`

### Root CLI and shared command context

- Create: `host-cli/internal/cli/root.go`
- Create: `host-cli/internal/cli/context.go`
- Create: `host-cli/internal/cli/root_test.go`
- Create: `host-cli/internal/cli/context_test.go`

### Vault manifest and vault discovery

- Create: `host-cli/internal/manifest/manifest.go`
- Create: `host-cli/internal/manifest/manifest_test.go`
- Create: `host-cli/internal/vault/find.go`
- Create: `host-cli/internal/vault/find_test.go`

### Built-in catalog, presets, and workspace-local content

- Create: `host-cli/internal/catalog/embed.go`
- Create: `host-cli/internal/catalog/catalog.go`
- Create: `host-cli/internal/catalog/catalog_test.go`
- Create: `host-cli/internal/catalog/testdata/presets/default-32.yaml`
- Create: `host-cli/internal/catalog/testdata/presets/default-128.yaml`
- Create: `host-cli/internal/catalog/testdata/recipes/wikipedia-en-nopic.yaml`
- Create: `host-cli/internal/catalog/testdata/recipes/ifixit.yaml`
- Create: `host-cli/internal/catalog/testdata/recipes/local-demo.yaml`

### Vault editing commands

- Create: `host-cli/internal/commands/init.go`
- Create: `host-cli/internal/commands/add.go`
- Create: `host-cli/internal/commands/remove.go`
- Create: `host-cli/internal/commands/preset.go`
- Create: `host-cli/internal/commands/init_test.go`
- Create: `host-cli/internal/commands/add_test.go`
- Create: `host-cli/internal/commands/remove_test.go`
- Create: `host-cli/internal/commands/preset_test.go`

### Planner and apply pipeline

- Create: `host-cli/internal/planner/plan.go`
- Create: `host-cli/internal/planner/plan_test.go`
- Create: `host-cli/internal/commands/plan.go`
- Create: `host-cli/internal/apply/apply.go`
- Create: `host-cli/internal/apply/apply_test.go`
- Create: `host-cli/internal/commands/apply.go`

### Import pipeline and local library integration

- Create: `host-cli/internal/importer/importer.go`
- Create: `host-cli/internal/importer/importer_test.go`
- Create: `host-cli/internal/commands/import.go`
- Create: `host-cli/internal/commands/import_test.go`

### Drive runtime asset generation bridge

- Create: `host-cli/internal/toolkit/toolkit.go`
- Create: `host-cli/internal/toolkit/toolkit_test.go`

### Docs

- Modify: `README.md`
- Modify: `docs/usage.md`

## Critical Invariants And Things To Look For

- `manifest.yaml` stays the vault marker. Do not introduce `vault.yaml`.
- Vault resolution is implicit: `--vault` overrides, otherwise use `cwd`, then walk upward to the nearest parent containing `manifest.yaml`.
- Desired state comes only from YAML. Never infer intent from existing `zim/`, `maps/`, `models/`, or other vault directories.
- `desired.items` is the canonical effective item set. `desired.presets` records provenance and can seed the initial item set, but commands like `add` and `remove` mutate `desired.items`.
- `import` creates a local item first. It only mutates a vault when `--add` is present.
- `add` and `remove` are metadata-only operations. They must not download, build, or copy artifacts.
- `plan` computes reconciliation. `apply` executes it. No other command should mutate realized artifacts.
- `apply` must be idempotent. Running it twice against a fully realized vault should produce an empty or no-op plan.
- Unmanaged files on disk must be reported as unmanaged or drift, not silently adopted into desired state.
- `drive-runtime/` remains a separate Go program. The host CLI only generates the runtime assets it expects.
- Do not copy Python command names or semantics just because they exist in `src/svalbard/cli.py`; use Python only as behavior reference for recipe parsing, artifact paths, and source handling.
- Watch for current path and slug behavior in:
  - `src/svalbard/commands.py`
  - `src/svalbard/local_sources.py`
  - `src/svalbard/presets.py`
  - `src/svalbard/toolkit_generator.py`

## Proposed CLI Contract

```text
svalbard init [PATH]
svalbard add <item...>
svalbard remove <item...>
svalbard plan
svalbard apply
svalbard import <input>
svalbard preset list
svalbard preset copy <source> <target>
```

Command rules:

- Every vault command accepts `--vault PATH`, but resolves the current vault implicitly from `cwd` or nearest parent.
- `init` creates `manifest.yaml` v2 and seeds `desired.items`.
- `preset list` and `preset copy` are catalog operations and do not require a vault.
- `plan` prints a deterministic diff of desired vs realized state.
- `apply` executes the diff and updates `realized`.

## Manifest v2 Shape

```yaml
version: 2
vault:
  name: my-vault
  created_at: 2026-04-15T12:00:00Z
desired:
  presets:
    - default-32
  items:
    - wikipedia-en-nopic
    - ifixit
    - local:my-video
  options:
    region: finland
    host_platforms:
      - macos-arm64
    index_strategy: standard
realized:
  applied_at: 2026-04-15T12:34:56Z
  entries:
    - id: wikipedia-en-nopic
      type: zim
      filename: wikipedia-en-nopic.zim
      relative_path: zim/wikipedia-en-nopic.zim
      size_bytes: 4500000000
      checksum_sha256: abc123
      source_strategy: download
  index:
    strategy: standard
    built_at: 2026-04-15T12:35:20Z
  runtime:
    generated_at: 2026-04-15T12:35:35Z
```

---

### Task 1: Scaffold the new Go module and root CLI contract

**Files:**
- Create: `host-cli/go.mod`
- Create: `host-cli/cmd/svalbard/main.go`
- Create: `host-cli/internal/cli/root.go`
- Test: `host-cli/internal/cli/root_test.go`

- [ ] **Step 1: Write the failing root CLI test**

Add `host-cli/internal/cli/root_test.go`:

```go
package cli

import "testing"

func TestNewRootCommandHasHardResetCommands(t *testing.T) {
	cmd := NewRootCommand()
	got := map[string]bool{}
	for _, child := range cmd.Commands() {
		got[child.Name()] = true
	}

	for _, name := range []string{"init", "add", "remove", "plan", "apply", "import", "preset"} {
		if !got[name] {
			t.Fatalf("missing command %q", name)
		}
	}
}
```

- [ ] **Step 2: Run the root CLI test to verify RED**

Run:

```bash
cd host-cli && go test ./internal/cli -run TestNewRootCommandHasHardResetCommands
```

Expected:

- FAIL because `host-cli` and `NewRootCommand()` do not exist yet

- [ ] **Step 3: Create the Go module and root command**

Add `host-cli/go.mod`:

```go
module github.com/pkronstrom/svalbard/host-cli

go 1.25.6

require (
	github.com/spf13/cobra v1.9.1
	gopkg.in/yaml.v3 v3.0.1
)
```

Add `host-cli/cmd/svalbard/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Add `host-cli/internal/cli/root.go`:

```go
package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "svalbard",
		Short:         "Provision and reconcile offline knowledge vaults",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	for _, child := range []*cobra.Command{
		{Use: "init"},
		{Use: "add"},
		{Use: "remove"},
		{Use: "plan"},
		{Use: "apply"},
		{Use: "import"},
		{Use: "preset"},
	} {
		cmd.AddCommand(child)
	}

	return cmd
}
```

- [ ] **Step 4: Run the root CLI test to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/cli -run TestNewRootCommandHasHardResetCommands
```

Expected:

- PASS

- [ ] **Step 5: Run the module smoke test**

Run:

```bash
cd host-cli && go test ./...
```

Expected:

- PASS with only the root CLI package present

- [ ] **Step 6: Commit the CLI skeleton**

```bash
git add host-cli/go.mod host-cli/cmd/svalbard/main.go host-cli/internal/cli/root.go host-cli/internal/cli/root_test.go
git commit -m "feat(host-cli): add hard reset root command skeleton"
```

---

### Task 2: Add manifest v2 and implicit vault discovery

**Files:**
- Create: `host-cli/internal/manifest/manifest.go`
- Create: `host-cli/internal/manifest/manifest_test.go`
- Create: `host-cli/internal/vault/find.go`
- Create: `host-cli/internal/vault/find_test.go`
- Create: `host-cli/internal/cli/context.go`
- Create: `host-cli/internal/cli/context_test.go`

- [ ] **Step 1: Write the failing manifest and vault resolution tests**

Add `host-cli/internal/vault/find_test.go`:

```go
package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootWalksUpToManifest(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "content", "maps")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot("", nested)
	if err != nil {
		t.Fatalf("FindRoot() error = %v", err)
	}
	if got != root {
		t.Fatalf("FindRoot() = %q, want %q", got, root)
	}
}
```

Add `host-cli/internal/manifest/manifest_test.go`:

```go
package manifest

import "testing"

func TestNewManifestUsesDesiredAndRealizedSections(t *testing.T) {
	m := New("demo-vault")
	if m.Version != 2 {
		t.Fatalf("Version = %d, want 2", m.Version)
	}
	if m.Desired.Items == nil {
		t.Fatal("Desired.Items should be initialized")
	}
	if m.Realized.Entries == nil {
		t.Fatal("Realized.Entries should be initialized")
	}
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run:

```bash
cd host-cli && go test ./internal/vault ./internal/manifest
```

Expected:

- FAIL because `FindRoot()` and `New()` do not exist yet

- [ ] **Step 3: Implement manifest v2 types and vault resolution**

Add `host-cli/internal/manifest/manifest.go`:

```go
package manifest

import "time"

type Manifest struct {
	Version  int          `yaml:"version"`
	Vault    VaultMeta    `yaml:"vault"`
	Desired  DesiredState `yaml:"desired"`
	Realized RealizedState `yaml:"realized"`
}

type VaultMeta struct {
	Name      string `yaml:"name"`
	CreatedAt string `yaml:"created_at"`
}

type DesiredState struct {
	Presets []string      `yaml:"presets"`
	Items   []string      `yaml:"items"`
	Options DesiredOptions `yaml:"options"`
}

type DesiredOptions struct {
	Region        string   `yaml:"region"`
	HostPlatforms []string `yaml:"host_platforms"`
	IndexStrategy string   `yaml:"index_strategy"`
}

type RealizedState struct {
	AppliedAt string          `yaml:"applied_at"`
	Entries   []RealizedEntry `yaml:"entries"`
}

type RealizedEntry struct {
	ID             string `yaml:"id"`
	Type           string `yaml:"type"`
	Filename       string `yaml:"filename"`
	RelativePath   string `yaml:"relative_path"`
	SizeBytes      int64  `yaml:"size_bytes"`
	ChecksumSHA256 string `yaml:"checksum_sha256"`
	SourceStrategy string `yaml:"source_strategy"`
}

func New(name string) Manifest {
	return Manifest{
		Version: 2,
		Vault: VaultMeta{
			Name:      name,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Desired: DesiredState{
			Presets: []string{},
			Items:   []string{},
		},
		Realized: RealizedState{
			Entries: []RealizedEntry{},
		},
	}
}
```

Add `host-cli/internal/vault/find.go`:

```go
package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRoot(explicit string, cwd string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}

	current, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(current, "manifest.yaml")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no vault found from %s; run 'svalbard init' or pass --vault", cwd)
		}
		current = parent
	}
}
```

- [ ] **Step 4: Add the shared vault context helper**

Add `host-cli/internal/cli/context.go`:

```go
package cli

import (
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/vault"
)

func ResolveVaultRoot(explicit string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return vault.FindRoot(explicit, cwd)
}
```

- [ ] **Step 5: Run the tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/vault ./internal/manifest ./internal/cli
```

Expected:

- PASS

- [ ] **Step 6: Commit manifest v2 and vault discovery**

```bash
git add host-cli/internal/manifest/manifest.go host-cli/internal/manifest/manifest_test.go host-cli/internal/vault/find.go host-cli/internal/vault/find_test.go host-cli/internal/cli/context.go
git commit -m "feat(host-cli): add manifest v2 and implicit vault discovery"
```

---

### Task 3: Add embedded built-in catalog loading and preset resolution

**Files:**
- Create: `host-cli/internal/catalog/embed.go`
- Create: `host-cli/internal/catalog/catalog.go`
- Create: `host-cli/internal/catalog/catalog_test.go`
- Create: `host-cli/internal/catalog/testdata/presets/default-32.yaml`
- Create: `host-cli/internal/catalog/testdata/presets/default-128.yaml`
- Create: `host-cli/internal/catalog/testdata/recipes/wikipedia-en-nopic.yaml`
- Create: `host-cli/internal/catalog/testdata/recipes/ifixit.yaml`

- [ ] **Step 1: Write the failing preset resolution test**

Add `host-cli/internal/catalog/catalog_test.go`:

```go
package catalog

import "testing"

func TestResolvePresetExpandsRecipeIDs(t *testing.T) {
	c := NewTestCatalog(t)
	preset, err := c.ResolvePreset("default-32")
	if err != nil {
		t.Fatalf("ResolvePreset() error = %v", err)
	}
	if len(preset.Items) == 0 {
		t.Fatal("expected preset items")
	}
	if preset.Items[0].ID == "" {
		t.Fatal("first preset item should have an ID")
	}
}
```

- [ ] **Step 2: Run the catalog test to verify RED**

Run:

```bash
cd host-cli && go test ./internal/catalog -run TestResolvePresetExpandsRecipeIDs
```

Expected:

- FAIL because catalog loading does not exist yet

- [ ] **Step 3: Add minimal embedded test fixtures**

Add `host-cli/internal/catalog/testdata/presets/default-32.yaml`:

```yaml
name: default-32
description: Core reference
target_size_gb: 32
region: default
sources:
  - wikipedia-en-nopic
  - ifixit
```

Add `host-cli/internal/catalog/testdata/recipes/wikipedia-en-nopic.yaml`:

```yaml
id: wikipedia-en-nopic
type: zim
description: English Wikipedia without images
size_gb: 4.5
strategy: download
url: https://example.invalid/wikipedia.zim
```

Add `host-cli/internal/catalog/testdata/recipes/ifixit.yaml`:

```yaml
id: ifixit
type: zim
description: Repair guides
size_gb: 1.2
strategy: download
url: https://example.invalid/ifixit.zim
```

- [ ] **Step 4: Implement catalog loading and preset expansion**

Add `host-cli/internal/catalog/catalog.go`:

```go
package catalog

import (
	"io/fs"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Item struct {
	ID          string  `yaml:"id"`
	Type        string  `yaml:"type"`
	Description string  `yaml:"description"`
	SizeGB      float64 `yaml:"size_gb"`
	Strategy    string  `yaml:"strategy"`
	URL         string  `yaml:"url"`
}

type Preset struct {
	Name   string   `yaml:"name"`
	Region string   `yaml:"region"`
	Sources []string `yaml:"sources"`
	Items  []Item   `yaml:"-"`
}

type Catalog struct {
	recipes map[string]Item
	presets map[string]Preset
}

func (c *Catalog) ResolvePreset(name string) (Preset, error) {
	p := c.presets[name]
	p.Items = make([]Item, 0, len(p.Sources))
	for _, id := range p.Sources {
		p.Items = append(p.Items, c.recipes[id])
	}
	return p, nil
}

func LoadFromFS(recipesFS fs.FS, presetsFS fs.FS) (*Catalog, error) {
	c := &Catalog{
		recipes: map[string]Item{},
		presets: map[string]Preset{},
	}

	if err := fs.WalkDir(recipesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		raw, err := fs.ReadFile(recipesFS, path)
		if err != nil {
			return err
		}
		var item Item
		if err := yaml.Unmarshal(raw, &item); err != nil {
			return err
		}
		c.recipes[item.ID] = item
		return nil
	}); err != nil {
		return nil, err
	}

	if err := fs.WalkDir(presetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		raw, err := fs.ReadFile(presetsFS, path)
		if err != nil {
			return err
		}
		var preset Preset
		if err := yaml.Unmarshal(raw, &preset); err != nil {
			return err
		}
		c.presets[preset.Name] = preset
		return nil
	}); err != nil {
		return nil, err
	}

	return c, nil
}
```

- [ ] **Step 5: Add `embed.go` and test helper**

Add `host-cli/internal/catalog/embed.go`:

```go
package catalog

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

//go:embed testdata/recipes/*.yaml testdata/presets/*.yaml
var testData embed.FS

func NewTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	recipesFS, err := fs.Sub(testData, "testdata/recipes")
	if err != nil {
		t.Fatal(err)
	}
	presetsFS, err := fs.Sub(testData, "testdata/presets")
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func NewDefaultCatalog() *Catalog {
	_, currentFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	c, err := LoadFromFS(
		os.DirFS(filepath.Join(repoRoot, "recipes")),
		os.DirFS(filepath.Join(repoRoot, "presets")),
	)
	if err != nil {
		panic(err)
	}
	return c
}
```

Also extend `host-cli/internal/catalog/catalog.go` with:

```go
func (c *Catalog) PresetNames() []string {
	names := make([]string, 0, len(c.presets))
	for name := range c.presets {
		names = append(names, name)
	}
	return names
}
```

- [ ] **Step 6: Run the catalog tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/catalog
```

Expected:

- PASS

- [ ] **Step 7: Commit catalog loading**

```bash
git add host-cli/internal/catalog
git commit -m "feat(host-cli): add embedded catalog and preset resolution"
```

---

### Task 4: Implement `init` and `preset` commands

**Files:**
- Create: `host-cli/internal/commands/init.go`
- Create: `host-cli/internal/commands/preset.go`
- Create: `host-cli/internal/commands/init_test.go`
- Create: `host-cli/internal/commands/preset_test.go`
- Modify: `host-cli/internal/cli/root.go`

- [ ] **Step 1: Write the failing `init` and `preset list` tests**

Add `host-cli/internal/commands/init_test.go`:

```go
package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestInitSeedsManifestFromPreset(t *testing.T) {
	dir := t.TempDir()
	c := catalog.NewTestCatalog(t)
	if err := InitVault(dir, "default-32", c); err != nil {
		t.Fatalf("InitVault() error = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatal("manifest should not be empty")
	}
}
```

Add `host-cli/internal/commands/preset_test.go`:

```go
package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestPresetListWritesKnownPresets(t *testing.T) {
	var out bytes.Buffer
	c := catalog.NewTestCatalog(t)
	if err := WritePresetList(&out, c); err != nil {
		t.Fatalf("WritePresetList() error = %v", err)
	}
	if !strings.Contains(out.String(), "default-32") {
		t.Fatalf("output = %q, want preset name", out.String())
	}
}
```

- [ ] **Step 2: Run the command tests to verify RED**

Run:

```bash
cd host-cli && go test ./internal/commands -run "TestInitSeedsManifestFromPreset|TestPresetListWritesKnownPresets"
```

Expected:

- FAIL because `InitVault()` and `WritePresetList()` do not exist yet

- [ ] **Step 3: Implement the command helpers**

Add `host-cli/internal/commands/init.go`:

```go
package commands

import (
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"gopkg.in/yaml.v3"
)

func InitVault(path string, presetName string, c *catalog.Catalog) error {
	preset, err := c.ResolvePreset(presetName)
	if err != nil {
		return err
	}
	m := manifest.New(filepath.Base(path))
	m.Desired.Presets = []string{presetName}
	for _, item := range preset.Items {
		m.Desired.Items = append(m.Desired.Items, item.ID)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "manifest.yaml"), raw, 0o644)
}
```

Add `host-cli/internal/commands/preset.go`:

```go
package commands

import (
	"fmt"
	"io"
	"sort"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func WritePresetList(w io.Writer, c *catalog.Catalog) error {
	names := c.PresetNames()
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Wire the commands into the root CLI**

Update `host-cli/internal/cli/root.go` to use actual handlers:

```go
initCmd := &cobra.Command{
	Use:  "init [path]",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "."
		if len(args) == 1 {
			target = args[0]
		}
		c := catalog.NewDefaultCatalog()
		return commands.InitVault(target, "default-32", c)
	},
}

presetCmd := &cobra.Command{Use: "preset"}
presetCmd.AddCommand(&cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := catalog.NewDefaultCatalog()
		return commands.WritePresetList(cmd.OutOrStdout(), c)
	},
})
```

- [ ] **Step 5: Run the command tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/commands ./internal/cli
```

Expected:

- PASS

- [ ] **Step 6: Commit `init` and `preset`**

```bash
git add host-cli/internal/commands/init.go host-cli/internal/commands/preset.go host-cli/internal/commands/init_test.go host-cli/internal/commands/preset_test.go host-cli/internal/cli/root.go
git commit -m "feat(host-cli): add init and preset commands"
```

---

### Task 5: Implement `add` and `remove` as desired-state-only edits

**Files:**
- Create: `host-cli/internal/commands/add.go`
- Create: `host-cli/internal/commands/remove.go`
- Create: `host-cli/internal/commands/add_test.go`
- Create: `host-cli/internal/commands/remove_test.go`
- Modify: `host-cli/internal/manifest/manifest.go`
- Modify: `host-cli/internal/cli/root.go`

- [ ] **Step 1: Write the failing `add` and `remove` tests**

Add `host-cli/internal/commands/add_test.go`:

```go
package commands

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestAddItemsDeduplicatesAndPreservesExistingEntries(t *testing.T) {
	m := manifest.New("demo")
	m.Desired.Items = []string{"wikipedia-en-nopic"}
	if err := AddItems(&m, []string{"ifixit", "wikipedia-en-nopic"}); err != nil {
		t.Fatalf("AddItems() error = %v", err)
	}
	if got, want := len(m.Desired.Items), 2; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
}
```

Add `host-cli/internal/commands/remove_test.go`:

```go
package commands

import (
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestRemoveItemsOnlyEditsDesiredItems(t *testing.T) {
	m := manifest.New("demo")
	m.Desired.Items = []string{"wikipedia-en-nopic", "ifixit"}
	if err := RemoveItems(&m, []string{"ifixit"}); err != nil {
		t.Fatalf("RemoveItems() error = %v", err)
	}
	want := []string{"wikipedia-en-nopic"}
	if !reflect.DeepEqual(m.Desired.Items, want) {
		t.Fatalf("Desired.Items = %#v, want %#v", m.Desired.Items, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run:

```bash
cd host-cli && go test ./internal/commands -run "TestAddItemsDeduplicatesAndPreservesExistingEntries|TestRemoveItemsOnlyEditsDesiredItems"
```

Expected:

- FAIL because `AddItems()` and `RemoveItems()` do not exist yet

- [ ] **Step 3: Implement the manifest mutation helpers**

Add `host-cli/internal/commands/add.go`:

```go
package commands

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

func AddItems(m *manifest.Manifest, ids []string) error {
	seen := map[string]bool{}
	for _, id := range m.Desired.Items {
		seen[id] = true
	}
	for _, id := range ids {
		if !seen[id] {
			m.Desired.Items = append(m.Desired.Items, id)
			seen[id] = true
		}
	}
	return nil
}
```

Add `host-cli/internal/commands/remove.go`:

```go
package commands

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

func RemoveItems(m *manifest.Manifest, ids []string) error {
	remove := map[string]bool{}
	for _, id := range ids {
		remove[id] = true
	}
	next := make([]string, 0, len(m.Desired.Items))
	for _, id := range m.Desired.Items {
		if !remove[id] {
			next = append(next, id)
		}
	}
	m.Desired.Items = next
	return nil
}
```

- [ ] **Step 4: Add manifest load/save helpers and wire the commands**

Extend `host-cli/internal/manifest/manifest.go` with:

```go
func Load(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func Save(path string, m Manifest) error {
	raw, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
```

Wire `add` and `remove` in `host-cli/internal/cli/root.go`:

```go
addCmd := &cobra.Command{
	Use:  "add <item...>",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := ResolveVaultRoot(vaultPath)
		if err != nil {
			return err
		}
		m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
		if err != nil {
			return err
		}
		if err := commands.AddItems(&m, args); err != nil {
			return err
		}
		return manifest.Save(filepath.Join(root, "manifest.yaml"), m)
	},
}
```

- [ ] **Step 5: Run the command tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/commands ./internal/cli ./internal/manifest
```

Expected:

- PASS

- [ ] **Step 6: Commit `add` and `remove`**

```bash
git add host-cli/internal/commands/add.go host-cli/internal/commands/remove.go host-cli/internal/commands/add_test.go host-cli/internal/commands/remove_test.go host-cli/internal/manifest/manifest.go host-cli/internal/cli/root.go
git commit -m "feat(host-cli): add desired state editing commands"
```

---

### Task 6: Add planner engine and `plan` command

**Files:**
- Create: `host-cli/internal/planner/plan.go`
- Create: `host-cli/internal/planner/plan_test.go`
- Create: `host-cli/internal/commands/plan.go`
- Modify: `host-cli/internal/cli/root.go`

- [ ] **Step 1: Write the failing planner test**

Add `host-cli/internal/planner/plan_test.go`:

```go
package planner

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestBuildPlanFindsDownloadsAndRemovals(t *testing.T) {
	m := manifest.New("demo")
	m.Desired.Items = []string{"wikipedia-en-nopic", "ifixit"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia-en-nopic", RelativePath: "zim/wikipedia-en-nopic.zim"},
		{ID: "old-source", RelativePath: "zim/old-source.zim"},
	}

	plan := Build(m)
	if len(plan.ToDownload) != 1 || plan.ToDownload[0] != "ifixit" {
		t.Fatalf("ToDownload = %#v", plan.ToDownload)
	}
	if len(plan.ToRemove) != 1 || plan.ToRemove[0] != "old-source" {
		t.Fatalf("ToRemove = %#v", plan.ToRemove)
	}
}
```

- [ ] **Step 2: Run the planner test to verify RED**

Run:

```bash
cd host-cli && go test ./internal/planner -run TestBuildPlanFindsDownloadsAndRemovals
```

Expected:

- FAIL because `Build()` does not exist yet

- [ ] **Step 3: Implement the planner core**

Add `host-cli/internal/planner/plan.go`:

```go
package planner

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

type Plan struct {
	ToDownload []string
	ToRemove   []string
	Unmanaged  []string
}

func Build(m manifest.Manifest) Plan {
	desired := map[string]bool{}
	for _, id := range m.Desired.Items {
		desired[id] = true
	}

	realized := map[string]bool{}
	var plan Plan
	for _, entry := range m.Realized.Entries {
		realized[entry.ID] = true
		if !desired[entry.ID] {
			plan.ToRemove = append(plan.ToRemove, entry.ID)
		}
	}

	for _, id := range m.Desired.Items {
		if !realized[id] {
			plan.ToDownload = append(plan.ToDownload, id)
		}
	}

	return plan
}
```

- [ ] **Step 4: Add the `plan` command output**

Add `host-cli/internal/commands/plan.go`:

```go
package commands

import (
	"fmt"
	"io"

	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

func WritePlan(w io.Writer, plan planner.Plan) error {
	if _, err := fmt.Fprintf(w, "download: %d\n", len(plan.ToDownload)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "remove: %d\n", len(plan.ToRemove)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "unmanaged: %d\n", len(plan.Unmanaged)); err != nil {
		return err
	}
	return nil
}
```

Wire the command in `host-cli/internal/cli/root.go`:

```go
planCmd := &cobra.Command{
	Use: "plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := ResolveVaultRoot(vaultPath)
		if err != nil {
			return err
		}
		m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
		if err != nil {
			return err
		}
		return commands.WritePlan(cmd.OutOrStdout(), planner.Build(m))
	},
}
```

- [ ] **Step 5: Run the planner and CLI tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/planner ./internal/commands ./internal/cli
```

Expected:

- PASS

- [ ] **Step 6: Commit planner v1**

```bash
git add host-cli/internal/planner/plan.go host-cli/internal/planner/plan_test.go host-cli/internal/commands/plan.go host-cli/internal/cli/root.go
git commit -m "feat(host-cli): add planner and plan command"
```

---

### Task 7: Port runtime/toolkit generation and `apply`

**Files:**
- Create: `host-cli/internal/toolkit/toolkit.go`
- Create: `host-cli/internal/toolkit/toolkit_test.go`
- Create: `host-cli/internal/apply/apply.go`
- Create: `host-cli/internal/apply/apply_test.go`
- Create: `host-cli/internal/commands/apply.go`
- Modify: `host-cli/internal/manifest/manifest.go`
- Modify: `host-cli/internal/cli/root.go`

- [ ] **Step 1: Write the failing toolkit and apply tests**

Add `host-cli/internal/toolkit/toolkit_test.go`:

```go
package toolkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateWritesDriveRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	if err := Generate(root, []string{"wikipedia-en-nopic"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", "actions.json")); err != nil {
		t.Fatalf("actions.json missing: %v", err)
	}
}
```

Add `host-cli/internal/apply/apply_test.go`:

```go
package apply

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

func TestApplyWritesRealizedEntriesForDownloads(t *testing.T) {
	root := t.TempDir()
	m := manifest.New("demo")
	m.Desired.Items = []string{"wikipedia-en-nopic"}
	plan := planner.Plan{ToDownload: []string{"wikipedia-en-nopic"}}
	if err := Run(root, &m, plan); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(m.Realized.Entries) != 1 {
		t.Fatalf("Realized.Entries = %#v", m.Realized.Entries)
	}
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run:

```bash
cd host-cli && go test ./internal/toolkit ./internal/apply
```

Expected:

- FAIL because `Generate()` and `Run()` do not exist yet

- [ ] **Step 3: Port the runtime asset generator**

Add `host-cli/internal/toolkit/toolkit.go`:

```go
package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type runtimeConfig struct {
	Version int `json:"version"`
	Groups  []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"groups"`
}

func Generate(root string, items []string) error {
	dir := filepath.Join(root, ".svalbard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfg := runtimeConfig{
		Version: 2,
		Groups: []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		}{
			{ID: "library", Label: "Library"},
		},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "actions.json"), append(raw, '\n'), 0o644)
}
```

- [ ] **Step 4: Implement `apply` as reconciliation and realized-state update**

Add `host-cli/internal/apply/apply.go`:

```go
package apply

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

func Run(root string, m *manifest.Manifest, plan planner.Plan) error {
	for _, id := range plan.ToDownload {
		relative := filepath.Join("managed", id+".artifact")
		target := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(id+"\n"), 0o644); err != nil {
			return err
		}
		m.Realized.Entries = append(m.Realized.Entries, manifest.RealizedEntry{
			ID:           id,
			Type:         "managed",
			Filename:     filepath.Base(target),
			RelativePath: relative,
			SourceStrategy: "download",
		})
	}
	m.Realized.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	return toolkit.Generate(root, m.Desired.Items)
}
```

- [ ] **Step 5: Wire the `apply` command and persist the updated manifest**

Add `host-cli/internal/commands/apply.go`:

```go
package commands

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

func ApplyVault(root string) error {
	path := filepath.Join(root, "manifest.yaml")
	m, err := manifest.Load(path)
	if err != nil {
		return err
	}
	if err := apply.Run(root, &m, planner.Build(m)); err != nil {
		return err
	}
	return manifest.Save(path, m)
}
```

- [ ] **Step 6: Run the apply path tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/toolkit ./internal/apply ./internal/commands
```

Expected:

- PASS

- [ ] **Step 7: Commit `apply` and runtime generation**

```bash
git add host-cli/internal/toolkit/toolkit.go host-cli/internal/toolkit/toolkit_test.go host-cli/internal/apply/apply.go host-cli/internal/apply/apply_test.go host-cli/internal/commands/apply.go host-cli/internal/cli/root.go
git commit -m "feat(host-cli): add apply pipeline and runtime generation"
```

---

### Task 8: Add local import and `import --add`

**Files:**
- Create: `host-cli/internal/importer/importer.go`
- Create: `host-cli/internal/importer/importer_test.go`
- Create: `host-cli/internal/commands/import.go`
- Create: `host-cli/internal/commands/import_test.go`
- Modify: `host-cli/internal/cli/root.go`

- [ ] **Step 1: Write the failing importer test**

Add `host-cli/internal/importer/importer_test.go`:

```go
package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportLocalFileCopiesIntoWorkspaceLibrary(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(t.TempDir(), "manual.pdf")
	if err := os.WriteFile(source, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := ImportLocalFile(workspace, source, "")
	if err != nil {
		t.Fatalf("ImportLocalFile() error = %v", err)
	}
	if id == "" {
		t.Fatal("expected local source id")
	}
}
```

- [ ] **Step 2: Run the importer test to verify RED**

Run:

```bash
cd host-cli && go test ./internal/importer -run TestImportLocalFileCopiesIntoWorkspaceLibrary
```

Expected:

- FAIL because `ImportLocalFile()` does not exist yet

- [ ] **Step 3: Implement library import**

Add `host-cli/internal/importer/importer.go`:

```go
package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ImportLocalFile(workspace string, source string, outputName string) (string, error) {
	base := outputName
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}
	id := "local:" + slug(base)
	destDir := filepath.Join(workspace, "library")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, filepath.Base(source))
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return id, nil
}

func slug(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "-")
	return strings.Trim(value, "-")
}
```

- [ ] **Step 4: Add `import --add` command behavior**

Add `host-cli/internal/commands/import.go`:

```go
package commands

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/importer"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func ImportAndMaybeAdd(workspace string, source string, outputName string, add bool, vaultRoot string) (string, error) {
	id, err := importer.ImportLocalFile(workspace, source, outputName)
	if err != nil {
		return "", err
	}
	if !add {
		return id, nil
	}

	path := filepath.Join(vaultRoot, "manifest.yaml")
	m, err := manifest.Load(path)
	if err != nil {
		return "", err
	}
	if err := AddItems(&m, []string{id}); err != nil {
		return "", err
	}
	if err := manifest.Save(path, m); err != nil {
		return "", err
	}
	return id, nil
}
```

- [ ] **Step 5: Run the importer tests to verify GREEN**

Run:

```bash
cd host-cli && go test ./internal/importer ./internal/commands
```

Expected:

- PASS

- [ ] **Step 6: Commit import support**

```bash
git add host-cli/internal/importer/importer.go host-cli/internal/importer/importer_test.go host-cli/internal/commands/import.go host-cli/internal/cli/root.go
git commit -m "feat(host-cli): add library import and import --add"
```

---

### Task 9: Tighten parity boundaries, add drift checks, and update docs

**Files:**
- Modify: `host-cli/internal/planner/plan.go`
- Modify: `host-cli/internal/planner/plan_test.go`
- Modify: `README.md`
- Modify: `docs/usage.md`

- [ ] **Step 1: Write the failing unmanaged-file planner test**

Add to `host-cli/internal/planner/plan_test.go`:

```go
func TestBuildPlanReportsUnmanagedFiles(t *testing.T) {
	m := manifest.New("demo")
	m.Desired.Items = []string{"wikipedia-en-nopic"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia-en-nopic", RelativePath: "zim/wikipedia-en-nopic.zim"},
	}

	onDisk := []string{"zim/wikipedia-en-nopic.zim", "zim/manual-drop.zim"}
	plan := BuildWithDisk(m, onDisk)
	if len(plan.Unmanaged) != 1 || plan.Unmanaged[0] != "zim/manual-drop.zim" {
		t.Fatalf("Unmanaged = %#v", plan.Unmanaged)
	}
}
```

- [ ] **Step 2: Run the planner test to verify RED**

Run:

```bash
cd host-cli && go test ./internal/planner -run TestBuildPlanReportsUnmanagedFiles
```

Expected:

- FAIL because `BuildWithDisk()` does not exist yet

- [ ] **Step 3: Implement unmanaged-file detection**

Extend `host-cli/internal/planner/plan.go`:

```go
func BuildWithDisk(m manifest.Manifest, onDisk []string) Plan {
	plan := Build(m)
	managed := map[string]bool{}
	for _, entry := range m.Realized.Entries {
		managed[entry.RelativePath] = true
	}
	for _, path := range onDisk {
		if !managed[path] {
			plan.Unmanaged = append(plan.Unmanaged, path)
		}
	}
	return plan
}
```

- [ ] **Step 4: Update the docs for the new vault-first CLI**

Update `README.md` and `docs/usage.md` with examples like:

```text
svalbard init /Volumes/MyStick
svalbard add wikipedia-en-nopic ifixit
svalbard plan
svalbard apply
svalbard import ~/Downloads/manual.pdf --add
```

Explicitly document:

- implicit vault resolution from `cwd` and parent directories
- `manifest.yaml` is the source of truth
- `plan` previews, `apply` executes
- `import` does not mutate a vault unless `--add` is used

- [ ] **Step 5: Run the full host CLI test suite**

Run:

```bash
cd host-cli && go test ./...
```

Expected:

- PASS

- [ ] **Step 6: Commit the CLI hard-reset docs and drift checks**

```bash
git add host-cli/internal/planner/plan.go host-cli/internal/planner/plan_test.go README.md docs/usage.md
git commit -m "feat(host-cli): document vault-first workflow and drift detection"
```

---

## Follow-Up Checks Before Broad Execution

- Verify whether `drive-runtime/` currently expects `.svalbard/actions.json`, `.svalbard/runtime.json`, or another filename before finalizing `host-cli/internal/toolkit/toolkit.go`.
- Compare current artifact placement rules in `src/svalbard/commands.py` before locking the Go `apply` directory layout. The exact subdirectories for `zim`, `maps`, `models`, `apps`, and `books` should match your intended v2 contract.
- Decide whether built-in catalog data should be embedded directly in `host-cli/` or copied/generated from repo-root `presets/` and `recipes/` as part of the build. Do not leave this implicit.
- Decide whether `init` should default to a fixed preset, an interactive preset picker, or require `--preset`. The command plumbing above is compatible with any of those, but the product UX should be made explicit before implementation starts.
- Keep `add` and `remove` free of side effects. If command handlers start calling planner or apply helpers, the boundary has slipped.
- Keep `apply` authoritative for realized state updates. If other commands modify `realized`, fix the boundary immediately.

## Recommended Execution Order

1. Tasks 1-3 first. They establish the module, manifest contract, and built-in catalog.
2. Task 4 next. `init` and `preset` are the first end-to-end usable commands.
3. Task 5 after that. `add` and `remove` make the desired-state model real.
4. Tasks 6-7 next. `plan` and `apply` make the product usable.
5. Task 8 after `apply`. `import --add` becomes straightforward once the vault editing path exists.
6. Task 9 last. Tighten drift handling and docs after the core flow works.

## Self-Review

- Spec coverage: this plan covers the agreed hard-reset CLI, implicit vault resolution, `manifest.yaml` v2, desired-vs-realized separation, `plan/apply`, and `import --add`. Deferred read-only verbs remain explicitly out of scope.
- Placeholder scan: no `TODO`, `TBD`, or “handle later” steps remain in the tasks. Follow-up checks are flagged as pre-execution decisions, not implementation placeholders.
- Type consistency: command names, manifest field names, and package boundaries are consistent across tasks (`desired.items`, `realized.entries`, `ResolveVaultRoot`, `Build`, `ApplyVault`, `ImportAndMaybeAdd`).
