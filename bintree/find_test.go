package bintree

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProgram(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The tree in <archive>.unpacked wins over the flat tree an older runtime
// left beside the archive, which sorts first, and over bin/<platform>/<name>/.
func TestFindPrefersTheUnpackedTree(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "linux-arm64")
	os.MkdirAll(bin, 0o755)
	llama := filepath.Join(bin, "llama-b12359-bin-ubuntu-arm64.tar.gz")
	writeTarGz(t, llama, []entry{{name: "llama-b12359/llama-server", body: "archive"}})
	writeProgram(t, filepath.Join(bin, "llama-b12359", "llama-server"), "legacy")
	writeProgram(t, filepath.Join(UnpackedDir(llama), "llama-b12359", "llama-server"), "current")
	pi := filepath.Join(bin, "pi-linux-arm64.tar.gz")
	writeTarGz(t, pi, []entry{{name: "pi/pi", body: "archive"}})
	writeProgram(t, filepath.Join(bin, "pi", "pi"), "legacy")
	writeProgram(t, filepath.Join(UnpackedDir(pi), "pi", "pi"), "current")

	for _, name := range []string{"llama-server", "pi"} {
		got, err := Find(name, root, "linux-arm64")
		if err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(got); string(data) != "current" {
			t.Errorf("Find(%s) = %s (%s), want the unpacked tree", name, got, data)
		}
	}
}

// An archive that cannot be unpacked, and a directory that cannot be read,
// do not hide the other tools beside them.
func TestFindGetsPastABadArchiveAndAnUnreadableDirectory(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "linux-arm64")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "aaa-broken.tar.gz"), []byte("not gzip"), 0o644)
	writeTarGz(t, filepath.Join(bin, "zzz-tool.tar.gz"), []entry{{name: "tool/tool", body: "t"}})
	if os.Getuid() != 0 {
		locked := filepath.Join(bin, "locked")
		os.MkdirAll(locked, 0o755)
		os.Chmod(locked, 0)
		t.Cleanup(func() { os.Chmod(locked, 0o755) })
	}
	got, err := Find("tool", root, "linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(bin, "zzz-tool.tar.gz.unpacked", "tool", "tool"); got != want {
		t.Fatalf("Find = %s, want %s", got, want)
	}
}
