package zimext

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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
}

// OpenArchive opens a ZIM. Close it when done.
func OpenArchive(filename string) (*Archive, error) {
	archive, err := zim.Open(filename)
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
	pages := map[string]bool{}
	var found []Embedded
	var catalogue string
	for entry, err := range a.zim.AllEntriesByNamespace(a.ns) {
		if err := ctx.Err(); err != nil {
			return nil, err
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

// ReadEntry returns an entry's content. An entry larger than limit bytes is
// not read: ErrEntryTooLarge.
func (a *Archive) ReadEntry(entryPath string, limit int64) ([]byte, error) {
	entry, err := a.zim.EntryByPath(string(a.ns) + "/" + entryPath)
	if err != nil {
		return nil, fmt.Errorf("find %s: %w", entryPath, err)
	}
	entry, err = entry.Resolve()
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
