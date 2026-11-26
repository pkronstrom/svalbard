package binary

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func Resolve(name, driveRoot string) (string, error) {
	if _, err := platform.Detect(); err != nil {
		return "", err
	}
	if path, err := ResolveOnDrive(name, driveRoot); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found", name)
}

// ResolveOnDrive is Resolve without the host's PATH, for a tool whose
// version matters: a host install would be another release, started with
// state written for the one on the drive.
func ResolveOnDrive(name, driveRoot string) (string, error) {
	platformName, err := platform.Detect()
	if err != nil {
		return "", err
	}
	for _, dir := range []string{
		filepath.Join(driveRoot, "bin", platformName, name),
		filepath.Join(driveRoot, "bin", platformName),
		filepath.Join(driveRoot, "bin", name),
		filepath.Join(driveRoot, "bin"),
	} {
		if path, err := resolveFromDir(name, dir); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}

// unpackedSuffix names the directory beside an archive that it is unpacked
// into. The host removes it with the archive on update and removal, so an old
// release is never found after its archive is replaced.
const unpackedSuffix = ".unpacked"

// partialMarker is in the name of a directory an unpack is still writing.
const partialMarker = ".partial-"

func resolveFromDir(name, dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}

	// Check if the binary already exists before extracting archives.
	if found, err := findMatchingBinary(dir, name); err == nil {
		return found, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	extracted := false
	for _, entry := range entries {
		// On an exFAT stick macOS keeps a file's extended attributes in a
		// "._name" file beside it, which has the archive's suffix and none
		// of its content.
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "._") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		switch {
		case strings.HasSuffix(entry.Name(), ".tar.gz"), strings.HasSuffix(entry.Name(), ".tgz"),
			strings.HasSuffix(entry.Name(), ".tar.bz2"), strings.HasSuffix(entry.Name(), ".zip"):
			if _, err := os.Stat(path + unpackedSuffix); err == nil {
				continue
			}
			if err := unpackArchive(path); err != nil {
				return "", err
			}
			extracted = true
		case strings.HasSuffix(entry.Name(), ".gz"):
			// A bare .gz is one program, written under the name asked for;
			// only its own name may unpack it (chisel_1.12.0_linux_arm64.gz
			// is chisel, never kiwix-serve).
			if !bareGzFor(entry.Name(), name) {
				continue
			}
			if err := extractBareGz(path, dir, name); err != nil {
				return "", err
			}
			extracted = true
		default:
			continue
		}
	}
	if extracted {
		if found, err := findMatchingBinary(dir, name); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("%s not found in %s", name, dir)
}

func bareGzFor(filename, name string) bool {
	rest, ok := strings.CutPrefix(filename, name)
	return ok && rest != "" && strings.ContainsRune("_-.", rune(rest[0]))
}

// unpackArchive unpacks an archive into <archive>.unpacked beside it, with
// plain files and directories only. exFAT, which every USB stick uses, has no
// symlinks: Linux cannot create one there, and macOS writes a small XSym file
// that only macOS reads as a link. Each link in the archive becomes a copy of
// what it points to; one that leaves the archive or points nowhere is left
// out. The tree is written under a .partial- name and renamed when complete,
// so a run that stops halfway leaves nothing that resolves.
func unpackArchive(archivePath string) error {
	dir, base := filepath.Split(archivePath)
	removeStalePartials(dir, base)
	tmp, err := os.MkdirTemp(dir, base+partialMarker)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	var links []archiveLink
	if strings.HasSuffix(base, ".zip") {
		links, err = extractZip(archivePath, tmp)
	} else {
		links, err = extractTar(archivePath, tmp)
	}
	if err != nil {
		return err
	}
	if err := materializeLinks(tmp, links); err != nil {
		return err
	}
	removeAppleDouble(tmp)
	if err := os.Rename(tmp, archivePath+unpackedSuffix); err != nil {
		// Another run finished the same archive first.
		if _, statErr := os.Stat(archivePath + unpackedSuffix); statErr == nil {
			return nil
		}
		return err
	}
	_ = os.Remove(filepath.Join(dir, "._"+base+unpackedSuffix))
	return nil
}

// removeStalePartials removes what an unpack of the archive that was stopped
// (a kill, a pulled stick) left behind; a younger one may still be writing.
func removeStalePartials(dir, base string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), base+partialMarker) {
			continue
		}
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
		}
	}
}

