package workspace

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/pdfreader"
)

// contentSecurityPolicy keeps a generated page from reaching out. Inline
// script and style run, and the page may load other workspace files, but it
// cannot fetch, post a form, or load anything from another origin. A page can
// still navigate away; no header prevents that.
const contentSecurityPolicy = "default-src 'none'; " +
	"script-src 'unsafe-inline' 'self'; style-src 'unsafe-inline' 'self'; " +
	"img-src data: blob: 'self'; font-src data: 'self'; media-src data: blob: 'self'; " +
	"form-action 'none'; base-uri 'none'; frame-ancestors "

// URL is the address of rel below base, the root URL Serve answers on.
func URL(base, rel string) string {
	if rel == "." {
		return base + "/"
	}
	segments := strings.Split(rel, "/")
	for i, segment := range segments {
		// Parentheses are legal in a path but would end a Markdown link early.
		segments[i] = strings.NewReplacer("(", "%28", ")", "%29").Replace(url.PathEscape(segment))
	}
	return base + "/" + strings.Join(segments, "/")
}

// Serve answers read-only requests for the workspace on listener until ctx is
// done, then closes the listener.
func (w *Workspace) Serve(ctx context.Context, listener net.Listener) error {
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: w.handler(port)}

	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func (w *Workspace) handler(port int) http.Handler {
	hosts := []string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("localhost:%d", port)}
	frameAncestor := "'none'"
	if w.opts.DashboardPort > 0 && w.opts.DashboardPort <= 65535 {
		frameAncestor = fmt.Sprintf("http://127.0.0.1:%d", w.opts.DashboardPort)
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		header := rw.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy+frameAncestor)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		// Another site's page must not embed a workspace file. Open WebUI is
		// the same site (localhost on another port) and may show an image the
		// model saved.
		header.Set("Cross-Origin-Resource-Policy", "same-site")
		// A page reloaded after an edit must show the edit.
		header.Set("Cache-Control", "no-store")

		// A name that resolves to this address from outside (DNS rebinding)
		// would otherwise make the workspace readable by that site.
		if !slices.Contains(hosts, r.Host) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			header.Set("Allow", "GET, HEAD")
			http.Error(rw, "read-only", http.StatusMethodNotAllowed)
			return
		}
		if w.opts.DashboardPort > 0 && w.opts.Serves != nil && strings.HasPrefix(r.URL.Path, pdfreader.Prefix) {
			pdfreader.Serve(rw, r, frameAncestor)
			return
		}
		// Every failure is the same 404, so a request cannot probe what lies
		// outside the workspace.
		rel, err := w.Resolve(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			http.NotFound(rw, r)
			return
		}
		info, err := w.root.Stat(rel)
		if err != nil {
			http.NotFound(rw, r)
			return
		}
		if info.IsDir() {
			if !strings.HasSuffix(r.URL.Path, "/") {
				// Relative links in a listing or an index page need the slash.
				http.Redirect(rw, r, path.Base(r.URL.Path)+"/", http.StatusMovedPermanently)
				return
			}
			index := path.Join(rel, "index.html")
			if indexInfo, err := w.root.Stat(index); err == nil && indexInfo.Mode().IsRegular() && w.serves(index) {
				rel, info = index, indexInfo
			} else {
				w.serveListing(rw, r, rel)
				return
			}
		}
		if !info.Mode().IsRegular() || !w.serves(rel) {
			http.NotFound(rw, r)
			return
		}
		file, err := w.root.Open(rel)
		if err != nil {
			http.NotFound(rw, r)
			return
		}
		defer file.Close()
		http.ServeContent(rw, r, path.Base(rel), info.ModTime(), file)
	})
}

// serves reports whether the file rel is one this folder serves.
func (w *Workspace) serves(rel string) bool {
	return w.opts.Serves == nil || w.opts.Serves(path.Base(rel))
}

// serveListing writes a plain index of one workspace folder.
func (w *Workspace) serveListing(rw http.ResponseWriter, r *http.Request, rel string) {
	entries, err := w.ReadDir(rel)
	if err != nil {
		http.NotFound(rw, r)
		return
	}
	title := filepath.Base(w.dir)
	if rel != "." {
		title += "/" + rel
	}
	var page strings.Builder
	fmt.Fprintf(&page, "<!doctype html>\n<meta charset=\"utf-8\">\n<title>%s</title>\n<h1>%s</h1>\n<ul>\n", html.EscapeString(title), html.EscapeString(title))
	if rel != "." {
		page.WriteString("<li><a href=\"../\">../</a></li>\n")
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || (!entry.IsDir() && !w.serves(name)) {
			continue
		}
		href := url.PathEscape(name)
		if entry.IsDir() {
			name, href = name+"/", href+"/"
		}
		fmt.Fprintf(&page, "<li><a href=\"%s\">%s</a></li>\n", href, html.EscapeString(name))
	}
	page.WriteString("</ul>\n")
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(rw, page.String())
}
