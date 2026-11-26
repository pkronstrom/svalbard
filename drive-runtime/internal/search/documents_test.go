package search

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// addDocument indexes one page of a PDF on the drive and writes the file.
func addDocument(t *testing.T, root, filename string, page int, title, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(filename))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF-1.4 "+filename), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sourceID int64
	if err := db.QueryRow(`SELECT id FROM sources WHERE filename = ?`, filename).Scan(&sourceID); err != nil {
		res, err := db.Exec(`INSERT INTO sources (filename) VALUES (?)`, filename)
		if err != nil {
			t.Fatal(err)
		}
		sourceID, _ = res.LastInsertId()
	}
	res, err := db.Exec(`INSERT INTO articles (source_id, path, title, body) VALUES (?, ?, ?, ?)`, sourceID, "page/"+strconv.Itoa(page), title, body)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO articles_fts(rowid, title, body) VALUES (?, ?, ?)`, id, title, body); err != nil {
		t.Fatal(err)
	}
}

func TestSessionOpensADocumentAtItsPageWithoutKiwix(t *testing.T) {
	root := newSearchDrive(t)
	// No kiwix-serve anywhere: a document must not need it.
	t.Setenv("PATH", t.TempDir())
	addDocument(t, root, "books/car manuals/volvo (1990).pdf", 112, "Volvo 240 Owner's Manual, p. 112 · Cooling system", "Coolant capacity incl. heater: 9.5 litres.")
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte("not for the browser"), 0o644); err != nil {
		t.Fatal(err)
	}

	var opened []string
	session, err := NewSession(root, func(address string) error {
		opened = append(opened, address)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := session.Search(context.Background(), ModeKeyword, "coolant capacity", 20)
	if err != nil || len(response.Results) != 1 {
		t.Fatalf("Search = %+v, %v", response.Results, err)
	}
	hit := response.Results[0]

	var out bytes.Buffer
	RenderResults(&out, response.Results)
	if want := "  1. [car manuals/volvo (1990).pdf] Volvo 240 Owner's Manual, p. 112 · Cooling system\n"; !strings.HasPrefix(out.String(), want) {
		t.Errorf("result line = %q, want it to start %q", out.String(), want)
	}

	if err := session.OpenResult(hit); err != nil {
		t.Fatalf("OpenResult: %v", err)
	}
	if err := session.OpenResult(hit); err != nil {
		t.Fatalf("OpenResult again: %v", err)
	}
	if len(opened) != 2 || opened[0] != opened[1] {
		t.Fatalf("opened = %q, want one server reused for the session", opened)
	}
	address, err := url.Parse(opened[0])
	if err != nil {
		t.Fatal(err)
	}
	if address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.Fragment != "page=112" || address.Path != "/car manuals/volvo (1990).pdf" {
		t.Fatalf("opened %q, want the document on 127.0.0.1 at #page=112", opened[0])
	}
	if strings.ContainsAny(opened[0], " ()") {
		t.Errorf("opened %q, want the path escaped", opened[0])
	}

	get := func(path, host string) (int, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, "http://"+address.Host+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host != "" {
			request.Host = host
		}
		// The raw path must reach the server as written.
		request.URL.Opaque = path
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	if status, body := get(address.EscapedPath(), ""); status != http.StatusOK || body != "%PDF-1.4 books/car manuals/volvo (1990).pdf" {
		t.Fatalf("GET the document = %d %q", status, body)
	}
	// Only books/ is served, to loopback names only.
	for _, path := range []string{"/../manifest.yaml", "/%2e%2e/manifest.yaml", "/car%20manuals/../../manifest.yaml", "/..%2fdata/search.db"} {
		if status, body := get(path, ""); status == http.StatusOK || strings.Contains(body, "not for the browser") {
			t.Errorf("GET %s = %d %q, want it refused", path, status, body)
		}
	}
	if status, _ := get(address.EscapedPath(), "evil.example:80"); status != http.StatusForbidden {
		t.Errorf("GET with a foreign Host = %d, want 403", status)
	}

	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get("http://" + address.Host + address.EscapedPath()); err == nil {
		t.Error("the document server still answers after the session closed")
	}
}

func TestSourceLabel(t *testing.T) {
	for filename, want := range map[string]string{
		"wikipedia_en_100_mini.zim":          "wikipedia_en_100_mini",
		"books/car-manuals/volvo.pdf":        "car-manuals/volvo.pdf",
		"books/WHO_Basic_Emergency_Care.pdf": "WHO_Basic_Emergency_Care.pdf",
	} {
		if got := SourceLabel(filename); got != want {
			t.Errorf("SourceLabel(%q) = %q, want %q", filename, got, want)
		}
	}
}
