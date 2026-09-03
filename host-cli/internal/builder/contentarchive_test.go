package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestBuildContentArchiveCopiesSameOriginSiteAndPackagesZIM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		if request.UserAgent() != archiveUserAgent {
			t.Errorf("User-Agent = %q", request.UserAgent())
		}
		switch request.URL.Path {
		case "/":
			_, _ = writer.Write([]byte(`<!-- crawler noise --><html><body><header>site chrome</header><main><nav class="promo">ad</nav><script>track()</script><a href="/about">About</a><img src="/assets/logo.png" data-src="https://cdn.example.test/lazy.png"><img src="/missing.png"><img src="https://cdn.example.test/logo.png"><svg><use href="/icons.svg#home"></use></svg><div style="background:url(https://cdn.example.test/bg.png)">content</div></main><footer>site footer</footer></body></html>`))
		case "/about":
			_, _ = writer.Write([]byte(`<html><title>About</title><body>offline</body></html>`))
		case "/assets/logo.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("png"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	source, _ := archiveURL(server.URL)

	root := t.TempDir()
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool == "zimwriterfs" {
			output := args[len(args)-1]
			if _, err := os.Stat(output); err == nil {
				return "", fmt.Errorf("archive already exists: %s", output)
			}
			return "", os.WriteFile(output, []byte("zim"), 0o644)
		}
		if tool == "zimcheck" {
			if _, err := os.Stat(args[0]); err != nil {
				t.Fatalf("zimcheck received missing archive: %v", err)
			}
		}
		return "", nil
	}
	defer func() { runToolCommand = previous }()

	recipe := catalog.Item{
		ID: "archive", Type: "zim", Description: "test archive",
		Build: &catalog.BuildSpec{
			Family: "content-archive", SourceURL: server.URL + "/", Config: map[string]string{"max_pages": "3"},
			ArchiveRules: []catalog.ArchiveRule{{Domain: source.Host, Content: "main", Remove: []string{"script", ".promo"}}},
		},
	}
	entries, err := buildContentArchive(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != "zim/archive.zim" {
		t.Fatalf("entries = %#v", entries)
	}
	for _, name := range []string{"index.html", "about.html", "assets/logo.png", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(root, ".staging", "build", "archive", "site", name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	index, err := os.ReadFile(filepath.Join(root, ".staging", "build", "archive", "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `href="about.html"`) || !strings.Contains(string(index), `src="assets/logo.png"`) {
		t.Fatalf("index did not rewrite local links: %s", index)
	}
	if strings.Contains(string(index), "track()") || strings.Contains(string(index), "ad</nav>") || strings.Contains(string(index), "cdn.example.test") || strings.Contains(string(index), "missing.png") || strings.Contains(string(index), "icons.svg") || strings.Contains(string(index), "site chrome") || strings.Contains(string(index), "site footer") || strings.Contains(string(index), "crawler noise") {
		t.Fatalf("index retained an offline dependency: %s", index)
	}
	if _, err := buildContentArchive(root, recipe, nil, Options{}); err != nil {
		t.Fatalf("rebuild did not replace archive: %v", err)
	}
}

func TestPruneMissingArchiveResourcesRemovesExternalAndMissingSources(t *testing.T) {
	root := t.TempDir()
	body := `<html><body><img src="https://cdn.example.test/image.png"><img src="missing.png"><img src="kept.png"></body></html>`
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kept.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pruneMissingArchiveResources(root, []archivedPage{{Path: "index.html", HTML: true}}, false); err != nil {
		t.Fatal(err)
	}
	clean, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(clean), "cdn.example.test") || strings.Contains(string(clean), "missing.png") || !strings.Contains(string(clean), "kept.png") {
		t.Fatalf("pruned archive = %s", clean)
	}
}

func TestBuildContentArchiveBuildsPDFLinkSeeds(t *testing.T) {
	var sourcePDF []byte
	projectRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/source.pdf":
			writer.Header().Set("Content-Type", "application/pdf")
			_, _ = writer.Write(sourcePDF)
		case "/project":
			projectRequests++
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte(`<html><head><title>DIY Plan</title><link rel="alternate" href="/feed.rss"></head><body><a href="/category">Category</a><img src="/assets/plan.png"></body></html>`))
		case "/assets/plan.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("png"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	sourcePDF = minimalTextPDF(server.URL + "/project")

	root := t.TempDir()
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool == "zimwriterfs" {
			return "", os.WriteFile(args[len(args)-1], []byte("zim"), 0o644)
		}
		return "", nil
	}
	defer func() { runToolCommand = previous }()

	recipe := catalog.Item{
		ID: "pdf-archive", Type: "zim",
		Build: &catalog.BuildSpec{Family: "content-archive", SourceURL: server.URL + "/source.pdf", Config: map[string]string{"source_format": "pdf-links"}},
	}
	if _, err := buildContentArchive(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	project, _ := archiveURL(server.URL + "/project")
	source := newArchiveSource(project, 1)
	site := filepath.Join(root, ".staging", "build", recipe.ID, "site", "projects", source.ID)
	for _, name := range []string{"project.html", "assets/plan.png", "project.json"} {
		if _, err := os.Stat(filepath.Join(site, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", "build", recipe.ID, "site", "index.html")); err != nil {
		t.Errorf("missing archive index: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(root, ".staging", "build", recipe.ID, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "DIY Plan") {
		t.Fatalf("archive index lost project title: %s", index)
	}
	manifest, err := os.ReadFile(filepath.Join(root, ".staging", "build", recipe.ID, "site", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"source_page": 1`) || !strings.Contains(string(manifest), source.ID) {
		t.Fatalf("manifest lost PDF project provenance: %s", manifest)
	}
	projectRecord, err := os.ReadFile(filepath.Join(site, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectRecord), `"title": "DIY Plan"`) {
		t.Fatalf("project record lost page title: %s", projectRecord)
	}
	projectHTML, err := os.ReadFile(filepath.Join(site, "project.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectHTML), `<a>Category</a>`) || strings.Contains(string(projectHTML), "href=\"/category\"") || strings.Contains(string(projectHTML), "feed.rss") {
		t.Fatalf("bounded archive retained broken local links: %s", projectHTML)
	}
	recordedProject, err := readArchiveProject(filepath.Join(site, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	recordedProject.Version--
	record, err := json.Marshal(recordedProject)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "project.json"), record, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "stale.bin"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectRequests = 0
	if _, err := buildContentArchive(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if projectRequests == 0 {
		t.Fatal("rebuild did not fetch stale project")
	}
	if _, err := os.Stat(filepath.Join(site, "stale.bin")); !os.IsNotExist(err) {
		t.Fatalf("stale project output exists: %v", err)
	}
	projectRequests = 0
	if _, err := buildContentArchive(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if projectRequests != 0 {
		t.Fatalf("resumed archive fetched completed project %d times", projectRequests)
	}
}
