package bintree

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

type entry struct {
	name, body, link string
	flag             byte
	mode             int64 // 0755 when unset
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
		mode := e.mode
		if mode == 0 {
			mode = 0o755
		}
		hdr := &tar.Header{Name: e.name, Typeflag: flag, Linkname: e.link, Mode: mode}
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
	importedLibraries = func(file string) ([]string, bool) { return imports[filepath.Base(file)], false }
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
	libs, _ := importedLibraries("/bin/ls")
	if !slices.ContainsFunc(libs, func(lib string) bool { return filepath.Base(lib) == "libSystem.B.dylib" }) {
		t.Fatalf("importedLibraries(/bin/ls) = %v, want libSystem.B.dylib", libs)
	}
	if libs, failed := importedLibraries("/etc/hosts"); len(libs) != 0 || failed {
		t.Fatalf("importedLibraries(/etc/hosts) = %v, want none", libs)
	}
}

func TestRemoveTreesTakesTheArchivesDirectoriesOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin", "linux-arm64")
	os.MkdirAll(dir, 0o755)
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

// Datasets, GeoPackages and PlatformIO toolchains are archives too, and the
// directories beside them belong to others: only the .unpacked directory of
// such an archive goes.
func TestRemoveTreesOutsidePlatformBinLeavesNeighbours(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"natural-earth.zip",       // a GeoPackage zip at the vault root
		"data/exploitdb-main.zip", // a dataset
		"tools/platformio/packages/macos-arm64/toolchain.tar.gz", // a toolchain with bin/, lib/, share/
		"bin/tool.tar.gz", // the old flat bin/
	} {
		archive := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(archive), 0o755)
		writeTarGz(t, archive, []entry{{name: "bin/x", body: "x"}, {name: "packages/y", body: "y"}, {name: "exploitdb-main/z", body: "z"}})
		for _, sibling := range []string{"bin", "packages", "exploitdb-main"} {
			os.MkdirAll(filepath.Join(filepath.Dir(archive), sibling, "keep"), 0o755)
		}
		os.MkdirAll(filepath.Join(UnpackedDir(archive), "x"), 0o755)
		if err := RemoveTrees(archive); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, UnpackedDir(archive))
		for _, sibling := range []string{"bin", "packages", "exploitdb-main"} {
			if _, err := os.Stat(filepath.Join(filepath.Dir(archive), sibling, "keep")); err != nil {
				t.Errorf("%s: removing it took %s beside it: %v", rel, sibling, err)
			}
		}
	}
}

// A drive runtime from before .unpacked unpacked llama-server flat beside its
// archive; on exFAT the links became XSym text files on macOS, or the unpack
// stopped at the first link on Linux. A plain apply converts it once.
func TestRepairLegacyConvertsFlatTreesOnce(t *testing.T) {
	stubImports(t, nil)
	bin := filepath.Join(t.TempDir(), "bin", "linux-arm64")
	os.MkdirAll(bin, 0o755)
	llama := filepath.Join(bin, "llama-b1-bin-ubuntu-arm64.tar.gz")
	writeTarGz(t, llama, []entry{
		{name: "llama-b1/libllama.so.1", link: "libllama.so.1.0.0", flag: tar.TypeSymlink},
		{name: "llama-b1/libllama.so.1.0.0", body: "lib"},
		{name: "llama-b1/llama-server", body: "server"},
	})
	ripgrep := filepath.Join(bin, "ripgrep.tar.gz")
	writeTarGz(t, ripgrep, []entry{{name: "ripgrep-15/rg", body: "rg"}})
	// Never unpacked flat, and it has no top directory: unpacked once all the
	// same, so it is never read again.
	fresh := filepath.Join(bin, "fzf.tar.gz")
	writeTarGz(t, fresh, []entry{{name: "fzf", body: "fzf"}})
	// The flat trees: Linux stopped at the link, so llama-server is missing.
	os.MkdirAll(filepath.Join(bin, "llama-b1"), 0o755)
	os.WriteFile(filepath.Join(bin, "llama-b1", "libllama.so.1.0.0"), []byte("lib"), 0o755)
	os.MkdirAll(filepath.Join(bin, "ripgrep-15"), 0o755)
	os.WriteFile(filepath.Join(bin, "ripgrep-15", "rg"), []byte("rg"), 0o755)

	converted, err := RepairLegacy([]string{llama, ripgrep, fresh})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(converted, ",") != strings.Join([]string{fresh, llama, ripgrep}, ",") {
		t.Fatalf("converted = %v", converted)
	}
	assertFile(t, filepath.Join(UnpackedDir(llama), "llama-b1", "llama-server"), "server")
	assertFile(t, filepath.Join(UnpackedDir(llama), "llama-b1", "libllama.so.1"), "lib")
	assertFile(t, filepath.Join(UnpackedDir(ripgrep), "ripgrep-15", "rg"), "rg")
	assertMissing(t, filepath.Join(bin, "llama-b1"))
	assertMissing(t, filepath.Join(bin, "ripgrep-15"))
	assertFile(t, filepath.Join(UnpackedDir(fresh), "fzf"), "fzf")

	if again, err := RepairLegacy([]string{llama, ripgrep, fresh}); err != nil || len(again) != 0 {
		t.Fatalf("second repair = %v, %v; want nothing", again, err)
	}
}

