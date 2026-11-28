package frontpage

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// An article is a small, inert document tree, never archived markup injected into the shell.
type articleNode struct {
	Tag      string            `json:"tag,omitempty"`
	Text     string            `json:"text,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Children []articleNode     `json:"children,omitempty"`
}
type adaptedArticle struct {
	Title string        `json:"title"`
	Lang  string        `json:"lang,omitempty"`
	Dir   string        `json:"dir,omitempty"`
	Nodes []articleNode `json:"nodes"`
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == name {
			return a.Val
		}
	}
	return ""
}
func archiveRelative(u, root *url.URL) (string, bool) {
	if u.Scheme != root.Scheme || u.Host != root.Host || u.User != nil || u.RawQuery != "" || !strings.HasPrefix(u.Path, root.Path) {
		return "", false
	}
	rel := strings.TrimPrefix(u.Path, root.Path)
	return rel, (rel == "" || fs.ValidPath(rel)) && !strings.ContainsAny(rel, "\\\x00")
}
func (d *dashboard) archiveContent(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	d.mu.Lock()
	j := d.jobs[key]
	var base string
	if spec := d.specs[key]; spec.name == "browse" && spec.target != "" && j != nil && j.Status == "running" && len(j.Links) > 0 {
		base = j.Links[0].URL
	}
	d.mu.Unlock()
	viewer, err := url.Parse(base)
	if err != nil || viewer == nil || viewer.Fragment == "" || !fs.ValidPath(viewer.Fragment) || strings.ContainsAny(viewer.Fragment, "/\\") {
		http.NotFound(w, r)
		return
	}
	root := &url.URL{Scheme: viewer.Scheme, Host: viewer.Host, Path: "/content/" + viewer.Fragment + "/"}
	rel := r.URL.Query().Get("path")
	target := *root
	target.Path += rel
	if _, ok := archiveRelative(&target, root); !ok {
		http.NotFound(w, r)
		return
	}
	client := http.Client{Timeout: 6 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if _, ok := archiveRelative(req.URL, root); !ok || len(via) > 4 {
			return errors.New("outside the archive")
		}
		return nil
	}}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		http.NotFound(w, r)
		return
	}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if r.PathValue("part") == "image" {
		if !slices.Contains([]string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/avif"}, kind) {
			http.NotFound(w, r)
			return
		}
		const maxImage = 12 << 20
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxImage+1))
		if err != nil || len(data) > maxImage {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", kind)
		w.Write(data)
		return
	}
	if kind != "text/html" && kind != "application/xhtml+xml" {
		writeJSON(w, adaptedArticle{})
		return
	}
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(data) > limit {
		writeJSON(w, adaptedArticle{})
		return
	}
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		writeJSON(w, adaptedArticle{})
		return
	}
	writeJSON(w, adaptArticle(doc, resp.Request.URL, root))
}
func adaptArticle(doc *html.Node, page, root *url.URL) adaptedArticle {
	out := adaptedArticle{}
	var body *html.Node
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "html":
			out.Lang = attr(n, "lang")
			out.Dir = attr(n, "dir")
		case "title":
			if n.FirstChild != nil {
				out.Title = n.FirstChild.Data
			}
		}
		if body == nil && slices.Contains(strings.Fields(attr(n, "class")), "mw-parser-output") {
			body = n
		}
	}
	if body == nil {
		return adaptedArticle{}
	}
	for n := range body.Descendants() {
		if n.Type == html.ElementNode && slices.Contains([]string{"canvas", "iframe", "video", "audio", "form", "object", "embed", "math", "svg"}, n.Data) {
			return adaptedArticle{}
		}
	}
	const allowed = " a p div span section h1 h2 h3 h4 h5 h6 ol ul li dl dt dd blockquote em i b strong small sup sub code pre br hr img figure figcaption table tbody thead tfoot tr td th caption details summary "
	var nodes int
	var truncated bool
	var convert func(*html.Node, int) []articleNode
	convert = func(n *html.Node, depth int) []articleNode {
		nodes++
		if depth > 80 || nodes > 25000 {
			truncated = true
			return nil
		}
		if n.Type == html.TextNode {
			return []articleNode{{Text: n.Data}}
		}
		if n.Type != html.ElementNode || slices.Contains([]string{"script", "style", "template", "noscript", "link", "meta", "button", "input"}, n.Data) {
			return nil
		}
		node := articleNode{Tag: n.Data, Attrs: map[string]string{}}
		for child := range n.ChildNodes() {
			node.Children = append(node.Children, convert(child, depth+1)...)
		}
		if !strings.Contains(allowed, " "+n.Data+" ") {
			return node.Children
		}
		for _, name := range []string{"id", "lang", "dir", "alt", "title", "colspan", "rowspan"} {
			if v := attr(n, name); v != "" {
				node.Attrs[name] = v
			}
		}
		if n.Data == "a" || n.Data == "img" {
			k := "href"
			if n.Data == "img" {
				k = "src"
			}
			if ref := attr(n, k); ref != "" {
				if address, err := page.Parse(ref); err == nil {
					if rel, ok := archiveRelative(address, root); ok {
						node.Attrs["path"] = rel
						node.Attrs["anchor"] = address.Fragment
					} else if n.Data == "a" && (address.Scheme == "https" || address.Scheme == "http") && address.User == nil {
						node.Attrs["external"] = address.String()
					}
				}
			}
		}
		return []articleNode{node}
	}
	for child := range body.ChildNodes() {
		out.Nodes = append(out.Nodes, convert(child, 0)...)
	}
	if truncated {
		return adaptedArticle{}
	}
	return out
}
