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
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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

// killInstance interrupts an instance in the middle of a call. A variable so
// a test can watch it.
var killInstance = func(instance pdfium.Pdfium) { _ = instance.Kill() }

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
	return e.DocumentFrom(ctx, file, info.Size())
}

// DocumentFrom is Document for a PDF that is not a file of its own, such as
// an entry read out of a ZIM: size bytes read through r.
func (e *Extractor) DocumentFrom(ctx context.Context, r io.ReadSeeker, size int64) (doc Doc, err error) {
	if err := ctx.Err(); err != nil {
		return Doc{}, err
	}
	if size == 0 {
		return Doc{}, errors.New("empty file")
	}

	instance, err := e.pool.GetInstanceWithContext(ctx)
	if err != nil {
		return Doc{}, fmt.Errorf("starting PDF reader: %w", err)
	}
	// Kill interrupts the instance from another goroutine; it also closes
	// it, so exactly one of Kill and Close runs, and Document does not
	// return while a Kill is still tearing the instance down: the caller may
	// close the extractor or ask for the next document right away.
	killed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(killed)
		killInstance(instance)
	})
	defer func() {
		if stop() {
			_ = instance.Close()
		} else {
			<-killed
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			doc, err = Doc{}, ctxErr
		}
	}()

	opened, err := instance.OpenDocument(&requests.OpenDocument{FileReader: r, FileReaderSize: size})
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
	// Words broken across lines are settled with the whole document in view.
	raw := make([]string, len(doc.Pages))
	for i, page := range doc.Pages {
		raw[i] = page.Text
	}
	vocab := buildVocabulary(raw)
	for i := range doc.Pages {
		doc.Pages[i].Text = cleanText(raw[i], vocab)
	}
	return doc, nil
}

// pageText returns a page's text as PDFium gives it, before cleanText.
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
	return normalizeLines(text.Text)
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

// PDFium replaces a hyphen at the end of a line with a marker (U+FFFE, U+0002
// in older builds) when the next line continues with a letter. It cannot say
// whether the hyphen belongs to the word ("longer-acting") or only breaks it
// ("medica-tions"), and dropping or keeping it blindly loses a searchable
// word either way. The document's own text decides instead: see vocabulary.
var (
	brokenWordRe = regexp.MustCompile(`(\pL+)[\x{FFFE}\x02]\n?[ \t]*(\pL*)`)
	wordRe       = regexp.MustCompile(`\pL+`)
	compoundRe   = regexp.MustCompile(`\pL+(?:[-\x{2010}\x{2011}]\pL+)+`)
	// softBreaks are marks that only ever break a word: the soft hyphen
	// typesetters emit, and the "¬" of archive.org's OCR text layers.
	softBreaks = strings.NewReplacer("\u00AD\n", "", "¬\n", "")
)

// vocabulary is what a document says about its own words outside the places
// where a line break split one.
type vocabulary struct {
	words     map[string]bool // every word, lowercased
	compounds map[string]bool // "long-acting": two words joined by a hyphen
}

func buildVocabulary(pages []string) vocabulary {
	v := vocabulary{words: map[string]bool{}, compounds: map[string]bool{}}
	for _, page := range pages {
		// The two halves of a broken word are not evidence of anything.
		page = strings.ToLower(brokenWordRe.ReplaceAllString(page, " "))
		for _, word := range wordRe.FindAllString(page, -1) {
			v.words[word] = true
		}
		for _, compound := range compoundRe.FindAllString(page, -1) {
			parts := wordRe.FindAllString(compound, -1)
			for i := 1; i < len(parts); i++ {
				v.compounds[parts[i-1]+"-"+parts[i]] = true
			}
		}
	}
	return v
}

// hyphenated reports whether a word broken into before and after at a line
// end keeps its hyphen. It does when the next line starts a capitalised
// word, when the document writes the same compound with a hyphen elsewhere,
// or when both halves are words of their own that the document never writes
// as one; in the last case both stay findable. Otherwise the halves are one
// word: the document writes it unbroken elsewhere, or a half is no word.
func (v vocabulary) hyphenated(before, after string) bool {
	if after == "" {
		return true
	}
	if first, _ := utf8.DecodeRuneInString(after); unicode.IsUpper(first) {
		return true
	}
	before, after = strings.ToLower(before), strings.ToLower(after)
	switch {
	case v.words[before+after]:
		return false
	case v.compounds[before+"-"+after]:
		return true
	}
	// A short first half is far more often a syllable ("in-formation",
	// "be-cause") than the first word of a compound.
	return utf8.RuneCountInString(before) >= 3 && utf8.RuneCountInString(after) >= 3 && v.words[before] && v.words[after]
}

// normalizeLines gives PDFium's page text plain "\n" line ends.
func normalizeLines(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
}

// cleanText turns one page of PDFium's text, already through normalizeLines,
// into trimmed lines separated by "\n". Words split across a line break are
// put back together, with or without their hyphen as the vocabulary says.
// Other control characters and the stand-ins for glyphs without a Unicode
// value are dropped.
func cleanText(raw string, vocab vocabulary) string {
	raw = softBreaks.Replace(raw)
	raw = brokenWordRe.ReplaceAllStringFunc(raw, func(match string) string {
		parts := brokenWordRe.FindStringSubmatch(match)
		if vocab.hyphenated(parts[1], parts[2]) {
			return parts[1] + "-" + parts[2]
		}
		return parts[1] + parts[2]
	})
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
		case r == '\x02' || r == 0xFFFE:
			// A line-end hyphen that was not followed by a letter.
			r = '-'
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		case unicode.IsControl(r) || r == 0xFFFF || r == unicode.ReplacementChar || r == '\u00AD':
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