// removeAppleDouble removes the ._ file macOS writes beside each file it
// creates on exFAT (for its provenance attribute): one more cluster per file
// that nothing reads. Deleting it removes only the attribute.
func removeAppleDouble(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "._") {
			return nil
		}
		if _, err := os.Lstat(filepath.Join(filepath.Dir(path), name[2:])); err == nil {
			_ = os.Remove(path)
		}
		return nil
	})
}

// archiveLink is a symlink or hard link from an archive. Target is relative
// to the archive root for a hard link, and to the link's directory for a
// symlink.
type archiveLink struct {
	Name   string
	Target string
	Hard   bool
}

// entryPath is where an archive entry goes under dest, or false for a name
// that would land outside it.
func entryPath(dest, name string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(dest, clean), true
}

func extractTar(archivePath, dest string) ([]archiveLink, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var r io.Reader
	if strings.HasSuffix(archivePath, ".tar.bz2") {
		r = bzip2.NewReader(file)
	} else {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		r = gzr
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
		target, ok := entryPath(dest, hdr.Name)
		if !ok {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			if err := writeFile(target, tr, os.FileMode(hdr.Mode)|0o111); err != nil {
				return nil, err
			}
		case tar.TypeSymlink:
			links = append(links, archiveLink{Name: hdr.Name, Target: hdr.Linkname})
		case tar.TypeLink:
			links = append(links, archiveLink{Name: hdr.Name, Target: hdr.Linkname, Hard: true})
		}
	}
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

// materializeLinks writes each link as a copy of the file or directory it
// resolves to inside dest, following links to links.
func materializeLinks(dest string, links []archiveLink) error {
	byName := make(map[string]archiveLink, len(links))
	for _, link := range links {
		byName[path.Clean(link.Name)] = link
	}
	resolve := func(link archiveLink) (string, bool) {
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
	// Files first, so a directory copied afterwards holds its links' copies.
	var dirs []archiveLink
	for _, link := range links {
		name, ok := entryPath(dest, link.Name)
		if !ok {
			continue
		}
		resolved, ok := resolve(link)
		if !ok {
			continue
		}
		source, ok := entryPath(dest, resolved)
		if !ok {
			continue
		}
		info, err := os.Stat(source)
		if err != nil {
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, archiveLink{Name: name, Target: source})
			continue
		}
		if err := copyFile(source, name, info.Mode()); err != nil {
			return err
		}
	}
	for _, link := range dirs {
		if err := copyDir(link.Target, link.Name); err != nil {
			return err
		}
	}
	return nil
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

func extractBareGz(archivePath, destDir, toolName string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Use the tool name so findMatchingBinary can locate it.
	outPath := filepath.Join(destDir, toolName)

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gzr); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func extractZip(archivePath, dest string) ([]archiveLink, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var links []archiveLink
	for _, file := range reader.File {
		target, ok := entryPath(dest, file.Name)
		if !ok {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		src, err := file.Open()
		if err != nil {
			return nil, err
		}
		if file.Mode()&os.ModeSymlink != 0 {
			// A zip stores a symlink's target as its content.
			linkTarget, err := io.ReadAll(io.LimitReader(src, 4096))
			src.Close()
			if err != nil {
				return nil, err
			}
			links = append(links, archiveLink{Name: file.Name, Target: string(linkTarget)})
			continue
		}
		err = writeFile(target, src, file.Mode()|0o111)
		src.Close()
		if err != nil {
			return nil, err
		}
	}
	return links, nil
}

// platformDirs are the per-platform directories of bin/. The walk of bin/
// itself skips them: a Linux run must never find the macOS llama-server.
var platformDirs = map[string]bool{"macos-arm64": true, "macos-x86_64": true, "linux-arm64": true, "linux-x86_64": true}

func findMatchingBinary(dir, name string) (string, error) {
	var direct string
	var nested string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dir && (platformDirs[d.Name()] || strings.Contains(d.Name(), partialMarker)) {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Base(path) != name || !isExecutable(path) {
			return nil
		}
		if filepath.Dir(path) == dir {
			direct = path
			return nil
		}
		nested = path
		return io.EOF
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	found := nested
	if found == "" {
		found = direct
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in extracted archive", name)
	}
	return found, nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