// A link whose path passes through a link to a directory (lib64 -> lib)
// resolves to the file behind it.
func TestPrepareResolvesLinksThroughDirectoryLinks(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "lib64", link: "lib", flag: tar.TypeSymlink},
		{name: "bin/libz.so.1", link: "../lib64/libz.so.1.2", flag: tar.TypeSymlink},
		{name: "lib/libz.so.1.2", body: "z"},
	})
	if _, err := Prepare(archive); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(UnpackedDir(archive), "bin", "libz.so.1"), "z")
	assertFile(t, filepath.Join(UnpackedDir(archive), "lib64", "libz.so.1.2"), "z")
}

// minimalELF is a shared object that loads needed, enough for debug/elf to
// read its DT_NEEDED entry.
func minimalELF(needed string) []byte {
	le := binary.LittleEndian
	dynstr := append(append([]byte{0}, needed...), 0)
	shstr := []byte("\x00.dynstr\x00.dynamic\x00.shstrtab\x00")
	const dynstrOff = 64
	dynOff := (dynstrOff + len(dynstr) + 7) &^ 7
	dyn := make([]byte, 32)
	le.PutUint64(dyn[0:], uint64(elf.DT_NEEDED))
	le.PutUint64(dyn[8:], 1) // the name's offset in .dynstr
	shstrOff := dynOff + len(dyn)
	shOff := (shstrOff + len(shstr) + 7) &^ 7
	out := make([]byte, shOff+4*64)
	copy(out, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	le.PutUint16(out[16:], uint16(elf.ET_DYN))
	le.PutUint16(out[18:], uint16(elf.EM_X86_64))
	le.PutUint32(out[20:], 1)
	le.PutUint64(out[40:], uint64(shOff))
	le.PutUint16(out[52:], 64) // header size
	le.PutUint16(out[58:], 64) // section header size
	le.PutUint16(out[60:], 4)  // sections
	le.PutUint16(out[62:], 3)  // .shstrtab
	copy(out[dynstrOff:], dynstr)
	copy(out[dynOff:], dyn)
	copy(out[shstrOff:], shstr)
	section := func(i int, name uint32, typ elf.SectionType, off, size int, link uint32, entsize uint64) {
		h := out[shOff+i*64:]
		le.PutUint32(h[0:], name)
		le.PutUint32(h[4:], uint32(typ))
		le.PutUint64(h[24:], uint64(off))
		le.PutUint64(h[32:], uint64(size))
		le.PutUint32(h[40:], link)
		le.PutUint64(h[48:], 1)
		le.PutUint64(h[56:], entsize)
	}
	section(1, 1, elf.SHT_STRTAB, dynstrOff, len(dynstr), 0, 0)
	section(2, 9, elf.SHT_DYNAMIC, dynOff, len(dyn), 1, 16)
	section(3, 18, elf.SHT_STRTAB, shstrOff, len(shstr), 0, 0)
	return out
}

// The ELF side of the rule, with the real reader: the link nothing loads is
// left out and the library takes the name the program loads.
func TestUnpackReadsWhatAnELFProgramLoads(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "tool/prog", body: string(minimalELF("libfoo.so.1"))},
		{name: "tool/libfoo.so", link: "libfoo.so.1", flag: tar.TypeSymlink},
		{name: "tool/libfoo.so.1", link: "libfoo.so.1.0", flag: tar.TypeSymlink},
		{name: "tool/libfoo.so.1.0", body: "foo"},
	})
	if libs, failed := importedLibraries(writeTemp(t, minimalELF("libfoo.so.1"))); failed || strings.Join(libs, ",") != "libfoo.so.1" {
		t.Fatalf("importedLibraries = %v, %v", libs, failed)
	}
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(UnpackedDir(archive), "tool")
	assertFile(t, filepath.Join(tree, "libfoo.so.1"), "foo")
	assertMissing(t, filepath.Join(tree, "libfoo.so"))
	assertMissing(t, filepath.Join(tree, "libfoo.so.1.0"))
}

