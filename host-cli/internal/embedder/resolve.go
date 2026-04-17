package embedder

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveBinary locates a named binary on the drive, extracting archives if
// needed, then falls back to system PATH.  This mirrors the logic in
// drive-runtime/internal/binary.Resolve so that host-cli can find drive
// binaries without importing the drive-runtime module.
func resolveBinary(name, driveRoot string) (string, error) {
	if driveRoot != "" {
		platform, err := detectPlatform()
		if err == nil {
			dirs := []string{
				filepath.Join(driveRoot, "bin", platform, name),
				filepath.Join(driveRoot, "bin", platform),
				filepath.Join(driveRoot, "bin", name),
				filepath.Join(driveRoot, "bin"),
			}
			for _, dir := range dirs {
				if path, err := resolveFromDir(name, dir); err == nil {
					return path, nil
				}
			}
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("embedder: %s not found in drive or PATH", name)
}

func detectPlatform() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "macos-arm64", nil
	case "darwin/amd64":
		return "macos-x86_64", nil
	case "linux/arm64":
		return "linux-arm64", nil
	case "linux/amd64":
		return "linux-x86_64", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func resolveFromDir(name, dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	extracted := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		ename := entry.Name()
		switch {
		case strings.HasSuffix(ename, ".tar.gz"), strings.HasSuffix(ename, ".tgz"):
			if err := extractTarGz(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(ename, ".tar.bz2"):
			if err := extractTarBz2(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(ename, ".zip"):
			if err := extractZip(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(ename, ".gz"):
			if err := extractBareGz(path, dir, name); err != nil {
				return "", err
			}
			extracted = true
		}
	}

	if extracted {
		if found, err := findMatchingBinary(dir, name); err == nil {
			return found, nil
		}
	}
	return findMatchingBinary(dir, name)
}

func findMatchingBinary(dir, name string) (string, error) {
	var direct, nested string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != name {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Mode()&0o111 == 0 {
			return nil
		}
		if filepath.Dir(path) == dir {
			direct = path
			return nil
		}
		nested = path
		return io.EOF
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	found := nested
	if found == "" {
		found = direct
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}
	_ = os.Chmod(found, 0o755)
	return found, nil
}

// --- archive extraction ---

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	return extractTar(tar.NewReader(gzr), destDir)
}

func extractTarBz2(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTar(tar.NewReader(bzip2.NewReader(f)), destDir)
}

func extractTar(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode)|0o111)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Remove(target)
			_ = os.Symlink(hdr.Linkname, target)
		}
	}
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.Clean(f.Name))
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		src, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode()|0o111)
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(out, src)
		src.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractBareGz(archivePath, destDir, toolName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	out, err := os.OpenFile(filepath.Join(destDir, toolName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, gzr)
	out.Close()
	return err
}
