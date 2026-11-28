package frontpage

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type publicationPart struct {
	ID         string `xml:"id,attr" json:"-"`
	Href       string `xml:"href,attr" json:"path"`
	Media      string `xml:"media-type,attr" json:"media"`
	Properties string `xml:"properties,attr" json:"-"`
	Bytes      int64  `xml:"-" json:"bytes,omitempty"`
}
type publication struct {
	Title    string            `xml:"metadata>title" json:"title"`
	Creator  string            `xml:"metadata>creator" json:"creator,omitempty"`
	Language string            `xml:"metadata>language" json:"language,omitempty"`
	Parts    []publicationPart `xml:"manifest>item" json:"-"`
	Spine    struct {
		TOC   string `xml:"toc,attr"`
		Order []struct {
			ID string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine" json:"-"`
}

// contentsEntry is one line of the book's own table of contents.
type contentsEntry struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Anchor string `json:"anchor,omitempty"`
	Depth  int    `json:"depth"`
}

const maxContents = 2000

func epubXML(z *zip.Reader, name string, dst any) error {
	f, err := z.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return xml.NewDecoder(io.LimitReader(f, 2<<20)).Decode(dst)
}

// contentsTarget resolves a contents link against the document that holds it.
func contentsTarget(base, href string) (string, string, bool) {
	link, err := url.Parse(strings.TrimSpace(href))
	if err != nil || link.Scheme != "" || link.Host != "" || link.Path == "" {
		return "", "", false
	}
	target := path.Join(path.Dir(base), link.Path)
	return target, link.Fragment, fs.ValidPath(target)
}

// navContents reads an EPUB 3 navigation document's toc list.
func navContents(z *zip.Reader, name string) []contentsEntry {
	f, err := z.Open(name)
	if err != nil {
		return nil
	}
	defer f.Close()
	decoder := xml.NewDecoder(io.LimitReader(f, 2<<20))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	var entries []contentsEntry
	inTOC, navDepth, listDepth := false, 0, 0
	var label strings.Builder
	href, inLink := "", false
	for len(entries) < maxContents {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "nav":
				if inTOC {
					navDepth++
					continue
				}
				for _, attr := range t.Attr {
					if attr.Name.Local == "type" && strings.Contains(" "+attr.Value+" ", " toc ") {
						inTOC, navDepth = true, 1
					}
				}
			case "ol":
				if inTOC {
					listDepth++
				}
			case "a":
				if inTOC {
					inLink, href = true, ""
					label.Reset()
					for _, attr := range t.Attr {
						if attr.Name.Local == "href" {
							href = attr.Value
						}
					}
				}
			}
		case xml.CharData:
			if inLink {
				label.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "a":
				if inLink {
					inLink = false
					text := strings.Join(strings.Fields(label.String()), " ")
					if target, anchor, ok := contentsTarget(name, href); ok && text != "" {
						entries = append(entries, contentsEntry{Label: text, Path: target, Anchor: anchor, Depth: max(0, listDepth-1)})
					}
				}
			case "ol":
				if inTOC && listDepth > 0 {
					listDepth--
				}
			case "nav":
				if inTOC {
					if navDepth--; navDepth == 0 {
						return entries
					}
				}
			}
		}
	}
	return entries
}

type navPoint struct {
	Label   string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []navPoint `xml:"navPoint"`
}

// ncxContents reads an EPUB 2 NCX navigation map.
func ncxContents(z *zip.Reader, name string) []contentsEntry {
	var ncx struct {
		Points []navPoint `xml:"navMap>navPoint"`
	}
	if epubXML(z, name, &ncx) != nil {
		return nil
	}
	var entries []contentsEntry
	var walk func([]navPoint, int)
	walk = func(points []navPoint, depth int) {
		for _, point := range points {
			if len(entries) >= maxContents {
				return
			}
			text := strings.Join(strings.Fields(point.Label), " ")
			if target, anchor, ok := contentsTarget(name, point.Content.Src); ok && text != "" {
				entries = append(entries, contentsEntry{Label: text, Path: target, Anchor: anchor, Depth: depth})
			}
			walk(point.Children, depth+1)
		}
	}
	walk(ncx.Points, 0)
	return entries
}

