package zimext

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/stazelabs/gozim/zim"
)

const maxArticleBodyChars = 4000

var errNoIndexableArticles = errors.New("zimext: no indexable articles found")

// ExtractArticles opens a ZIM archive and returns searchable articles plus a
// human-readable archive title.
func ExtractArticles(path string) ([]searchdb.Article, string, error) {
	archive, err := zim.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open zim: %w", err)
	}
	defer archive.Close()

	archiveTitle := filepath.Base(path)
	if metaTitle, err := archive.Metadata("Title"); err == nil && strings.TrimSpace(metaTitle) != "" {
		archiveTitle = strings.TrimSpace(metaTitle)
	}

	// Newer ZIM files (new namespace scheme) store content in 'C'; older ones
	// use 'A' for articles.  Try 'C' first, fall back to 'A'.
	ns := byte('C')
	if archive.EntryCountByNamespace('C') == 0 {
		ns = 'A'
	}

	articles := make([]searchdb.Article, 0, archive.EntryCountByNamespace(ns))
	for entry, iterErr := range archive.AllEntriesByNamespace(ns) {
		if iterErr != nil {
			return nil, "", fmt.Errorf("iterate content entries: %w", iterErr)
		}
		if entry.IsRedirect() || !isIndexableMIME(entry.MIMEType()) {
			continue
		}

		content, err := entry.ReadContent()
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", entry.FullPath(), err)
		}

		// Extract sections from raw HTML before stripping.
		sections := ExtractSections(string(content))
		var sectionsJSON string
		if len(sections) > 0 {
			if data, err := json.Marshal(sections); err == nil {
				sectionsJSON = string(data)
			}
		}

		body := StripHTML(string(content))
		if body == "" {
			continue
		}

		articles = append(articles, searchdb.Article{
			Path:     "/" + strings.TrimLeft(entry.Path(), "/"),
			Title:    strings.TrimSpace(entry.Title()),
			Body:     TruncateText(body, maxArticleBodyChars),
			Sections: sectionsJSON,
		})
	}

	if len(articles) == 0 {
		return nil, "", errNoIndexableArticles
	}

	return articles, archiveTitle, nil
}

func isIndexableMIME(mime string) bool {
	switch {
	case strings.HasPrefix(mime, "text/html"):
		return true
	case strings.HasPrefix(mime, "text/plain"):
		return true
	default:
		return false
	}
}
