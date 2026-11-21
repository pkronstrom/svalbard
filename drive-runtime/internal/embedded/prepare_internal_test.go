package embedded

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestPrepareReusesReadyGenerationAndChangesForContentOrRelocation(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("SVALBARD_PIO_CACHE", cache)
	root := completeFixture(t)
	original := runBootstrapFn
	t.Cleanup(func() { runBootstrapFn = original })
	calls := 0
	runBootstrapFn = func(_ context.Context, _ io.Writer, _, generation, _ string, _ []inventoryItem) error {
		calls++
		return os.MkdirAll(filepath.Join(generation, "core"), 0o755)
	}
	first, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || calls != 1 {
		t.Fatalf("ready generation reuse = %q, %q; bootstrap calls = %d", first, second, calls)
	}
	archive := filepath.Join(root, "tools", "platformio", "packages", "platform.tar.gz")
	if err := os.WriteFile(archive, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	updateFixtureChecksum(t, root, archive)
	changed, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first || calls != 2 {
		t.Fatalf("content change did not make a generation: %q, calls %d", changed, calls)
	}
	updateFixtureOwner(t, root)
	metadataChanged, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	if metadataChanged == changed || calls != 3 {
		t.Fatalf("inventory identity change did not make a generation: %q, calls %d", metadataChanged, calls)
	}
	originalBootstrap := bootstrap
	t.Cleanup(func() { bootstrap = originalBootstrap })
	bootstrap = append(append([]byte(nil), bootstrap...), '\n')
	helperChanged, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	if helperChanged == metadataChanged || calls != 4 {
		t.Fatalf("bootstrap change did not make a generation: %q, calls %d", helperChanged, calls)
	}
	relocated, err := Prepare(context.Background(), io.Discard, completeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if relocated == first || calls != 5 {
		t.Fatalf("relocation did not make a generation: %q, calls %d", relocated, calls)
	}
}

func TestPrepareRequiresBundledPythonTrees(t *testing.T) {
	root := completeFixture(t)
	name, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "runtime", "python", name, "tools", "espidf-python")); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), io.Discard, root); err == nil {
		t.Fatal("Prepare() succeeded without espidf-python")
	}
}

func TestPrepareCleansFailedGenerationBeforeRetry(t *testing.T) {
	t.Setenv("SVALBARD_PIO_CACHE", t.TempDir())
	root := completeFixture(t)
	original := runBootstrapFn
	t.Cleanup(func() { runBootstrapFn = original })
	runBootstrapFn = func(context.Context, io.Writer, string, string, string, []inventoryItem) error { return os.ErrInvalid }
	if _, err := Prepare(context.Background(), io.Discard, root); err == nil {
		t.Fatal("Prepare() succeeded after bootstrap failure")
	}
	runBootstrapFn = func(_ context.Context, _ io.Writer, _, generation, _ string, _ []inventoryItem) error {
		return os.MkdirAll(filepath.Join(generation, "core"), 0o755)
	}
	generation, err := Prepare(context.Background(), io.Discard, root)
	if err != nil {
		t.Fatal(err)
	}
	if !ready(generation, filepath.Base(generation)) {
		t.Fatal("retry did not publish a ready generation")
	}
}

func TestPrepareDoesNotPublishReadyAfterBootstrapCancelsContext(t *testing.T) {
	t.Setenv("SVALBARD_PIO_CACHE", t.TempDir())
	root := completeFixture(t)
	original := runBootstrapFn
	t.Cleanup(func() { runBootstrapFn = original })
	ctx, cancel := context.WithCancel(context.Background())
	runBootstrapFn = func(context.Context, io.Writer, string, string, string, []inventoryItem) error {
		cancel()
		return nil
	}
	if _, err := Prepare(ctx, io.Discard, root); err != context.Canceled {
		t.Fatalf("Prepare() error = %v, want context cancellation", err)
	}
	cache, err := cacheDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	ready, _ := filepath.Glob(filepath.Join(cache, "generations", "*", ".ready"))
	if len(ready) != 0 {
		t.Fatalf("unexpected ready markers: %v", ready)
	}
}

func TestBootstrapValidatesEsptoolRuntimeDependencies(t *testing.T) {
	content := string(bootstrap)
	for _, required := range []string{
		`"bitstring": ">=3.1.6,!=4.2.0"`,
		`"ecdsa": ">=0.16.0"`,
		`"pyserial": ">=3.3"`,
		`"reedsolo": ">=1.5.3,<1.8"`,
		`"PyYAML": ">=5.1"`,
		`"intelhex": ""`,
		`modules["argcomplete"] = "argcomplete"`,
		`"pyserial": "serial"`,
		`"PyYAML": "yaml"`,
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("bootstrap venv validation missing %s", required)
		}
	}
}

func TestBootstrapCopiesTopLevelFilesIntoMissingDestination(t *testing.T) {
	bootstrapPath, err := filepath.Abs("bootstrap.py")
	if err != nil {
		t.Fatal(err)
	}
	fixture := `
import importlib.util
import tempfile
from pathlib import Path
import sys
spec = importlib.util.spec_from_file_location("bootstrap", sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
with tempfile.TemporaryDirectory() as temp:
    root = Path(temp)
    source = root / "source"
    source.mkdir()
    (source / ".lock").write_text("lock")
    (source / "package").mkdir()
    (source / "package" / "module.py").write_text("ok")
    destination = root / "missing" / "_contrib"
    module.copied_site_packages(source, destination)
    assert (destination / ".lock").read_text() == "lock"
    assert (destination / "package" / "module.py").read_text() == "ok"
`
	cmd := exec.Command("python3", "-c", fixture, bootstrapPath)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap copied_site_packages fixture: %v\n%s", err, output)
	}
}

