// Package pdfext reads the text of PDF files page by page with PDFium
// compiled to WebAssembly. The module runs inside wazero with no access to
// the disk or the network: a file reaches it only through a reader that Go
// holds open.
package pdfext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	pdfiumerrors "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// ErrPassword is returned for a file that needs a password to be opened.
var ErrPassword = errors.New("password-protected")

const (
	// imageOnlyMaxChars is the most non-space characters a page may hold and
	// still count as an image without a text layer (a stamp or page number).
	imageOnlyMaxChars = 20
	// partlyScannedShare is the share of image-only pages from which a
	// document that does have text is reported as partly scanned.
	partlyScannedShare = 0.25
	maxSectionRunes    = 80
	maxBookmarks       = 20000
)

// Page is one physical page of a document.
type Page struct {
	Number  int    // 1-based
	Section string // the outline entry the page belongs to; empty without an outline
	Text    string // lines as PDFium lays them out, trimmed; empty without a text layer
	Images  int    // image objects on the page
}

// ImageOnly reports whether the page is a picture without a text layer,
// which is what a scanned page looks like.
func (p Page) ImageOnly() bool {
	if p.Images == 0 {
		return false
	}
	chars := 0
	for _, r := range p.Text {
		if !unicode.IsSpace(r) {
			chars++
			if chars >= imageOnlyMaxChars {
				return false
			}
		}
	}
	return true
}

// Doc is an extracted document.
type Doc struct {
	Title string // the Title metadata field; often empty
	Pages []Page // every page in order, with or without text
}

// Classify returns a note for a document that is not fully searchable, or
// "" for one that is.
func Classify(doc Doc) string {
	scanned, withText := 0, 0
	for _, page := range doc.Pages {
		switch {
		case page.ImageOnly():
			scanned++
		case strings.TrimSpace(page.Text) != "":
			withText++
		}
	}
	switch {
	case withText == 0 && scanned > 0:
		return "scanned: no text layer, needs OCR"
	case withText == 0:
		return "no text found"
	case float64(scanned) >= partlyScannedShare*float64(len(doc.Pages)):
		return fmt.Sprintf("%d of %d pages are scanned images without text", scanned, len(doc.Pages))
	}
	return ""
}

// Extractor holds the compiled PDFium module. Each document gets a fresh
// instance of it, so the memory one file needed is released before the next
// and a file that broke its instance cannot affect another.
type Extractor struct {
	pool pdfium.Pool
}

// Open compiles the PDFium module, which takes about a second. Close it when
// the run is done.
func Open() (*Extractor, error) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  0,
		MaxIdle:  1,
		MaxTotal: 1,
		// No mounts: the module sees no files of its own.
		FSConfig: wazero.NewFSConfig(),
		// PDFium's setjmp/longjmp is lowered to exception handling. Closing
		// on context cancellation lets Document stop a page that hangs.
		RuntimeConfig: wazero.NewRuntimeConfig().
			WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling).
			WithCloseOnContextDone(true),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("starting PDF reader: %w", err)
	}
	return &Extractor{pool: pool}, nil
}

// Close releases the module.
func (e *Extractor) Close() error {
	return e.pool.Close()
}

// Document extracts every page of the PDF at path. It returns ErrPassword
// for a file that needs a password; a file locked only against editing reads
// normally. Cancelling ctx stops the extraction, also in the middle of a page.
func (e *Extractor) Document(ctx context.Context, path string) (doc Doc, err error) {
	if err := ctx.Err(); err != nil {
		return Doc{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Doc{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Doc{}, err
	}
	if info.Size() == 0 {
		return Doc{}, errors.New("empty file")
	}

	instance, err := e.pool.GetInstanceWithContext(ctx)
	if err != nil {
		return Doc{}, fmt.Errorf("starting PDF reader: %w", err)
	}
	// Kill interrupts the instance from another goroutine; it also closes
	// it, so exactly one of Kill and Close runs.
	stop := context.AfterFunc(ctx, func() { _ = instance.Kill() })
	defer func() {
		if stop() {
			_ = instance.Close()
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			doc, err = Doc{}, ctxErr
		}
	}()

	opened, err := instance.OpenDocument(&requests.OpenDocument{FileReader: file, FileReaderSize: info.Size()})
	if err != nil {
		if errors.Is(err, pdfiumerrors.ErrPassword) {
			return Doc{}, ErrPassword
		}
		return Doc{}, err
	}
	ref := opened.Document
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: ref})

	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: ref})
	if err != nil {
		return Doc{}, err
	}
	if meta, err := instance.GetMetaData(&requests.GetMetaData{Document: ref, Tags: &[]string{"Title"}}); err == nil {
		for _, tag := range meta.Tags {
			if tag.Tag == "Title" {
				doc.Title = cleanLine(tag.Value)
			}
		}
	}
	// An outline is optional and sometimes malformed; a document reads fine
	// without one.
	var sections []string
	if outline, err := instance.GetBookmarks(&requests.GetBookmarks{Document: ref}); err == nil {
		sections = pageSections(outline.Bookmarks, count.PageCount)
	}

	doc.Pages = make([]Page, 0, count.PageCount)
	for index := 0; index < count.PageCount; index++ {
		if err := ctx.Err(); err != nil {
			return Doc{}, err
		}
		page := Page{Number: index + 1}
		if index < len(sections) {
			page.Section = sections[index]
		}
		// A page that cannot be loaded is kept as an empty page so the page
		// numbers of the others stay right.
		if loaded, err := instance.FPDF_LoadPage(&requests.FPDF_LoadPage{Document: ref, Index: index}); err == nil {
			pageRef := requests.Page{ByReference: &loaded.Page}
			page.Text = pageText(instance, pageRef)
			page.Images = pageImages(instance, pageRef)
			instance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: loaded.Page})
		}
		doc.Pages = append(doc.Pages, page)
	}
	return doc, nil
}

