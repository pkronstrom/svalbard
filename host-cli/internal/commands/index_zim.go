package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

// A ZIM is indexed in two passes. The HTML pass makes its pages the articles
// of the ZIM's source, leaving out the files of the archive's own web app.
// The document pass reads the PDFs and EPUBs stored inside it, each into a
// source of its own named "<zim>/<entry path>" with one article per page,
// as documents under books/ are. The ZIM source's reader version is written
// last: until it is, the archive is visited again by the next run, which
// skips the documents already done.

// zimReaderVersion is the version of the ZIM pass. A ZIM indexed by an
// older one is read again where the new one would index it differently:
// version 2 skips web-app files and indexes the documents inside.
const zimReaderVersion = 2

// maxEmbeddedDocumentBytes bounds a document read out of a ZIM into memory.
var maxEmbeddedDocumentBytes int64 = 512 << 20

// embeddedKinds maps the MIME type of a document inside a ZIM to the
// documentKinds extension that reads it.
var embeddedKinds = map[string]string{zimext.MIMEPDF: ".pdf", zimext.MIMEEPUB: ".epub"}

// embeddedSourceName is the source name of a document inside a ZIM.
func embeddedSourceName(zimFilename, entryPath string) string {
	return zimFilename + "/" + entryPath
}

// readerSet opens each kind of document reader once per index run, when
// the first document of the kind is read.
type readerSet struct {
	readers map[string]documentReader
}

func newReaderSet() *readerSet { return &readerSet{readers: map[string]documentReader{}} }

func (s *readerSet) get(ext string) (documentReader, error) {
	if reader, ok := s.readers[ext]; ok {
		return reader, nil
	}
	kind, ok := documentKinds[ext]
	if !ok {
		return nil, fmt.Errorf("no reader for %s files", ext)
	}
	reader, err := kind.open()
	if err != nil {
		return nil, err
	}
	s.readers[ext] = reader
	return reader, nil
}

func (s *readerSet) Close() {
	for _, reader := range s.readers {
		reader.Close()
	}
}

