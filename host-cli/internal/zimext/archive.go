package zimext

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"path"
	"sort"
	"strings"

	"github.com/stazelabs/gozim/zim"
)

// The MIME types of the documents a ZIM may hold besides its pages.
const (
	MIMEPDF  = "application/pdf"
	MIMEEPUB = "application/epub+zip"
)

// ErrEntryTooLarge is returned by ReadEntry for an entry above its limit.
var ErrEntryTooLarge = errors.New("zimext: entry too large")

// Embedded is a document stored inside a ZIM: a PDF or an EPUB entry.
type Embedded struct {
	Path  string // entry path in the content namespace: "files/guide.pdf"
	MIME  string
	Title string // the title the archive's catalogue gives it; "" without one
	Size  int64
}

// Archive is an open ZIM, for listing and reading the documents it holds.
type Archive struct {
	zim *zim.Archive
	ns  byte
	// A long directory entry can also break the library's path lookup.
	// Retain the document entries when listing needed the fallback scan.
	documentEntries map[string]zim.Entry
}

// OpenArchive opens a ZIM. Close it when done. Documents are read once each,
// so no cluster is worth caching and the file is read, not mapped: with
// gozim's defaults, a run over 180 PDFs in four ZIMs peaked at 1.6 GB, and
// at 0.9 GB with this.
func OpenArchive(filename string) (*Archive, error) {
	return openArchive(filename, false)
}

// OpenIndexArchive maps the archive for the two directory scans and article
// reads of a keyword build. The same handle can read its documents afterwards.
func OpenIndexArchive(filename string) (*Archive, error) {
	return openArchive(filename, true)
}

func openArchive(filename string, mapped bool) (*Archive, error) {
	archive, err := zim.OpenWithOptions(filename, zim.WithCacheSize(1), zim.WithMmap(mapped))
	if err != nil {
		return nil, fmt.Errorf("open zim: %w", err)
	}
	ns := byte('C')
	if archive.EntryCountByNamespace('C') == 0 {
		ns = 'A'
	}
	return &Archive{zim: archive, ns: ns}, nil
}

// Close releases the archive.
func (a *Archive) Close() error { return a.zim.Close() }

// UUID is the archive's UUID, hex encoded.
func (a *Archive) UUID() string {
	uuid := a.zim.UUID()
	return hex.EncodeToString(uuid[:])
}

// Title is the archive's Title metadata, or "".
func (a *Archive) Title() string {
	title, err := a.zim.Metadata("Title")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(title)
}

// FullTextLanguage reports the language of the embedded full-text index.
// The title-only index at X/title/xapian is insufficient for keyword search.
func (a *Archive) FullTextLanguage() string {
	for _, path := range []string{"X/fulltext/xapian", "Z//fulltextIndex/xapian"} {
		entry, err := a.zim.EntryByPath(path)
		if err == nil && !entry.IsRedirect() {
			language, _ := a.zim.Metadata("Language")
			if language = strings.TrimSpace(language); language != "" {
				return language
			}
			return "und"
		}
	}
	return ""
}

// HoldsDocuments reports, from the archive header alone, whether any entry
// is a PDF or an EPUB.
func (a *Archive) HoldsDocuments() bool {
	for _, mime := range a.zim.MIMETypes() {
		if mime == MIMEPDF || mime == MIMEEPUB {
			return true
		}
	}
	return false
}

// Documents lists the PDF and EPUB entries, sorted by path. An entry beside
// an HTML entry of the same path without the extension is left out: that is
// a book offered both ways, like every title in a Gutenberg ZIM, and its
// HTML is indexed already. Titles come from a nautilus catalogue when the
// archive has one.
func (a *Archive) Documents(ctx context.Context) ([]Embedded, error) {
	if a.zim.EntryCountByNamespace(a.ns) != 0 {
		documents, err := a.listDocuments(ctx, a.zim.AllEntriesByNamespace(a.ns), nil)
		if !isLongDirectoryEntry(err) {
			return documents, err
		}
	}
	entries := make(map[string]zim.Entry)
	documents, err := a.listDocuments(ctx, readableContentEntries(ctx, a.zim), entries)
	if err == nil {
		a.documentEntries = entries
	}
	return documents, err
}

