package search

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
)

type Archive struct {
	Filename string
	Size     int64
}

// ListArchives orders the installed ZIMs by size so the biggest sources
// are easy to find in the search filter.
func ListArchives(root string) ([]Archive, error) {
	paths, err := server.ZIMFiles(root)
	if err != nil {
		return nil, err
	}
	var archives []Archive
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		archives = append(archives, Archive{Filename: filepath.Base(path), Size: info.Size()})
	}
	sort.SliceStable(archives, func(i, j int) bool { return archives[i].Size > archives[j].Size })
	return archives, nil
}

// ToggleWikiArchives excludes all Wikipedias and Wiktionaries, or includes
// them again if they were all excluded. Other archive filters stay selected.
func ToggleWikiArchives(excluded []string, archives []Archive) []string {
	isWiki := func(filename string) bool {
		return strings.HasPrefix(filename, "wikipedia_") || strings.HasPrefix(filename, "wiktionary_")
	}
	next := slices.DeleteFunc(slices.Clone(excluded), isWiki)
	if slices.ContainsFunc(archives, func(archive Archive) bool {
		return isWiki(archive.Filename) && !slices.Contains(excluded, archive.Filename)
	}) {
		for _, archive := range archives {
			if isWiki(archive.Filename) {
				next = append(next, archive.Filename)
			}
		}
	}
	return next
}
