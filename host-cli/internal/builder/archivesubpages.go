package builder

import (
	"fmt"
	"html/template"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// maxArchiveSubpages bounds how many continuation pages one project takes in:
// enough for a multi-page guide, not a whole site.
const maxArchiveSubpages = 30

var (
	githubRepoPath    = regexp.MustCompile(`^/([^/]+)/([^/]+)/?$`)
	archiveIndexPage  = regexp.MustCompile(`(?i)^index\.(html?|php|shtml)$`)
	archivePageSuffix = map[string]bool{"": true, ".html": true, ".htm": true, ".php": true, ".shtml": true}
	// archiveSkippedSegment names feeds and machinery below a post that
	// are not part of its text.
	// archivePhotoSlug matches WordPress attachment pages, named after
	// the camera's file name.
	archivePhotoSlug      = regexp.MustCompile(`(?i)^(img|dsc|dscn|dscf|pxl|pict|photo|p)[-_]?\d+([-_]\d+)?$`)
	archiveSkippedSegment = map[string]bool{"feed": true, "amp": true, "embed": true, "trackback": true, "comments": true, "tag": true, "category": true, "attachment": true, "featured_products": true}
)

// archiveLinks lists the page's same-site links in document order, without
// fragments or repeats. It reads the whole page, since a guide's chapter
// list is often in a sidebar the reader drops.
func archiveLinks(document *html.Node, base *url.URL) []*url.URL {
	seen := make(map[string]bool)
	var links []*url.URL
	for _, anchor := range cascadia.QueryAll(document, cascadia.MustCompile("a[href]")) {
		target, err := base.Parse(archiveAttribute(anchor, "href"))
		if err != nil || !strings.EqualFold(target.Host, base.Host) || target.RawQuery != "" {
			continue
		}
		target.Fragment, target.RawFragment = "", ""
		key := strings.TrimSuffix(target.String(), "/")
		if !seen[key] {
			seen[key] = true
			links = append(links, target)
		}
	}
	return links
}

// archiveSubpages picks the pages a project continues on: a guide's
// chapters below the project's own address, a Hackaday project's
// instructions, files and logs, or the Markdown docs a GitHub README links.
func archiveSubpages(base *url.URL, links []*url.URL) []*url.URL {
	own := strings.TrimSuffix(base.Path, "/")
	var keep func(string) bool
	var subpages []*url.URL
	switch {
	case base.Host == "hackaday.io":
		match := hackadayProjectPath.FindStringSubmatch(base.Path)
		if match == nil {
			return nil
		}
		project := "/project/" + match[1]
		for _, name := range []string{"instructions", "files"} {
			subpages = append(subpages, base.ResolveReference(&url.URL{Path: project + "/" + name}))
		}
		logs := regexp.MustCompile(`^` + project + `/log/\d+`)
		keep = func(candidate string) bool { return logs.MatchString(candidate) }
	case base.Host == "github.com":
		match := githubRepoPath.FindStringSubmatch(base.Path)
		if match == nil {
			return nil
		}
		repo := `(?i)^/` + regexp.QuoteMeta(match[1]+"/"+match[2])
		docs := regexp.MustCompile(repo + `/(blob/[^/]+/.+\.(md|markdown)|wiki(/[^/]+)?)$`)
		readme := regexp.MustCompile(repo + `/blob/[^/]+/readme\.md$`)
		keep = func(candidate string) bool { return docs.MatchString(candidate) && !readme.MatchString(candidate) }
	default:
		scope := own + "/"
		if name := path.Base(base.Path); path.Ext(name) != "" {
			if !archiveIndexPage.MatchString(name) {
				return nil
			}
			scope = path.Dir(base.Path) + "/"
		}
		if base.Host == "learn.adafruit.com" {
			// A guide link may name one chapter; the chapters sit under
			// the guide.
			scope = "/" + strings.SplitN(strings.TrimPrefix(base.Path, "/"), "/", 2)[0] + "/"
		}
		if scope == "/" {
			return nil
		}
		keep = func(candidate string) bool {
			if !strings.HasPrefix(candidate, scope) || archiveIndexPage.MatchString(path.Base(candidate)) {
				return false
			}
			for _, segment := range strings.Split(strings.TrimPrefix(candidate, scope), "/") {
				if archiveSkippedSegment[segment] || strings.HasPrefix(segment, "wp-") || archivePhotoSlug.MatchString(segment) {
					return false
				}
			}
			return archivePageSuffix[strings.ToLower(path.Ext(candidate))]
		}
	}
	seen := make(map[string]bool, len(subpages))
	for _, subpage := range subpages {
		seen[subpage.Path] = true
	}
	for _, link := range links {
		candidate := strings.TrimSuffix(link.Path, "/")
		if len(subpages) >= maxArchiveSubpages {
			break
		}
		if candidate != own && !seen[candidate] && keep(candidate) {
			seen[candidate] = true
			subpages = append(subpages, link)
		}
	}
	return subpages
}

// appendArchiveSubpages adds each subpage that has real text as a section of
// the article.
func appendArchiveSubpages(fetcher *archiveFetcher, article *archiveArticle, rule *compiledArchiveRule) {
	for _, subpage := range archiveSubpages(article.Base, article.Links) {
		if fetcher.ctx.Err() != nil {
			return
		}
		part, err := archiveHTMLArticle(fetcher, subpage, rule)
		if err != nil {
			continue
		}
		// Measure the text without the page chrome: an empty tab can still
		// carry a dialog's worth of words.
		pruneArchiveChrome(part.Content)
		appendArchiveSection(article, part, archiveSubpageTitle(article, part, subpage))
	}
}

// appendArchiveSection moves a page's content into the article as a section
// under title, if it has real text. Its links are made absolute first, since the article resolves
// everything against its own address.
func appendArchiveSection(article, part *archiveArticle, title string) {
	if len(strings.TrimSpace(archiveNodeText(part.Content))) < 200 {
		return
	}
	archiveAbsoluteLinks(part.Content, part.Base)
	section, err := html.ParseFragment(strings.NewReader(fmt.Sprintf(`<section><h2>%s</h2></section>`, template.HTMLEscapeString(strings.TrimSpace(title)))), archiveContainer())
	if err != nil || len(section) == 0 {
		return
	}
	for child := part.Content.FirstChild; child != nil; {
		next := child.NextSibling
		part.Content.RemoveChild(child)
		section[0].AppendChild(child)
		child = next
	}
	article.Content.AppendChild(section[0])
	article.Videos = append(article.Videos, part.Videos...)
}

func archiveAbsoluteLinks(node *html.Node, base *url.URL) {
	if node.Type == html.ElementNode {
		for index, attribute := range node.Attr {
			if attribute.Key != "href" && attribute.Key != "src" {
				continue
			}
			if resolved, err := base.Parse(attribute.Val); err == nil {
				node.Attr[index].Val = resolved.String()
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		archiveAbsoluteLinks(child, base)
	}
}

// archiveSubpageTitle heads a subpage's section: its own title, its document
// title where that repeats the project's, or a GitHub document's file name.
func archiveSubpageTitle(article, part *archiveArticle, subpage *url.URL) string {
	if subpage.Host == "github.com" && strings.Contains(subpage.Path, "/blob/") {
		return path.Base(subpage.Path)
	}
	title := part.Title
	if title == article.Title {
		title = part.PageTitle
	}
	title, _, _ = strings.Cut(title, " | ")
	title, _, _ = strings.Cut(title, " · ")
	if title = strings.TrimSpace(title); title == "" || title == article.Title {
		title = path.Base(subpage.Path)
	}
	return title
}
