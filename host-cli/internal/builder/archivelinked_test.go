package builder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestArchiveLinkedGuidesFollowsBuildSitesAndGuideLinks(t *testing.T) {
	source, _ := url.Parse("https://www.printables.com/model/7164-pigrrl")
	content, _ := html.Parse(strings.NewReader(`<div>
		<a href="https://learn.adafruit.com/pigrrl-zero">PiGRRL Zero</a>
		<a href="https://www.printables.com/model/99-other">Other model</a>
		<a href="https://example.com/blog/table">Free plans here</a>
		<a href="https://example.com/about">About us</a>
		<a href="https://www.youtube.com/watch?v=x">Full build guide</a>
		<a href="https://github.com/o/r">Assembly instructions</a>
		<a href="https://example.com/case.stl">Build guide STL</a>
		<a href="https://www.kregtool.com/">Free plans</a>
		<a href="https://www.thingiverse.com/thing:3050437/remixes">Remixes</a>
		<a href="https://www.instructables.com/Third-Guide/">Third guide</a>
	</div>`))
	article := &archiveArticle{Base: source, Content: content}
	var got []string
	for _, guide := range archiveLinkedGuides(article, source) {
		got = append(got, guide.String())
	}
	if want := []string{"https://learn.adafruit.com/pigrrl-zero", "https://example.com/blog/table"}; !slices.Equal(got, want) {
		t.Errorf("guides = %v, want %v", got, want)
	}
}

func TestArchiveReaderAddsLinkedGuideAndItsFiles(t *testing.T) {
	var blog *httptest.Server
	blog = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/table":
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte(`<html><head><title>Coffee Table Plans</title></head><body><article><p>` + strings.Repeat("Cut the legs to 16 inches. ", 20) +
				`</p><a href="/table-plans.pdf">Cut list PDF</a></article></body></html>`))
		case "/table-plans.pdf":
			_, _ = writer.Write([]byte("%PDF-1.4 plans"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer blog.Close()
	site := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><head><title>Table</title></head><body><article><p>` + strings.Repeat("A sturdy table. ", 30) +
			`</p><a href="` + blog.URL + `/table">Get the free plans</a></article></body></html>`))
	}))
	defer site.Close()

	target, _ := url.Parse(site.URL + "/table")
	output := t.TempDir()
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 1), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, "table.html"))
	for _, want := range []string{"<h2>Coffee Table Plans (127.0.0.1", "Cut the legs to 16 inches.", `href="_files/`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
}
