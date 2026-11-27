package embedded

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/shell"
)

const preparationFormat = "1"

//go:embed bootstrap.py
var bootstrap []byte

type inventory struct {
	Version  int             `json:"version"`
	Packages []inventoryItem `json:"packages"`
}
type inventoryItem struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Platform string `json:"platform"`
}

// PackageName remains for callers that display a human-readable archive name.
// Preparation itself uses the inventory's registry identity instead.
func PackageName(filename string) string {
	name := filename
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.xz", ".zip"} {
		name = strings.TrimSuffix(name, suffix)
	}
	for _, marker := range []string{"-linux_", "-darwin_", "-windows_"} {
		if index := strings.Index(name, marker); index >= 0 {
			name = name[:index]
		}
	}
	if index := strings.LastIndex(name, "-"); index >= 0 && index+1 < len(name) && name[index+1] >= '0' && name[index+1] <= '9' {
		name = name[:index]
	}
	return name
}

func Environment(cache string) map[string]string {
	return map[string]string{
		"PLATFORMIO_CORE_DIR":                             cache,
		"PLATFORMIO_SETTING_ENABLE_TELEMETRY":             "no",
		"PLATFORMIO_DISABLE_UPGRADE_CHECK":                "true",
		"PLATFORMIO_SETTING_CHECK_PRUNE_SYSTEM_THRESHOLD": "0",
		"PIP_NO_INDEX":                                    "1",
		"PIP_DISABLE_PIP_VERSION_CHECK":                   "1",
		"PYTHONNOUSERSITE":                                "1",
		"PYTHONDONTWRITEBYTECODE":                         "1",
	}
}

// Prepare returns a ready cache generation. It never moves a generation after
// creation because Python venv scripts contain absolute paths.
func Prepare(ctx context.Context, stdout io.Writer, driveRoot string) (string, error) {
	platformName, err := platform.Detect()
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(driveRoot)
	if err != nil {
		return "", err
	}
	items, fingerprint, python, err := validateClosure(ctx, root, platformName)
	if err != nil {
		return "", err
	}
	cache, err := cacheDirectory(root)
	if err != nil {
		return "", err
	}
	generation := filepath.Join(cache, "generations", fingerprint)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if ready(generation, fingerprint) {
		return generation, nil
	}
	lock, err := lockContext(ctx, filepath.Join(cache, "locks", fingerprint+".lock"))
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if ready(generation, fingerprint) {
		return generation, nil
	}
	if err := os.RemoveAll(generation); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(generation, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(generation, "bootstrap.py"), bootstrap, 0o644); err != nil {
		return "", failGeneration(generation, err)
	}
	if err := runBootstrapFn(ctx, stdout, python, generation, root, items); err != nil {
		return "", failGeneration(generation, err)
	}
	if err := ctx.Err(); err != nil {
		return "", failGeneration(generation, err)
	}
	tmp := filepath.Join(generation, ".ready.tmp")
	if err := os.WriteFile(tmp, []byte(fingerprint+"\n"), 0o644); err != nil {
		return "", failGeneration(generation, err)
	}
	if err := os.Rename(tmp, filepath.Join(generation, ".ready")); err != nil {
		return "", failGeneration(generation, err)
	}
	return generation, nil
}

func Run(ctx context.Context, stdout io.Writer, driveRoot string) error {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/bash"
	}
	if err := shell.CheckEmbeddedShell(shellPath); err != nil {
		return err
	}
	root, err := filepath.Abs(driveRoot)
	if err != nil {
		return err
	}
	generation, err := Prepare(ctx, stdout, root)
	if err != nil {
		return err
	}
	platformName, err := platform.Detect()
	if err != nil {
		return err
	}
	env := Environment(filepath.Join(generation, "core"))
	env["DRIVE_ROOT"] = root
	binDir := filepath.Join(root, "bin", platformName)
	tempDir, err := os.MkdirTemp("", "svalbard-embedded-shell-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	cmd, extra, err := shell.NewInteractiveShellCommandWithEnvironment(ctx, shellPath, tempDir, env, binDir)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	env["PATH"] = binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	cmd.Env = shell.MergeEnvironment(os.Environ(), env)
	cmd.Env = shell.MergeEnvironment(cmd.Env, environmentMap(extra))
	fmt.Fprintf(stdout, "\nEmbedded dev shell ready.\n  PlatformIO cache: %s\n\n", generation)
	return cmd.Run()
}

