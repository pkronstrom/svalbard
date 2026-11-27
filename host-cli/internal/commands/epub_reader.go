package commands

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

// An EPUB is a zip of XHTML files. META-INF/container.xml names the package
// file (OPF), whose spine gives the reading order. Each spine item with text
// is a chapter; its text becomes parts of at most maxDocumentPartChars,
// numbered page/N across the whole book so the drive reads them as it reads
// a PDF's pages, and cited by chapter: "ch. 3", "ch. 3 (cont.)".

// maxEPUBItemBytes bounds one XHTML file read out of the zip.
const maxEPUBItemBytes = 64 << 20

type epubReader struct{}

func openEPUBReader() (documentReader, error) { return epubReader{}, nil }

func (epubReader) Close() error { return nil }

type epubContainer struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Title    []string `xml:"metadata>title"`
	Manifest []struct {
		ID   string `xml:"id,attr"`
		Href string `xml:"href,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

type epubEncryption struct {
	References []struct {
		URI string `xml:"URI,attr"`
	} `xml:"EncryptedData>CipherData>CipherReference"`
}

var (
	epubHeadingRe = regexp.MustCompile(`(?is)<h[12][^>]*>(.*?)</h[12]>`)
	epubTitleRe   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	epubHeadRe    = regexp.MustCompile(`(?is)<head[\s>].*?</head>`)
)

func (epubReader) Read(ctx context.Context, src documentSource) (documentText, error) {
	if err := ctx.Err(); err != nil {
		return documentText{}, err
	}
	z, err := zip.NewReader(src.Data, src.Size)
	if err != nil {
		return documentText{Note: "unreadable EPUB: " + err.Error()}, nil
	}
	files := make(map[string]*zip.File, len(z.File))
	for _, f := range z.File {
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("%s is missing", name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, maxEPUBItemBytes))
	}

	var container epubContainer
	raw, err := read("META-INF/container.xml")
	if err != nil || xml.Unmarshal(raw, &container) != nil || len(container.Rootfiles) == 0 || files[container.Rootfiles[0].FullPath] == nil {
		return documentText{Note: "unreadable EPUB: no package file"}, nil
	}
	opfPath := container.Rootfiles[0].FullPath
	var pkg epubPackage
	if raw, err = read(opfPath); err != nil || xml.Unmarshal(raw, &pkg) != nil {
		return documentText{Note: "unreadable EPUB: broken package file"}, nil
	}
	hrefs := make(map[string]string, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		href, err := url.PathUnescape(item.Href)
		if err != nil {
			href = item.Href
		}
		hrefs[item.ID] = path.Join(path.Dir(opfPath), href)
	}
	if isDRMProtected(files, read, hrefs) {
		return documentText{Note: "DRM-protected"}, nil
	}

	text := documentText{}
	if len(pkg.Title) > 0 {
		text.Title = strings.TrimSpace(pkg.Title[0])
	}
	chapter := 0
	for _, ref := range pkg.Spine {
		if err := ctx.Err(); err != nil {
			return documentText{}, err
		}
		item, err := read(hrefs[ref.IDRef])
		if err != nil {
			continue
		}
		// The head holds the book's title in every file, not text.
		body := strings.TrimSpace(zimext.TextWithParagraphs(epubHeadRe.ReplaceAllString(string(item), " ")))
		if body == "" {
			continue
		}
		chapter++
		heading := ""
		if m := epubHeadingRe.FindSubmatch(item); m != nil {
			heading = zimext.StripHTML(string(m[1]))
		} else if m := epubTitleRe.FindSubmatch(item); m != nil {
			heading = zimext.StripHTML(string(m[1]))
		}
		for i, piece := range splitAtLines(body, maxDocumentPartChars) {
			label := fmt.Sprintf("ch. %d", chapter)
			if i > 0 {
				label += " (cont.)"
			}
			text.Parts = append(text.Parts, documentPart{
				Path:    fmt.Sprintf("page/%d", len(text.Parts)+1),
				Label:   label,
				Section: heading,
				Text:    piece,
			})
		}
	}
	return text, nil
}

// isDRMProtected reports whether the book's text is encrypted. Many EPUBs
// carry an encryption.xml only to obfuscate their fonts; that leaves the
// text readable. Adobe's DRM also writes rights.xml.
func isDRMProtected(files map[string]*zip.File, read func(string) ([]byte, error), hrefs map[string]string) bool {
	if files["META-INF/rights.xml"] != nil {
		return true
	}
	raw, err := read("META-INF/encryption.xml")
	if err != nil {
		return false
	}
	var encryption epubEncryption
	if xml.Unmarshal(raw, &encryption) != nil {
		return false
	}
	content := make(map[string]bool, len(hrefs))
	for _, href := range hrefs {
		content[href] = true
	}
	for _, ref := range encryption.References {
		uri, err := url.PathUnescape(ref.URI)
		if err != nil {
			uri = ref.URI
		}
		if content[uri] && !isFontPath(uri) {
			return true
		}
	}
	return false
}

func isFontPath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".ttf", ".otf", ".woff", ".woff2":
		return true
	}
	return false
}
