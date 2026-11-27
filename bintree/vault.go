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
	if !IsArchive(filepath.Base(archive)) {
		return nil
	}
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
	listing, err := listArchive(listFrom)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dir := filepath.Dir(archive)
	for _, name := range listing.tops {
		if _, err := removeLegacyDir(dir, name, listing.entries); err != nil {
			return err
		}
	}
	return nil
}

// removeLegacyDir removes dir/name if it is what an older drive runtime
// unpacked from the archive listed in entries: every file in it is an entry
// of the archive, or the ._ file macOS wrote beside one. A directory holding
// anything else is left alone, and so is a name the current layout uses.
func removeLegacyDir(dir, name string, entries map[string]bool) (bool, error) {
	target, ok := entryPath(dir, name)
	if !ok || platformDirs[name] || strings.Contains(name, UnpackedSuffix) || isPartialDir(name) {
		return false, nil
	}
	if info, err := os.Lstat(target); err != nil || !info.IsDir() {
		return false, nil
	}
	foreign := false
	_ = filepath.WalkDir(target, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			foreign = true
			return filepath.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		base := path.Base(rel)
		if entries[rel] || (strings.HasPrefix(base, "._") && entries[path.Join(path.Dir(rel), base[2:])]) {
			return nil
		}
		foreign = true
		return filepath.SkipAll
	})
	if foreign {
		return false, nil
	}
	return true, os.RemoveAll(target)
}

// archiveListing is what an archive holds: the first component of each of
// its paths that is a directory, and every path (files, links, directories).
type archiveListing struct {
	tops    []string
	entries map[string]bool
}

func listArchive(archive string) (archiveListing, error) {
	tops := map[string]bool{}
	entries := map[string]bool{}
	add := func(name string, isDir bool) {
		clean := strings.TrimPrefix(path.Clean(name), "./")
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return
		}
		entries[clean] = true
		if first, _, nested := strings.Cut(clean, "/"); nested || isDir {
			tops[first] = true
		}
	}
	links, err := walkArchive(archive, func(name string, _ io.Reader, _ os.FileMode) error {
		add(name, false)
		return nil
	}, func(name string) error {
		add(name, true)
		return nil
	})
	if err != nil {
		return archiveListing{}, err
	}
	for _, link := range links {
		add(link.Name, false)
	}
	listing := archiveListing{entries: entries}
	for name := range tops {
		listing.tops = append(listing.tops, name)
	}
	sort.Strings(listing.tops)
	return listing, nil
}

// RepairLegacy converts the trees a drive runtime from before .unpacked
// unpacked flat beside binary archives in bin/<platform>/: on exFAT those of
// an archive with links hold macOS's XSym files, which Linux reads as text,
// or stop at the first link Linux could not create. It returns the archives
// converted, so an existing stick heals on its next apply without an update.
//
// It acts only in a directory that holds a directory no current layout put
// there. Each archive there without a tree is unpacked into
// <archive>.unpacked, once: that runtime unpacked every archive of the
// directory at its first miss, and an archive left unread would be read again
// on every apply. A flat tree beside an archive that already has its tree
// (one an older CLI or runtime made again) is removed too; such an archive is
// read only when a candidate's name begins its file name, as every flat tree
// in the catalog's archives does (llama-b12359/, pi/, kiwix-tools_.../). A
// directory goes only if all it holds came from the archive.
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
			_, statErr := os.Stat(UnpackedDir(archive))
			hasTree := statErr == nil
			if hasTree && !prefixOfAny(candidates, filepath.Base(archive)) {
				continue
			}
			listing, err := listArchive(archive)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return converted, err
			}
			if !hasTree {
				if _, err := Unpack(archive); err != nil {
					return converted, fmt.Errorf("%s: %w", filepath.Base(archive), err)
				}
				converted = append(converted, archive)
			}
			for _, name := range listing.tops {
				if !candidates[name] {
					continue
				}
				removed, err := removeLegacyDir(dir, name, listing.entries)
				if err != nil {
					return converted, err
				}
				if removed && hasTree {
					converted = append(converted, archive)
				}
			}
		}
	}
	sort.Strings(converted)
	return slicesCompact(converted), nil
}

func prefixOfAny(candidates map[string]bool, filename string) bool {
	for name := range candidates {
		if strings.HasPrefix(filename, name) {
			return true
		}
	}
	return false
}

func slicesCompact(sorted []string) []string {
	out := sorted[:0]
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
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
		if entry.IsDir() && !strings.Contains(name, UnpackedSuffix) && !isPartialDir(name) && !strings.HasPrefix(name, ".") {
			candidates[name] = true
		}
	}
	return candidates
}