func (a *Archive) listDocuments(ctx context.Context, entries iter.Seq2[zim.Entry, error], lookup map[string]zim.Entry) ([]Embedded, error) {
	pages := map[string]bool{}
	var found []Embedded
	var catalogue string
	for entry, err := range entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			return nil, fmt.Errorf("iterate content entries: %w", err)
		}
		if entry.IsRedirect() {
			continue
		}
		mime := entry.MIMEType()
		switch {
		case strings.HasPrefix(mime, "text/html"):
			pages[entry.Path()] = true
		case mime == MIMEPDF || mime == MIMEEPUB:
			size, err := entry.ContentSize()
			if err != nil {
				return nil, fmt.Errorf("size of %s: %w", entry.Path(), err)
			}
			found = append(found, Embedded{Path: entry.Path(), MIME: mime, Size: size})
			if lookup != nil {
				lookup[entry.Path()] = entry
			}
		case entry.Path() == nautilusCatalogue:
			if content, err := entry.ReadContent(); err == nil {
				catalogue = string(content)
			}
		}
	}
	titles := parseNautilusTitles(catalogue, a.Title())
	documents := found[:0]
	for _, doc := range found {
		if pages[strings.TrimSuffix(doc.Path, path.Ext(doc.Path))] {
			continue
		}
		doc.Title = titles[path.Base(doc.Path)]
		documents = append(documents, doc)
	}
	sort.Slice(documents, func(i, j int) bool { return documents[i].Path < documents[j].Path })
	return documents, nil
}

func isLongDirectoryEntry(err error) bool {
	return errors.Is(err, zim.ErrInvalidEntry) && strings.Contains(err.Error(), "missing null terminator")
}

// readableContentEntries works around gozim's fixed directory-entry buffer
// only on the fallback path. Verify the archive before overlooking entries
// its parser cannot read, so download corruption still fails the index.
func readableContentEntries(ctx context.Context, archive *zim.Archive) iter.Seq2[zim.Entry, error] {
	return func(yield func(zim.Entry, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(zim.Entry{}, err)
			return
		}
		if err := archive.Verify(); err != nil {
			yield(zim.Entry{}, err)
			return
		}
		for i := range archive.EntryCount() {
			if err := ctx.Err(); err != nil {
				yield(zim.Entry{}, err)
				return
			}
			entry, err := archive.EntryByIndex(i)
			if isLongDirectoryEntry(err) {
				slog.Warn("skipping ZIM entry beyond reader's directory-entry buffer", "index", i)
				continue
			}
			if err != nil {
				yield(zim.Entry{}, err)
				return
			}
			if (entry.Namespace() == 'A' || entry.Namespace() == 'C') && !yield(entry, nil) {
				return
			}
		}
	}
}

// MIMEType returns the MIME type of the content entry at entryPath, and
// false when there is none.
func (a *Archive) MIMEType(entryPath string) (string, bool) {
	entry, err := a.zim.EntryByPath(string(a.ns) + "/" + entryPath)
	if err != nil {
		return "", false
	}
	return entry.MIMEType(), true
}

// ReadEntry returns an entry's content. An entry larger than limit bytes is
// not read: ErrEntryTooLarge.
func (a *Archive) ReadEntry(entryPath string, limit int64) ([]byte, error) {
	entry, ok := a.documentEntries[entryPath]
	if !ok {
		var err error
		entry, err = a.zim.EntryByPath(string(a.ns) + "/" + entryPath)
		if err != nil {
			return nil, fmt.Errorf("find %s: %w", entryPath, err)
		}
	}
	entry, err := entry.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", entryPath, err)
	}
	size, err := entry.ContentSize()
	if err != nil {
		return nil, fmt.Errorf("size of %s: %w", entryPath, err)
	}
	if size > limit {
		return nil, fmt.Errorf("%s is %d bytes: %w", entryPath, size, ErrEntryTooLarge)
	}
	content, err := entry.ReadContentCopy()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", entryPath, err)
	}
	return content, nil
}
