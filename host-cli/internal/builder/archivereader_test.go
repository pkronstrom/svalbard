package builder

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

var testArchiveSite = archiveSiteInfo{title: "Catalog", home: "../../index.html", stylesheet: "../../" + archiveStylesheet}

func TestArchiveReaderKeepsContentImagesAndFilesFromOtherHosts(t *testing.T) {
	picture := bytes.Repeat([]byte("jpeg"), 200)
	cdn := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/photo.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write(picture)
		case "/part.stl":
			writer.Header().Set("Content-Type", "model/stl")
			_, _ = writer.Write([]byte("solid part"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer cdn.Close()
	var final *httptest.Server
	final = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><head><title>Lamp | Site</title><meta property="og:title" content="Lamp"></head><body>
			<nav><a href="/">Home</a></nav><div class="cookie-banner">Accept cookies</div>
			<article><p>` + strings.Repeat("Build the lamp. ", 30) + `</p>
			<img class="lazy" src="data:image/gif;base64,R0lGOD" data-lzl-src="` + cdn.URL + `/photo.jpg">
			<a href="` + cdn.URL + `/part.stl">Download part</a><a href="/other">Other</a><a href="/">Root</a></article>
			<footer>Copyright</footer></body></html>`))
	}))
	defer final.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, final.URL+"/lamp", http.StatusMovedPermanently)
	}))
	defer origin.Close()

	target, _ := url.Parse(origin.URL + "/lamp")
	root := t.TempDir()
	output := filepath.Join(root, "projects", "lamp")
	pages, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 3), output, testArchiveSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	for index := range pages {
		pages[index].Path = filepath.ToSlash(filepath.Join("projects", "lamp", pages[index].Path))
	}
	if err := pruneMissingArchiveResources(root, pages, true); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(output, "lamp.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{"<h1>Lamp</h1>", "Build the lamp.", `<img src="_assets/`, `href="_files/`, "<a>Other</a>", "<a>Root</a>", `href="../../archive.css"`, "catalog page 3"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	for _, unwanted := range []string{"Accept cookies", "Copyright", "Home", "data:image"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("page kept %q: %s", unwanted, page)
		}
	}
	if len(pages) != 3 {
		t.Fatalf("pages = %+v", pages)
	}
}

func TestInstructablesArticleReadsStepsFromJSON(t *testing.T) {
	picture := bytes.Repeat([]byte("jpeg"), 200)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json-api/showInstructableModel":
			if request.URL.Query().Get("urlString") != "Tiny-Lamp" {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"title":"Tiny Lamp","author":{"screenName":"maker"},"supplies":"<ul><li>LED</li></ul>","steps":[
				{"title":"Tiny Lamp","body":"<p>Intro text</p>","files":[{"image":true,"name":"cover.jpg","downloadUrl":"` + server.URL + `/cover.jpg"}]},
				{"title":"Solder","body":"<p>Solder the LED</p>","files":[{"image":false,"name":"base.stl","downloadUrl":"` + server.URL + `/base.stl"}]}]}`))
		case "/cover.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write(picture)
		case "/base.stl":
			_, _ = writer.Write([]byte("solid base"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/Tiny-Lamp/")
	archiveSiteParsers[target.Host] = instructablesArticle
	defer delete(archiveSiteParsers, target.Host)

	output := filepath.Join(t.TempDir(), "projects", "lamp")
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(output, "tiny-lamp", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{"<h1>Tiny Lamp</h1>", "by maker", "Intro text", "<h2>Supplies</h2>", "<h2>Step 1: Solder</h2>", `<img src="../_assets/`, `href="../_files/`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
}

func TestArchivePDFListingReadsCategoryAndEntries(t *testing.T) {
	for name, text := range map[string]string{
		"leading number":  "\n007    GAMING / CONSOLES\n01 ESPBoy Retro multitool - by RomanS (via ESPBoy). 02 Ploopy Mouse Runs QMK - by Ploopy.\n01\n02",
		"trailing number": "\nGAMING / CONSOLES    00801 ESPBoy Retro multitool - by RomanS (via ESPBoy). 02 Ploopy Mouse Runs QMK - by Ploopy.",
		"picture labels":  "\n007    GAMING / CONSOLES\n0204\n0103\n01 ESPBoy Retro multitool - by RomanS (via ESPBoy). 02 Ploopy Mouse Runs QMK - by Ploopy.",
	} {
		category, items := archivePDFListing(text)
		if category != "GAMING / CONSOLES" || len(items) != 2 {
			t.Fatalf("%s: category %q, items %+v", name, category, items)
		}
		if items[0] != (archiveListing{label: "ESPBoy Retro multitool", author: "RomanS"}) || items[1] != (archiveListing{label: "Ploopy Mouse Runs QMK", author: "Ploopy"}) {
			t.Errorf("%s: items %+v", name, items)
		}
	}
	if category, items := archivePDFListing("Across the world creative people are making things."); category != "" || items != nil {
		t.Errorf("prose page parsed as listing: %q %+v", category, items)
	}
}

func TestArchiveMainContentSkipsCommentsAndTinyLandmarks(t *testing.T) {
	parse := func(body string) *html.Node {
		document, err := html.Parse(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return document
	}
	post := strings.Repeat("Sew the sleeve. ", 120)
	comments := strings.Repeat("Great tutorial! ", 300)
	selected := archiveMainContent(parse(`<body><article>` + post + `</article><article id="comments">` + comments + `</article></body>`))
	if selected == nil || strings.Contains(archiveNodeText(selected), "Great tutorial") {
		t.Fatalf("selected the comment thread: %v", selected)
	}
	if selected := archiveMainContent(parse(`<body><div role="main">Welcome</div><div>` + post + `</div></body>`)); selected != nil {
		t.Fatalf("selected a landmark that misses the page content: %s", archiveNodeText(selected))
	}
}

func TestArchiveHTMLArticleResolvesBaseHrefAndPlaceholderImages(t *testing.T) {
	placeholder := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg data-u="https%3A%2F%2Fcdn.example.test%2Fbin.jpg"></svg>`))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><head><base href="/"></head><body><img src="images/plan.png"><img src="` + placeholder + `" srcset=" "></body></html>`))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/tutorial/compost-bin")
	article, err := archiveHTMLArticle(newArchiveFetcher(context.Background()), target, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for child := article.Content.FirstChild; child != nil; child = child.NextSibling {
		resolved, _ := article.Base.Parse(archiveAttribute(child, "src"))
		sources = append(sources, resolved.String())
	}
	if want := []string{server.URL + "/images/plan.png", "https://cdn.example.test/bin.jpg"}; strings.Join(sources, " ") != strings.Join(want, " ") {
		t.Fatalf("image sources = %v, want %v", sources, want)
	}
}

func TestArchiveEntryPathGivesFileLinksAnHTMLPage(t *testing.T) {
	for raw, want := range map[string]string{
		"https://example.test/manuals/DIY.pdf": "manuals/diy.pdf.html",
		"https://example.test/plans/bench":     "plans/bench.html",
		"https://example.test/tutorial.php":    "tutorial.php",
	} {
		source, _ := url.Parse(raw)
		if got := archiveEntryPath(source); got != want {
			t.Errorf("archiveEntryPath(%s) = %q, want %q", raw, got, want)
		}
	}
}
