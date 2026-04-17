# Structured Logging (slog) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add stdlib `log/slog` structured logging across the Go codebase, replacing ad-hoc `fmt.Fprintf(os.Stderr, ...)` calls with leveled, structured log output that writes to a file (always) and stderr (CLI mode only).

**Architecture:** A thin `logging` package in host-cli sets up slog with a multi-handler: file handler (JSON, always active) + optional stderr handler (text, CLI mode). The default slog logger is set at startup — no need to thread a logger through every function signature. TUI mode logs to file only since Bubble Tea owns the terminal. Log file lives at `<vault>/.svalbard/svalbard.log`, falling back to a temp file if vault is unknown.

**Tech Stack:** Go stdlib `log/slog` (zero external deps)

---

### Task 1: Create the logging package

**Files:**
- Create: `host-cli/internal/logging/logging.go`
- Test: `host-cli/internal/logging/logging_test.go`

**Step 1: Write the failing test**

```go
// host-cli/internal/logging/logging_test.go
package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cleanup, err := Init(Options{LogFile: logPath})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer cleanup()

	slog.Info("hello", "key", "val")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
}

func TestInitStderrMode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cleanup, err := Init(Options{LogFile: logPath, Stderr: true})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer cleanup()

	// Just verify it doesn't panic
	slog.Info("hello from stderr mode")
}

func TestInitDebugLevel(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	cleanup, err := Init(Options{LogFile: logPath, Debug: true})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer cleanup()

	slog.Debug("debug msg", "x", 1)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("debug message not written")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-cli && go test ./internal/logging/ -v`
Expected: compilation error — package doesn't exist

**Step 3: Write minimal implementation**

```go
// host-cli/internal/logging/logging.go
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Options configures the logger.
type Options struct {
	LogFile string // path to log file (required)
	Stderr  bool   // also log to stderr (CLI mode)
	Debug   bool   // enable debug-level logging
}

// Init sets up the default slog logger. Returns a cleanup function that
// flushes and closes the log file. Call cleanup in a defer.
func Init(opts Options) (cleanup func(), err error) {
	if opts.LogFile == "" {
		return func() {}, fmt.Errorf("LogFile is required")
	}

	if err := os.MkdirAll(filepath.Dir(opts.LogFile), 0o755); err != nil {
		return func() {}, fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return func() {}, fmt.Errorf("open log file: %w", err)
	}

	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}

	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if opts.Stderr {
		handler = slog.NewTextHandler(
			io.MultiWriter(f, os.Stderr), handlerOpts,
		)
	} else {
		handler = slog.NewJSONHandler(f, handlerOpts)
	}

	slog.SetDefault(slog.New(handler))

	return func() { f.Close() }, nil
}

// DefaultLogPath returns the standard log file path for a vault.
// If vaultRoot is empty, falls back to os.TempDir().
func DefaultLogPath(vaultRoot string) string {
	if vaultRoot != "" {
		return filepath.Join(vaultRoot, ".svalbard", "svalbard.log")
	}
	return filepath.Join(os.TempDir(), "svalbard.log")
}
```

**Step 4: Run test to verify it passes**

Run: `cd host-cli && go test ./internal/logging/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-cli/internal/logging/
git commit -m "feat(logging): add slog-based structured logging package"
```

---

### Task 2: Wire logging into host-cli entry point

**Files:**
- Modify: `host-cli/cmd/svalbard/main.go`
- Modify: `host-cli/internal/cli/root.go`

**Step 1: Update main.go to init logging**

