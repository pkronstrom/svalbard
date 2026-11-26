// Package binarytree unpacks a binary recipe's archive on the vault when the
// archive holds links, so that no stick has to create one at first use.
//
// exFAT, which every USB stick uses, has no symlinks: Linux cannot create one
// there, and macOS writes a small XSym file that only macOS reads as a link.
// The drive runtime unpacks other archives at first use into the same
// <archive>.unpacked directory, and turns links into copies when it does.
package binarytree

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
)

// UnpackedSuffix names the directory beside an archive that holds its tree.
// The drive runtime looks for it under the same name.
const UnpackedSuffix = ".unpacked"

const partialMarker = ".partial-"

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

// Prepare readies the tree of a binary archive just written to the vault. An
// earlier tree is removed: it belongs to the archive this one replaced, which
// may have had the same name. An archive with links is unpacked now; one
// without is left to the drive runtime. It returns what happened to each link,
// one line per link, for the log.
func Prepare(archive string) ([]string, error) {
	if !IsArchive(filepath.Base(archive)) {
		return nil, nil
	}
	if err := os.RemoveAll(UnpackedDir(archive)); err != nil {
		return nil, err
	}
	links, err := readLinks(archive)
	if err != nil || len(links) == 0 {
		return nil, err
	}
	return unpack(archive)
}

// RemoveTrees removes what was unpacked from an archive before the archive
// goes: its .unpacked directory, and the top-level directories a drive
// runtime from before that directory unpacked beside the archive. Otherwise
// the old release would still be found after an update. Top-level files are
// left: two archives may both hold a LICENSE there.
func RemoveTrees(archive string) error {
	if !IsArchive(filepath.Base(archive)) {
		return nil
	}
	if err := os.RemoveAll(UnpackedDir(archive)); err != nil {
		return err
	}
	// The first component of each path, if it is a directory.
	tops := map[string]bool{}
	top := func(name string, isDir bool) {
		first, _, nested := strings.Cut(strings.TrimPrefix(path.Clean(name), "./"), "/")
		if nested || isDir {
			tops[first] = true
		}
	}
	links, err := walkArchive(archive, func(name string, _ io.Reader, _ os.FileMode) error {
		top(name, false)
		return nil
	}, func(name string) error {
		top(name, true)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, link := range links {
		top(link.Name, false)
	}
	dir := filepath.Dir(archive)
	for name := range tops {
		target, ok := entryPath(dir, name)
		if !ok || name == "." {
			continue
		}
		if info, err := os.Lstat(target); err == nil && info.IsDir() {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
	}
	return nil
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

// unpack writes the archive's tree to <archive>.unpacked with plain files
// and directories only:
//   - a link to a shared library that no program or library in the archive
//     loads by that name (libllama.so, the name only a compiler uses) is left out
//   - a library whose own name nothing loads (libllama.so.1.0.0) takes the
//     name that is loaded (libllama.so.1) instead of being copied to it
//   - any other link to a file or directory becomes a copy of it
//   - a link out of the archive or to nothing is left out
func unpack(archive string) ([]string, error) {
	dir, base := filepath.Split(archive)
	tmp, err := os.MkdirTemp(dir, base+partialMarker)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	var files []string
	links, err := walkArchive(archive, func(name string, r io.Reader, mode os.FileMode) error {
		target, ok := entryPath(tmp, name)
		if !ok {
			return nil
		}
		files = append(files, target)
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
	for _, file := range files {
		for _, lib := range importedLibraries(file) {
			loaded[path.Base(lib)] = true
		}
	}
	report, err := resolveLinks(tmp, links, loaded)
	if err != nil {
		return nil, err
	}
	removeAppleDouble(tmp)
	if err := os.Rename(tmp, UnpackedDir(archive)); err != nil {
		return nil, err
	}
	_ = os.Remove(filepath.Join(dir, "._"+base+UnpackedSuffix))
	return report, nil
}

func resolveLinks(root string, links []archiveLink, loaded map[string]bool) ([]string, error) {
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

func relTo(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// resolve follows a link, through links to links, to a path in the archive.
func resolve(link archiveLink, byName map[string]archiveLink) (string, bool) {
	for range 40 {
		var next string
		if link.Hard {
			next = path.Clean(link.Target)
		} else {
			if path.IsAbs(link.Target) {
				return "", false
			}
			next = path.Join(path.Dir(path.Clean(link.Name)), link.Target)
		}
		further, isLink := byName[next]
		if !isLink {
			return next, true
		}
		link = further
	}
	return "", false
}

func sharedLibrary(name string) bool {
	return strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.")
}

// importedLibraries lists the shared libraries an ELF or Mach-O file loads,
// as written in it (libllama.so.1, @rpath/libllama.1.dylib). Anything else
// loads nothing.
var importedLibraries = func(file string) []string {
	if f, err := elf.Open(file); err == nil {
		defer f.Close()
		libs, _ := f.ImportedLibraries()
		return libs
	}
	if f, err := macho.Open(file); err == nil {
		defer f.Close()
		libs, _ := f.ImportedLibraries()
		return libs
	}
	if f, err := macho.OpenFat(file); err == nil {
		defer f.Close()
		var libs []string
		for _, arch := range f.Arches {
			more, _ := arch.ImportedLibraries()
			libs = append(libs, more...)
		}
		return libs
	}
	return nil
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
