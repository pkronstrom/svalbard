package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/pdfext"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

const (
	// documentsDir is the vault directory of document collections: PDFs and
	// EPUBs, whose source name in the index is the vault-relative path.
	documentsDir = "books"
	// maxDocumentPartChars caps the text indexed for one page.
	maxDocumentPartChars = 16000
	// givenTitlesKey is the index's record of the title the collection file
	// gave each document when it was last indexed (JSON, path to title; no
	// entry for a document without one). The indexed title alone cannot tell
	// a title that was given from the file's own, so a title that is later
	// removed from the collection file would never be noticed.
	givenTitlesKey = "document_titles"
	// documentTimeout bounds the extraction of one file, so a page PDFium
	// cannot get through does not hold up the rest of the index.
	documentTimeout = 20 * time.Minute
)

// documentKind is one kind of file the document pass indexes.
type documentKind struct {
	// unit names a part of the document in reports: "page".
	unit string
	// open starts the reader. It is called once per index run, and only when
	// a file of the kind has to be read.
	open func() (documentReader, error)
}

// documentKinds decides which files are indexed, by extension, and what reads
// them. A kind is added here and nowhere else; documentRoots says where on
// the vault files of it are looked for.
var documentKinds = map[string]documentKind{
	".pdf":  {unit: "page", open: openPDFReader},
	".epub": {unit: "part", open: openEPUBReader},
	".md":   {unit: "section", open: openMarkdownReader},
}

// documentRoot is a vault directory whose files are indexed as documents,
// each a source named by its vault-relative path.
type documentRoot struct {
	dir string
	// extensions are the documentKinds looked for below dir.
	extensions []string
	// folders, when set, lists the directories below dir that are walked in
	// place of dir itself, relative to it.
	folders func(base string) ([]string, error)
	// within, when set, narrows the files found to those it accepts, by
	// vault-relative path.
	within func(rel string) bool
}

// documentRoots are the indexed directories: document collections, agent
// skills, and the quickstarts of JS libraries. Skills and quickstarts are
// markdown, kept apart from the knowledge sources when the drive searches.
var documentRoots = []documentRoot{
	{dir: documentsDir, extensions: []string{".pdf", ".epub"}},
	{dir: "skills", extensions: []string{".md"}},
	{dir: "lib/js", extensions: []string{".md"}, folders: libraryDocFolders, within: isLibraryDoc},
}

// libraryDocFolders lists the docs/ folder of each library recipe below
// lib/js. Nothing else there is walked: the packages in node_modules hold
// thousands of files and markdown of their own.
func libraryDocFolders(base string) ([]string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var folders []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		// Stat, not the entry's type: a recipe folder may be a symlink. One
		// that cannot be looked at is not an absent one: its quickstarts
		// would be pruned.
		info, err := os.Stat(filepath.Join(base, entry.Name()))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err != nil || !info.IsDir() {
			continue
		}
		docs := filepath.Join(entry.Name(), "docs")
		if info, err := os.Stat(filepath.Join(base, docs)); err == nil && info.IsDir() {
			folders = append(folders, docs)
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return folders, nil
}

// isLibraryDoc reports whether a vault path is a library quickstart:
// lib/js/<recipe>/docs/<file>.md, and nothing deeper.
func isLibraryDoc(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 5 && parts[0] == "lib" && parts[1] == "js" && parts[3] == "docs"
}

// documentReader extracts the text of one kind of document.
type documentReader interface {
	// Read returns what the document holds. A document that cannot be read,
	// or has no text, is not an error: it comes back with a Note and few or
	// no parts. Only cancellation is returned as an error.
	Read(ctx context.Context, src documentSource) (documentText, error)
	Close() error
}

// readSeekerAt is what a reader may need of a document's bytes: a PDF is
// read with seeks, an EPUB as a zip.
type readSeekerAt interface {
	io.ReadSeeker
	io.ReaderAt
}

// documentSource is one document to read: a file on the vault, or an entry
// read out of a ZIM into memory.
type documentSource struct {
	// Name is the document's source name, the vault path or
	// "<zim>/<entry>"; a title falls back to it.
	Name string
	Data readSeekerAt
	Size int64
}

// fileSource opens a vault file as a documentSource named rel. The returned
// function closes it.
func fileSource(file, rel string) (documentSource, func() error, error) {
	f, err := os.Open(file)
	if err != nil {
		return documentSource{}, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return documentSource{}, nil, err
	}
	return documentSource{Name: rel, Data: f, Size: info.Size()}, f.Close, nil
}

// documentText is what a reader found in one file.
type documentText struct {
	Title string         // the file's own title, when it states one
	Parts []documentPart // the parts that have text, in order
	Note  string         // why the file is not, or not fully, searchable
}

// documentPart becomes one article: a page of a PDF.
type documentPart struct {
	Path    string // article path within the source: "page/12"
	Label   string // how the part is cited after the title: "p. 12"
	Section string // the heading the part falls under, if known
	Text    string // lines separated by "\n"
}

// IsDocumentSource reports whether an index source name is a document path
// rather than a ZIM file name.
func IsDocumentSource(filename string) bool {
	return documentRootOf(filename) != ""
}

// documentRootOf names the document root an index source was read from:
// "books", "skills" or "lib/js", and "" for a ZIM file name.
func documentRootOf(filename string) string {
	for _, root := range documentRoots {
		if strings.HasPrefix(filename, root.dir+"/") {
			return root.dir
		}
	}
	return ""
}

// ScanDocuments returns the vault-relative, slash-separated paths of the
// indexable documents below every document root, sorted. A root that does not
// exist has none; ScanDocumentRoots says which those are.
func ScanDocuments(root string) ([]string, error) {
	documents, _, err := ScanDocumentRoots(root)
	return documents, err
}

// ScanDocumentRoots lists the indexable documents below every document root,
// sorted, and the roots that do not exist, for the caller to judge: a vault
// that was never given books has no books/, and so has one that is only half
// mounted. Hidden entries, including the "._" sidecars macOS writes on exFAT,
// are skipped. Any other failure to read a root or a folder below it is an
// error, so that a tree that could not be listed is never taken for an empty
// one. A root may be a symlink.
func ScanDocumentRoots(root string) (documents, missing []string, err error) {
	for _, documentRoot := range documentRoots {
		found, err := documentRoot.scan(root)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, documentRoot.dir)
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("scanning %s directory: %w", documentRoot.dir, err)
		}
		documents = append(documents, found...)
	}
	sort.Strings(documents)
	return documents, missing, nil
}