func pageText(instance pdfium.Pdfium, page requests.Page) string {
	textPage, err := instance.FPDFText_LoadPage(&requests.FPDFText_LoadPage{Page: page})
	if err != nil {
		return ""
	}
	defer instance.FPDFText_ClosePage(&requests.FPDFText_ClosePage{TextPage: textPage.TextPage})
	chars, err := instance.FPDFText_CountChars(&requests.FPDFText_CountChars{TextPage: textPage.TextPage})
	if err != nil || chars.Count <= 0 {
		return ""
	}
	text, err := instance.FPDFText_GetText(&requests.FPDFText_GetText{TextPage: textPage.TextPage, StartIndex: 0, Count: chars.Count})
	if err != nil {
		return ""
	}
	return CleanText(text.Text)
}

func pageImages(instance pdfium.Pdfium, page requests.Page) int {
	objects, err := instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: page})
	if err != nil {
		return 0
	}
	images := 0
	for index := 0; index < objects.Count; index++ {
		object, err := instance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: page, Index: index})
		if err != nil {
			continue
		}
		kind, err := instance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: object.PageObject})
		if err == nil && kind.Type == enums.FPDF_PAGEOBJ_IMAGE {
			images++
		}
	}
	return images
}

// pageSections names the outline entry each page belongs to: the first entry
// that starts on the page, otherwise the last one that started before it.
func pageSections(outline []responses.GetBookmarksBookmark, pages int) []string {
	type start struct {
		page  int
		title string
	}
	var starts []start
	var walk func([]responses.GetBookmarksBookmark)
	walk = func(entries []responses.GetBookmarksBookmark) {
		for _, entry := range entries {
			if len(starts) >= maxBookmarks {
				return
			}
			if title := cleanLine(entry.Title); title != "" && entry.DestInfo != nil && entry.DestInfo.PageIndex >= 0 && entry.DestInfo.PageIndex < pages {
				starts = append(starts, start{entry.DestInfo.PageIndex, truncateRunes(title, maxSectionRunes)})
			}
			walk(entry.Children)
		}
	}
	walk(outline)
	if len(starts) == 0 {
		return nil
	}
	sort.SliceStable(starts, func(i, j int) bool { return starts[i].page < starts[j].page })
	sections := make([]string, pages)
	current, next := "", 0
	for page := 0; page < pages; page++ {
		first := ""
		for next < len(starts) && starts[next].page == page {
			if first == "" {
				first = starts[next].title
			}
			current = starts[next].title
			next++
		}
		if first != "" {
			sections[page] = first
		} else {
			sections[page] = current
		}
	}
	return sections
}

// lineBreakHyphens are the marks of a word split across a line break: U+0002
// is PDFium's own, the soft hyphen is what typesetters emit, and "¬" is what
// the OCR text layer of archive.org scans uses.
var lineBreakHyphens = strings.NewReplacer("\x02\n", "", "\u00AD\n", "", "¬\n", "")

// CleanText turns PDFium's page text into trimmed lines separated by "\n".
// A word hyphenated across a line break is joined so it can be found. Other
// control characters and the U+FFFE/U+FFFF stand-ins for glyphs without a
// Unicode value are dropped.
func CleanText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = lineBreakHyphens.Replace(raw)
	var lines []string
	blank := false
	for _, line := range strings.Split(raw, "\n") {
		line = cleanLine(line)
		if line == "" {
			blank = len(lines) > 0
			continue
		}
		if blank {
			lines = append(lines, "")
			blank = false
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// cleanLine drops unprintable characters and collapses runs of spaces.
func cleanLine(line string) string {
	var b strings.Builder
	space := false
	for _, r := range line {
		switch {
		case r == '\x02':
			// A hyphenation mark that is not at a line end is a plain hyphen.
			r = '-'
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		case unicode.IsControl(r) || r == 0xFFFE || r == 0xFFFF || r == unicode.ReplacementChar || r == '\u00AD':
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}
