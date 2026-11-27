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
	ID    string `xml:"id,attr" json:"-"`
	Href  string `xml:"href,attr" json:"path"`
	Media string `xml:"media-type,attr" json:"media"`
}
type publication struct {
	Title string            `xml:"metadata>title" json:"title"`
	Parts []publicationPart `xml:"manifest>item" json:"-"`
	Order []struct {
		ID string `xml:"idref,attr"`
	} `xml:"spine>itemref" json:"-"`
}

func epubXML(z *zip.Reader, name string, dst any) error {
	f, err := z.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return xml.NewDecoder(io.LimitReader(f, 2<<20)).Decode(dst)
}

func (d *dashboard) ebook(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if !strings.HasPrefix(source, "books/") || !strings.EqualFold(path.Ext(source), ".epub") {
		http.NotFound(w, r)
		return
	}
	found := false
	for _, g := range d.inv.Groups {
		for _, e := range g.Items {
			if e.Source == source && e.Icon == "documents" {
				found = true
				break
			}
		}
	}
	if !found {
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
	for _, ref := range book.Order {
		part, ok := byID[ref.ID]
		if ok && (part.Media == "application/xhtml+xml" || part.Media == "text/html") {
			chapters = append(chapters, part)
		}
	}
	writeJSON(w, struct {
		Title    string            `json:"title"`
		Chapters []publicationPart `json:"chapters"`
	}{book.Title, chapters})
}
