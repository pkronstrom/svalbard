package frontpage

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
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
	FileCatalogue bool          `json:"fileCatalogue,omitempty"`
	Title         string        `json:"title"`
	Lang          string        `json:"lang,omitempty"`
	Dir           string        `json:"dir,omitempty"`
	Nodes         []articleNode `json:"nodes"`
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

// Preserve useful MediaWiki layout hints without applying an archive's CSS in
// the dashboard document. Percentages belong to tables, pixel sizes to images.
func archiveLayout(n *html.Node, attrs map[string]string) {
	classes := strings.Fields(attr(n, "class"))
	for _, class := range []string{"mw-halign-right", "mw-halign-left", "mw-halign-center", "thumb", "tright", "tleft", "wikitable", "toccolours", "infobox"} {
		if slices.Contains(classes, class) {
			attrs["layout"] = strings.TrimSpace(attrs["layout"] + " " + class)
		}
	}
	if n.Data == "table" && attr(n, "border") == "0" {
		attrs["layout"] = strings.TrimSpace(attrs["layout"] + " columns")
	}
	if n.Data == "table" || n.Data == "td" || n.Data == "th" {
		if width := attr(n, "width"); strings.HasSuffix(width, "%") {
			if value, err := strconv.Atoi(strings.TrimSuffix(width, "%")); err == nil && value > 0 && value <= 100 {
				attrs["width"] = strconv.Itoa(value) + "%"
			}
		}
		if v := attr(n, "align"); slices.Contains([]string{"left", "center", "right"}, v) {
			attrs["align"] = v
		}
		if v := attr(n, "valign"); slices.Contains([]string{"top", "middle", "bottom"}, v) {
			attrs["valign"] = v
		}
	}
	if n.Data == "img" {
		for _, key := range []string{"width", "height"} {
			if value, err := strconv.Atoi(attr(n, key)); err == nil && value > 0 && value <= 16000 {
				attrs[key] = strconv.Itoa(value)
			}
		}
	}
}
func hasClass(n *html.Node, class string) bool {
	return slices.Contains(strings.Fields(attr(n, "class")), class)
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			b.WriteString(d.Data)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func firstWith(n *html.Node, match func(*html.Node) bool) *html.Node {
	for d := range n.Descendants() {
		if d.Type == html.ElementNode && match(d) {
			return d
		}
	}
	return nil
}

// stackPost is one question or answer of a Stack Exchange archive page.
type stackPost struct {
	answer, accepted bool
	votes, tags      string
	body             *html.Node
}

// stackPosts finds the question and its answers on a sotoki question page.
// Listing pages, users and tags have no post bodies and stay in the original.
func stackPosts(doc *html.Node) []stackPost {
	var posts []stackPost
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Data != "div" {
			continue
		}
		question := attr(n, "id") == "question" || hasClass(n, "question")
		answer := hasClass(n, "answer")
		if !question && !answer {
			continue
		}
		body := firstWith(n, func(d *html.Node) bool { return hasClass(d, "js-post-body") })
		if body == nil || (question && len(posts) > 0) {
			continue
		}
		post := stackPost{answer: answer, accepted: hasClass(n, "accepted-answer"), body: body}
		if votes := firstWith(n, func(d *html.Node) bool { return hasClass(d, "js-vote-count") }); votes != nil {
			post.votes = textOf(votes)
		}
		if question {
			if list := firstWith(n, func(d *html.Node) bool { return hasClass(d, "post-taglist") }); list != nil {
				var tags []string
				for d := range list.Descendants() {
					if d.Type == html.ElementNode && d.Data == "a" {
						tags = append(tags, textOf(d))
					}
				}
				post.tags = strings.Join(tags, ", ")
			}
		}
		posts = append(posts, post)
	}
	if len(posts) == 0 || posts[0].answer {
		return nil
	}
	return posts
}

func adaptArticle(doc *html.Node, page, root *url.URL) adaptedArticle {
	out := adaptedArticle{}
	var body *html.Node
	var heading string
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "script":
			if src, err := page.Parse(attr(n, "src")); err == nil {
				if rel, ok := archiveRelative(src, root); ok && rel == "nautilus.js" {
					out.FileCatalogue = true
				}
			}
		case "html":
			out.Lang = attr(n, "lang")
			out.Dir = attr(n, "dir")
		case "title":
			if n.FirstChild != nil {
				out.Title = n.FirstChild.Data
			}
		case "h1":
			if heading == "" && n.Parent != nil && attr(n.Parent, "id") == "question-header" {
				heading = textOf(n)
			}
		}
		if body == nil && hasClass(n, "mw-parser-output") {
			body = n
		}
	}
	var posts []stackPost
	if body == nil {
		if posts = stackPosts(doc); posts == nil {
			return out
		}
		if heading != "" {
			out.Title = heading
		}
	}
	unsafe := func(n *html.Node) bool {
		for d := range n.Descendants() {
			if d.Type == html.ElementNode && slices.Contains([]string{"canvas", "iframe", "video", "audio", "form", "object", "embed", "math", "svg"}, d.Data) {
				return true
			}
		}
		return false
	}
	if body != nil && unsafe(body) {
		return adaptedArticle{}
	}
	for _, post := range posts {
		if unsafe(post.body) {
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
		if n.Data == "hr" && attr(n, "width") == "0" {
			return nil
		}
		// A plain MediaWiki file has a hidden metadata caption. Thumbnails
		// and framed files have visible captions in the original.
		if n.Data == "figcaption" && n.Parent != nil && slices.Contains(strings.Fields(attr(n.Parent, "typeof")), "mw:File") {
			return nil
		}
		node := articleNode{Tag: n.Data, Attrs: map[string]string{}}
		for child := range n.ChildNodes() {
			node.Children = append(node.Children, convert(child, depth+1)...)
		}
		if !strings.Contains(allowed, " "+n.Data+" ") {
			return node.Children
		}
		archiveLayout(n, node.Attrs)
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
	if body != nil {
		for child := range body.ChildNodes() {
			out.Nodes = append(out.Nodes, convert(child, 0)...)
		}
	}
	answers := 0
	for _, post := range posts {
		if post.answer {
			answers++
		}
	}
	for i, post := range posts {
		if i == 1 {
			label := strconv.Itoa(answers) + " answers"
			if answers == 1 {
				label = "1 answer"
			}
			out.Nodes = append(out.Nodes, articleNode{Tag: "h2", Children: []articleNode{{Text: label}}})
		}
		meta := []string{"Question"}
		if post.answer {
			meta[0] = "Answer"
		}
		if post.votes != "" {
			meta = append(meta, post.votes+" votes")
		}
		if post.accepted {
			meta = append(meta, "Accepted")
		}
		if post.tags != "" {
			meta = append(meta, post.tags)
		}
		section := articleNode{Tag: "section", Attrs: map[string]string{"layout": "post"}}
		section.Children = append(section.Children, articleNode{Tag: "p", Attrs: map[string]string{"layout": "post-meta"}, Children: []articleNode{{Text: strings.Join(meta, " · ")}}})
		for child := range post.body.ChildNodes() {
			section.Children = append(section.Children, convert(child, 0)...)
		}
		out.Nodes = append(out.Nodes, section)
	}
	if truncated {
		return adaptedArticle{}
	}
	return out
}
