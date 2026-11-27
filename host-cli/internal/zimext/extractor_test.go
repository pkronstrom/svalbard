package zimext

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

func testSmallZIMPath(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}

	path := filepath.Join(strings.TrimSpace(string(out)), "github.com", "stazelabs", "gozim@v0.1.0", "testdata", "small.zim")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat test zim %s: %v", path, err)
	}
	return path
}

func TestExtractArticlesContextReturnsArchiveUUID(t *testing.T) {
	articles, title, uuid, err := ExtractArticlesContext(context.Background(), testSmallZIMPath(t))
	if err != nil {
		t.Fatalf("ExtractArticlesContext: %v", err)
	}
	if len(articles) == 0 || title == "" {
		t.Fatalf("articles=%d title=%q", len(articles), title)
	}
	if uuid == "" {
		t.Fatal("archive UUID is empty")
	}
}

func TestArticleWorkersPreserveResults(t *testing.T) {
	archive := openDocuments(t)
	read := func(workers int) []searchdb.Article {
		t.Helper()
		var articles []searchdb.Article
		for article, err := range archive.Articles(context.Background(), workers) {
			if err != nil {
				t.Fatal(err)
			}
			articles = append(articles, article)
		}
		return articles
	}
	serial, parallel := read(1), read(4)
	if len(serial) != 5 || !slices.Equal(serial, parallel) {
		t.Fatalf("serial = %v, parallel = %v", serial, parallel)
	}
}

func TestExtractArticlesContextStopsBeforeIteration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := ExtractArticlesContext(ctx, testSmallZIMPath(t))
	if err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestExtractArticlesFromZIM(t *testing.T) {
	articles, archiveTitle, err := ExtractArticles(testSmallZIMPath(t))
	if err != nil {
		t.Fatalf("ExtractArticles: %v", err)
	}

	if archiveTitle != "small.zim" && archiveTitle == "" {
		t.Fatalf("archive title should not be empty")
	}
	if len(articles) == 0 {
		t.Fatal("expected extracted articles")
	}

	article := articles[0]
	if article.Path != "/main.html" {
		t.Fatalf("article path = %q, want %q", article.Path, "/main.html")
	}
	if article.Title != "Test ZIM file" {
		t.Fatalf("article title = %q, want %q", article.Title, "Test ZIM file")
	}
	if article.Body == "" {
		t.Fatal("article body should not be empty")
	}
	if strings.Contains(article.Body, "<html") {
		t.Fatalf("article body should be stripped HTML, got %q", article.Body)
	}
	if !strings.Contains(article.Body, "Test ZIM file") {
		t.Fatalf("article body missing expected content: %q", article.Body)
	}
}