```go
// host-cli/cmd/svalbard/main.go
package main

import (
	"fmt"
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/cli"
	"github.com/pkronstrom/svalbard/host-cli/internal/logging"
)

func main() {
	// Init logging early — vault path unknown yet, so use temp fallback.
	// Once vault is resolved, log path is <vault>/.svalbard/svalbard.log.
	logPath := logging.DefaultLogPath("")
	cleanup, err := logging.Init(logging.Options{
		LogFile: logPath,
		Stderr:  true,
		Debug:   os.Getenv("SVALBARD_DEBUG") != "",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: logging init failed: %v\n", err)
	} else {
		defer cleanup()
	}

	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 2: Add a `--debug` persistent flag in root.go**

At the top of `NewRootCommand()`, after the vault flag (line 41), add:

```go
root.PersistentFlags().Bool("debug", false, "enable debug logging")
```

No further wiring needed — the env var `SVALBARD_DEBUG` covers it. The flag is for discoverability.

**Step 3: Commit**

```bash
git add host-cli/cmd/svalbard/main.go host-cli/internal/cli/root.go
git commit -m "feat(cli): init slog logger at startup with SVALBARD_DEBUG support"
```

---

### Task 3: Wire logging into host-tui entry point

**Files:**
- Modify: `host-tui/cmd/svalbard-tui/main.go`

**Step 1: Update TUI main.go to init file-only logging**

The TUI must NOT log to stderr (Bubble Tea owns it). Log to file only.

```go
// At top of run(), before any other logic:
logPath := logging.DefaultLogPath("")
cleanup, err := logging.Init(logging.Options{
	LogFile: logPath,
	Stderr:  false,  // TUI mode — file only
	Debug:   os.Getenv("SVALBARD_DEBUG") != "",
})
if err == nil {
	defer cleanup()
}
```

This requires `host-tui` to import `host-cli/internal/logging`. Check if this creates a circular dependency — if so, extract the logging package to a shared module (e.g., `pkg/logging`). If host-tui already imports host-cli, it's fine.

**Alternative if circular:** Move logging to `host-cli/internal/logging` and have host-tui call `slog.SetDefault()` directly with a `slog.NewJSONHandler(file, opts)` inline — 5 lines, no new dependency.

**Step 2: Commit**

```bash
git add host-tui/cmd/svalbard-tui/main.go
git commit -m "feat(tui): init file-only slog logger at startup"
```

---

### Task 4: Replace fmt.Fprintf in apply.go with slog

**Files:**
- Modify: `host-cli/internal/apply/apply.go`

**Step 1: Replace all `fmt.Fprintf(os.Stderr, ...)` calls with slog equivalents**

| Line | Old | New |
|------|-----|-----|
| 94 | `fmt.Fprintf(os.Stderr, "skip %s: %v\n", id, err)` | `slog.Warn("skipping item", "id", id, "reason", err)` |
| 109 | `fmt.Fprintf(os.Stderr, "skip %s: native build failed: %v\n", id, err)` | `slog.Warn("native build failed, skipping", "id", id, "error", err)` |
| 117 | `fmt.Fprintf(os.Stderr, "skip %s: docker build failed: %v\n", id, err)` | `slog.Warn("docker build failed, skipping", "id", id, "error", err)` |
| 126 | `fmt.Fprintf(os.Stderr, "skip %s: no download URL, platforms, or build config\n", id)` | `slog.Warn("no acquisition strategy", "id", id)` |
| 234 | `fmt.Fprintf(os.Stderr, "skip %s for %s: no download URL (available: %v)\n", id, platform, platformKeys(...))` | `slog.Debug("no platform URL", "id", id, "platform", platform, "available", platformKeys(...))` |
| 345 | `fmt.Fprintf(os.Stderr, "building %s (family=%s) via docker...\n", id, family)` | `slog.Info("building via docker", "id", id, "family", family)` |

Also add structured logging at key decision points:

```go
// At top of Run(), after progress setup:
slog.Info("apply started",
	"downloads", len(plan.ToDownload),
	"removals", len(plan.ToRemove),
	"vault", root,
)

// At end of Run(), before return nil:
slog.Info("apply completed",
	"realized", len(m.Realized.Entries),
)

// In downloadItem, after resolve:
slog.Debug("resolved URL", "id", id, "url", resolvedURL)

