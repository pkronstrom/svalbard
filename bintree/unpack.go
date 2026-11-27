// Package bintree unpacks and finds the programs of binary recipes, the same
// way for the host CLI and the drive runtime: each archive in bin/<platform>/
// is unpacked into <archive>.unpacked beside it, with plain files and
// directories only.
//
// exFAT, which every USB stick uses, has no symlinks: Linux cannot create one
// there, and macOS writes a small XSym file that only macOS reads as a link.
// Apply unpacks an archive that holds links (Prepare); the drive runtime and
// the host's semantic index unpack the others when a program in them is first
// asked for (Find).
package bintree

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// UnpackedSuffix names the directory beside an archive that holds its tree.
// The drive runtime looks for it under the same name.
const UnpackedSuffix = ".unpacked"

const partialMarker = ".partial-"

// isPartialDir reports whether name is a directory an unpack is still
// writing, or left when stopped: <archive>.partial-<digits>.
func isPartialDir(name string) bool {
	at := strings.LastIndex(name, partialMarker)
	if at <= 0 {
		return false
	}
	digits := name[at+len(partialMarker):]
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return false
	}
	archive := name[:at]
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.bz2", ".zip"} {
		if strings.HasSuffix(archive, suffix) {
			return true
		}
	}
	return false
}

// UnpackedDir is the directory archive is unpacked into.
func UnpackedDir(archive string) string { return archive + UnpackedSuffix }

// IsArchive reports whether the drive runtime unpacks a file of this name
// into a tree (a bare .gz is one program and has no tree).
func IsArchive(name string) bool {
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.bz2", ".zip"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// archiveLink is a symlink or hard link from an archive. Target is relative
// to the archive root for a hard link, and to the link's directory for a
// symlink.
type archiveLink struct {
	Name   string
	Target string
	Hard   bool
}

func readLinks(archive string) ([]archiveLink, error) {
	return walkArchive(archive, func(string, io.Reader, os.FileMode) error { return nil }, func(string) error { return nil })
}

// walkArchive calls file for each regular file and dir for each directory,
// in archive order, and returns the links.
func walkArchive(archive string, file func(name string, r io.Reader, mode os.FileMode) error, dir func(name string) error) ([]archiveLink, error) {
	if strings.HasSuffix(archive, ".zip") {
		return walkZip(archive, file, dir)
	}
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader
	if strings.HasSuffix(archive, ".tar.bz2") {
		r = bzip2.NewReader(f)
	} else {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	tr := tar.NewReader(r)
	var links []archiveLink
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return links, nil
		}
		if err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			err = dir(hdr.Name)
		case tar.TypeReg:
			err = file(hdr.Name, tr, os.FileMode(hdr.Mode))
		case tar.TypeSymlink:
			links = append(links, archiveLink{Name: hdr.Name, Target: hdr.Linkname})
		case tar.TypeLink:
			links = append(links, archiveLink{Name: hdr.Name, Target: hdr.Linkname, Hard: true})
		}
		if err != nil {
			return nil, err
		}
	}
}

func walkZip(archive string, file func(string, io.Reader, os.FileMode) error, dir func(string) error) ([]archiveLink, error) {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	var links []archiveLink
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			if err := dir(f.Name); err != nil {
				return nil, err
			}
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		if f.Mode()&os.ModeSymlink != 0 {
			// A zip stores a symlink's target as its content.
			target, err := io.ReadAll(io.LimitReader(r, 4096))
			r.Close()
			if err != nil {
				return nil, err
			}
			links = append(links, archiveLink{Name: f.Name, Target: string(target)})
			continue
		}
		err = file(f.Name, r, f.Mode())
		r.Close()
		if err != nil {
			return nil, err
		}
	}
	return links, nil
}

