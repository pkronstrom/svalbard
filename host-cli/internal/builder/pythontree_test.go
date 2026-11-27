package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
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
	dropped, err := rewriteTreeArchive(raw, clean, skipInterpreterEntry)
	if err != nil {
		t.Fatal(err)
	}
	q := strings.TrimPrefix(p, "./")
	wantDropped := []string{
		"cpython-3.12-linux-aarch64-gnu (leads out of the tree)",
		q + "bin/escape (leads out of the tree)",
		fmt.Sprintf("%slib/libpython3.12.so (links a %d-byte file outside bin/)", q, len(big)),
		q + "lib/libdir (links a directory)",
	}
	if !slices.Equal(dropped, wantDropped) {
		t.Errorf("dropped =\n%v\nwant\n%v", dropped, wantDropped)
	}

	got := map[string]string{}
	var names []string
	err = walkTar(clean, func(name string, h *tar.Header, r io.Reader) error {
		data, _ := io.ReadAll(r)
		names = append(names, name)
		got[name] = fmt.Sprintf("%c %o %s", h.Typeflag, h.Mode, data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
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
		if want := "0 755 #!/bin/sh\nself=$(realpath -- \"$0\" 2>/dev/null) || self=$0\nexec \"$(dirname -- \"$self\")/python3.12\" \"$@\"\n"; got[q+"bin/"+name] != want {
			t.Errorf("%s = %q, want a script that runs python3.12", name, got[q+"bin/"+name])
		}
	}
}

func TestRewriteTreeArchiveRejectsNamesAVolumeHoldsAsOne(t *testing.T) {
	for _, pair := range [][2]string{
		{"Foo.py", "foo.py"},
		{"straße.py", "strasse.py"},       // full case folding
		{"caf\u00e9.py", "cafe\u0301.py"}, // NFC and NFD
		{"ſ.py", "s.py"},
	} {
		dir := t.TempDir()
		raw := filepath.Join(dir, "raw.tar")
		if err := writeTestTar(raw, []testTarEntry{
			{name: "./cpython-3.12.16-x/lib/" + pair[0]},
			{name: "./cpython-3.12.16-x/lib/" + pair[1]},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := rewriteTreeArchive(raw, filepath.Join(dir, "clean.tar"), nil)
		if err == nil || !strings.Contains(err.Error(), "differ only in case or Unicode normalization") {
			t.Errorf("%q and %q: err = %v, want a collision", pair[0], pair[1], err)
		}
	}
}

func TestWalkTarRejectsEntriesOutsideTheDirectory(t *testing.T) {
	for _, name := range []string{"../x", "./a/../../x", "/abs"} {
		raw := filepath.Join(t.TempDir(), "raw.tar")
		if err := writeTestTar(raw, []testTarEntry{{name: "./ok"}, {name: name}}); err != nil {
			t.Fatal(err)
		}
		_, err := rewriteTreeArchive(raw, raw+".clean", nil)
		if err == nil || !strings.Contains(err.Error(), "leaves its directory") {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
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
	for _, legacy := range []string{".staging/uv-cache", ".staging/build/svalbard-python", ".staging"} {
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

// hostPythonContainer runs the container scripts with this computer's python3
// in place of the tools container, with paths as they are here.
func hostPythonContainer(t *testing.T, failUnpack bool) *[][]string {
	t.Helper()
	var calls [][]string
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3 on this computer")
	}
	previous := runToolCommand
	runToolCommand = func(ctx context.Context, _, _, _, tool string, args []string) (string, error) {
		if tool != "python3" {
			t.Fatalf("tool = %s", tool)
		}
		calls = append(calls, args)
		out, err := exec.CommandContext(ctx, python, args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%w: %s", err, out)
		}
		if failUnpack && args[1] == containerUnpackScript {
			return "", errors.New("container died after writing")
		}
		return "", nil
	}
	t.Cleanup(func() { runToolCommand = previous })
	return &calls
}

func TestInstallTreeRunsTheContainerScripts(t *testing.T) {
	calls := hostPythonContainer(t, false)
	previousOwner := containerOwner
	containerOwner = func() []int { return []int{os.Getuid(), os.Getgid()} }
	t.Cleanup(func() { containerOwner = previousOwner })
	root, workdir := t.TempDir(), t.TempDir()
	dest := filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "pkg")
	for _, stale := range []string{dest, dest + ".partial"} {
		if err := os.MkdirAll(stale, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stale, "stale"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	commands := [][]string{{"sh", "-c", `mkdir -p "$0/bin" "$0/lib" && printf '#!/bin/sh\necho hi\n' > "$0/bin/tool" && chmod 755 "$0/bin/tool" && ln -s tool "$0/bin/alias" && ln -s /etc "$0/lib/etc" && printf %s "$GREETING" > "$0/lib/greeting"`, "{dir}"}}
	if err := installTree(context.Background(), root, workdir, []string{"GREETING=hello"}, commands, nil, dest); err != nil {
		t.Fatal(err)
	}
	var tree []string
	_ = filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		rel, _ := filepath.Rel(dest, path)
		if d.Type()&fs.ModeSymlink != 0 {
			t.Errorf("symlink %s", rel)
		}
		tree = append(tree, filepath.ToSlash(rel))
		return err
	})
	if want := []string{".", "bin", "bin/alias", "bin/tool", "lib", "lib/greeting"}; !slices.Equal(tree, want) {
		t.Errorf("tree = %v, want %v", tree, want)
	}
	out, err := exec.Command(filepath.Join(dest, "bin", "alias")).CombinedOutput()
	if err != nil || string(out) != "hi\n" {
		t.Errorf("alias ran %q, %v", out, err)
	}
	if greeting, _ := os.ReadFile(filepath.Join(dest, "lib", "greeting")); string(greeting) != "hello" {
		t.Errorf("commands ran without their environment: greeting %q", greeting)
	}
	if info, err := os.Stat(filepath.Join(dest, "bin", "tool")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("tool lost its mode: %v %v", info, err)
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Errorf("staging left: %v", err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(workdir, "*.tar")); len(leftovers) > 0 {
		t.Errorf("archives left: %v", leftovers)
	}
	// Both containers hand what they write to the owner (on Linux, the user).
	owner := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if len(*calls) != 2 || !strings.Contains((*calls)[0][2], fmt.Sprintf(`"owner":[%d,%d]`, os.Getuid(), os.Getgid())) || len((*calls)[1]) != 5 || (*calls)[1][4] != owner {
		t.Errorf("container calls do not pass the owner: %q", *calls)
	}

	// A failing command fails the install with its status and leaves dest.
	err = installTree(context.Background(), root, workdir, nil, [][]string{{"sh", "-c", "echo broken >&2; exit 3"}}, nil, dest)
	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "broken") {
		t.Errorf("err = %v, want the command's status and output", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "bin", "tool")); err != nil {
		t.Errorf("failed install touched dest: %v", err)
	}
}

func TestInstallTreeCleansAFailedUnpack(t *testing.T) {
	_ = hostPythonContainer(t, true)
	root, workdir := t.TempDir(), t.TempDir()
	dest := filepath.Join(root, "tools", "pkg")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	commands := [][]string{{"sh", "-c", `printf x > "$0/file"`, "{dir}"}}
	if err := installTree(context.Background(), root, workdir, nil, commands, nil, dest); err == nil {
		t.Fatal("unpack failure not reported")
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Errorf("staging left after a failed unpack: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest removed by a failed unpack: %v", err)
	}
}

func TestContainerUnpackNamesAVolumeCollision(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3 on this computer")
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "clean.tar")
	if err := writeTestTar(archive, []testTarEntry{{name: "lib/Foo.py"}, {name: "lib/foo.py"}}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")
	out, err := exec.Command(python, "-c", containerUnpackScript, archive, dest).CombinedOutput()
	if _, statErr := os.Stat(filepath.Join(dest, "lib", "Foo.py")); err == nil && statErr == nil {
		t.Skip("this volume tells the names apart")
	}
	if err == nil || !strings.Contains(string(out), "lib/foo.py already exists") || strings.Contains(string(out), "Traceback") {
		t.Errorf("unpack = %v, %s; want a one-line explanation", err, out)
	}
}

func TestInterpreterShimWorksThroughALink(t *testing.T) {
	dir := t.TempDir()
	raw, clean := filepath.Join(dir, "raw.tar"), filepath.Join(dir, "clean.tar")
	// A python3.12 too large to copy, so python3 becomes the script.
	binary := append([]byte("#!/bin/sh\necho \"real $*\"\nexit 0\n#"), bytes.Repeat([]byte{'x'}, treeLinkCopyLimit)...)
	if err := writeTestTar(raw, []testTarEntry{
		{name: "./cpython-3.12.16-x/bin/", dir: true},
		{name: "./cpython-3.12.16-x/bin/python3.12", data: binary, mode: 0o755},
		{name: "./cpython-3.12.16-x/bin/python3", link: "python3.12"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := rewriteTreeArchive(raw, clean, nil); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(dir, "drive with space")
	if err := extractTestTar(clean, tree); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(dir, "home", "bin")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(elsewhere, "python3")
	if err := os.Symlink(filepath.Join(tree, "cpython-3.12.16-x", "bin", "python3"), link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(tree, "cpython-3.12.16-x", "bin", "python3"), link} {
		out, err := exec.Command(path, "-V").CombinedOutput()
		if err != nil || string(out) != "real -V\n" {
			t.Errorf("%s -V = %q, %v", path, out, err)
		}
	}
}