func (d *dashboard) ebook(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if !strings.HasPrefix(source, "books/") || !strings.EqualFold(path.Ext(source), ".epub") {
		http.NotFound(w, r)
		return
	}
	if !d.hasBook(source) {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(d.root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	f, z, err := libraryArchive(root, source)
	if err != nil {
		http.Error(w, "The original book could not be opened.", 422)
		return
	}
	defer f.Close()
	var container struct {
		Roots []struct {
			File string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	var book publication
	if epubXML(z, "META-INF/container.xml", &container) != nil || len(container.Roots) == 0 || !fs.ValidPath(container.Roots[0].File) || epubXML(z, container.Roots[0].File, &book) != nil {
		http.Error(w, "This edition has no readable publication manifest.", 422)
		return
	}
	sizes := map[string]int64{}
	for _, member := range z.File {
		sizes[member.Name] = int64(member.UncompressedSize64)
	}
	byID := map[string]publicationPart{}
	file := r.URL.Query().Get("file")
	for _, part := range book.Parts {
		href, err := url.PathUnescape(part.Href)
		if err != nil {
			continue
		}
		part.Href = path.Join(path.Dir(container.Roots[0].File), href)
		if !fs.ValidPath(part.Href) {
			continue
		}
		part.Bytes = sizes[part.Href]
		byID[part.ID] = part
		if part.Href != file {
			continue
		}
		var limit int64 = 2 << 20
		switch part.Media {
		case "application/xhtml+xml", "text/html":
		case "image/jpeg", "image/png", "image/webp", "image/gif":
			limit = 8 << 20
		default:
			http.NotFound(w, r)
			return
		}
		stream, err := z.Open(file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer stream.Close()
		data, err := io.ReadAll(io.LimitReader(stream, limit+1))
		if err != nil || int64(len(data)) > limit {
			http.Error(w, "This part exceeds the reading preview limit. Open the original file.", 422)
			return
		}
		if strings.HasPrefix(part.Media, "image/") {
			w.Header().Set("Content-Type", part.Media)
			w.Header().Set("Cache-Control", "private, max-age=3600")
			_, _ = w.Write(data)
		} else {
			writeJSON(w, struct {
				Text string `json:"text"`
			}{string(data)})
		}
		return
	}
	if file != "" {
		http.NotFound(w, r)
		return
	}
	chapters := []publicationPart{}
	inSpine := map[string]bool{}
	for _, ref := range book.Spine.Order {
		part, ok := byID[ref.ID]
		if ok && (part.Media == "application/xhtml+xml" || part.Media == "text/html") {
			chapters = append(chapters, part)
			inSpine[part.Href] = true
		}
	}
	// The book's own contents: EPUB 3 navigation first, then the EPUB 2 NCX.
	// Entries that point outside the reading order cannot be opened here.
	var contents []contentsEntry
	for _, part := range book.Parts {
		if strings.Contains(" "+part.Properties+" ", " nav ") {
			contents = navContents(z, byID[part.ID].Href)
			break
		}
	}
	if len(contents) == 0 {
		if ncx, ok := byID[book.Spine.TOC]; ok {
			contents = ncxContents(z, ncx.Href)
		}
	}
	readable := []contentsEntry{}
	for _, entry := range contents {
		if inSpine[entry.Path] {
			readable = append(readable, entry)
		}
	}
	writeJSON(w, struct {
		Title    string            `json:"title"`
		Creator  string            `json:"creator,omitempty"`
		Language string            `json:"language,omitempty"`
		Chapters []publicationPart `json:"chapters"`
		Contents []contentsEntry   `json:"contents"`
	}{strings.TrimSpace(book.Title), strings.TrimSpace(book.Creator), strings.TrimSpace(book.Language), chapters, readable})
}