// scan lists one root's documents. A root that is not there is fs.ErrNotExist.
func (r documentRoot) scan(root string) ([]string, error) {
	// WalkDir does not follow a symlink given as its root.
	base, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(r.dir)))
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(base); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, errors.New("not a directory")
	}
	folders := []string{"."}
	if r.folders != nil {
		if folders, err = r.folders(base); err != nil {
			return nil, err
		}
	}
	var documents []string
	for _, folder := range folders {
		top := filepath.Join(base, folder)
		err = filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p != top && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if _, ok := documentKinds[ext]; !ok || !slices.Contains(r.extensions, ext) {
				return nil
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			rel = path.Join(r.dir, filepath.ToSlash(rel))
			if r.within != nil && !r.within(rel) {
				return nil
			}
			documents = append(documents, rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return documents, nil
}

// indexDocuments reconciles the index with the documents on the vault. A
// document whose size and modification time match its indexed source is
// skipped without being opened; any other is hashed, read and replaced as one
// source with an article per part. Scanned, password-protected and broken
// files are recorded with a note and no articles, so they are reported once
// and not read again until the file changes. A file that cannot be read at
// all (a permission or disk error) is reported and skipped, keeping whatever
// the index held for it; one document never stops the others. The caller
// holds the writer lock and prunes sources that are no longer present.
func indexDocuments(ctx context.Context, root string, documents []string, force bool, db *searchdb.DB, indexed map[string]searchdb.Source, readers *readerSet, w io.Writer, notify func(IndexProgress)) error {
	if len(documents) == 0 {
		return nil
	}
	metadata, err := toolkit.ReadDocumentsMetadata(root)
	if err != nil {
		return fmt.Errorf("reading document metadata: %w", err)
	}

	given := loadGivenTitles(db)
	recorded := make(map[string]string, len(given))
	for _, rel := range documents {
		if title := given[rel]; title != "" {
			recorded[rel] = title
		}
	}

	type job struct {
		rel  string
		info os.FileInfo
	}
	var pending []job
	var noted []string // unchanged documents whose note is worth repeating
	unchanged := 0
	for _, rel := range documents {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			// Reported when its turn comes; the stat is repeated there.
			pending = append(pending, job{rel, nil})
			continue
		}
		stored := indexed[rel]
		// The identity check is size and time only; hashing every file on
		// every run would read the whole stick. The title the collection
		// file gives is part of every article title, so setting, changing or
		// removing it since the last run makes the document stale too.
		current := stored.UUID != "" && stored.Size == info.Size() && stored.ModTimeNS == info.ModTime().UnixNano() &&
			given[rel] == metadata[rel].Title
		switch {
		case force || !current:
			pending = append(pending, job{rel, info})
		case stored.Note != "":
			noted = append(noted, rel)
			notify(IndexProgress{File: rel, Status: "skip", Detail: stored.Note})
		default:
			unchanged++
			notify(IndexProgress{File: rel, Status: "skip", Detail: "already indexed"})
		}
	}
	if len(pending) == 0 && len(noted) == 0 {
		return saveGivenTitles(ctx, db, given, recorded)
	}

	width := 0
	for _, rel := range noted {
		width = max(width, len(displayDocument(rel)))
	}
	for _, doc := range pending {
		width = max(width, len(displayDocument(doc.rel)))
	}
	report := func(rel, outcome string) {
		fmt.Fprintf(w, "  %-*s  %s\n", width, displayDocument(rel), outcome)
	}
	fmt.Fprintln(w, "Indexing documents:")
	if len(noted) > 0 {
		counts, err := sourceArticleCounts(db)
		if err != nil {
			return err
		}
		for _, rel := range noted {
			report(rel, documentOutcome(counts[rel], documentKindOf(rel).unit, indexed[rel].Note))
		}
	}

	for _, doc := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		ext := strings.ToLower(path.Ext(doc.rel))
		kind := documentKinds[ext]
		reader, err := readers.get(ext)
		if err != nil {
			return err
		}
		notify(IndexProgress{File: doc.rel, Status: "extracting", Detail: "reading " + kind.unit + "s..."})
		// skip leaves the index as it was for this document and goes on.
		skip := func(reason string) {
			report(doc.rel, "skipped: "+reason)
			notify(IndexProgress{File: doc.rel, Status: "failed", Detail: reason})
		}

		file := filepath.Join(root, filepath.FromSlash(doc.rel))
		info := doc.info
		if info == nil {
			if info, err = os.Stat(file); err != nil {
				skip("cannot read file: " + unwrapPathError(err))
				continue
			}
		}
		identity, err := documentIdentity(ctx, file, doc.rel, info)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			skip("cannot read file: " + unwrapPathError(err))
			continue
		}
		src, closeFile, err := fileSource(file, doc.rel)
		if err != nil {
			skip("cannot read file: " + unwrapPathError(err))
			continue
		}
		text, err := reader.Read(ctx, src)
		closeFile()
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			skip(err.Error())
			continue
		}
		after, err := os.Stat(file)
		if err != nil || after.Size() != identity.Size || after.ModTime().UnixNano() != identity.ModTimeNS {
			skip("the file changed while it was read; run the index again")
			continue
		}

		identity.Title = documentTitle(metadata[doc.rel].Title, text.Title, doc.rel)
		identity.Note = text.Note
		articles := documentArticles(identity.Title, text.Parts)
		if err := db.ReplaceSource(ctx, identity, articles); err != nil {
			return fmt.Errorf("replacing articles for %s: %w", doc.rel, err)
		}
		if title := metadata[doc.rel].Title; title != "" {
			recorded[doc.rel] = title
		} else {
			delete(recorded, doc.rel)
		}
		outcome := documentOutcome(int64(len(articles)), kind.unit, text.Note)
		report(doc.rel, outcome)
		notify(IndexProgress{File: doc.rel, Status: "done", Detail: outcome, Current: int64(len(articles)), Total: int64(len(articles))})
	}
	if unchanged > 0 {
		fmt.Fprintf(w, "  %s unchanged\n", plural(int64(unchanged), "document"))
	}
	return saveGivenTitles(ctx, db, given, recorded)
}

