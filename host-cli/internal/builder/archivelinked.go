package builder

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
)

// maxArchiveLinkedGuides bounds the pages on other sites one project takes in:
// the guide its page sends readers to, not the pages that guide links.
const maxArchiveLinkedGuides = 2

// archiveGuidePages are the project pages of build sites: a link to one
// is a link to a build guide, whatever its text says.
var archiveGuidePages = map[string]*regexp.Regexp{
	"learn.adafruit.com":    regexp.MustCompile(`^/[^/]+(/[^/]+)?/?$`),
	"www.instructables.com": regexp.MustCompile(`^/[A-Za-z0-9]+(-[A-Za-z0-9]+)+/?$`),
	"instructables.com":     regexp.MustCompile(`^/[A-Za-z0-9]+(-[A-Za-z0-9]+)+/?$`),
	"hackaday.io":           regexp.MustCompile(`^/project/\d+`),
	"www.printables.com":    regexp.MustCompile(`^/model/\d+`),
	"printables.com":        regexp.MustCompile(`^/model/\d+`),
	"www.thingiverse.com":   regexp.MustCompile(`^/thing:\d+/?$`),
	"thingiverse.com":       regexp.MustCompile(`^/thing:\d+/?$`),
	"www.hackster.io":       regexp.MustCompile(`^/[^/]+/[^/]+$`),
	"hackster.io":           regexp.MustCompile(`^/[^/]+/[^/]+$`),
}

// archiveGuideText matches link text that sends the reader to the build
// itself.
var archiveGuideText = regexp.MustCompile(`(?i)\b(build|assembly|step[- ]by[- ]step|full|complete|detailed|written)\s+(guide|instructions?|tutorial|manual|write-?up|docs|documentation)\b|` +
	`\b(instructions|tutorial|build guide|assembly guide|building plans?|free plans?|the plans?|download plans?|printable plans?|cut ?list)\b`)

// archiveGuideSkipped are hosts a guide link is never followed to: video,
// shops and social sites, and GitHub, whose repositories are packed instead.
var archiveGuideSkipped = regexp.MustCompile(`(?i)(^|\.)(youtube\.com|youtu\.be|vimeo\.com|amazon\.[a-z.]+|ebay\.[a-z.]+|etsy\.com|aliexpress\.com|` +
	`facebook\.com|instagram\.com|twitter\.com|x\.com|pinterest\.[a-z.]+|reddit\.com|discord\.(gg|com)|patreon\.com|github\.com|google\.com|linktr\.ee|myshopify\.com)$`)

// archiveLinkedGuides lists the build guides on other pages that the
// article links: project pages of build sites, and links whose text names a
// guide or plans.
func archiveLinkedGuides(article *archiveArticle, source *url.URL) []*url.URL {
	seen := map[string]bool{strings.TrimSuffix(source.String(), "/"): true, strings.TrimSuffix(article.Base.String(), "/"): true}
	var guides []*url.URL
	for _, anchor := range cascadia.QueryAll(article.Content, cascadia.MustCompile("a[href]")) {
		if len(guides) >= maxArchiveLinkedGuides {
			break
		}
		target, err := article.Base.Parse(archiveAttribute(anchor, "href"))
		// A site's home page is a credit or a shop, not a guide.
		if err != nil || target.Scheme != "http" && target.Scheme != "https" || strings.Trim(target.Path, "/") == "" ||
			archiveGuideSkipped.MatchString(target.Hostname()) ||
			archiveFileExtensions[strings.ToLower(path.Ext(target.Path))] || archiveImageExtension(strings.ToLower(path.Ext(target.Path))) {
			continue
		}
		target.Fragment, target.RawFragment = "", ""
		key := strings.TrimSuffix(target.String(), "/")
		if seen[key] {
			continue
		}
		// Same-site links are subpages, or the site's other projects.
		host := strings.ToLower(target.Host)
		if strings.EqualFold(host, source.Host) || strings.EqualFold(host, article.Base.Host) {
			continue
		}
		pattern := archiveGuidePages[host]
		if !(pattern != nil && pattern.MatchString(target.Path)) && !archiveGuideText.MatchString(archiveNodeText(anchor)) {
			continue
		}
		seen[key] = true
		guides = append(guides, target)
	}
	return guides
}

// appendArchiveLinkedGuides adds each linked guide, with its own chapters,
// as a section, and its attachments to the article's.
func appendArchiveLinkedGuides(fetcher *archiveFetcher, article *archiveArticle, source *url.URL) {
	for _, guide := range archiveLinkedGuides(article, source) {
		if fetcher.ctx.Err() != nil {
			return
		}
		rule := archiveRuleFor(guide.Host, fetcher.rules)
		var part *archiveArticle
		var err error
		if parser := archiveSiteParsers[strings.ToLower(guide.Host)]; parser != nil {
			part, err = parser(fetcher, guide)
		} else {
			part, err = archiveHTMLArticle(fetcher, guide, rule)
		}
		if err != nil {
			continue
		}
		appendArchiveSubpages(fetcher, part, rule)
		for _, file := range part.Files {
			if target, err := part.Base.Parse(file.URL); err == nil {
				file.URL = target.String()
				article.Files = append(article.Files, file)
			}
		}
		pruneArchiveChrome(part.Content)
		title := part.Title
		if title == "" {
			title = guide.Host
		}
		appendArchiveSection(article, part, title+" ("+strings.TrimPrefix(guide.Host, "www.")+")")
	}
}
