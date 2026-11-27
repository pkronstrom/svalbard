package bintree

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Prepare readies the tree of a binary archive just written to the vault. An
// earlier tree is removed: it belongs to the archive this one replaced, which
// may have had the same name. An archive with links is unpacked now; one
// without is left to the drive runtime. It returns what happened to each link,
// one line per link, for the log.
func Prepare(archive string) ([]string, error) {
	if !IsArchive(filepath.Base(archive)) {
		return nil, nil
	}
	if err := RemoveUnpacked(archive); err != nil {
		return nil, err
	}
	links, err := readLinks(archive)
	if err != nil || len(links) == 0 {
		return nil, err
	}
	return Unpack(archive)
}

// platformDirs are the directories of bin/ that hold one platform's binaries.
var platformDirs = map[string]bool{"macos-arm64": true, "macos-x86_64": true, "linux-arm64": true, "linux-x86_64": true}

// InPlatformBin reports whether path is a file directly in bin/<platform>/,
// where binary recipes keep their archives and nothing else is unpacked.
func InPlatformBin(path string) bool {
	dir := filepath.Dir(path)
	return platformDirs[filepath.Base(dir)] && filepath.Base(filepath.Dir(dir)) == "bin"
}

// RemoveUnpacked removes the archive's .unpacked directory and what an
// unpack of it that was stopped left. Both belong to the archive alone.
func RemoveUnpacked(archive string) error {
	if err := os.RemoveAll(UnpackedDir(archive)); err != nil {
		return err
	}
	dir, base := filepath.Split(archive)
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), base+partialMarker) && isPartialDir(entry.Name()) {
			if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// RemoveTrees removes everything unpacked from a binary archive before the
// archive goes: RemoveUnpacked, then RemoveLegacyTrees.
func RemoveTrees(archive string) error {
	if err := RemoveUnpacked(archive); err != nil {
		return err
	}
	return RemoveLegacyTrees(archive, archive)
}

// RemoveLegacyTrees removes the top-level directories that a drive runtime
// from before .unpacked unpacked beside archive, as listed in listFrom (the
// archive, or its set-aside copy during an update). Otherwise the old release
// would still be found after an update. It acts only on an archive in
// bin/<platform>/: datasets, GeoPackages and toolchains are archives too, and
// the directories beside them are not theirs. Top-level files are left: two
// archives may both hold a LICENSE there.
func RemoveLegacyTrees(archive, listFrom string) error {
	if !IsArchive(filepath.Base(archive)) || !InPlatformBin(archive) {
		return nil
	}
	tops, err := topDirs(listFrom)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dir := filepath.Dir(archive)
	for _, name := range tops {
		if err := removeLegacyDir(dir, name); err != nil {
			return err
		}
	}
	return nil
}

func removeLegacyDir(dir, name string) error {
	target, ok := entryPath(dir, name)
	if !ok || platformDirs[name] || strings.HasSuffix(name, UnpackedSuffix) || isPartialDir(name) {
		return nil
	}
	if info, err := os.Lstat(target); err == nil && info.IsDir() {
		return os.RemoveAll(target)
	}
	return nil
}

// topDirs lists the first component of each path in the archive that is a
// directory.
func topDirs(archive string) ([]string, error) {
	tops := map[string]bool{}
	top := func(name string, isDir bool) {
		first, _, nested := strings.Cut(strings.TrimPrefix(path.Clean(name), "./"), "/")
		if (nested || isDir) && first != "." && first != ".." {
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
		return nil, err
	}
	for _, link := range links {
		top(link.Name, false)
	}
	names := make([]string, 0, len(tops))
	for name := range tops {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// RepairLegacy converts the trees a drive runtime from before .unpacked
// unpacked flat beside binary archives in bin/<platform>/: on exFAT those of
// an archive with links hold macOS's XSym files, which Linux reads as text,
// or stop at the first link Linux could not create. Each archive with such a
// tree is unpacked into <archive>.unpacked, links resolved, and the flat tree
// removed, so an existing stick heals on its next apply without an update.
// Archives are read only when their directory holds a directory that could
// be such a tree, and each is converted once. It returns the archives
// converted.
func RepairLegacy(archives []string) ([]string, error) {
	byDir := map[string][]string{}
	for _, archive := range archives {
		if IsArchive(filepath.Base(archive)) && InPlatformBin(archive) {
			byDir[filepath.Dir(archive)] = append(byDir[filepath.Dir(archive)], archive)
		}
	}
	var converted []string
	for dir, list := range byDir {
		for _, archive := range list {
			removeStalePartials(dir, filepath.Base(archive))
		}
		candidates := legacyCandidates(dir)
		if len(candidates) == 0 {
			continue
		}
		sort.Strings(list)
		for _, archive := range list {
			if _, err := os.Stat(UnpackedDir(archive)); err == nil {
				continue
			}
			tops, err := topDirs(archive)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return converted, err
			}
			var legacy []string
			for _, name := range tops {
				if candidates[name] {
					legacy = append(legacy, name)
				}
			}
			if len(legacy) == 0 {
				continue
			}
			if _, err := Unpack(archive); err != nil {
				return converted, fmt.Errorf("%s: %w", filepath.Base(archive), err)
			}
			for _, name := range legacy {
				if err := removeLegacyDir(dir, name); err != nil {
					return converted, err
				}
			}
			converted = append(converted, archive)
		}
	}
	sort.Strings(converted)
	return converted, nil
}

// legacyCandidates are the directories in dir that no current layout put
// there.
func legacyCandidates(dir string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	candidates := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && !strings.HasSuffix(name, UnpackedSuffix) && !isPartialDir(name) && !strings.HasPrefix(name, ".") {
			candidates[name] = true
		}
	}
	return candidates
}
