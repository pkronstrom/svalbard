package bintree

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Find returns the path of the program name for platform on the drive at
// driveRoot, unpacking an archive in bin/ into <archive>.unpacked when no
// tree holds it yet. It never looks at another platform's directory, and
// never unpacks into bin/<platform>/ itself.
//
// A tree in <archive>.unpacked is the current layout and wins over the flat
// tree an older drive runtime unpacked beside the archive, and over a
// bin/<platform>/<name>/ directory: lexical order would otherwise pick
// llama-b12359/ before llama-b12359-bin-macos-arm64.tar.gz.unpacked/.
func Find(name, driveRoot, platform string) (string, error) {
	dirs := []string{
		filepath.Join(driveRoot, "bin", platform, name),
		filepath.Join(driveRoot, "bin", platform),
		filepath.Join(driveRoot, "bin", name),
		filepath.Join(driveRoot, "bin"),
	}
	for _, dir := range dirs {
		if found, ok := findInUnpacked(dir, name); ok {
			return found, nil
		}
	}
	for _, dir := range dirs {
		if found, err := findInDir(name, dir); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}

// archivesIn lists the archives in dir, without macOS's ._ companions (on an
// exFAT stick a "._name" file sits beside each file, with the archive's
// suffix and none of its content).
func archivesIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var archives []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "._") {
			continue
		}
		archives = append(archives, entry.Name())
	}
	sort.Strings(archives)
	return archives
}

func findInUnpacked(dir, name string) (string, bool) {
	for _, entry := range archivesIn(dir) {
		if !IsArchive(entry) {
			continue
		}
		unpacked := UnpackedDir(filepath.Join(dir, entry))
		if info, err := os.Stat(unpacked); err != nil || !info.IsDir() {
			continue
		}
		if found, err := findMatchingBinary(unpacked, name); err == nil {
			return found, true
		}
	}
	return "", false
}

// findInDir finds name in dir as it is (a bare program, or an older flat
// tree), and otherwise unpacks the archives in dir that have no tree yet and
// looks again. An archive that cannot be unpacked does not keep the others
// from being tried.
func findInDir(name, dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}
	if found, err := findMatchingBinary(dir, name); err == nil {
		return found, nil
	}

	var firstErr error
	for _, entry := range archivesIn(dir) {
		path := filepath.Join(dir, entry)
		var err error
		switch {
		case IsArchive(entry):
			if _, statErr := os.Stat(UnpackedDir(path)); statErr == nil {
				continue
			}
			_, err = Unpack(path)
		case strings.HasSuffix(entry, ".gz") && bareGzFor(entry, name):
			err = extractBareGz(path, dir, name)
		default:
			continue
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if found, err := findMatchingBinary(dir, name); err == nil {
		return found, nil
	}
	if firstErr != nil {
		return "", firstErr
	}
	return "", fmt.Errorf("%s not found in %s", name, dir)
}

// bareGzFor reports whether a bare .gz is the program name. It is unpacked
// under the name asked for, so only its own name may unpack it:
// chisel_1.12.0_linux_arm64.gz is chisel, never kiwix-serve.
func bareGzFor(filename, name string) bool {
	rest, ok := strings.CutPrefix(filename, name)
	return ok && rest != "" && strings.ContainsRune("_-.", rune(rest[0]))
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
	return writeFile(filepath.Join(destDir, toolName), gzr, 0o755)
}

// findMatchingBinary walks dir for an executable named name, preferring one
// nested in a tree over one directly in dir. The walk of bin/ skips the
// platform directories (a Linux run must never find the macOS llama-server),
// and every walk skips unpacks still being written and directories it cannot
// read.
func findMatchingBinary(dir, name string) (string, error) {
	var direct, nested string
	walkingBin := filepath.Base(dir) == "bin"
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path != dir && d != nil && d.IsDir() {
				return filepath.SkipDir // one unreadable tree hides no other tool
			}
			return err
		}
		if d.IsDir() && filepath.Dir(path) == dir && ((walkingBin && platformDirs[d.Name()]) || isPartialDir(d.Name())) {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != name || !isExecutable(path) {
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
		return "", fmt.Errorf("%s not found in %s", name, dir)
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
