package workspace

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serve starts the workspace server and returns its root URL.
func serve(t *testing.T, ws *Workspace) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ws.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return "http://" + listener.Addr().String()
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	// Redirects are part of what is tested.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, string(body)
}

func TestURLEscapesEachSegment(t *testing.T) {
	for rel, want := range map[string]string{
		".":                      "http://127.0.0.1:8086/",
		"page.html":              "http://127.0.0.1:8086/page.html",
		"my notes/50% #1?.html":  "http://127.0.0.1:8086/my%20notes/50%25%20%231%3F.html",
		"reports/2026/page.html": "http://127.0.0.1:8086/reports/2026/page.html",
		"draft (2).html":         "http://127.0.0.1:8086/draft%20%282%29.html",
	} {
		if got := URL("http://127.0.0.1:8086", rel); got != want {
			t.Errorf("URL(%q) = %q, want %q", rel, got, want)
		}
	}
}

func TestServeSendsPagesWithHeadersThatKeepThemOffline(t *testing.T) {
	ws, _ := newWorkspace(t)
	page := "<!doctype html><title>t</title><script>document.title='ran'</script>"
	if err := ws.WriteFile("my notes/page (1).html", []byte(page)); err != nil {
		t.Fatal(err)
	}
	base := serve(t, ws)

	response, body := get(t, URL(base, "my notes/page (1).html"))
	if response.StatusCode != http.StatusOK || body != page {
		t.Fatalf("GET page = %d %q", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	policy := response.Header.Get("Content-Security-Policy")
	directives := map[string]string{}
	for _, directive := range strings.Split(policy, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(directive), " ")
		directives[name] = value
	}
	for name, want := range map[string]string{
		// Everything not named, connections included, is denied.
		"default-src":     "'none'",
		"script-src":      "'unsafe-inline' 'self'",
		"style-src":       "'unsafe-inline' 'self'",
		"img-src":         "data: blob: 'self'",
		"form-action":     "'none'",
		"base-uri":        "'none'",
		"frame-ancestors": "'none'",
	} {
		if directives[name] != want {
			t.Errorf("CSP %s = %q, want %q", name, directives[name], want)
		}
	}
	if _, set := directives["connect-src"]; set {
		t.Error("CSP sets connect-src; it must fall back to default-src 'none'")
	}
	for _, remote := range []string{"http:", "https:", "*", "ws:"} {
		if strings.Contains(policy, " "+remote) {
			t.Errorf("CSP %q allows remote source %q", policy, remote)
		}
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-site",
		"Cache-Control":                "no-store",
	} {
		if got := response.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}

func TestServeListsFoldersAndPrefersTheirIndexPage(t *testing.T) {
	ws, _ := newWorkspace(t)
	for rel, content := range map[string]string{
		"page.html":        "page",
		"notes/a&b.md":     "# note",
		"site/index.html":  "<h1>site</h1>",
		"site/style.css":   "body{}",
		"notes/deep/x.txt": "x",
	} {
		if err := ws.WriteFile(rel, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws.Dir(), ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := serve(t, ws)

	response, body := get(t, base+"/")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d", response.StatusCode)
	}
	for _, want := range []string{`<a href="notes/">notes/</a>`, `<a href="page.html">page.html</a>`, `<a href="site/">site/</a>`} {
		if !strings.Contains(body, want) {
			t.Errorf("root listing lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, ".DS_Store") {
		t.Errorf("root listing shows a hidden file:\n%s", body)
	}
	if response.Header.Get("Content-Security-Policy") == "" {
		t.Error("listing has no Content-Security-Policy")
	}

	if _, body := get(t, base+"/notes/"); !strings.Contains(body, `<a href="a&b.md">a&amp;b.md</a>`) || !strings.Contains(body, `<a href="../">`) {
		t.Errorf("folder listing = %s", body)
	}
	if response, _ := get(t, base+"/notes"); response.StatusCode != http.StatusMovedPermanently || response.Header.Get("Location") != "/notes/" {
		t.Errorf("GET /notes = %d to %q, want a redirect to /notes/", response.StatusCode, response.Header.Get("Location"))
	}
	if response, body := get(t, base+"/site/"); response.StatusCode != http.StatusOK || body != "<h1>site</h1>" {
		t.Errorf("GET /site/ = %d %q, want its index page", response.StatusCode, body)
	}
	if response, _ := get(t, base+"/site/style.css"); !strings.HasPrefix(response.Header.Get("Content-Type"), "text/css") {
		t.Errorf("style.css Content-Type = %q", response.Header.Get("Content-Type"))
	}
}

func TestServeAnswersNothingOutsideTheWorkspace(t *testing.T) {
	ws, outside := newWorkspace(t)
	if err := ws.WriteFile("page.html", []byte("page")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.Dir(), ".hidden.html"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The drive's own files sit one level above the workspace.
	if err := os.WriteFile(filepath.Join(filepath.Dir(ws.Dir()), "manifest.yaml"), []byte("drive"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlink(t, outside, filepath.Join(ws.Dir(), "out"))
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(ws.Dir(), "secret-link.txt"))

	handler := ws.handler(8086)
	request := func(method, host, path string) *httptest.ResponseRecorder {
		// The path is set after parsing so it reaches the handler as sent,
		// as a raw request line does.
		r := httptest.NewRequest(method, "http://"+host+"/", nil)
		r.URL.Path = path
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder
	}

	if got := request(http.MethodGet, "127.0.0.1:8086", "/page.html"); got.Code != http.StatusOK || got.Body.String() != "page" {
		t.Fatalf("GET /page.html = %d %q", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, "localhost:8086", "/page.html"); got.Code != http.StatusOK {
		t.Errorf("GET with Host localhost = %d", got.Code)
	}
	for _, path := range []string{
		"/../manifest.yaml",
		"/notes/../../manifest.yaml",
		"//etc/passwd",
		"/" + filepath.Join(outside, "secret.txt"),
		"/out/",
		"/out/secret.txt",
		"/secret-link.txt",
		"/.hidden.html",
		"/missing.html",
		`/..\manifest.yaml`,
	} {
		got := request(http.MethodGet, "127.0.0.1:8086", path)
		if got.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d %q, want 404", path, got.Code, got.Body.String())
		}
		if strings.Contains(got.Body.String(), "secret") || strings.Contains(got.Body.String(), "drive") {
			t.Errorf("GET %s leaked content: %q", path, got.Body.String())
		}
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if got := request(method, "127.0.0.1:8086", "/page.html"); got.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /page.html = %d, want 405", method, got.Code)
		}
	}
	if data, _ := ws.ReadFile("page.html", 100); string(data) != "page" {
		t.Errorf("page changed to %q", data)
	}
	// A page on another site that resolves its own name to this address.
	for _, host := range []string{"evil.example:8086", "127.0.0.1:9999", "127.0.0.1"} {
		if got := request(http.MethodGet, host, "/page.html"); got.Code != http.StatusForbidden {
			t.Errorf("GET with Host %s = %d, want 403", host, got.Code)
		}
	}
}

func TestServeStopsAndReleasesThePortOnCancel(t *testing.T) {
	ws, _ := newWorkspace(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ws.Serve(ctx, listener) }()
	if response, _ := get(t, fmt.Sprintf("http://%s/", listener.Addr())); response.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d", response.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after cancellation")
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		t.Fatal("listener still accepts connections after cancellation")
	}
}

func TestServeLimitedToTheFilesOptionsAccept(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"a.pdf":          "%PDF",
		"a.pdf.json":     "{}",
		"sub/b.pdf":      "%PDF-b",
		"sub/index.html": "<p>page</p>",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	onlyPDF := func(name string) bool { return strings.HasSuffix(name, ".pdf") }

	limited, err := OpenWith(dir, Options{Serves: onlyPDF})
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	base := serve(t, limited)

	if response, body := get(t, base+"/"); response.StatusCode != http.StatusOK || !strings.Contains(body, `href="a.pdf"`) || !strings.Contains(body, `href="sub/"`) || strings.Contains(body, "a.pdf.json") {
		t.Errorf("listing = %d %q, want a.pdf and sub/ only", response.StatusCode, body)
	}
	// An index page that is not served does not stand in for the listing.
	if response, body := get(t, base+"/sub/"); response.StatusCode != http.StatusOK || !strings.Contains(body, `href="b.pdf"`) || strings.Contains(body, "page") {
		t.Errorf("sub listing = %d %q, want the listing with b.pdf", response.StatusCode, body)
	}
	if response, body := get(t, base+"/sub/b.pdf"); response.StatusCode != http.StatusOK || body != "%PDF-b" {
		t.Errorf("accepted file = %d %q", response.StatusCode, body)
	}
	for _, name := range []string{"/a.pdf.json", "/sub/index.html"} {
		if response, _ := get(t, base+name); response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", name, response.StatusCode)
		}
	}
	// The limit is on serving alone.
	if data, err := limited.ReadFile("a.pdf.json", 1024); err != nil || string(data) != "{}" {
		t.Errorf("ReadFile = %q, %v", data, err)
	}

	// Without the option every file is served, as the workspace needs.
	all, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer all.Close()
	base = serve(t, all)
	if response, body := get(t, base+"/a.pdf.json"); response.StatusCode != http.StatusOK || body != "{}" {
		t.Errorf("unlimited file = %d %q", response.StatusCode, body)
	}
	if response, body := get(t, base+"/sub/"); response.StatusCode != http.StatusOK || body != "<p>page</p>" {
		t.Errorf("unlimited index = %d %q", response.StatusCode, body)
	}
}
