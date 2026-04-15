package runtimebinary_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebinary"
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

	got, err := runtimebinary.Resolve("kiwix-serve", driveRoot, func() (string, error) {
		return "macos-arm64", nil
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := filepath.Join(binDir, "kiwix-serve"); got != want {
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
