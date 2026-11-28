// Package weblauncher installs the optional graphical entry points and atlas.
package weblauncher

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

//go:embed package ui all:traybins
var payload embed.FS

const atlasPath = ".svalbard/web-launcher.zip"

type asset struct {
	name string
	data []byte
	mode fs.FileMode
}

func assets() ([]asset, error) {
	ui, err := fs.Sub(payload, "ui")
	if err != nil {
		return nil, err
	}
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	if err := archive.AddFS(ui); err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	files := []asset{{atlasPath, data.Bytes(), 0644}}
	err = fs.WalkDir(payload, "package", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		contents, err := payload.ReadFile(path)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(path, "package/")
		mode := fs.FileMode(0644)
		if name == "Svalbard-Linux" || name == "Svalbard.desktop" || strings.HasSuffix(name, "/MacOS/svalbard-launcher") {
			mode = 0755
		}
		files = append(files, asset{name, contents, mode})
		return nil
	})
	return files, err
}

// Every owned path is a regular file. A collision on the first install leaves
// both the user's file and the rest of the vault alone.
func Install(dir, id, kind string, platforms []string) ([]manifest.RealizedEntry, error) {
	if id != "web-launcher" || kind != "web-launcher" {
		return nil, fmt.Errorf("the web-launcher package requires id and type web-launcher")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files, err := assets()
	if err != nil {
		return nil, err
	}
	mac, linux := selectedOS(platforms)
	files = slices.DeleteFunc(files, func(f asset) bool { return !includePath(f.name, mac, linux) })
	trayFiles, err := builtTrayFiles(platforms)
	if err != nil {
		return nil, err
	}
	files = append(files, trayFiles...)
	if len(trayFiles) > 0 {
		notice, err := payload.ReadFile("traybins/NOTICES.txt")
		if err != nil {
			return nil, err
		}
		files = append(files, asset{trayDir + "/NOTICES.txt", notice, 0644})
	}
	present := map[string]bool{}
	for _, f := range files {
		if err := checkParents(root, f.name); err != nil {
			return nil, err
		}
		if info, err := root.Lstat(f.name); !errors.Is(err, fs.ErrNotExist) {
			if err != nil {
				return nil, err
			}
			if info.Mode().IsRegular() && info.Size() == int64(len(f.data)) {
				previous, err := root.ReadFile(f.name)
				if err != nil {
					return nil, err
				}
				if bytes.Equal(previous, f.data) {
					present[f.name] = true
					continue
				}
			}
			return nil, fmt.Errorf("%s already exists. Move your existing file aside before adding web-launcher", f.name)
		}
	}
	var created []string
	complete := false
	defer func() {
		if !complete {
			for _, name := range slices.Backward(created) {
				_ = root.Remove(name)
			}
			if mac {
				cleanBundleDirs(root)
			}
		}
	}()
	var entries []manifest.RealizedEntry
	for _, f := range files {
		if !present[f.name] {
			if err := root.MkdirAll(filepath.Dir(f.name), 0755); err != nil {
				return nil, err
			}
			out, err := root.OpenFile(f.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.mode)
			if err != nil {
				return nil, err
			}
			created = append(created, f.name)
			_, writeErr := out.Write(f.data)
			closeErr := out.Close()
			if err := errors.Join(writeErr, closeErr); err != nil {
				return nil, err
			}
		}
		entries = append(entries, manifest.RealizedEntry{ID: id, Type: kind, RelativePath: f.name, Filename: filepath.Base(f.name), SizeBytes: int64(len(f.data)), ChecksumSHA256: fmt.Sprintf("%x", sha256.Sum256(f.data)), SourceStrategy: "build"})
	}
	complete = true
	return entries, nil
}

func checkParents(root *os.Root, name string) error {
	for p := filepath.Dir(name); p != "."; p = filepath.Dir(p) {
		info, err := root.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("launcher directory %s must be a real directory", p)
		}
	}
	return nil
}

// Remove deletes the exact recorded regular file. A replacement, a symlink or
// an extra file inside an app bundle remains the owner's.
func Remove(dir string, entry manifest.RealizedEntry) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if !filepath.IsLocal(entry.RelativePath) {
		return fmt.Errorf("invalid launcher path %q", entry.RelativePath)
	}
	if err := checkParents(root, entry.RelativePath); err != nil {
		return err
	}
	info, err := root.Lstat(entry.RelativePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != entry.SizeBytes {
		return nil
	}
	data, err := root.ReadFile(entry.RelativePath)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != entry.ChecksumSHA256 {
		return nil
	}
	if err := root.Remove(entry.RelativePath); err != nil {
		return err
	}
	if strings.HasPrefix(entry.RelativePath, "Svalbard.app/") {
		return cleanBundleDirs(root)
	}
	return nil
}

