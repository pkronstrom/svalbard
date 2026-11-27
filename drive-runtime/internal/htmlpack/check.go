package htmlpack

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// resourceAttrs are the attributes that make a browser load something, by
// element. Links (<a href>) and form actions load nothing by themselves.
var resourceAttrs = map[atom.Atom][]string{
	atom.Script: {"src"},
	atom.Img:    {"src", "srcset"},
	atom.Source: {"src", "srcset"},
	atom.Audio:  {"src"},
	atom.Video:  {"src", "poster"},
	atom.Track:  {"src"},
	atom.Iframe: {"src"},
	atom.Embed:  {"src"},
	atom.Object: {"data"},
	atom.Input:  {"src"},
}

// loadingRels are the <link rel> values that fetch their href.
var loadingRels = map[string]bool{
	"stylesheet": true, "icon": true, "apple-touch-icon": true, "preload": true, "modulepreload": true,
	"prefetch": true, "manifest": true, "mask-icon": true,
}

var (
	cssURLRe    = regexp.MustCompile(`(?i)url\(\s*["']?([^"')\s]+)`)
	cssImportRe = regexp.MustCompile(`(?i)@import\s+["']([^"']+)["']`)
	// Calls in script text that load a module, a script or data from a
	// literal address.
	jsRemoteRe = regexp.MustCompile("(?:\\bimport\\s*\\(|\\bfrom|\\bimportScripts\\s*\\(|\\bnew\\s+(?:Shared)?Worker\\s*\\(|\\bfetch\\s*\\(|\\bnew\\s+EventSource\\s*\\(|\\bnew\\s+WebSocket\\s*\\()\\s*[\"'`]((?:https?:|wss?:)?//[^\"'`]*)")
	jsFetchRe  = regexp.MustCompile("\\bfetch\\s*\\(\\s*[\"'`]([^\"'`]*)[\"'`]")
	// d3.csv("x.csv") and friends fetch too.
	jsLoaderRe = regexp.MustCompile("\\.(?:csv|tsv|dsv|json|xml)\\s*\\(\\s*[\"'`]([^\"'`]+\\.[A-Za-z0-9]{2,8})[\"'`]")
	jsXHRRe    = regexp.MustCompile("\\.open\\s*\\(\\s*[\"'](?:GET|POST)[\"']\\s*,\\s*[\"'`]([^\"'`]*)[\"'`]")
)

// Check looks for what keeps a single HTML file from working offline: a
// resource loaded from the network (remote-resource), a fetch of a file
// beside the page (relative-fetch), and a reference to a local file, which
// a single file does not have beside it (missing-asset).
func Check(page []byte) []Problem {
	var problems []Problem
	add := func(kind, format string, args ...any) {
		problems = append(problems, Problem{Kind: kind, Detail: fmt.Sprintf(format, args...)})
	}
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return []Problem{{Kind: "build", Detail: "the output is not HTML: " + err.Error()}}
	}
	reference := func(where, ref string) {
		ref = strings.TrimSpace(ref)
		switch {
		case isRemote(ref):
			add("remote-resource", "%s loads %s from the network", where, ref)
		case !isInline(ref):
			problems = append(problems, Problem{Kind: "missing-asset", Detail: fmt.Sprintf("%s names %s, a file a single page does not have beside it; import it or put it in the page", where, ref), Target: ref})
		}
	}
	checkCSS := func(where, css string) {
		for _, m := range cssURLRe.FindAllStringSubmatch(css, -1) {
			reference(where+" url()", m[1])
		}
		for _, m := range cssImportRe.FindAllStringSubmatch(css, -1) {
			reference(where+" @import", m[1])
		}
	}
	checkScript := func(where, code string) {
		problems = append(problems, CheckScript(where, code)...)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := "<" + n.Data + ">"
			for _, key := range resourceAttrs[n.DataAtom] {
				if val, ok := attr(n, key); ok {
					if key == "srcset" {
						for _, candidate := range strings.Split(val, ",") {
							if fields := strings.Fields(candidate); len(fields) > 0 {
								reference(tag+" srcset", fields[0])
							}
						}
						continue
					}
					reference(tag+" "+key, val)
				}
			}
			if n.DataAtom == atom.Link {
				rel, _ := attr(n, "rel")
				href, _ := attr(n, "href")
				for _, r := range strings.Fields(strings.ToLower(rel)) {
					if loadingRels[r] {
						reference("<link rel="+r+">", href)
						break
					}
				}
			}
			// SVG <image> and <use> load what their href names.
			if n.Namespace == "svg" && (n.Data == "image" || n.Data == "use" || n.Data == "feImage") {
				for _, a := range n.Attr {
					if a.Key == "href" {
						reference("<"+n.Data+"> href", a.Val)
					}
				}
			}
			if n.DataAtom == atom.Base {
				if href, ok := attr(n, "href"); ok && isRemote(href) {
					add("remote-resource", "<base href=%q> resolves the page's references on the network", href)
				}
			}
			if n.DataAtom == atom.Meta {
				if equiv, _ := attr(n, "http-equiv"); strings.EqualFold(strings.TrimSpace(equiv), "refresh") {
					content, _ := attr(n, "content")
					if _, target, ok := strings.Cut(strings.ToLower(content), "url="); ok {
						reference("<meta refresh>", strings.Trim(strings.TrimSpace(content[len(content)-len(target):]), `"'`))
					}
				}
			}
			if style, ok := attr(n, "style"); ok {
				checkCSS(tag+" style", style)
			}
			switch n.DataAtom {
			case atom.Style:
				checkCSS("<style>", textOf(n))
			case atom.Script:
				kind, _ := attr(n, "type")
				if kind = strings.TrimSpace(kind); kind == "" || strings.Contains(strings.ToLower(kind), "javascript") || strings.EqualFold(kind, "module") {
					checkScript("<script>", textOf(n))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return problems
}

// CheckScript looks in script text for loads from the network and fetches of
// files beside the page.
func CheckScript(where, code string) []Problem {
	var problems []Problem
	add := func(kind, format string, args ...any) {
		problems = append(problems, Problem{Kind: kind, Detail: fmt.Sprintf(format, args...)})
	}
	for _, m := range jsRemoteRe.FindAllStringSubmatch(code, -1) {
		add("remote-resource", "%s loads %s from the network", where, m[1])
	}
	for _, re := range []*regexp.Regexp{jsFetchRe, jsLoaderRe, jsXHRRe} {
		for _, m := range re.FindAllStringSubmatch(code, -1) {
			switch target := m[1]; {
			case isRemote(target) && re == jsFetchRe:
				// jsRemoteRe reported it.
			case isRemote(target):
				add("remote-resource", "%s loads %s from the network", where, target)
			case !isInline(target) && !strings.HasPrefix(target, "$"):
				problems = append(problems, Problem{Kind: "relative-fetch", Detail: fmt.Sprintf("%s: %s (%s)", where, relativeFetchMessage, target), Target: target})
			}
		}
	}
	return problems
}
