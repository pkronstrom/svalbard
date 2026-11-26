package search

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// PageLink represents an internal link found in a page.
type PageLink struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

// Page holds the extracted text content of an HTML page.
type Page struct {
	Title string     `json:"title"`
	Body  string     `json:"body"`
	Links []PageLink `json:"links,omitempty"`
}

// blockElements is the set of HTML elements that produce line breaks.
var blockElements = map[string]bool{
	"p": true, "br": true, "div": true,
	"h1": true, "h2": true, "h3": true, "h4": true,
	"li": true, "tr": true,
}

// skipElements is the set of elements whose content should be ignored: code,
// and the site navigation an archived web page repeats around every article.
var skipElements = map[string]bool{
	"script": true, "style": true,
	"nav": true, "footer": true, "noscript": true, "svg": true, "template": true,
}

// kiwixChromeIDs is the set of element IDs that belong to Kiwix serving chrome.
var kiwixChromeIDs = map[string]bool{
	"kiwix_serve_taskbar": true,
	"kiwix-header":        true,
	"kiwix_searchbar":     true,
}

// ExtractText parses rawHTML and returns the plain-text content, title, and internal links.
func ExtractText(rawHTML string) (Page, error) {
	return ExtractPage(rawHTML, "")
}

// ExtractPage is ExtractText for a page whose path inside its archive is
// known. Link paths are resolved against pagePath, so "../warfarin/" on
// "medicines/paracetamol/" comes back as "medicines/warfarin/", a path that
// can be read. Each target is listed once; links to the page itself and to
// images and other page resources are left out.
func ExtractPage(rawHTML, pagePath string) (Page, error) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return Page{}, err
	}
	base := &url.URL{Path: "/" + strings.TrimLeft(pagePath, "/")}
	seen := map[string]bool{strings.TrimLeft(pagePath, "/"): pagePath != ""}

	var page Page
	var bodyBuf strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Skip Kiwix chrome elements entirely.
		if n.Type == html.ElementNode {
			if hasKiwixChromeID(n) {
				return
			}
		}

		// Skip script/style elements entirely.
		if n.Type == html.ElementNode && skipElements[n.Data] {
			return
		}

		// Add newline before block elements.
		if n.Type == html.ElementNode && blockElements[n.Data] {
			bodyBuf.WriteByte('\n')
		}

		// Extract title from <title> or first <h1>.
		if n.Type == html.ElementNode && (n.Data == "title" || n.Data == "h1") && page.Title == "" {
			page.Title = strings.TrimSpace(collectText(n))
		}
		// The title is returned on its own; the body starts with the text.
		if n.Type == html.ElementNode && n.Data == "title" {
			return
		}

		// Extract internal links.
		if n.Type == html.ElementNode && n.Data == "a" {
			if href := getAttr(n, "href"); href != "" {
				if path := resolveLink(base, href); path != "" && !seen[path] {
					seen[path] = true
					label := strings.Join(strings.Fields(collectText(n)), " ")
					page.Links = append(page.Links, PageLink{Path: path, Label: label})
				}
			}
		}

		// Collect text nodes.
		if n.Type == html.TextNode {
			bodyBuf.WriteString(n.Data)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		// Add newline after br (void element, has no children).
		if n.Type == html.ElementNode && n.Data == "br" {
			bodyBuf.WriteByte('\n')
		}
	}

	walk(doc)

	page.Body = collapseWhitespace(bodyBuf.String())
	return page, nil
}

// collectText recursively collects all text content from a node and its descendants.
func collectText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

// getAttr returns the value of the named attribute on the node.
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasKiwixChromeID returns true if the node has an id attribute matching Kiwix chrome.
func hasKiwixChromeID(n *html.Node) bool {
	id := getAttr(n, "id")
	return id != "" && kiwixChromeIDs[id]
}

// isExternal returns true if the href points to an external URL.
func isExternal(href string) bool {
	return strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "//")
}

// resourceSuffixes are link targets that are not articles.
var resourceSuffixes = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".css", ".js", ".woff", ".woff2"}

// resolveLink returns the archive path an href on the page at base leads to,
// or "" when it leaves the archive or is not an article.
func resolveLink(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	lower := strings.ToLower(href)
	if isExternal(lower) || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "tel:") || strings.HasPrefix(lower, "data:") {
		return ""
	}
	var path string
	if strings.HasPrefix(href, "/") {
		path = normalizeKiwixPath(href)
	} else {
		ref, err := url.Parse(href)
		if err != nil {
			return ""
		}
		ref.Fragment, ref.RawQuery = "", ""
		if ref.Path == "" {
			return ""
		}
		path = strings.TrimLeft(base.ResolveReference(ref).Path, "/")
	}
	for _, suffix := range resourceSuffixes {
		if strings.HasSuffix(strings.ToLower(path), suffix) {
			return ""
		}
	}
	return path
}

// normalizeKiwixPath strips the /content/bookname/ prefix from Kiwix content paths
// and URL-decodes the result.
func normalizeKiwixPath(href string) string {
	// Strip fragment.
	if idx := strings.Index(href, "#"); idx >= 0 {
		href = href[:idx]
	}
	if href == "" {
		return ""
	}

	// Strip /content/bookname/ prefix if present.
	if strings.HasPrefix(href, "/content/") {
		// /content/bookname/article/path -> article/path
		rest := href[len("/content/"):]
		if idx := strings.Index(rest, "/"); idx >= 0 {
			href = rest[idx+1:]
		} else {
			href = rest
		}
	} else if strings.HasPrefix(href, "/") {
		// Absolute path without /content/ prefix — keep as-is but strip leading slash.
		href = href[1:]
	}

	// URL-decode the path.
	decoded, err := url.PathUnescape(href)
	if err != nil {
		return href
	}
	return decoded
}

// collapseWhitespace trims blank lines and normalizes whitespace in the body text.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}