// A file that claims to be a program but cannot be read may load anything,
// so no link is left out.
func TestUnpackKeepsEveryLinkWhenAProgramCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "tool/prog", body: string(minimalELF("libfoo.so.1"))},
		{name: "tool/broken", body: "\x7fELF\x02\x01 not really"},
		{name: "tool/libfoo.so", link: "libfoo.so.1", flag: tar.TypeSymlink},
		{name: "tool/libfoo.so.1", body: "foo"},
	})
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(UnpackedDir(archive), "tool", "libfoo.so"), "foo")
}

func writeTemp(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "obj")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// A link to a program in another directory becomes a script that runs it
// where it is, so it still finds what lies beside it (bin/npm ->
// ../lib/npm/cli.js requires ../lib/...).
func TestUnpackRunsAProgramLinkedFromAnotherDirectoryInPlace(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "node.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "node/lib/npm/cli.sh", body: "#!/bin/sh\necho \"$(dirname \"$0\") $*\"\n"},
		{name: "node/bin/npm", link: "../lib/npm/cli.sh", flag: tar.TypeSymlink},
		{name: "node/share/data.txt", body: "data", mode: 0o644},
		{name: "node/bin/data.txt", link: "../share/data.txt", flag: tar.TypeSymlink},
	})
	// Data has no exec bit in the archive.
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	root := UnpackedDir(archive)
	out, err := exec.Command(filepath.Join(root, "node", "bin", "npm"), "install").CombinedOutput()
	if err != nil {
		t.Fatalf("bin/npm: %v\n%s", err, out)
	}
	ranIn, args, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	got, _ := filepath.EvalSymlinks(ranIn)
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "node", "lib", "npm"))
	if got != want || args != "install" {
		t.Fatalf("bin/npm ran as %q, want in %s with install", out, want)
	}
	assertFile(t, filepath.Join(root, "node", "bin", "data.txt"), "data")
}

// A library renamed to the name programs load may land in a directory that
// holds nothing else yet.
func TestUnpackRenamesALibraryIntoANewDirectory(t *testing.T) {
	stubImports(t, map[string][]string{"prog": {"libx.so.1"}})
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "bin/prog", body: "p"},
		{name: "real/libx.so.1.0", body: "x"},
		{name: "lib/libx.so.1", link: "../real/libx.so.1.0", flag: tar.TypeSymlink},
	})
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(UnpackedDir(archive), "lib", "libx.so.1"), "x")
}

