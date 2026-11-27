package zimext

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

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
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open zim: %w", err)
	}
	defer file.Close()
	var header [80]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return "", fmt.Errorf("read ZIM header: %w", err)
	}
	if binary.LittleEndian.Uint32(header[:4]) != 0x044d495a {
		return "", zim.ErrInvalidMagic
	}
	if version := binary.LittleEndian.Uint16(header[4:6]); version != 5 && version != 6 {
		return "", fmt.Errorf("%w: version %d", zim.ErrUnsupportedVersion, version)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(header[8:24]), nil
}

// ExtractArticlesContext opens a ZIM archive and returns its searchable
// articles, title, and UUID. It checks cancellation while walking entries so a
// cancelled refresh cannot proceed to database replacement.
func ExtractArticlesContext(ctx context.Context, path string) ([]searchdb.Article, string, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", "", err
	}
	archive, err := OpenIndexArchive(path)
	if err != nil {
		return nil, "", "", err
	}
	defer archive.Close()

	var articles []searchdb.Article
	for article, err := range archive.Articles(ctx, IndexWorkers()) {
		if err != nil {
			return nil, "", "", err
		}
		articles = append(articles, article)
	}
	title := archive.Title()
	if title == "" {
		title = filepath.Base(path)
	}
	return articles, title, archive.UUID(), nil
}

// IndexWorkers leaves indexing usable on smaller machines and bounds the
// concurrent allocations made by HTML extraction on large ones.
func IndexWorkers() int { return min(runtime.GOMAXPROCS(0), 6) }

type articleJob struct {
	entry   zim.Entry
	content []byte
	article *searchdb.Article
}

// Articles streams articles from one open archive in cluster order. A single
// reader decompresses each cluster once. Workers extract HTML in parallel
// from independent copies, with one bounded batch of results in memory.
func (a *Archive) Articles(ctx context.Context, workers int) iter.Seq2[searchdb.Article, error] {
	return func(yield func(searchdb.Article, error) bool) {
		if err := a.eachArticle(ctx, max(1, workers), yield); err != nil {
			yield(searchdb.Article{}, err)
		}
	}
}

func (a *Archive) eachArticle(ctx context.Context, workers int, yield func(searchdb.Article, error) bool) error {
	count := a.zim.EntryCountByNamespace(a.ns)
	refs, err := articleReferences(ctx, a.zim.AllEntriesByNamespace(a.ns), count)
	if a.zim.HasTitleIndex() && (count == 0 || isLongDirectoryEntry(err)) {
		// A long auxiliary URL can make the namespace listing unreadable.
		refs, err = articleReferences(ctx, a.zim.AllEntriesByTitle(), 0)
	}
	if isLongDirectoryEntry(err) {
		refs, err = articleReferences(ctx, readableContentEntries(ctx, a.zim), 0)
	}
	if err != nil {
		return err
	}
	slices.Sort(refs)

	jobs := make(chan articleJob, workers)
	var pool, batch sync.WaitGroup
	for range workers {
		pool.Go(func() {
			for job := range jobs {
				*job.article = parseArticle(job.entry, job.content)
				batch.Done()
			}
		})
	}
	defer func() {
		close(jobs)
		pool.Wait()
	}()

	const batchSize = 128
	var articles [batchSize]searchdb.Article
	var found bool
	for refs := range slices.Chunk(refs, batchSize) {
		for i, ref := range refs {
			if err := ctx.Err(); err != nil {
				return err
			}
			entry, err := a.zim.EntryByIndex(uint32(ref))
			if err != nil {
				return fmt.Errorf("read content entry %d: %w", uint32(ref), err)
			}
			content, err := entry.ReadContentCopy()
			if err != nil {
				return fmt.Errorf("read %s: %w", entry.FullPath(), err)
			}
			batch.Add(1)
			jobs <- articleJob{entry: entry, content: content, article: &articles[i]}
		}
		batch.Wait()
		for i := range refs {
			if article := articles[i]; article.Path != "" {
				found = true
				if !yield(article, nil) {
					return nil
				}
			}
		}
		clear(articles[:])
	}
	if !found {
		return ErrNoIndexableArticles
	}
	return ctx.Err()
}

func articleReferences(ctx context.Context, entries iter.Seq2[zim.Entry, error], count int) ([]uint64, error) {
	refs := make([]uint64, 0, count)
	for entry, iterErr := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if iterErr != nil {
			return nil, fmt.Errorf("iterate content entries: %w", iterErr)
		}
		if entry.Namespace() != 'A' && entry.Namespace() != 'C' {
			continue
		}
		if entry.IsRedirect() || !isIndexableMIME(entry.MIMEType()) || IsJunkEntry(entry.MIMEType(), entry.Path()) {
			continue
		}

		refs = append(refs, uint64(entry.ClusterNum())<<32|uint64(entry.Index()))
	}
	return refs, ctx.Err()
}

func parseArticle(entry zim.Entry, content []byte) searchdb.Article {
	if strings.HasPrefix(entry.MIMEType(), "text/html") && IsTemplateTitle(htmlTitle(content)) {
		return searchdb.Article{}
	}
	raw := string(content)
	body := StripHTML(raw)
	if body == "" {
		return searchdb.Article{}
	}
	var sectionsJSON string
	if sections := ExtractSections(raw); len(sections) > 0 {
		if data, err := json.Marshal(sections); err == nil {
			sectionsJSON = string(data)
		}
	}
	return searchdb.Article{
		Path:     "/" + strings.TrimLeft(entry.Path(), "/"),
		Title:    strings.TrimSpace(entry.Title()),
		Body:     TruncateText(body, maxArticleBodyChars),
		Sections: sectionsJSON,
	}
}

// junkExtensions are the files of a ZIM's own web app that are stored with a
// text MIME type but are not content: scripts, styles, data, source maps and
// templates. openZIM's nautilus ZIMs hold a dozen of them and little else.
var junkExtensions = map[string]bool{
	".map": true, ".js": true, ".mjs": true, ".css": true, ".json": true, ".svg": true, ".wasm": true,
	".handlebars": true, ".hbs": true, ".mustache": true, ".tmpl": true, ".tpl": true, ".ejs": true,
}

// IsJunkEntry reports whether a ZIM entry is part of the archive's web app
// rather than content: a file stored as text that is not a page, named like a
// script, style, source map or template. A page is never junk by its name:
// Wikipedia keeps its "Node.js" article at the path Node.js.
func IsJunkEntry(mime, path string) bool {
	return !strings.HasPrefix(mime, "text/html") && junkExtensions[strings.ToLower(filepath.Ext(path))]
}

// HasJunkExtension reports whether a path is named like a web-app file, the
// first test of IsJunkEntry for a caller that does not know the MIME type.
func HasJunkExtension(path string) bool {
	return junkExtensions[strings.ToLower(filepath.Ext(path))]
}

var templateTitleRe = regexp.MustCompile(`^\s*\{\{[^{}]*\}\}\s*$`)

// IsTemplateTitle reports whether a page title is nothing but an unrendered
// template placeholder ("{{ title }}"): the page is a template, not an
// article. A title that mentions braces ("How to escape {{ in Jinja") is one.
func IsTemplateTitle(title string) bool {
	return templateTitleRe.MatchString(title)
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
