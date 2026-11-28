package frontpage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestArchiveFrameServesOnlyActiveArchiveAndReader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/content/book/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/content/book/":
			http.Redirect(w, r, "/content/book/home", 302)
		case "/content/book/home":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<!doctype html><title>Guide</title>")
		case "/content/book/guide.pdf":
			http.ServeContent(w, r, "guide.pdf", time.Time{}, strings.NewReader("%PDF-12345"))
		default:
			http.NotFound(w, r)
		}
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()
	d := dashboard{jobs: map[string]*job{"guide": {state: state{Status: "running", Links: []link{{URL: upstream.URL + "/viewer#book"}}}}}, specs: map[string]serviceSpec{"guide": {name: "browse"}}}
	handler := d.archiveFrameHandler("http://127.0.0.1:4321")
	get := func(p, byteRange string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", p, nil)
		if byteRange != "" {
			r.Header.Set("Range", byteRange)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, p := range []string{"/api/terminal", "/api/inventory", "/content/unserved/guide.pdf"} {
		if w := get(p, ""); w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	if w := get("/content/book/", ""); w.Header().Get("Location") != "/content/book/home" {
		t.Fatal(w.Header())
	}
	if w := get("/content/book/home", ""); !strings.Contains(w.Body.String(), "/archive-links.js") {
		t.Fatal(w.Body)
	}
	if w := get("/content/book/guide.pdf", "bytes=0-4"); w.Code != 206 || w.Body.String() != "%PDF-" {
		t.Fatal(w.Code, w.Body)
	}
	if w := get("/.svalbard-reader/", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	d.jobs["guide"].Status = "stopped"
	if w := get("/content/book/guide.pdf", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestWikiReadingLayoutAndFileCatalogueDetection(t *testing.T) {
	base, _ := url.Parse("http://localhost:4100/content/book/")
	parse := func(input string) adaptedArticle {
		doc, err := html.Parse(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		return adaptArticle(doc, base, base)
	}
	if !parse(`<script src="nautilus.js"></script>`).FileCatalogue || parse(`<script src="https://example.org/nautilus.js"></script>`).FileCatalogue {
		t.Fatal("unexpected file catalogue detection")
	}
	a := parse(`<div class="mw-parser-output"><figure typeof="mw:File" class="mw-halign-right"><img width="350" height="217" src="pic.jpg"><figcaption>borderless</figcaption></figure><figure typeof="mw:File/Thumb"><figcaption>Field notes</figcaption></figure></div>`)
	if len(a.Nodes) != 2 {
		t.Fatal(a)
	}
	figure := a.Nodes[0]
	if figure.Attrs["layout"] != "mw-halign-right" || len(figure.Children) != 1 || figure.Children[0].Attrs["width"] != "350" || a.Nodes[1].Children[0].Tag != "figcaption" {
		t.Fatal(a)
	}
}
