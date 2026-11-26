package builder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type testTarEntry struct {
	name string
	link string // symlink target
	data []byte
	mode int64
	dir  bool
}

func writeTestTar(file string, entries []testTarEntry) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.data)), Typeflag: tar.TypeReg}
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		switch {
		case e.dir:
			h.Typeflag, h.Mode = tar.TypeDir, 0o755
		case e.link != "":
			h.Typeflag, h.Linkname, h.Mode = tar.TypeSymlink, e.link, 0o777
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write(e.data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0o644)
}

// tarTestDir archives a directory as the container's install script does.
func tarTestDir(dir, file string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = "./" + filepath.ToSlash(rel)
		if err := tw.WriteHeader(h); err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0o644)
}

// extractTestTar unpacks regular files and directories, and fails on anything
// a vault filesystem may not hold.
func extractTestTar(archive, dest string) error {
	return walkTar(archive, func(name string, h *tar.Header, r io.Reader) error {
		target := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			return os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, fs.FileMode(h.Mode))
		}
		return fmt.Errorf("%s: entry type %c in a rewritten interpreter archive", name, h.Typeflag)
	})
}

func TestRewriteInterpreterArchive(t *testing.T) {
	dir := t.TempDir()
	raw, clean := filepath.Join(dir, "raw.tar"), filepath.Join(dir, "clean.tar")
	big := bytes.Repeat([]byte{1}, treeLinkCopyLimit+1)
	p := "./cpython-3.12.16-linux-aarch64-gnu/"
	if err := writeTestTar(raw, []testTarEntry{
		{name: "./", dir: true},
		{name: "./.gitignore", data: []byte("*")},
		{name: "./.lock"},
		{name: "./.temp/", dir: true},
		{name: "./cpython-3.12-linux-aarch64-gnu", link: "/tmp/tmp.x/cpython-3.12.16-linux-aarch64-gnu"},
		{name: p, dir: true},
		{name: p + "bin/", dir: true},
		{name: p + "bin/python3", link: "python3.12"},
		{name: p + "bin/python3.12", data: big, mode: 0o755},
		{name: p + "bin/python", link: "python3"},
		{name: p + "bin/pydoc3.12", data: []byte("#!/bin/sh\npydoc\n"), mode: 0o755},
		{name: p + "bin/pydoc3", link: "pydoc3.12"},
		{name: p + "bin/escape", link: "../../../etc/passwd"},
		{name: p + "lib/", dir: true},
		{name: p + "lib/libpython3.12.so.1.0", data: big, mode: 0o755},
		{name: p + "lib/libpython3.12.so", link: "libpython3.12.so.1.0"},
		{name: p + "lib/python3.12/", dir: true},
		{name: p + "lib/libdir", link: "python3.12"},
		{name: p + "share/terminfo/h/hp2621", data: []byte("ti")},
		{name: p + "share/terminfo/2/2621A", link: "../h/hp2621"},
		{name: p + "share/terminfo/2/2621a", link: "../h/hp2621"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := rewriteTreeArchive(raw, clean, skipInterpreterEntry); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	var names []string
	err := walkTar(clean, func(name string, h *tar.Header, r io.Reader) error {
		data, _ := io.ReadAll(r)
		names = append(names, name)
		got[name] = fmt.Sprintf("%c %o %s", h.Typeflag, h.Mode, data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	q := strings.TrimPrefix(p, "./")
	want := []string{
		q[:len(q)-1], q + "bin", q + "bin/python3.12", q + "bin/pydoc3.12", q + "lib", q + "lib/libpython3.12.so.1.0", q + "lib/python3.12",
		q + "bin/pydoc3", q + "bin/python", q + "bin/python3",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("entries =\n%v\nwant\n%v", names, want)
	}
	if want := "0 755 #!/bin/sh\npydoc\n"; got[q+"bin/pydoc3"] != want {
		t.Errorf("pydoc3 = %q, want a copy %q", got[q+"bin/pydoc3"], want)
	}
	for _, name := range []string{"python", "python3"} {
		if want := "0 755 #!/bin/sh\nexec \"$(dirname -- \"$0\")/python3.12\" \"$@\"\n"; got[q+"bin/"+name] != want {
			t.Errorf("%s = %q, want a script that runs python3.12", name, got[q+"bin/"+name])
		}
	}
}

func TestRewriteInterpreterArchiveRejectsCaseOnlyDifferences(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.tar")
	if err := writeTestTar(raw, []testTarEntry{
		{name: "./cpython-3.12.16-x/lib/Foo.py"},
		{name: "./cpython-3.12.16-x/lib/foo.py"},
	}); err != nil {
		t.Fatal(err)
	}
	err := rewriteTreeArchive(raw, filepath.Join(dir, "clean.tar"), nil)
	if err == nil || !strings.Contains(err.Error(), "differ only in case") {
		t.Fatalf("err = %v, want a case collision", err)
	}
}

func TestBuildPythonVenvKeepsBuildFilesOffTheVault(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	calls := fakeUV(t, nil)
	for _, legacy := range []string{".staging/uv-cache/archive-v0", ".staging/build/svalbard-python"} {
		if err := os.MkdirAll(filepath.Join(root, legacy), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}); err != nil {
		t.Fatal(err)
	}

	// The container's /work is the build dir.
	buildDir, _ := pythonBuildDir()
	if info, err := os.Stat(filepath.Join(buildDir, "uv-cache")); err != nil || !info.IsDir() {
		t.Errorf("no uv cache dir in the build dir: %v", err)
	}
	for _, call := range *calls {
		if at := slices.Index(call, "--cache-dir"); at >= 0 {
			if call[at+1] != "/work/uv-cache" {
				t.Errorf("uv cache = %s, want /work/uv-cache", call[at+1])
			}
			if !strings.Contains(strings.Join(call, " "), "--link-mode copy") {
				t.Errorf("pip install without --link-mode copy: %v", call)
			}
		}
	}
	for _, legacy := range []string{".staging/uv-cache", ".staging/build/svalbard-python"} {
		if _, err := os.Stat(filepath.Join(root, legacy)); !os.IsNotExist(err) {
			t.Errorf("%s left on the vault: %v", legacy, err)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(buildDir, "*.tar")); len(leftovers) > 0 {
		t.Errorf("interpreter archives left in the build dir: %v", leftovers)
	}

	interpreter := filepath.Join(root, "runtime", "python", "linux-arm64", "python")
	var tree []string
	err := filepath.WalkDir(interpreter, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			t.Errorf("symlink on the vault: %s", path)
		}
		rel, _ := filepath.Rel(interpreter, path)
		tree = append(tree, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "cpython-3.12.14-linux-aarch64-gnu", "cpython-3.12.14-linux-aarch64-gnu/bin", "cpython-3.12.14-linux-aarch64-gnu/bin/python3", "cpython-3.12.14-linux-aarch64-gnu/bin/python3.12"}
	if !slices.Equal(tree, want) {
		t.Errorf("interpreter tree = %v, want %v", tree, want)
	}
	if _, err := os.Stat(interpreter + ".partial"); !os.IsNotExist(err) {
		t.Errorf("staging dir left: %v", err)
	}
}