func TestBootstrapCancellationKillsSurvivingProcessGroupChild(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	python := filepath.Join(dir, "python")
	script := "#!/bin/sh\n(trap '' TERM HUP; while :; do sleep 1; done) &\necho $! > " + shellQuote(pidFile) + "\ntrap 'exit 0' TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(python, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := false
	t.Cleanup(func() {
		cancel()
		if !finished {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("cancellation fixture did not reap its process group")
			}
		}
	})
	go func() { done <- runBootstrap(ctx, io.Discard, python, dir, dir, nil) }()

	var data []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		data, err = os.ReadFile(pidFile)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatal("bootstrap child did not report its PID before deadline")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	select {
	case err := <-done:
		finished = true
		if err != context.Canceled {
			t.Fatalf("runBootstrap() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runBootstrap did not reap process group after cancellation")
	}
	deadline = time.Now().Add(time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("surviving child %d remains: %v", pid, err)
	}
}

func TestLockContextHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prepare.lock")
	if os.Getenv("SVALBARD_LOCK_HELPER") == "1" {
		path = os.Getenv("SVALBARD_LOCK_PATH")
		lock, err := lockContext(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := os.WriteFile(os.Getenv("SVALBARD_LOCK_READY"), []byte("ready"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestLockContextHonorsCancellation")
	ready := filepath.Join(filepath.Dir(path), "ready")
	cmd.Env = append(os.Environ(), "SVALBARD_LOCK_HELPER=1", "SVALBARD_LOCK_PATH="+path, "SVALBARD_LOCK_READY="+ready)
	cmd.Args = append(cmd.Args, "-test.v")
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lock helper did not acquire lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := lockContext(ctx, path); err != context.DeadlineExceeded {
		t.Fatalf("lockContext() error = %v, want cancellation", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func completeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	name, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	archives := map[string]string{
		"tools/platformio/packages/platform.tar.gz": "platform",
		"tools/platformio/packages/esptool.tar.gz":  "esptool",
	}
	items := make([]inventoryItem, 0, len(archives))
	for path, contents := range archives {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(contents))
		item := inventoryItem{Path: path, SHA256: hex.EncodeToString(sum[:]), Owner: "platformio", Kind: "tool"}
		if contents == "platform" {
			item.Kind, item.Name, item.Platform = "platform", "espressif32", name
		} else {
			item.Name = "tool-esptoolpy"
		}
		items = append(items, item)
	}
	data, err := json.Marshal(inventory{Version: 1, Packages: items})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "platformio.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(root, "runtime", "python", name, "python", "cpython-test", "bin", "python3")
	if err := os.MkdirAll(filepath.Dir(python), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(python, []byte("python"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"platformio-core", "espidf-python"} {
		if err := os.MkdirAll(filepath.Join(root, "runtime", "python", name, "tools", pkg), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func updateFixtureChecksum(t *testing.T, root, archive string) {
	t.Helper()
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	inventoryPath := filepath.Join(root, ".svalbard", "platformio.json")
	raw, err := os.ReadFile(inventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	var inv inventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatal(err)
	}
	for index := range inv.Packages {
		if inv.Packages[index].Path == "tools/platformio/packages/platform.tar.gz" {
			inv.Packages[index].SHA256 = hex.EncodeToString(sum[:])
		}
	}
	raw, err = json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventoryPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func updateFixtureOwner(t *testing.T, root string) {
	t.Helper()
	inventoryPath := filepath.Join(root, ".svalbard", "platformio.json")
	raw, err := os.ReadFile(inventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	var inv inventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatal(err)
	}
	for index := range inv.Packages {
		if inv.Packages[index].Path == "tools/platformio/packages/platform.tar.gz" {
			inv.Packages[index].Owner = "different-owner"
		}
	}
	raw, err = json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventoryPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestCacheDirectoryResolvesSymlinkedCacheRoot(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "cache-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_PIO_CACHE", link)
	cache, err := cacheDirectory("/Volumes/VAULT")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	// ESP-IDF's builder compares resolved framework paths with unresolved ones.
	if filepath.Dir(cache) != resolved {
		t.Fatalf("cacheDirectory() = %q, want a child of resolved %q", cache, resolved)
	}
}

func TestCacheDirectoryRejectsMacOSTmp(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CMake rewrites /private/tmp only on macOS")
	}
	for _, base := range []string{"/tmp/svalbard-pio-test", "/private/tmp/svalbard-pio-test"} {
		t.Setenv("SVALBARD_PIO_CACHE", base)
		if _, err := cacheDirectory("/Volumes/VAULT"); err == nil || !strings.Contains(err.Error(), "/tmp") {
			t.Fatalf("cacheDirectory() with %s error = %v, want /tmp rejection", base, err)
		}
	}
}
