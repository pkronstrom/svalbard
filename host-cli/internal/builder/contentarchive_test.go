package builder

import (
	"context"
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
		switch request.URL.Path {
		case "/":
			_, _ = writer.Write([]byte(`<html><body><a href="/about">About</a><img src="/assets/logo.png"></body></html>`))
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

	root := t.TempDir()
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool == "zimwriterfs" {
			return "", os.WriteFile(args[len(args)-1], []byte("zim"), 0o644)
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
		Build: &catalog.BuildSpec{Family: "content-archive", SourceURL: server.URL + "/", Config: map[string]string{"max_pages": "3"}},
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
}