func loadGivenTitles(db *searchdb.DB) map[string]string {
	titles := map[string]string{}
	if raw, err := db.GetMeta(givenTitlesKey); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &titles)
	}
	return titles
}

// saveGivenTitles stores the record when it changed. A run that stops early
// leaves the old record, so at worst a document is read once more.
func saveGivenTitles(ctx context.Context, db *searchdb.DB, before, after map[string]string) error {
	if maps.Equal(before, after) {
		return nil
	}
	raw, err := json.Marshal(after)
	if err != nil {
		return err
	}
	if err := db.SetMetaContext(ctx, givenTitlesKey, string(raw)); err != nil {
		return fmt.Errorf("recording document titles: %w", err)
	}
	return nil
}

// documentIdentity hashes a document. The hash names the content; size and
// modification time are what later runs compare.
func documentIdentity(ctx context.Context, file, rel string, info os.FileInfo) (searchdb.Source, error) {
	f, err := os.Open(file)
	if err != nil {
		return searchdb.Source{}, err
	}
	defer f.Close()
	hash := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return searchdb.Source{}, err
		}
		n, readErr := f.Read(buf)
		hash.Write(buf[:n])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return searchdb.Source{}, readErr
		}
	}
	return searchdb.Source{
		Filename:  rel,
		UUID:      "sha256:" + hex.EncodeToString(hash.Sum(nil)),
		Size:      info.Size(),
		ModTimeNS: info.ModTime().UnixNano(),
	}, nil
}

