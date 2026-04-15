package runtimeembedded

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

var (
	platformSuffixPattern = regexp.MustCompile(`-(linux|darwin|windows)_.*$`)
	versionSuffixPattern  = regexp.MustCompile(`-[0-9].*$`)
)

func PackageName(filename string) string {
	name := filename
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.xz", ".zip"} {
		name = strings.TrimSuffix(name, suffix)
	}
	name = platformSuffixPattern.ReplaceAllString(name, "")
	name = versionSuffixPattern.ReplaceAllString(name, "")
	return name
}

func Environment(pioCache string) map[string]string {
	return map[string]string{
		"PLATFORMIO_CORE_DIR":  pioCache,
		"PLATFORMIO_BUILD_DIR": "/tmp/svalbard-pio-build",
	}
}

func Run(ctx context.Context, stdout io.Writer, driveRoot string) error {
	platformName, err := platform.Detect()
	if err != nil {
		return err
	}
	pioCache := os.Getenv("SVALBARD_PIO_CACHE")
	if pioCache == "" {
		pioCache = "/tmp/svalbard-pio"
	}
	pkgDir := filepath.Join(pioCache, "packages")
	drivePkg := filepath.Join(driveRoot, "tools", "platformio", "packages")
	driveLib := filepath.Join(driveRoot, "tools", "platformio", "lib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}

	if err := extractArchives(filepath.Join(drivePkg, platformName), pkgDir, stdout); err != nil {
		return err
	}
	if err := extractArchives(drivePkg, pkgDir, stdout); err != nil {
		return err
	}

	if info, err := os.Stat(driveLib); err == nil && info.IsDir() {
		_ = os.Remove(filepath.Join(pioCache, "lib"))
		_ = os.Symlink(driveLib, filepath.Join(pioCache, "lib"))
	}

	env := append(os.Environ(), envMapToList(Environment(pioCache))...)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Embedded dev shell ready.")
	fmt.Fprintf(stdout, "  Toolchains: %s\n", pkgDir)
	if _, err := os.Stat(driveLib); err == nil {
		fmt.Fprintf(stdout, "  Libraries:  %s\n", driveLib)
	}
	fmt.Fprintf(stdout, "  Build dir:  %s\n\n", Environment(pioCache)["PLATFORMIO_BUILD_DIR"])
	fmt.Fprintln(stdout, "  pio init --board esp32dev --project-option 'framework=espidf'")
	fmt.Fprintln(stdout, "  pio run")
	fmt.Fprintln(stdout, "  pio run -t upload")
	fmt.Fprintln(stdout, "  pio device monitor")
	fmt.Fprintln(stdout)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.CommandContext(ctx, shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}

func extractArchives(dir, destRoot string, stdout io.Writer) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}
		pkgName := PackageName(name)
		dest := filepath.Join(destRoot, pkgName)
		if _, err := os.Stat(filepath.Join(dest, ".extracted")); err == nil {
			continue
		}
		fmt.Fprintf(stdout, "  Extracting %s...\n", pkgName)
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		archivePath := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(name, ".tar.gz"):
			if err := extractTarGzInto(archivePath, dest); err != nil {
				return err
			}
		case strings.HasSuffix(name, ".zip"):
			if err := extractZipInto(archivePath, dest); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(dest, ".extracted"), []byte("ok"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func extractTarGzInto(archivePath, dest string) error {
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

	var topPrefix string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if topPrefix == "" {
			parts := strings.Split(name, string(filepath.Separator))
			if len(parts) > 1 {
				topPrefix = parts[0]
			}
		}
		if topPrefix != "" && strings.HasPrefix(name, topPrefix+string(filepath.Separator)) {
			name = strings.TrimPrefix(name, topPrefix+string(filepath.Separator))
		}
		if name == "" || name == "." {
			continue
		}
		target := filepath.Join(dest, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(hdr.Mode))
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
		}
	}
}

func extractZipInto(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	var topPrefix string
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if topPrefix == "" {
			parts := strings.Split(name, string(filepath.Separator))
			if len(parts) > 1 {
				topPrefix = parts[0]
			}
		}
		if topPrefix != "" && strings.HasPrefix(name, topPrefix+string(filepath.Separator)) {
			name = strings.TrimPrefix(name, topPrefix+string(filepath.Separator))
		}
		if name == "" || name == "." {
			continue
		}
		target := filepath.Join(dest, name)
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
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode())
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

func envMapToList(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}