// An unpack stopped more than an hour ago is swept; a younger one may still
// be writing and stays.
func TestUnpackSweepsOnlyStalePartials(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{{name: "tool", body: "t"}})
	stale, young := archive+partialMarker+"1", archive+partialMarker+"2"
	os.MkdirAll(stale, 0o755)
	os.MkdirAll(young, 0o755)
	old := time.Now().Add(-2 * staleAfter)
	os.Chtimes(stale, old, old)
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, stale)
	if _, err := os.Stat(young); err != nil {
		t.Fatalf("young partial removed: %v", err)
	}
}

// macOS writes a ._ file beside each file this computer creates on exFAT;
// the unpack removes those beside its own files.
func TestUnpackRemovesAppleDoubleFiles(t *testing.T) {
	stubImports(t, nil)
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archive, []entry{
		{name: "tool/prog", body: "p"},
		{name: "tool/._prog", body: "\x00\x05\x16\x07"},
	})
	if _, err := Unpack(archive); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(UnpackedDir(archive), "tool", "._prog"))
}

// A directory beside the archive that holds anything the archive does not is
// not the archive's flat tree, whatever its name.
func TestRepairLegacyKeepsADirectoryWithForeignFiles(t *testing.T) {
	stubImports(t, nil)
	bin := filepath.Join(t.TempDir(), "bin", "linux-arm64")
	os.MkdirAll(bin, 0o755)
	archive := filepath.Join(bin, "tool-1.0.tar.gz")
	writeTarGz(t, archive, []entry{{name: "tool/tool", body: "t"}})
	os.MkdirAll(filepath.Join(bin, "tool"), 0o755)
	os.WriteFile(filepath.Join(bin, "tool", "tool"), []byte("t"), 0o755)
	os.WriteFile(filepath.Join(bin, "tool", "._tool"), []byte("mac"), 0o644)
	os.WriteFile(filepath.Join(bin, "tool", "notes.txt"), []byte("mine"), 0o644)
	if _, err := RepairLegacy([]string{archive}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(bin, "tool", "notes.txt"), "mine")
	assertFile(t, filepath.Join(UnpackedDir(archive), "tool", "tool"), "t")
}

// A flat tree made again beside an archive that has its tree (by an older
// CLI or runtime) goes on the next apply; an archive with a tree is not read
// for a directory whose name does not begin its file name.
func TestRepairLegacyRemovesAFlatTreeBesideAnUnpackedOne(t *testing.T) {
	stubImports(t, nil)
	bin := filepath.Join(t.TempDir(), "bin", "linux-arm64")
	os.MkdirAll(bin, 0o755)
	llama := filepath.Join(bin, "llama-b1-bin.tar.gz")
	writeTarGz(t, llama, []entry{{name: "llama-b1/llama-server", body: "s"}})
	os.MkdirAll(filepath.Join(UnpackedDir(llama), "llama-b1"), 0o755)
	os.WriteFile(filepath.Join(UnpackedDir(llama), "llama-b1", "llama-server"), []byte("s"), 0o755)
	os.MkdirAll(filepath.Join(bin, "llama-b1"), 0o755)
	os.WriteFile(filepath.Join(bin, "llama-b1", "llama-server"), []byte("s"), 0o755)
	// Not an archive any more, but with a tree: reading it would fail.
	broken := filepath.Join(bin, "other.tar.gz")
	os.WriteFile(broken, []byte("not gzip"), 0o644)
	os.MkdirAll(UnpackedDir(broken), 0o755)
	os.MkdirAll(filepath.Join(bin, "removed-recipe"), 0o755)

	converted, err := RepairLegacy([]string{llama, broken})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(converted, ",") != llama {
		t.Fatalf("converted = %v", converted)
	}
	assertMissing(t, filepath.Join(bin, "llama-b1"))
	if _, err := os.Stat(filepath.Join(bin, "removed-recipe")); err != nil {
		t.Fatalf("an unclaimed directory was removed: %v", err)
	}
}
