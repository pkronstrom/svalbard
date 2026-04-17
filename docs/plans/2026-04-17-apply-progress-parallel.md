# Apply: Download Progress, Parallel Downloads & Error Containment

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show per-item download progress (bytes/total/%), run downloads in parallel (4 workers), resume partial downloads (already done), and stop docker stderr from bleeding into the TUI.

**Architecture:** The progress event struct gains `Downloaded`/`Total` int64 fields. The downloader wraps its `io.Copy` with a counting writer that calls back every 64KB. `apply.Run` dispatches downloads into a goroutine pool and collects results via a channel. Docker build output is captured to a buffer instead of stderr.

**Tech Stack:** Go stdlib `sync`, `io`, `context`. No new dependencies.

---

### Task 1: Downloader progress callback

**Files:**
- Modify: `host-cli/internal/downloader/downloader.go`

Add a `ProgressFunc` type and an optional callback to `Download`. Wrap `streamToFile` with a counting reader that fires the callback every chunk.

```go
// Add to downloader.go

// ProgressFunc reports download progress. Called every 64KB.
type ProgressFunc func(downloaded, total int64)

// Change Download signature:
func Download(ctx context.Context, url, destPath, expectedSHA256 string, onProgress ...ProgressFunc) (Result, error) {

// Inside Download, after resp is received:
// total = resp.ContentLength (may be -1)
// For 200: total = resp.ContentLength
// For 206: total = existingSize + resp.ContentLength

// Replace streamToFile calls with streamToFileWithProgress
```

The counting reader wraps `resp.Body`:

```go
type countingReader struct {
    reader     io.Reader
    downloaded int64
    total      int64
    onProgress ProgressFunc
}

func (cr *countingReader) Read(p []byte) (int, error) {
    n, err := cr.reader.Read(p)
    cr.downloaded += int64(n)
    if cr.onProgress != nil {
        cr.onProgress(cr.downloaded, cr.total)
    }
    return n, err
}
```

---

### Task 2: Richer apply progress events

**Files:**
- Modify: `host-cli/internal/apply/apply.go` (ProgressFunc type + parallel loop)

Change `ProgressFunc` to carry download bytes:

```go
type ProgressEvent struct {
    ID         string
    Status     string // tui.Status* constants
    Downloaded int64  // bytes so far (only during StatusActive)
    Total      int64  // total bytes (-1 if unknown)
    Error      string
}

type ProgressFunc func(ProgressEvent)
```

---

### Task 3: Parallel download worker pool

**Files:**
- Modify: `host-cli/internal/apply/apply.go`

Replace sequential `for _, id := range plan.ToDownload` with:

```go
const maxWorkers = 4

type downloadJob struct {
    id     string
    recipe catalog.Item
}

type downloadResult struct {
    id      string
    entries []manifest.RealizedEntry
    err     error
}
```

- Fan-out: launch up to `maxWorkers` goroutines pulling from a job channel
- Fan-in: collect results on a result channel
- Continue on error: don't abort all downloads on first failure
- Report per-item progress via the `ProgressFunc`, threading the downloader's byte callback through

---

### Task 4: Capture docker build output

**Files:**
- Modify: `host-cli/internal/apply/apply.go` (buildItem function)

Replace:
```go
cmd.Stderr = os.Stderr
cmd.Stdout = os.Stderr
```

With:
```go
var buf bytes.Buffer
cmd.Stderr = &buf
cmd.Stdout = &buf
// On error, include buf.String() in the error message (truncated to ~200 chars)
```

---

### Task 5: Update hosttui ApplyEvent types

**Files:**
- Modify: `host-tui/dashboarddeps.go` (ApplyEvent)
- Modify: `host-tui/internal/plan/model.go` (ApplyEvent, applyStep, view)
- Modify: `host-tui/internal/wizard/types.go` (ApplyEvent)
- Modify: `host-tui/internal/wizard/applymodel.go` (applyStep, view)

Add `Downloaded` and `Total` int64 fields to all ApplyEvent structs. Update `applyStep` to track them. Update the view to render progress:

```
  ✓  SQLite CLI — required for cross-ZIM search
  ·  Gemma 4 E4B IT  2.1/5.3 GB  39%
     OWASP Cheat Sheets
     Kiwix tools
```

Footer while applying: `3/11 done  2 active  1.4 GB/s`

---

### Task 6: Wire progress through root.go callbacks

**Files:**
- Modify: `host-cli/internal/cli/root.go` (buildDashboardDeps, RunApply callback)

Bridge the new `apply.ProgressEvent` to `hosttui.ApplyEvent` including the byte fields.

---

### Task 7: Verify and commit

Run `go test ./...` across `host-cli`, `host-tui`, and `tui` modules. Fix any compile errors. Commit.
