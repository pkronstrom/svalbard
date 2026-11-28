package frontpage

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/pdfreader"
)

// This origin serves archived content and the PDF reader only. Archived
// scripts must never share the dashboard's origin or terminal credential.
func (d *dashboard) archiveFrameHandler(dashboardOrigin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+pdfreader.Prefix, func(w http.ResponseWriter, r *http.Request) {
		pdfreader.Serve(w, r, dashboardOrigin)
	})
	mux.HandleFunc("GET /archive-links.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		io.WriteString(w, archivePDFLinks)
	})
	mux.HandleFunc("GET /content/{book}/{file...}", d.serveArchiveFrame)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; worker-src 'none'; object-src 'none'; form-action 'self'; base-uri 'self'; frame-ancestors "+dashboardOrigin)
		mux.ServeHTTP(w, r)
	})
}

func (d *dashboard) activeArchiveRoot(book string) *url.URL {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, j := range d.jobs {
		if d.specs[key].name != "browse" || j.Status != "running" || len(j.Links) == 0 {
			continue
		}
		u, err := url.Parse(j.Links[0].URL)
		if err == nil && u.Fragment == book && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
			return &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/content/" + book + "/"}
		}
	}
	return nil
}

func (d *dashboard) serveArchiveFrame(w http.ResponseWriter, r *http.Request) {
	root := d.activeArchiveRoot(r.PathValue("book"))
	if root == nil || r.Header.Get("Service-Worker") == "script" {
		http.NotFound(w, r)
		return
	}
	target := *root
	target.Path += r.PathValue("file")
	if _, ok := archiveRelative(&target, root); !ok {
		http.NotFound(w, r)
		return
	}
	// Let the browser resolve relative links against Kiwix's canonical page.
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if value := r.Header.Get("Range"); value != "" && strings.EqualFold(path.Ext(target.Path), ".pdf") {
		req.Header.Set("Range", value)
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Archive service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location, err := resp.Location()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, ok := archiveRelative(location, root); !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, location.EscapedPath(), http.StatusFound)
		return
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		w.Header().Set("Content-Range", resp.Header.Get("Content-Range"))
		w.WriteHeader(resp.StatusCode)
		return
	default:
		http.NotFound(w, r)
		return
	}
	kind := resp.Header.Get("Content-Type")
	isHTML := strings.EqualFold(strings.TrimSpace(strings.SplitN(kind, ";", 2)[0]), "text/html")
	w.Header().Set("Content-Type", kind)
	if !isHTML {
		for _, k := range []string{"Content-Length", "Content-Range", "Accept-Ranges"} {
			if value := resp.Header.Get(k); value != "" {
				w.Header().Set(k, value)
			}
		}
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, resp.Body); err == nil && isHTML {
		io.WriteString(w, `<script src="/archive-links.js"></script>`)
	}
}

// Nautilus creates its chapter links after loading the local catalogue. Handle
// the actual click so keyboard activation and Back work on those links too.
const archivePDFLinks = `document.addEventListener("click", event => {
  const link = event.target.closest?.("a[href]");
  if (!link || event.defaultPrevented || event.button !== 0) return;
  const file = new URL(link.href, location.href);
  if (file.origin !== location.origin || !file.pathname.startsWith("/content/") ||
      !file.pathname.toLowerCase().endsWith(".pdf")) return;
  event.preventDefault();
  const reader = new URL("/.svalbard-reader/", location.origin);
  reader.searchParams.set("file", file.pathname + file.hash);
  location.assign(reader.href);
}, true);`