// unwrapPathError drops the path from a file error; the report line names it.
func unwrapPathError(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

var titleFileSuffixRe = regexp.MustCompile(`(?i)\.(pdf|docx?|indd|qxd|dvi|tex|pptx?|pages|odt)$`)

// documentTitle picks the title a document is cited by: the one given in the
// collection file, else the file's own, else the file name. Many PDFs carry
// the name of the file they were exported from as their title; that is no
// better than the name on the vault.
func documentTitle(given, own, rel string) string {
	if given = strings.TrimSpace(given); given != "" {
		return given
	}
	own = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(own), "Microsoft Word - "))
	if own != "" && !strings.EqualFold(own, "untitled") && !strings.EqualFold(own, "PowerPoint Presentation") && !titleFileSuffixRe.MatchString(own) {
		return own
	}
	base := path.Base(rel)
	return strings.TrimSuffix(base, path.Ext(base))
}

// documentArticles builds one article per part. The title carries the
// citation ("Manual, p. 12 · Cooling system"); Body is one line for keyword
// search and snippets, and Sections keeps the line structure for reading the
// part back and for semantic chunking, with no heading of its own so chunks
// are headed by the article title.
func documentArticles(title string, parts []documentPart) []searchdb.Article {
	articles := make([]searchdb.Article, 0, len(parts))
	for _, part := range parts {
		body := strings.Join(strings.Fields(part.Text), " ")
		if body == "" {
			continue
		}
		heading := title + ", " + part.Label
		if part.Section != "" && !strings.EqualFold(part.Section, title) {
			heading += " · " + part.Section
		}
		sections, _ := json.Marshal([]zimext.Section{{Body: zimext.TruncateText(part.Text, maxDocumentPartChars)}})
		articles = append(articles, searchdb.Article{
			Path:     part.Path,
			Title:    heading,
			Body:     zimext.TruncateText(body, maxDocumentPartChars),
			Sections: string(sections),
		})
	}
	return articles
}

// documentOutcome is the report line for one document.
func documentOutcome(articles int64, unit, note string) string {
	switch {
	case articles == 0 && note == "":
		return "not searchable: no text found"
	case articles == 0:
		return "not searchable: " + note
	case note != "":
		return plural(articles, unit) + "; " + note
	}
	return plural(articles, unit)
}

func plural(n int64, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// displayDocument shortens a source path for reports: a collection's document
// is named without the books/ prefix. Skills and quickstarts keep their path.
func displayDocument(rel string) string {
	return strings.TrimPrefix(rel, documentsDir+"/")
}

func documentKindOf(rel string) documentKind {
	return documentKinds[strings.ToLower(path.Ext(rel))]
}

func sourceArticleCounts(db *searchdb.DB) (map[string]int64, error) {
	sources, err := db.Sources()
	if err != nil {
		return nil, fmt.Errorf("counting indexed articles: %w", err)
	}
	counts := make(map[string]int64, len(sources))
	for _, source := range sources {
		counts[source.Filename] = source.ArticleCount
	}
	return counts, nil
}

// pdfReader reads PDFs with PDFium.
type pdfReader struct {
	extractor *pdfext.Extractor
}

func openPDFReader() (documentReader, error) {
	extractor, err := pdfext.Open()
	if err != nil {
		return nil, err
	}
	return pdfReader{extractor}, nil
}

func (r pdfReader) Close() error { return r.extractor.Close() }

var pdfiumCodeRe = regexp.MustCompile(`^\d+: `)

func (r pdfReader) Read(ctx context.Context, src documentSource) (documentText, error) {
	limited, cancel := context.WithTimeout(ctx, documentTimeout)
	defer cancel()
	doc, err := r.extractor.DocumentFrom(limited, src.Data, src.Size)
	switch {
	case ctx.Err() != nil:
		return documentText{}, ctx.Err()
	case errors.Is(err, context.DeadlineExceeded):
		return documentText{Note: fmt.Sprintf("unreadable PDF: gave up after %s", documentTimeout)}, nil
	case errors.Is(err, pdfext.ErrPassword):
		return documentText{Note: "password-protected"}, nil
	case err != nil:
		return documentText{Note: "unreadable PDF: " + pdfiumCodeRe.ReplaceAllString(err.Error(), "")}, nil
	}
	text := documentText{Title: doc.Title, Note: pdfext.Classify(doc)}
	for _, page := range doc.Pages {
		if page.Text == "" {
			continue
		}
		text.Parts = append(text.Parts, documentPart{
			Path:    fmt.Sprintf("page/%d", page.Number),
			Label:   fmt.Sprintf("p. %d", page.Number),
			Section: page.Section,
			Text:    page.Text,
		})
	}
	return text, nil
}
