package binarytree

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	flag             byte
}

func writeTarGz(t *testing.T, path string, entries []entry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		flag := e.flag
		if flag == 0 {
			flag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: flag, Linkname: e.link, Mode: 0o755}
		if flag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
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
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubImports(t *testing.T, imports map[string][]string) {
	t.Helper()
	saved := importedLibraries
	importedLibraries = func(file string) []string { return imports[filepath.Base(file)] }
	t.Cleanup(func() { importedLibraries = saved })
}

func assertFile(t *testing.T, path, content string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %v, want a plain file", path, info.Mode())
	}
	if data, _ := os.ReadFile(path); string(data) != content {
		t.Fatalf("%s holds %q, want %q", path, data, content)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists (%v), want it left out", path, err)
	}
}

// The shape of llama-server's archive: each library is a chain
// libX.so -> libX.so.1 -> libX.so.1.0.0, and programs load libX.so.1.
func TestPrepareResolvesLibraryChainsToTheLoadedName(t *testing.T) {
	stubImports(t, map[string][]string{
		"llama-server":      {"libllama.so.1", "libc.so.6"},
		"libllama.so.1.0.0": {"libggml.so.1"},
	})
	dir := t.TempDir()
	archive := filepath.Join(dir, "llama-b1-bin-ubuntu-arm64.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "llama-b1/", flag: tar.TypeDir},
		{name: "llama-b1/libllama.so", link: "libllama.so.1", flag: tar.TypeSymlink},
		{name: "llama-b1/libllama.so.1", link: "libllama.so.1.0.0", flag: tar.TypeSymlink},
		{name: "llama-b1/libllama.so.1.0.0", body: "llama"},
		{name: "llama-b1/libggml.so.1", link: "libggml.so.1.0.0", flag: tar.TypeSymlink},
		{name: "llama-b1/libggml.so.1.0.0", body: "ggml"},
		{name: "llama-b1/libggml-cpu-armv8.0_1.so", body: "backend, opened by path"},
		{name: "llama-b1/llama-server", body: "server"},
		{name: "llama-b1/llama", link: "llama-server", flag: tar.TypeSymlink},
		{name: "llama-b1/outside", link: "../../etc/passwd", flag: tar.TypeSymlink},
	})
	report, err := Prepare(archive)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(UnpackedDir(archive), "llama-b1")
	assertFile(t, filepath.Join(tree, "libllama.so.1"), "llama")
	assertFile(t, filepath.Join(tree, "libggml.so.1"), "ggml")
	assertMissing(t, filepath.Join(tree, "libllama.so.1.0.0"))
	assertMissing(t, filepath.Join(tree, "libggml.so.1.0.0"))
	assertMissing(t, filepath.Join(tree, "libllama.so"))
	assertMissing(t, filepath.Join(tree, "outside"))
	assertFile(t, filepath.Join(tree, "libggml-cpu-armv8.0_1.so"), "backend, opened by path")
	assertFile(t, filepath.Join(tree, "llama"), "server")
	assertFile(t, filepath.Join(tree, "llama-server"), "server")
	if len(report) != 5 {
		t.Fatalf("report = %q, want one line per link", report)
	}
	partials, _ := filepath.Glob(filepath.Join(dir, "*"+partialMarker+"*"))
	if len(partials) != 0 {
		t.Fatalf("partial directories left: %v", partials)
	}
}

// Without a program to read, nothing is known to be unneeded: every link is
// a copy.
func TestPrepareCopiesEveryLinkWhenNothingLoadsLibraries(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "lib/libz.so", link: "libz.so.1", flag: tar.TypeSymlink},
		{name: "lib/libz.so.1", body: "z"},
		{name: "share", link: "lib", flag: tar.TypeSymlink},
		{name: "bin/hard", link: "lib/libz.so.1", flag: tar.TypeLink},
	})
	if _, err := Prepare(archive); err != nil {
		t.Fatal(err)
	}
	root := UnpackedDir(archive)
	assertFile(t, filepath.Join(root, "lib", "libz.so"), "z")
	assertFile(t, filepath.Join(root, "lib", "libz.so.1"), "z")
	assertFile(t, filepath.Join(root, "share", "libz.so"), "z")
	assertFile(t, filepath.Join(root, "bin", "hard"), "z")
}

// An archive without links is left to the drive runtime, and a tree from the
// archive this one replaced (same name, older release) is removed.
func TestPrepareLeavesArchiveWithoutLinksAndDropsOldTree(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "duckdb-cli-osx-arm64.tar.gz")
	writeTarGz(t, archive, []entry{{name: "duckdb", body: "new"}})
	old := filepath.Join(UnpackedDir(archive), "duckdb")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(archive); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, UnpackedDir(archive))
}

func TestImportedLibrariesReadsMachO(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs a Mach-O program")
	}
	libs := importedLibraries("/bin/ls")
	if !slices.ContainsFunc(libs, func(lib string) bool { return filepath.Base(lib) == "libSystem.B.dylib" }) {
		t.Fatalf("importedLibraries(/bin/ls) = %v, want libSystem.B.dylib", libs)
	}
	if libs := importedLibraries("/etc/hosts"); len(libs) != 0 {
		t.Fatalf("importedLibraries(/etc/hosts) = %v, want none", libs)
	}
}

func TestRemoveTreesTakesTheArchivesDirectoriesOnly(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ffuf_2.3.0_linux_arm64.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "./ffuf", body: "bin"},
		{name: "LICENSE", body: "shared name"},
		{name: "doc/README.md", body: "readme"},
	})
	for _, p := range []string{"ffuf", "LICENSE", "doc/README.md", "other-tool/x", UnpackedSuffix[1:] + "/x"} {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte("x"), 0o755)
	}
	os.MkdirAll(filepath.Join(UnpackedDir(archive), "x"), 0o755)
	if err := RemoveTrees(archive); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(dir, "doc"))
	assertMissing(t, UnpackedDir(archive))
	for _, kept := range []string{"ffuf", "LICENSE", "other-tool/x", archive} {
		if _, err := os.Stat(filepath.Join(dir, strings.TrimPrefix(kept, dir))); err != nil {
			t.Errorf("%s removed: %v", kept, err)
		}
	}
}

func TestPrepareMakesATreeOthersCanRead(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "bin/tool-1", body: "x"},
		{name: "bin/tool", link: "tool-1", flag: tar.TypeSymlink},
	})
	if _, err := Prepare(archive); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(UnpackedDir(archive)); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("unpacked dir = %v, %v; want mode 0755", info, err)
	}
}

func TestRemoveTreesTakesStoppedUnpacks(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{{name: "tool", body: "x"}})
	partial := archive + partialMarker + "123"
	if err := os.MkdirAll(filepath.Join(partial, "half"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTrees(archive); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, partial)
}
