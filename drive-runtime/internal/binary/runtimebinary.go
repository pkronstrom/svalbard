package binary

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
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func Resolve(name, driveRoot string, detectPlatform func() (string, error)) (string, error) {
	if detectPlatform == nil {
		detectPlatform = platform.Detect
	}

	platformName, err := detectPlatform()
	if err != nil {
		return "", err
	}
	for _, dir := range []string{
		filepath.Join(driveRoot, "bin", platformName, name),
		filepath.Join(driveRoot, "bin", platformName),
		filepath.Join(driveRoot, "bin", name),
		filepath.Join(driveRoot, "bin"),
	} {
		if path, err := resolveFromDir(name, dir); err == nil {
			return path, nil
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found", name)
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
		switch {
		case strings.HasSuffix(entry.Name(), ".tar.gz"), strings.HasSuffix(entry.Name(), ".tgz"):
			if err := extractTarGz(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(entry.Name(), ".tar.bz2"):
			if err := extractTarBz2(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(entry.Name(), ".zip"):
			if err := extractZip(path, dir); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(entry.Name(), ".gz"):
			if err := extractBareGz(path, dir, name); err != nil {
				return "", err
			}
			extracted = true
		default:
			continue
		}
	}
	if extracted {
		if found, err := findMatchingBinary(dir, name); err == nil {
			return found, nil
		}
	}
	if found, err := findMatchingBinary(dir, name); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("%s not found in %s", name, dir)
}

func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	return extractTarReader(tr, destDir)
}

func extractTarBz2(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	tr := tar.NewReader(bzip2.NewReader(file))
	return extractTarReader(tr, destDir)
}

func extractTarReader(tr *tar.Reader, destDir string) error {
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
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode)|0o111)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
}

func extractBareGz(archivePath, destDir, toolName string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Use the tool name so findMatchingBinary can locate it.
	outPath := filepath.Join(destDir, toolName)

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gzr); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target := filepath.Join(destDir, filepath.Clean(file.Name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode()|0o111)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			src.Close()
			out.Close()
			return err
		}
		src.Close()
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func findMatchingBinary(dir, name string) (string, error) {
	var direct string
	var nested string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != name || !isExecutable(path) {
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
		return "", fmt.Errorf("%s not found in extracted archive", name)
	}
	if err := os.Chmod(found, 0o755); err != nil {
		return "", err
	}
	return found, nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