func validateClosure(ctx context.Context, root, platformName string) ([]inventoryItem, string, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", "", err
	}
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "platformio.json"))
	if err != nil {
		return nil, "", "", fmt.Errorf("reading platformio inventory: %w", err)
	}
	var inv inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, "", "", fmt.Errorf("parsing platformio inventory: %w", err)
	}
	if inv.Version != 1 || len(inv.Packages) == 0 {
		return nil, "", "", errors.New("unsupported or empty platformio inventory")
	}
	seen := map[string]bool{}
	selected := make([]inventoryItem, 0, len(inv.Packages))
	hasPlatform, hasEsptool := false, false
	for _, item := range inv.Packages {
		if item.Platform != "" && item.Platform != platformName {
			continue
		}
		if item.Path == "" || item.SHA256 == "" || item.Owner == "" || item.Name == "" || (item.Kind != "tool" && item.Kind != "platform") {
			return nil, "", "", fmt.Errorf("invalid platformio inventory package %q", item.Path)
		}
		if !filepath.IsLocal(item.Path) || strings.HasPrefix(item.Path, "..") {
			return nil, "", "", fmt.Errorf("platformio inventory path escapes vault: %q", item.Path)
		}
		key := item.Kind + "/" + item.Owner + "/" + item.Name
		if seen[key] {
			return nil, "", "", fmt.Errorf("duplicate platformio package identity %s", key)
		}
		seen[key] = true
		path := filepath.Join(root, filepath.FromSlash(item.Path))
		if !within(root, path) {
			return nil, "", "", fmt.Errorf("platformio inventory path escapes vault: %q", item.Path)
		}
		sum, err := fileSHA256(ctx, path)
		if err != nil {
			return nil, "", "", fmt.Errorf("reading platformio archive %s: %w", item.Path, err)
		}
		if !strings.EqualFold(sum, item.SHA256) {
			return nil, "", "", fmt.Errorf("platformio archive checksum mismatch for %s", item.Path)
		}
		hasPlatform = hasPlatform || item.Kind == "platform"
		hasEsptool = hasEsptool || item.Name == "tool-esptoolpy"
		selected = append(selected, item)
	}
	if !hasPlatform || !hasEsptool {
		return nil, "", "", errors.New("platformio inventory is missing the pinned ESP-IDF platform or esptool")
	}
	pythonRoot := filepath.Join(root, "runtime", "python", platformName)
	python, err := bundledPython(pythonRoot)
	if err != nil {
		return nil, "", "", err
	}
	for _, name := range []string{"platformio-core", "espidf-python"} {
		if info, err := os.Stat(filepath.Join(pythonRoot, "tools", name)); err != nil || !info.IsDir() {
			return nil, "", "", fmt.Errorf("missing bundled Python package tree %s for %s", name, platformName)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	h := sha256.New()
	io.WriteString(h, preparationFormat+"\n"+platformName+"\n"+root+"\n")
	bootstrapHash := sha256.Sum256(bootstrap)
	io.WriteString(h, hex.EncodeToString(bootstrapHash[:])+"\n")
	for _, item := range selected {
		record, _ := json.Marshal(item)
		_, _ = h.Write(record)
		_, _ = h.Write([]byte("\n"))
	}
	for _, name := range []string{"platformio-core", "espidf-python"} {
		sum, err := treeSHA256(ctx, filepath.Join(pythonRoot, "tools", name))
		if err != nil {
			return nil, "", "", err
		}
		io.WriteString(h, name+"\n"+sum+"\n")
	}
	interpreterRoot := filepath.Dir(filepath.Dir(python))
	sum, err := treeSHA256(ctx, interpreterRoot)
	if err != nil {
		return nil, "", "", err
	}
	io.WriteString(h, interpreterRoot+"\n"+sum+"\n")
	return selected, hex.EncodeToString(h.Sum(nil)), python, nil
}

func cacheDirectory(root string) (string, error) {
	base := os.Getenv("SVALBARD_PIO_CACHE")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(base, "svalbard", "platformio")
	}
	// ESP-IDF's builder computes object paths from unresolved sources relative
	// to the resolved framework, so the cache path must already be canonical.
	// CMake on macOS also rewrites /private/tmp to /tmp, which no resolution fixes.
	if err := rejectMacOSTmp(base); err != nil {
		return "", err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	if err := rejectMacOSTmp(resolved); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(root))
	base = resolved
	return filepath.Join(base, hex.EncodeToString(sum[:8])), nil
}
func rejectMacOSTmp(path string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	for _, tmp := range []string{"/tmp", "/private/tmp"} {
		if path == tmp || strings.HasPrefix(path, tmp+"/") {
			return fmt.Errorf("PlatformIO cache %s must not be under /tmp on macOS; set SVALBARD_PIO_CACHE elsewhere", path)
		}
	}
	return nil
}

