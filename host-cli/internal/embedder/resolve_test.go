package embedder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarEntry struct {
	name, body, link string
	flag             byte
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
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
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func platformBin(t *testing.T) (string, string) {
	t.Helper()
	platform, err := detectPlatform()
	if err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin", platform)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, bin
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Semantic indexing on the host finds llama-server where the drive runtime
// does: in the tree apply unpacked, not in an older flat tree, and it writes
// nothing into bin/<platform>/: no flat tree, no chisel saved as llama-server.
func TestResolveBinaryUsesTheUnpackedTreeAndWritesNothing(t *testing.T) {
	root, bin := platformBin(t)
	archive := filepath.Join(bin, "llama-b1-bin.tar.gz")
	writeTarGz(t, archive, []tarEntry{{name: "llama-b1/llama-server", body: "archive"}})
	current := filepath.Join(archive+".unpacked", "llama-b1", "llama-server")
	os.MkdirAll(filepath.Dir(current), 0o755)
	os.WriteFile(current, []byte("current"), 0o755)
	legacy := filepath.Join(bin, "llama-b1", "llama-server")
	os.MkdirAll(filepath.Dir(legacy), 0o755)
	os.WriteFile(legacy, []byte("legacy"), 0o755)
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write([]byte("chisel"))
	w.Close()
	os.WriteFile(filepath.Join(bin, "chisel_1.12.0_linux_arm64.gz"), gz.Bytes(), 0o644)
	before := names(t, bin)

	got, err := resolveBinary("llama-server", root)
	if err != nil {
		t.Fatal(err)
	}
	if got != current {
		t.Fatalf("resolveBinary = %s, want %s", got, current)
	}
	if after := names(t, bin); len(after) != len(before) {
		t.Fatalf("bin/<platform> changed from %v to %v", before, after)
	}
}

// An archive with no tree yet is unpacked into <archive>.unpacked, links as
// copies, and an entry that climbs out with ../ is left out.
func TestResolveBinaryUnpacksIntoTheArchivesOwnTree(t *testing.T) {
	root, bin := platformBin(t)
	archive := filepath.Join(bin, "llama-b2-bin.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "../escaped", body: "no"},
		{name: "llama-b2/../../escaped-too", body: "no"},
		{name: "llama-b2/libllama.so.1.0.0", body: "lib"},
		{name: "llama-b2/libllama.so.1", link: "libllama.so.1.0.0", flag: tar.TypeSymlink},
		{name: "llama-b2/llama-server", body: "server"},
	})

	got, err := resolveBinary("llama-server", root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(archive+".unpacked", "llama-b2", "llama-server"); got != want {
		t.Fatalf("resolveBinary = %s, want %s", got, want)
	}
	for _, escaped := range []string{filepath.Join(root, "bin", "escaped"), filepath.Join(bin, "escaped"), filepath.Join(bin, "escaped-too")} {
		if _, err := os.Lstat(escaped); !os.IsNotExist(err) {
			t.Errorf("%s written: %v", escaped, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(bin, "llama-b2")); !os.IsNotExist(err) {
		t.Errorf("a flat tree was unpacked into bin/<platform>: %v", err)
	}
	info, err := os.Lstat(filepath.Join(archive+".unpacked", "llama-b2", "libllama.so.1"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("libllama.so.1 = %v, %v; want a plain file", info, err)
	}
}