// Unpack writes the archive's tree to <archive>.unpacked with plain files
// and directories only, and returns what happened to each link, one line per
// link, for the log:
//   - a link to a shared library that no program or library in the archive
//     loads by that name (libllama.so, the name only a compiler uses) is left
//     out, unless a program in the archive could not be read
//   - a library whose own name nothing loads (libllama.so.1.0.0) takes the
//     name that is loaded (libllama.so.1) instead of being copied to it
//   - a link to a program in another directory (bin/npm ->
//     ../lib/npm/bin/npm-cli.js) becomes a script that runs it where it is,
//     so what it finds beside itself stays the same
//   - any other link to a file or directory becomes a copy of it
//   - a link out of the archive or to nothing is left out
//
// The tree is written under a .partial- name and renamed when complete, so a
// run that stops halfway leaves nothing that resolves; another run that
// finished the same archive first wins.
func Unpack(archive string) ([]string, error) {
	dir, base := filepath.Split(archive)
	removeStalePartials(dir, base)
	tmp, err := os.MkdirTemp(dir, base+partialMarker)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := os.Chmod(tmp, 0o755); err != nil { // MkdirTemp makes it 0700
		return nil, err
	}

	var files []string
	// Each file's mode in the archive: every file is written executable, as
	// the drive runtime always did, but only a program gets a script.
	modes := map[string]os.FileMode{}
	links, err := walkArchive(archive, func(name string, r io.Reader, mode os.FileMode) error {
		target, ok := entryPath(tmp, name)
		if !ok {
			return nil
		}
		files = append(files, target)
		modes[target] = mode
		return writeFile(target, r, mode|0o111)
	}, func(name string) error {
		if target, ok := entryPath(tmp, name); ok {
			return os.MkdirAll(target, 0o755)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	loaded := map[string]bool{}
	unreadable := false
	for _, file := range files {
		libs, failed := importedLibraries(file)
		unreadable = unreadable || failed
		for _, lib := range libs {
			loaded[path.Base(lib)] = true
		}
	}
	if unreadable {
		// What that program loads is unknown, so every link is kept.
		loaded = map[string]bool{}
	}
	report, err := resolveLinks(tmp, links, loaded, modes)
	if err != nil {
		return nil, err
	}
	removeAppleDouble(tmp)
	if err := os.Rename(tmp, UnpackedDir(archive)); err != nil {
		if _, statErr := os.Stat(UnpackedDir(archive)); statErr == nil {
			return report, nil
		}
		return nil, err
	}
	_ = os.Remove(filepath.Join(dir, "._"+base+UnpackedSuffix))
	return report, nil
}

// removeStalePartials removes what an unpack of the archive that was stopped
// (a kill, a pulled stick) left in dir; a younger one may still be writing.
func removeStalePartials(dir, base string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), base+partialMarker) || !isPartialDir(entry.Name()) {
			continue
		}
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > staleAfter {
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}
}

// staleAfter is how old a .partial- directory is when nothing writes it any more.
const staleAfter = time.Hour

func resolveLinks(root string, links []archiveLink, loaded map[string]bool, modes map[string]os.FileMode) ([]string, error) {
	byName := make(map[string]archiveLink, len(links))
	for _, link := range links {
		byName[path.Clean(link.Name)] = link
	}
	var report []string
	// Links to the same file, in archive order, keyed by that file.
	byFile := map[string][]string{}
	var fileOrder []string
	var dirLinks [][2]string
	for _, link := range links {
		name := path.Clean(link.Name)
		dest, ok := entryPath(root, name)
		if !ok {
			continue
		}
		resolved, ok := resolve(link, byName)
		var source string
		if ok {
			source, ok = entryPath(root, resolved)
		}
		var info os.FileInfo
		if ok {
			var err error
			info, err = os.Stat(source)
			ok = err == nil
		}
		switch {
		case !ok:
			report = append(report, fmt.Sprintf("%s: left out, points outside the archive or to nothing (%s)", name, link.Target))
		case info.IsDir():
			dirLinks = append(dirLinks, [2]string{source, dest})
			report = append(report, fmt.Sprintf("%s: copy of directory %s", name, resolved))
		case sharedLibrary(path.Base(name)) && len(loaded) > 0 && !loaded[path.Base(name)]:
			report = append(report, fmt.Sprintf("%s: left out, nothing in the archive loads it", name))
		case modes[source]&0o111 != 0 && !sharedLibrary(path.Base(resolved)) && filepath.Dir(source) != filepath.Dir(dest):
			if err := writeFile(dest, strings.NewReader(runScript(dest, source)), 0o755); err != nil {
				return nil, err
			}
			report = append(report, fmt.Sprintf("%s: script that runs %s", name, resolved))
		default:
			if _, seen := byFile[source]; !seen {
				fileOrder = append(fileOrder, source)
			}
			byFile[source] = append(byFile[source], dest)
		}
	}
	for _, source := range fileOrder {
		dests := byFile[source]
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, source)
		if base := filepath.Base(source); sharedLibrary(base) && len(loaded) > 0 && !loaded[base] {
			if err := os.MkdirAll(filepath.Dir(dests[0]), 0o755); err != nil {
				return nil, err
			}
			if err := os.Rename(source, dests[0]); err != nil {
				return nil, err
			}
			report = append(report, fmt.Sprintf("%s: renamed to %s, the name programs load", rel, relTo(root, dests[0])))
			source, dests = dests[0], dests[1:]
		}
		for _, dest := range dests {
			if err := copyFile(source, dest, info.Mode()); err != nil {
				return nil, err
			}
			report = append(report, fmt.Sprintf("%s: copy of %s", relTo(root, dest), rel))
		}
	}
	for _, link := range dirLinks {
		if err := copyDir(link[0], link[1]); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// runScript is a script at dest that runs the program at source, found
// relative to the script itself; a link to the script still finds it.
func runScript(dest, source string) string {
	rel, _ := filepath.Rel(filepath.Dir(dest), source)
	return "#!/bin/sh\nself=$(realpath -- \"$0\" 2>/dev/null) || self=$0\nexec \"$(dirname -- \"$self\")/" + filepath.ToSlash(rel) + "\" \"$@\"\n"
}

func relTo(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// resolve follows a link, through links to links and through links to
// directories on the way, to a path in the archive.
func resolve(link archiveLink, byName map[string]archiveLink) (string, bool) {
	return resolveLink(link, byName, 0)
}

func resolveLink(link archiveLink, byName map[string]archiveLink, depth int) (string, bool) {
	if depth > 40 {
		return "", false
	}
	var target string
	if link.Hard {
		target = path.Clean(link.Target)
	} else {
		if path.IsAbs(link.Target) {
			return "", false
		}
		target = path.Join(path.Dir(path.Clean(link.Name)), link.Target)
	}
	current := ""
	for _, part := range strings.Split(target, "/") {
		current = path.Join(current, part)
		if current == ".." || strings.HasPrefix(current, "../") {
			return "", false
		}
		if further, isLink := byName[current]; isLink {
			resolved, ok := resolveLink(further, byName, depth+1)
			if !ok {
				return "", false
			}
			current = resolved
		}
	}
	return current, true
}

func sharedLibrary(name string) bool {
	return strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.")
}

// importedLibraries lists the shared libraries an ELF or Mach-O file loads,
// as written in it (libllama.so.1, @rpath/libllama.1.dylib), and whether the
// file is one of those but could not be read. Anything else loads nothing.
var importedLibraries = func(file string) ([]string, bool) {
	f, err := os.Open(file)
	if err != nil {
		return nil, false
	}
	magic := make([]byte, 4)
	_, err = io.ReadFull(f, magic)
	f.Close()
	if err != nil {
		return nil, false
	}
	switch {
	case string(magic) == elf.ELFMAG:
		ef, err := elf.Open(file)
		if err != nil {
			return nil, true
		}
		defer ef.Close()
		libs, err := ef.ImportedLibraries()
		return libs, err != nil
	case machoMagic(magic):
		if mf, err := macho.Open(file); err == nil {
			defer mf.Close()
			return machoLibraries(mf), false
		}
		ff, err := macho.OpenFat(file)
		if err != nil {
			return nil, true
		}
		defer ff.Close()
		var libs []string
		for _, arch := range ff.Arches {
			libs = append(libs, machoLibraries(arch.File)...)
		}
		return libs, false
	}
	return nil, false
}

func machoMagic(magic []byte) bool {
	for _, m := range []uint32{macho.Magic32, macho.Magic64, macho.MagicFat} {
		be := []byte{byte(m >> 24), byte(m >> 16), byte(m >> 8), byte(m)}
		le := []byte{be[3], be[2], be[1], be[0]}
		if string(magic) == string(be) || string(magic) == string(le) {
			return true
		}
	}
	return false
}

// machoLibraries reads every load command that names a library: plain,
// weak, re-exported, upward and lazy. debug/macho reads only the plain one.
func machoLibraries(f *macho.File) []string {
	var libs []string
	for _, load := range f.Loads {
		raw := load.Raw()
		if len(raw) < 12 {
			continue
		}
		switch f.ByteOrder.Uint32(raw[0:4]) {
		case 0xc, 0x80000018, 0x8000001f, 0x80000023, 0x20:
			offset := f.ByteOrder.Uint32(raw[8:12])
			if int(offset) >= len(raw) {
				continue
			}
			name := raw[offset:]
			if end := strings.IndexByte(string(name), 0); end >= 0 {
				name = name[:end]
			}
			libs = append(libs, string(name))
		}
	}
	return libs
}

// entryPath is where an archive entry goes under root, or false for a name
// that would land outside it.
func entryPath(root, name string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(root, clean), true
}

func writeFile(target string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyFile(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(target, in, mode)
}

func copyDir(source, target string) error {
	if strings.HasPrefix(target+string(filepath.Separator), source+string(filepath.Separator)) {
		return nil // a link to a directory that holds it
	}
	return filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		out := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		return copyFile(p, out, info.Mode())
	})
}

// removeAppleDouble removes the ._ file macOS writes beside each file this
// computer creates on exFAT (for its provenance attribute): one more cluster
// per file that nothing reads. Deleting it removes only the attribute.
func removeAppleDouble(root string) {
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "._") {
			return nil
		}
		if _, err := os.Lstat(filepath.Join(filepath.Dir(p), d.Name()[2:])); err == nil {
			_ = os.Remove(p)
		}
		return nil
	})
}
