package binary_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
)

type tarEntry struct {
	name, body, link string
	flag             byte
}

func buildTarGzEntries(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		flag := e.flag
		if flag == 0 {
			flag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: flag, Linkname: e.link, Mode: 0o755, Size: int64(len(e.body))}
		if flag != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if flag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertPlainFile(t *testing.T, path, content string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %v, want a plain file", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("%s holds %q, want %q", path, data, content)
	}
}

// llama-server's archive is a chain of shared-library links. On exFAT none of
// them can be a link, so each becomes a copy; a link out of the archive or to
// nothing is left out, and nothing in the tree is a symlink.
func TestResolveUnpacksLinksAsPlainFiles(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", currentPlatform(t))
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(binDir, "llama-b1-bin.tar.gz")
	if err := os.WriteFile(archive, buildTarGzEntries(t, []tarEntry{
		{name: "llama-b1/", flag: tar.TypeDir},
		{name: "llama-b1/libllama.so", link: "libllama.so.1", flag: tar.TypeSymlink}, // before its target
		{name: "llama-b1/libllama.so.1", link: "libllama.so.1.0.0", flag: tar.TypeSymlink},
		{name: "llama-b1/libllama.so.1.0.0", body: "lib"},
		{name: "llama-b1/llama-server", body: "#!/bin/sh\nexit 0\n"},
		{name: "llama-b1/llama-hard", link: "llama-b1/llama-server", flag: tar.TypeLink},
		{name: "llama-b1/share/data.txt", body: "data"},
		{name: "llama-b1/data", link: "share", flag: tar.TypeSymlink},
		{name: "llama-b1/outside", link: "../../../etc/passwd", flag: tar.TypeSymlink},
		{name: "llama-b1/absolute", link: "/etc/passwd", flag: tar.TypeSymlink},
		{name: "llama-b1/dangling", link: "missing", flag: tar.TypeSymlink},
		{name: "../escape.txt", body: "no"},
	}), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := binary.Resolve("llama-server", driveRoot)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	tree := filepath.Join(archive+".unpacked", "llama-b1")
	if want := filepath.Join(tree, "llama-server"); got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
	assertPlainFile(t, filepath.Join(tree, "libllama.so"), "lib")
	assertPlainFile(t, filepath.Join(tree, "libllama.so.1"), "lib")
	assertPlainFile(t, filepath.Join(tree, "llama-hard"), "#!/bin/sh\nexit 0\n")
	assertPlainFile(t, filepath.Join(tree, "data", "data.txt"), "data")
	for _, name := range []string{"outside", "absolute", "dangling"} {
		if _, err := os.Lstat(filepath.Join(tree, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was written: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(binDir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("an entry outside the archive was written: %v", err)
	}
	_ = filepath.WalkDir(binDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Type()&os.ModeSymlink != 0 {
			t.Errorf("%s is a symlink", path)
		}
		return nil
	})
}

func TestResolveUnpacksZipSymlinkAsCopy(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", currentPlatform(t))
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string, mode os.FileMode) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("tool/tool-1.0", "#!/bin/sh\nexit 0\n", 0o755)
	add("tool/tool", "tool-1.0", os.ModeSymlink|0o777)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(binDir, "tool.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := binary.Resolve("tool", driveRoot)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	assertPlainFile(t, got, "#!/bin/sh\nexit 0\n")
}

// A Linux run on a stick whose Linux archive cannot be unpacked used to walk
// all of bin/ and start the macOS llama-server ("exec format error").
func TestResolveNeverFindsAnotherPlatformsBinary(t *testing.T) {
	driveRoot := t.TempDir()
	other := "linux-x86_64"
	if currentPlatform(t) == other {
		other = "macos-arm64"
	}
	otherTree := filepath.Join(driveRoot, "bin", other, "llama.tar.gz.unpacked", "llama")
	if err := os.MkdirAll(otherTree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherTree, "llama-server"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(driveRoot, "bin", currentPlatform(t)), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := binary.ResolveOnDrive("llama-server", driveRoot); err == nil {
		t.Fatalf("ResolveOnDrive() = %q, want not found", got)
	}
}

// A bare .gz is written under the name asked for, so another tool's .gz in
// the same directory must not be unpacked as kiwix-serve.
func TestResolveUnpacksOnlyTheAskedToolsBareGz(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", currentPlatform(t))
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("#!/bin/sh\necho chisel\n"))
	gz.Close()
	if err := os.WriteFile(filepath.Join(binDir, "chisel_1.12.0_linux_arm64.gz"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := binary.ResolveOnDrive("kiwix-serve", driveRoot); err == nil {
		t.Fatalf("ResolveOnDrive(kiwix-serve) = %q, want not found", got)
	}
	if _, err := os.Stat(filepath.Join(binDir, "kiwix-serve")); !os.IsNotExist(err) {
		t.Fatalf("chisel was written as kiwix-serve: %v", err)
	}
}

// An unpack that stopped halfway leaves a .partial- directory; nothing in it
// resolves, and the next run unpacks again.
func TestResolveIgnoresPartialUnpack(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", currentPlatform(t))
	partial := filepath.Join(binDir, "kiwix-tools.tar.gz.partial-123", "kiwix-tools")
	if err := os.MkdirAll(partial, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partial, "kiwix-serve"), []byte("half"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "kiwix-tools.tar.gz"), buildTarGz(t, map[string]string{
		"kiwix-tools/kiwix-serve": "#!/bin/sh\nexit 0\n",
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := binary.Resolve("kiwix-serve", driveRoot)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := filepath.Join(binDir, "kiwix-tools.tar.gz.unpacked", "kiwix-tools", "kiwix-serve"); got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

// macOS writes a ._ file beside each file it creates on exFAT; the unpack
// removes those beside its own files.
func TestUnpackRemovesAppleDoubleCompanions(t *testing.T) {
	driveRoot := t.TempDir()
	binDir := filepath.Join(driveRoot, "bin", currentPlatform(t))
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "kiwix-tools.tar.gz"), buildTarGzEntries(t, []tarEntry{
		{name: "kiwix-tools/kiwix-serve", body: "#!/bin/sh\nexit 0\n"},
		{name: "kiwix-tools/._kiwix-serve", body: "\x00\x05\x16\x07"},
		{name: "kiwix-tools/._lonely", body: "kept: no file beside it"},
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := binary.Resolve("kiwix-serve", driveRoot)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(got), "._kiwix-serve")); !os.IsNotExist(err) {
		t.Fatalf("._kiwix-serve kept: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(got), "._lonely")); err != nil {
		t.Fatalf("._lonely removed: %v", err)
	}
}
