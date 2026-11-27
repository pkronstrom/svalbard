package frontpage

import (
	"archive/zip"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestInteriorReadersUseInstalledFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".svalbard/recipes.json", "{}")
	writeFile(t, root, "manifest.yaml", "realized:\n  - id: observations\n    relative_path: data/observations.csv\n")
	writeFile(t, root, "data/observations.csv", "sample,value\n"+strings.Repeat("rock,2\n", 30))
	writeFile(t, root, "books/field.epub", "")
	f, err := os.Create(filepath.Join(root, "books", "field.epub"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, content := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="Book/package.opf"/></rootfiles></container>`,
		"Book/package.opf":       `<package><metadata><title>A field guide</title></metadata><manifest><item id="intro" href="intro.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="intro"/></spine></package>`,
		"Book/intro.xhtml":       `<h1>Ice and rock</h1>`,
	} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprint(w, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Items: []config.MenuItem{{ID: "field", Label: "Field guide", Action: config.BuiltinAction("documents", map[string]string{"path": "field.epub"})}}}}}
	client, address, _ := startDashboardLink(t, root, cfg)
	origin, key, _ := strings.Cut(address, "/#terminal-key=")
	if got := requestBody(t, client, origin+"/api/ebook?source=books/field.epub", 200); !strings.Contains(got, "Book/intro.xhtml") {
		t.Fatal(got)
	}
	requestBody(t, client, origin+"/api/ebook?source=books/field.epub&file=Book/intro.xhtml", 200)
	requestBody(t, client, origin+"/api/ebook?source=books/field.epub&file=META-INF/container.xml", 404)
	requestBody(t, client, origin+"/api/ebook?source=books/other.epub", 404)
	var sample struct{ Rows [][]string }
	readResponse(t, requestBody(t, client, origin+"/api/dataset?id=data/observations.csv", 200), &sample)
	if len(sample.Rows) != 20 {
		t.Fatalf("rows = %d", len(sample.Rows))
	}
	requestBody(t, client, origin+"/api/download?id="+url.QueryEscape("data/observations.csv"), 200)
	requestBody(t, client, origin+"/api/download?id=data/search.db", 404)
	postToDashboard(t, client, origin, "/api/preferences", origin, `{"key":"ai.context","value":"fast"}`, 403)
	req, err := http.NewRequest("POST", origin+"/api/preferences", strings.NewReader(`{"key":"ai.context","value":"fast"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Svalbard-Request", "1")
	req.Header.Set("X-Svalbard-Key", key)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	if got := requestBody(t, client, origin+"/api/preferences", 200); !strings.Contains(got, `"value":"fast"`) {
		t.Fatal(got)
	}
}
