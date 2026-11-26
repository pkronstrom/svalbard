package zimext

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/stazelabs/gozim/zim"
)

const maxArticleBodyChars = 4000

// ErrNoIndexableArticles is returned for an archive without a single article
// with text. The caller decides whether that fails it: one whose content is
// documents (PDFs, EPUBs) still has something to index.
var ErrNoIndexableArticles = errors.New("zimext: no indexable articles found")

// ExtractArticles opens a ZIM archive and returns searchable articles plus a
// human-readable archive title.
func ExtractArticles(path string) ([]searchdb.Article, string, error) {
	articles, title, _, err := ExtractArticlesContext(context.Background(), path)
	return articles, title, err
}

// ArchiveUUID returns the UUID recorded in a ZIM header without reading its
// article payloads.
func ArchiveUUID(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	archive, err := zim.Open(path)
	if err != nil {
		return "", fmt.Errorf("open zim: %w", err)
	}
	defer archive.Close()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	uuid := archive.UUID()
	return hex.EncodeToString(uuid[:]), nil
}

// ExtractArticlesContext opens a ZIM archive and returns its searchable
// articles, title, and UUID. It checks cancellation while walking entries so a
// cancelled refresh cannot proceed to database replacement.
func ExtractArticlesContext(ctx context.Context, path string) ([]searchdb.Article, string, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", "", err
	}
	archive, err := zim.Open(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("open zim: %w", err)
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
		if err := ctx.Err(); err != nil {
			return nil, "", "", err
		}
		if iterErr != nil {
			return nil, "", "", fmt.Errorf("iterate content entries: %w", iterErr)
		}
		if entry.IsRedirect() || !isIndexableMIME(entry.MIMEType()) || IsJunkPath(entry.Path()) {
			continue
		}

		content, err := entry.ReadContent()
		if err != nil {
			return nil, "", "", fmt.Errorf("read %s: %w", entry.FullPath(), err)
		}

		if strings.HasPrefix(entry.MIMEType(), "text/html") && IsTemplateTitle(htmlTitle(content)) {
			continue
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
		return nil, "", "", ErrNoIndexableArticles
	}

	uuid := archive.UUID()
	return articles, archiveTitle, hex.EncodeToString(uuid[:]), nil
}

// junkExtensions are the files of a ZIM's own web app that are stored with a
// text MIME type but are not content: scripts, styles, data, source maps and
// templates. openZIM's nautilus ZIMs hold a dozen of them and little else.
var junkExtensions = map[string]bool{
	".map": true, ".js": true, ".mjs": true, ".css": true, ".json": true, ".svg": true, ".wasm": true,
	".handlebars": true, ".hbs": true, ".mustache": true, ".tmpl": true, ".tpl": true, ".ejs": true,
}

// IsJunkPath reports whether a ZIM entry is part of the archive's web app
// rather than content, by its path.
func IsJunkPath(path string) bool {
	return junkExtensions[strings.ToLower(filepath.Ext(path))]
}

// IsTemplateTitle reports whether a page title is an unrendered template
// placeholder ("{{ title }}"): the page is a template, not an article.
func IsTemplateTitle(title string) bool {
	return strings.Contains(title, "{{")
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// htmlTitle is the text of a page's <title>, or "".
func htmlTitle(content []byte) string {
	if m := titleRe.FindSubmatch(content); m != nil {
		return string(m[1])
	}
	return ""
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