// In fetchAndRecord, after download:
slog.Info("downloaded", "id", id, "path", destPath, "sha256", result.SHA256, "cached", result.Cached)
```

**Step 2: Remove the `"fmt"` and `"os"` imports if no longer needed** (likely `"os"` is still used for RemoveAll etc., and `"fmt"` for Errorf)

**Step 3: Run tests**

Run: `cd host-cli && go test ./internal/apply/ -v`
Expected: PASS (tests should not depend on stderr output)

**Step 4: Commit**

```bash
git add host-cli/internal/apply/apply.go
git commit -m "refactor(apply): replace fmt.Fprintf with structured slog logging"
```

---

### Task 5: Replace fmt.Fprintf in builder/pipeline.go with slog

**Files:**
- Modify: `host-cli/internal/builder/pipeline.go`

**Step 1: Replace all `fmt.Fprintf(os.Stderr, ...)` calls**

| Line | Old | New |
|------|-----|-----|
| 115 | `fmt.Fprintf(os.Stderr, "  downloading %s\n", filepath.Base(dest))` | `slog.Info("downloading", "file", filepath.Base(dest))` |
| 131 | `fmt.Fprintf(os.Stderr, "  extracting %s\n", filepath.Base(archivePath))` | `slog.Info("extracting", "file", filepath.Base(archivePath))` |
| 147 | `fmt.Fprintf(os.Stderr, "  exec %s %s\n", tool, strings.Join(args, " "))` | `slog.Info("exec", "tool", tool, "args", args)` |
| 161 | `fmt.Fprintf(os.Stderr, "  (using docker %s for %s)\n", image, tool)` | `slog.Info("exec via docker", "tool", tool, "image", image)` |

**Step 2: Replace fmt.Fprintf in pythonvenv.go**

| Line | Old | New |
|------|-----|-----|
| 75 | `fmt.Fprintf(os.Stderr, "  downloading wheels for %s...\n", platform)` | `slog.Info("downloading wheels", "platform", platform)` |
| 109 | `fmt.Fprintf(os.Stderr, "  installing python %s for %s...\n", pythonSpec, platform)` | `slog.Info("installing python", "version", pythonSpec, "platform", platform)` |
| 132 | `fmt.Fprintf(os.Stderr, "  creating venv for %s (%s)...\n", pkg.ID, platform)` | `slog.Info("creating venv", "package", pkg.ID, "platform", platform)` |

**Step 3: Run tests**

Run: `cd host-cli && go test ./internal/builder/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add host-cli/internal/builder/pipeline.go host-cli/internal/builder/pythonvenv.go
git commit -m "refactor(builder): replace fmt.Fprintf with structured slog logging"
```

---

### Task 6: Add debug logging to downloader

**Files:**
- Modify: `host-cli/internal/downloader/downloader.go`

The downloader currently has NO logging. Add debug-level logging for diagnostics:

**Step 1: Add slog calls at key points**

```go
// After cache hit (line 42):
slog.Debug("cache hit", "path", destPath, "sha256", expectedSHA256)
return Result{...}, nil

// Before HTTP request (after line 64):
slog.Debug("downloading", "url", url, "resume_from", existingSize)

// After response (after line 73):
slog.Debug("download response", "url", url, "status", resp.StatusCode)

// After hash verification (line 107):
slog.Debug("download complete", "path", destPath, "sha256", hash)
```

**Step 2: Run tests**

Run: `cd host-cli && go test ./internal/downloader/ -v`
Expected: PASS

**Step 3: Commit**

```bash
git add host-cli/internal/downloader/downloader.go
git commit -m "feat(downloader): add debug-level slog logging for diagnostics"
```

---

### Task 7: Add logging to commands/init.go and commands/apply.go

**Files:**
- Modify: `host-cli/internal/commands/apply.go`
- Modify: `host-cli/internal/commands/init.go`

**Step 1: Add slog calls**

```go
// commands/apply.go — in ApplyVault:
slog.Info("apply vault", "root", root)
// After manifest load:
slog.Debug("manifest loaded", "desired", len(m.Desired.Items), "realized", len(m.Realized.Entries))

// commands/init.go — in InitVault / InitVaultWithOptions:
slog.Info("init vault", "path", path, "preset", presetName)
```

**Step 2: Run full test suite**

Run: `cd host-cli && go test ./... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add host-cli/internal/commands/apply.go host-cli/internal/commands/init.go
git commit -m "feat(commands): add structured logging to init and apply commands"
```

---

### Task 8: Verify end-to-end

**Step 1: Build and test CLI logging**

```bash
cd host-cli && go build -o ../bin/svalbard ./cmd/svalbard/
SVALBARD_DEBUG=1 ../bin/svalbard plan
cat /tmp/svalbard.log  # verify structured output
```

**Step 2: Build and test TUI logging**

```bash
cd host-tui && go build -o ../bin/svalbard-tui ./cmd/svalbard-tui/
SVALBARD_DEBUG=1 ../bin/svalbard-tui
cat /tmp/svalbard.log  # verify file-only output, no stderr noise
```

**Step 3: Final commit if any cleanup needed**

---

## Summary of changes

| File | Change |
|------|--------|
| `host-cli/internal/logging/logging.go` | **New** — slog init, options, default path |
| `host-cli/internal/logging/logging_test.go` | **New** — tests |
| `host-cli/cmd/svalbard/main.go` | Init logger at startup |
| `host-cli/internal/cli/root.go` | Add --debug flag |
| `host-tui/cmd/svalbard-tui/main.go` | Init file-only logger |
| `host-cli/internal/apply/apply.go` | Replace 6x fmt.Fprintf → slog, add start/complete logs |
| `host-cli/internal/builder/pipeline.go` | Replace 4x fmt.Fprintf → slog |
| `host-cli/internal/builder/pythonvenv.go` | Replace 3x fmt.Fprintf → slog |
| `host-cli/internal/downloader/downloader.go` | Add 4x debug-level slog calls |
| `host-cli/internal/commands/apply.go` | Add info/debug slog calls |
| `host-cli/internal/commands/init.go` | Add info slog call |
