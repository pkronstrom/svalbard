package binary_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
)

func TestResolveExtractsTarGzBinaryFromPlatformBinDir(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", "macos-arm64")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	archivePath := filepath.Join(binDir, "kiwix-tools.tar.gz")
	if err := os.WriteFile(archivePath, buildTarGz(t, map[string]string{
		"kiwix-tools/kiwix-serve": "#!/bin/sh\nexit 0\n",
	}), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := binary.Resolve("kiwix-serve", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := filepath.Join(binDir, "kiwix-tools", "kiwix-serve"); got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("resolved binary mode = %v, want executable", info.Mode())
	}
}

func TestResolvePrefersToolSpecificPlatformDir(t *testing.T) {
	driveRoot := t.TempDir()
	toolDir := filepath.Join(driveRoot, "bin", "macos-arm64", "kiwix-serve")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	binaryPath := filepath.Join(toolDir, "kiwix-serve")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := binary.Resolve("kiwix-serve", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != binaryPath {
		t.Fatalf("Resolve() = %q, want %q", got, binaryPath)
	}
}

func TestResolveExtractsTarBz2BinaryFromPlatformBinDir(t *testing.T) {
	driveRoot := t.TempDir()
	toolDir := filepath.Join(driveRoot, "bin", "macos-arm64", "goose")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	archivePath := filepath.Join(toolDir, "goose.tar.bz2")
	if err := os.WriteFile(archivePath, buildTarBz2(t, map[string]string{
		"goose-bundle/goose": "#!/bin/sh\nexit 0\n",
	}), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := binary.Resolve("goose", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := filepath.Join(toolDir, "goose-bundle", "goose"); got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveKeepsExtractedBinaryInPlaceWhenNestedLibrariesMayExist(t *testing.T) {
	driveRoot := t.TempDir()
	toolDir := filepath.Join(driveRoot, "bin", "macos-arm64", "llama-server")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	archivePath := filepath.Join(toolDir, "llama-server.tar.gz")
	if err := os.WriteFile(archivePath, buildTarGz(t, map[string]string{
		"bundle/bin/llama-server":        "#!/bin/sh\nexit 0\n",
		"bundle/lib/libmtmd.0.dylib":     "fake",
		"bundle/lib/other-support.dylib": "fake",
	}), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := binary.Resolve("llama-server", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := filepath.Join(toolDir, "bundle", "bin", "llama-server")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(toolDir, "bundle", "lib", "libmtmd.0.dylib")); err != nil {
		t.Fatalf("support library missing after resolve: %v", err)
	}
}

func TestResolveExtractsTarSymlinkedLibraries(t *testing.T) {
	driveRoot := t.TempDir()
	toolDir := filepath.Join(driveRoot, "bin", "macos-arm64", "llama-server")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	archivePath := filepath.Join(toolDir, "llama-server.tar.gz")
	if err := os.WriteFile(archivePath, buildTarGzWithSymlinks(t,
		map[string]string{
			"llama-b8799/llama-server":      "#!/bin/sh\nexit 0\n",
			"llama-b8799/libmtmd.0.0.dylib": "real",
		},
		map[string]string{
			"llama-b8799/libmtmd.0.dylib": "libmtmd.0.0.dylib",
		},
	), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := binary.Resolve("llama-server", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := filepath.Join(toolDir, "llama-b8799", "llama-server")
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
	linkPath := filepath.Join(toolDir, "llama-b8799", "libmtmd.0.dylib")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != "libmtmd.0.0.dylib" {
		t.Fatalf("Readlink() = %q, want %q", target, "libmtmd.0.0.dylib")
	}
}

func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		body := []byte(content)
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close error = %v", err)
	}
	return buf.Bytes()
}

func buildTarGzWithSymlinks(t *testing.T, files map[string]string, symlinks map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		body := []byte(content)
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	for name, target := range symlinks {
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeSymlink,
			Linkname: target,
			Mode:     0o755,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(symlink %q) error = %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close error = %v", err)
	}
	return buf.Bytes()
}

func buildTarBz2(t *testing.T, files map[string]string) []byte {
	t.Helper()
	src := buildTar(t, files)
	cmd := exec.Command("bzip2", "-c")
	cmd.Stdin = bytes.NewReader(src)
	var dst bytes.Buffer
	cmd.Stdout = &dst
	cmd.Stderr = &dst
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("bzip2 command not available")
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("bzip2 run error = %v (%s)", err, dst.String())
	}
	return dst.Bytes()
}

func TestResolveBareGzExtraction(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", "linux-x86_64", "chisel")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a bare .gz file containing a fake binary.
	content := []byte("#!/bin/sh\necho chisel")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(content); err != nil {
		t.Fatalf("gzip write error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close error = %v", err)
	}

	gzPath := filepath.Join(binDir, "chisel_1.0_linux_amd64.gz")
	if err := os.WriteFile(gzPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := binary.Resolve("chisel", driveRoot, func() (string, error) {
		return "linux-x86_64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify extracted file exists and has correct content.
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("extracted content = %q, want %q", data, content)
	}

	// Verify it's executable.
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("extracted file is not executable")
	}
}

func buildTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		body := []byte(content)
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error = %v", err)
	}
	return buf.Bytes()
}