func cleanBundleDirs(root *os.Root) error {
	for _, d := range []string{"Svalbard.app/Contents/MacOS", "Svalbard.app/Contents/Resources", "Svalbard.app/Contents", "Svalbard.app"} {
		if err := checkParents(root, d); err != nil {
			continue
		}
		info, err := root.Lstat(d)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			continue
		}
		err = root.Remove(d)
		if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			return err
		}
	}
	return nil
}

// Fingerprint is the content identity of the host's bundled recipe inputs.
// Include it in acquisition identity so a host upgrade can offer the updated
// launcher using the ordinary recipe update path.
var Fingerprint = sync.OnceValue(func() string {
	h := sha256.New()
	err := fs.WalkDir(payload, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := payload.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%x\n", path, sha256.Sum256(data))
		return nil
	})
	if err != nil {
		panic(err)
	} // compiled embed.FS only
	return fmt.Sprintf("%x", h.Sum(nil))
})

// CheckUpdate rejects files replaced by the owner before apply sets installed
// files aside. In particular a user-created directory can never be recursively
// removed as though it were one of the previous package's regular files.
func CheckUpdate(dir string, entries []manifest.RealizedEntry) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, entry := range entries {
		if !filepath.IsLocal(entry.RelativePath) {
			return fmt.Errorf("invalid launcher path %q", entry.RelativePath)
		}
		if err := checkParents(root, entry.RelativePath); err != nil {
			return err
		}
		info, err := root.Lstat(entry.RelativePath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Size() == entry.SizeBytes {
			data, err := root.ReadFile(entry.RelativePath)
			if err != nil {
				return err
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) == entry.ChecksumSHA256 {
				continue
			}
		}
		return fmt.Errorf("move your replacement for %s aside before updating web-launcher", entry.RelativePath)
	}
	return nil
}

func selectedOS(platforms []string) (mac, linux bool) {
	if len(platforms) == 0 {
		return runtime.GOOS == "darwin", runtime.GOOS == "linux"
	}
	for _, platform := range platforms {
		osName, _, _ := strings.Cut(platform, "-")
		mac = mac || osName == "macos" || osName == "darwin"
		linux = linux || osName == "linux"
	}
	return mac, linux
}
func includePath(name string, mac, linux bool) bool {
	switch {
	case strings.HasPrefix(name, "Svalbard.app/"):
		return mac
	case name == "Svalbard-Linux" || name == "Svalbard.desktop":
		return linux
	default:
		return true
	}
}

const trayDir = ".svalbard/tray"

func builtTrayFiles(platforms []string) ([]asset, error) {
	if len(platforms) == 0 {
		osName := runtime.GOOS
		if osName == "darwin" {
			osName = "macos"
		}
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "x86_64"
		}
		platforms = []string{osName + "-" + arch}
	}
	var files []asset
	for _, platform := range platforms {
		rel := platform + "/svalbard-tray"
		data, err := payload.ReadFile("traybins/" + rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files = append(files, asset{trayDir + "/" + rel, data, 0755})
	}
	return files, nil
}

// MatchesPlatforms lets the planner reconcile a later change in host targets.
func MatchesPlatforms(entries []manifest.RealizedEntry, platforms []string) bool {
	mac, linux := selectedOS(platforms)
	hasMac, hasLinux := false, false
	for _, entry := range entries {
		hasMac = hasMac || strings.HasPrefix(entry.RelativePath, "Svalbard.app/")
		hasLinux = hasLinux || entry.RelativePath == "Svalbard-Linux"
	}
	if mac != hasMac || linux != hasLinux {
		return false
	}
	want, err := builtTrayFiles(platforms)
	if err != nil {
		return false
	}
	var have []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.RelativePath, trayDir+"/") && strings.HasSuffix(entry.RelativePath, "/svalbard-tray") {
			have = append(have, entry.RelativePath)
		}
	}
	return len(have) == len(want) && !slices.ContainsFunc(want, func(f asset) bool { return !slices.Contains(have, f.name) })
}

// CleanBundle removes empty app directories after an update drops macOS.
func CleanBundle(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	return cleanBundleDirs(root)
}