// indexZIM brings one ZIM's sources up to date and returns the names of all
// of them, the ZIM's own and its documents', for the caller's prune.
func indexZIM(ctx context.Context, db *searchdb.DB, root, zf string, force bool, indexed map[string]searchdb.Source, readers *readerSet, w io.Writer, notify func(IndexProgress)) ([]string, error) {
	path := filepath.Join(root, "zim", zf)
	identity, err := archiveSourceIdentity(ctx, path, zf)
	if err != nil {
		notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
		return nil, fmt.Errorf("reading identity for %s: %w", zf, err)
	}
	present := []string{zf}
	stored, known := indexed[zf]
	identityCurrent := known && stored.IdentityCurrent(identity)
	if !force && identityCurrent && stored.ReaderVersion >= zimReaderVersion {
		for name := range indexed {
			if strings.HasPrefix(name, zf+"/") {
				present = append(present, name)
			}
		}
		notify(IndexProgress{File: zf, Status: "skip", Detail: "already indexed"})
		return present, nil
	}

	archive, err := zimext.OpenIndexArchive(path)
	if err != nil {
		notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
		return nil, fmt.Errorf("opening %s: %w", zf, err)
	}
	defer archive.Close()

	htmlStale := force || !identityCurrent
	if !htmlStale && archive.HoldsDocuments() {
		// Indexed by an older reader: its pages need extracting again only
		// if it kept files the HTML pass now leaves out. That web app is
		// what an archive of documents wraps them in; an archive without
		// documents, such as Wikipedia, is not scanned for it, and any it
		// kept clears when it is next updated.
		err := db.EachArticleHead(ctx, zf, func(head searchdb.ArticleHead) bool {
			path := strings.TrimLeft(head.Path, "/")
			if zimext.HasJunkExtension(path) {
				if mime, ok := archive.MIMEType(path); !ok || zimext.IsJunkEntry(mime, path) {
					htmlStale = true
				}
			}
			if zimext.IsTemplateTitle(head.Title) {
				htmlStale = true
			}
			return !htmlStale
		})
		if err != nil {
			return nil, err
		}
	}
	if htmlStale {
		if err := indexZIMPages(ctx, db, archive, path, zf, identity, notify); err != nil {
			return nil, err
		}
	}
	if archive.HoldsDocuments() {
		names, err := indexEmbedded(ctx, db, archive, zf, identity, force, indexed, readers, w, notify)
		if err != nil {
			return nil, err
		}
		present = append(present, names...)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := db.SetSourceReaderVersion(ctx, zf, zimReaderVersion); err != nil {
		return nil, err
	}
	return present, nil
}

// indexZIMPages is the HTML pass. The source it writes has reader version 0,
// so the archive counts as unfinished until its documents are done too.
func indexZIMPages(ctx context.Context, db *searchdb.DB, archive *zimext.Archive, path, zf string, identity searchdb.Source, notify func(IndexProgress)) error {
	workers := zimext.IndexWorkers()
	notify(IndexProgress{File: zf, Status: "extracting", Detail: "extracting articles..."})
	if err := ctx.Err(); err != nil {
		return err
	}
	title := archive.Title()
	if title == "" {
		title = strings.TrimSuffix(zf, filepath.Ext(zf))
	}
	identity.Title = title
	identity.ReaderVersion = 0
	var count int64
	err := db.ReplaceSourceStream(ctx, identity, func(yield func(searchdb.Article, error) bool) {
		for article, err := range archive.Articles(ctx, workers) {
			if errors.Is(err, zimext.ErrNoIndexableArticles) && archive.HoldsDocuments() {
				// Its content is the documents inside.
				break
			}
			if count == 0 && err == nil {
				notify(IndexProgress{File: zf, Status: "extracting", Detail: "inserting articles..."})
			}
			if !yield(article, err) || err != nil {
				return
			}
			count++
			if count%50000 == 0 {
				notify(IndexProgress{File: zf, Status: "extracting", Detail: fmt.Sprintf("processed %d articles", count), Current: count})
			}
		}
		current, err := archiveSourceIdentity(ctx, path, zf)
		if err != nil {
			yield(searchdb.Article{}, fmt.Errorf("rechecking source identity: %w", err))
		} else if current.UUID != archive.UUID() || !identity.IdentityCurrent(current) {
			yield(searchdb.Article{}, fmt.Errorf("source %s changed during extraction; retry indexing", zf))
		}
	})
	if err != nil {
		if cancelErr := ctx.Err(); cancelErr != nil {
			return cancelErr
		}
		notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
		return fmt.Errorf("replacing articles for %s: %w", zf, err)
	}
	notify(IndexProgress{
		File: zf, Status: "done",
		Detail: fmt.Sprintf("%d articles", count), Current: count, Total: count,
	})
	return nil
}

// indexEmbedded is the document pass: every PDF and EPUB inside the ZIM
// whose source is not current is read and replaced. One that cannot be read
// is recorded with a note, as a document under books/ is.
func indexEmbedded(ctx context.Context, db *searchdb.DB, archive *zimext.Archive, zf string, identity searchdb.Source, force bool, indexed map[string]searchdb.Source, readers *readerSet, w io.Writer, notify func(IndexProgress)) ([]string, error) {
	documents, err := archive.Documents(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing documents in %s: %w", zf, err)
	}
	counts, err := sourceArticleCounts(db)
	if err != nil {
		return nil, err
	}
	type outcome struct {
		entry string
		pages int64
		note  string
	}
	var outcomes []outcome
	names := make([]string, 0, len(documents))
	for _, doc := range documents {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := embeddedSourceName(zf, doc.Path)
		names = append(names, name)
		want := searchdb.Source{
			Filename:      name,
			UUID:          "zim:" + identity.UUID,
			Size:          identity.Size,
			ModTimeNS:     identity.ModTimeNS,
			ReaderVersion: zimReaderVersion,
		}
		if stored, ok := indexed[name]; ok && !force && stored.IdentityCurrent(want) && stored.ReaderVersion == zimReaderVersion {
			outcomes = append(outcomes, outcome{doc.Path, counts[name], stored.Note})
			continue
		}

		notify(IndexProgress{File: name, Status: "extracting", Detail: "reading..."})
		ext := embeddedKinds[doc.MIME]
		var text documentText
		if doc.Size > maxEmbeddedDocumentBytes {
			text.Note = fmt.Sprintf("too large to index (%d MB)", doc.Size>>20)
		} else if data, err := archive.ReadEntry(doc.Path, maxEmbeddedDocumentBytes); err != nil {
			text.Note = "unreadable: " + err.Error()
		} else {
			reader, err := readers.get(ext)
			if err != nil {
				return nil, err
			}
			text, err = reader.Read(ctx, documentSource{Name: name, Data: bytes.NewReader(data), Size: int64(len(data))})
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err != nil {
				text = documentText{Note: err.Error()}
			}
		}
		want.Title = documentTitle(doc.Title, text.Title, doc.Path)
		want.Note = text.Note
		articles := documentArticles(want.Title, text.Parts)
		if err := db.ReplaceSource(ctx, want, articles); err != nil {
			return nil, fmt.Errorf("replacing articles for %s: %w", name, err)
		}
		result := documentOutcome(int64(len(articles)), documentKinds[ext].unit, text.Note)
		notify(IndexProgress{File: name, Status: "done", Detail: result, Current: int64(len(articles)), Total: int64(len(articles))})
		outcomes = append(outcomes, outcome{doc.Path, int64(len(articles)), text.Note})
	}

	var pages int64
	var unsearchable []outcome
	for _, o := range outcomes {
		pages += o.pages
		if o.pages == 0 {
			unsearchable = append(unsearchable, o)
		}
	}
	line := fmt.Sprintf("  %s: %s, %s", strings.TrimSuffix(zf, filepath.Ext(zf)), plural(int64(len(outcomes)), "document"), plural(pages, "page"))
	if len(unsearchable) > 0 {
		line += fmt.Sprintf("; %d not searchable", len(unsearchable))
	}
	fmt.Fprintln(w, "Documents in "+zf+":")
	fmt.Fprintln(w, line)
	width := 0
	for _, o := range unsearchable {
		width = max(width, len(o.entry))
	}
	for _, o := range unsearchable {
		fmt.Fprintf(w, "    %-*s  %s\n", width, o.entry, documentOutcome(0, "page", o.note))
	}
	return names, nil
}
