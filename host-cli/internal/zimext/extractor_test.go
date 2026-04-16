package zimext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