// bundledPython returns the interpreter binary itself, python3.<minor>, before
// any other python3*: newer builders turn bin/python3 into a script, and older
// ones on exFAT left it as a file macOS wrote for a symlink, which does not
// run on Linux.
func bundledPython(root string) (string, error) {
	matches, _ := filepath.Glob(filepath.Join(root, "python", "cpython-*", "bin", "python3*"))
	sort.SliceStable(matches, func(i, j int) bool {
		return exactPython.MatchString(filepath.Base(matches[i])) && !exactPython.MatchString(filepath.Base(matches[j]))
	})
	for _, path := range matches {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing bundled Python interpreter under %s", root)
}

var exactPython = regexp.MustCompile(`^python3\.[0-9]+$`)

var runBootstrapFn = runBootstrap

func runBootstrap(ctx context.Context, stdout io.Writer, python, generation, root string, items []inventoryItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	itemsFile := filepath.Join(generation, "packages.json")
	if err := os.WriteFile(itemsFile, data, 0o644); err != nil {
		return err
	}
	cmd := exec.Command(python, filepath.Join(generation, "bootstrap.py"), "--generation", generation, "--packages", itemsFile, "--drive-root", root)
	cmd.Stdout, cmd.Stderr = stdout, stdout
	cmd.Env = shell.MergeEnvironment(os.Environ(), Environment(filepath.Join(generation, "core")))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		reaped := false
		select {
		case <-done:
			reaped = true
		case <-time.After(2 * time.Second):
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return err
		}
		if !reaped {
			<-done
		}
		return ctx.Err()
	}
}
func ready(generation, fingerprint string) bool {
	data, err := os.ReadFile(filepath.Join(generation, ".ready"))
	return err == nil && string(data) == fingerprint+"\n"
}
func failGeneration(generation string, err error) error { _ = os.RemoveAll(generation); return err }
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func fileSHA256(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}
func treeSHA256(ctx context.Context, root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() != "__pycache__" && !strings.HasSuffix(d.Name(), ".pyc") {
			paths = append(paths, path)
		}
		return ctx.Err()
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		sum, err := fileSHA256(ctx, path)
		if err != nil {
			return "", err
		}
		io.WriteString(h, filepath.ToSlash(rel)+"\n"+sum+"\n")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type fileLock struct{ file *os.File }

func (l *fileLock) Close() error {
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	return l.file.Close()
}
func lockContext(ctx context.Context, path string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return &fileLock{f}, nil
		} else if err != unix.EWOULDBLOCK {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func environmentMap(entries []string) map[string]string {
	out := map[string]string{}
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}
