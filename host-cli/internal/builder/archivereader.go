package builder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

const (
	maxArchiveImageBytes  = 12 << 20
	maxArchiveImages      = 150
	maxArchiveFiles       = 40
	maxArchiveVideos      = 3
	archiveHostConcurrent = 3
	archiveVideoWorkers   = 2
	archiveMaxPace        = 30 * time.Second
	// archiveUnreachableAfter is how many connection failures in a row mark
	// a host unreachable.
	archiveUnreachableAfter = 3
)

// archiveArticle is one curated link reduced to what a reader needs offline.
type archiveArticle struct {
	Title       string
	Author      string
	Description string
	Base        *url.URL
	Content     *html.Node
	Image       string
	License     string
	Files       []archiveFile
	// Videos lists YouTube video IDs to download; Wayback is the timestamp of
	// the Wayback Machine capture the article came from, if any. Links are
	// the page's same-site links, where its subpages are found.
	Videos  []string
	Wayback string
	Links   []*url.URL
	// PageTitle is the document's own <title>, which names a guide's
	// chapter when the og:title names only the guide.
	PageTitle string
}

type archiveFile struct {
	Name string
	URL  string
}

// archiveSiteInfo locates the shared pages relative to one project directory.
type archiveSiteInfo struct {
	title      string
	home       string
	stylesheet string
}

// archiveFetcher follows redirects, bounds concurrent requests per host, and
// carries what the site parsers and the video download need.
type archiveFetcher struct {
	ctx    context.Context
	client *http.Client
	hosts  sync.Map
	// authorization holds an Authorization header value per API host, and
	// pace the least time between requests to a rate-limited host.
	authorization map[string]string
	pace          map[string]time.Duration
	paceMutex     sync.Mutex
	nextRequest   map[string]time.Time
	// root and workdir locate the build for tool containers.
	root, workdir string
	videos        chan struct{}
	// unreachable counts consecutive connection failures per host; a host
	// that keeps failing is skipped for the rest of the build.
	unreachable map[string]int
	// rules are the recipe's page rules, for pages a project links to.
	rules        []compiledArchiveRule
	fileBudget   int
	embedVideos  bool
	videoHeight  int
	videoMinutes int
}

func newArchiveFetcher(ctx context.Context) *archiveFetcher {
	fetcher := &archiveFetcher{
		ctx: ctx, client: archiveHTTPClient(), authorization: make(map[string]string), unreachable: make(map[string]int),
		pace: map[string]time.Duration{thingiverseAPIHost(): thingiverseRequestInterval}, nextRequest: make(map[string]time.Time),
		videos: make(chan struct{}, archiveVideoWorkers), fileBudget: 160 << 20, videoHeight: 480, videoMinutes: 60,
	}
	if token := os.Getenv("THINGIVERSE_TOKEN"); token != "" {
		fetcher.authorization[thingiverseAPIHost()] = "Bearer " + token
	}
	// Without a token the GitHub API allows 60 requests an hour, too few
	// for the repositories of a large collection.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		if api, err := url.Parse(githubAPI); err == nil {
			fetcher.authorization[api.Host] = "Bearer " + token
		}
	}
	return fetcher
}

func (f *archiveFetcher) get(target *url.URL, limit int) ([]byte, string, *url.URL, error) {
	return f.request(http.MethodGet, target, nil, nil, limit)
}

// request sends one request, retrying rate limits and gateway errors.
func (f *archiveFetcher) request(method string, target *url.URL, body []byte, header http.Header, limit int) ([]byte, string, *url.URL, error) {
	slot, _ := f.hosts.LoadOrStore(target.Host, make(chan struct{}, archiveHostConcurrent))
	slot.(chan struct{}) <- struct{}{}
	defer func() { <-slot.(chan struct{}) }()
	if f.skipHost(target.Host) {
		return nil, "", nil, fmt.Errorf("GET %s: host unreachable, skipped", target)
	}
	for attempt := 0; ; attempt++ {
		if err := f.wait(f.turn(target.Host)); err != nil {
			return nil, "", nil, err
		}
		req, err := http.NewRequestWithContext(f.ctx, method, target.String(), bytes.NewReader(body))
		if err != nil {
			return nil, "", nil, err
		}
		req.Header.Set("User-Agent", archiveUserAgent)
		for key, values := range header {
			req.Header[key] = values
		}
		if authorization := f.authorization[target.Host]; authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		response, err := f.client.Do(req)
		f.recordConnection(target.Host, err)
		if err != nil {
			return nil, "", nil, err
		}
		delay := time.Duration(2<<attempt) * time.Second
		switch response.StatusCode {
		case http.StatusTooManyRequests:
			// Rate windows run for minutes; wait them out rather than fail.
			delay = time.Duration(15<<attempt) * time.Second
			if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			fallthrough
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			if attempt < 4 {
				response.Body.Close()
				slog.Info("archive request retry", "host", target.Host, "status", response.StatusCode, "wait", delay)
				f.holdHost(target.Host, time.Now().Add(delay))
				if err := f.wait(time.Now().Add(delay)); err != nil {
					return nil, "", nil, err
				}
				continue
			}
		}
		return archiveReadResponse(response, target, limit)
	}
}

// paced reports whether archiving source waits on a rate-limited API, so it
// can run apart from the main worker pool.
func (f *archiveFetcher) paced(source *url.URL) bool {
	switch strings.ToLower(source.Host) {
	case "www.thingiverse.com", "thingiverse.com":
		f.paceMutex.Lock()
		defer f.paceMutex.Unlock()
		return f.authorization[thingiverseAPIHost()] != "" && f.pace[thingiverseAPIHost()] > 0
	}
	return false
}

// archiveHTTPClient gives up quickly on hosts that do not answer, while still
// letting large design files finish downloading.
func archiveHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 15 * time.Second
	transport.ResponseHeaderTimeout = 45 * time.Second
	return &http.Client{Timeout: 3 * time.Minute, Transport: transport}
}

func (f *archiveFetcher) skipHost(host string) bool {
	f.paceMutex.Lock()
	defer f.paceMutex.Unlock()
	return f.unreachable[host] >= archiveUnreachableAfter
}

// recordConnection counts consecutive connection failures; any answer from
// the host, even an error status, resets the count.
func (f *archiveFetcher) recordConnection(host string, err error) {
	f.paceMutex.Lock()
	defer f.paceMutex.Unlock()
	if err == nil {
		delete(f.unreachable, host)
		return
	}
	if f.ctx.Err() == nil {
		f.unreachable[host]++
		if f.unreachable[host] == archiveUnreachableAfter {
			slog.Warn("archive host unreachable, skipping it", "host", host, "err", err)
		}
	}
}

// holdHost keeps every request to a paced host back until the given time,
// so one rate-limit answer does not turn into one per worker, and halves the
// host's request rate: its real limit is lower than the pace assumed.
func (f *archiveFetcher) holdHost(host string, until time.Time) {
	f.paceMutex.Lock()
	defer f.paceMutex.Unlock()
	if f.pace[host] == 0 {
		return
	}
	if until.After(f.nextRequest[host]) {
		f.nextRequest[host] = until
		f.pace[host] = min(2*f.pace[host], archiveMaxPace)
		slog.Info("archive request pace lowered", "host", host, "interval", f.pace[host])
	}
}

// turn reserves the next request time for a paced host.
func (f *archiveFetcher) turn(host string) time.Time {
	f.paceMutex.Lock()
	defer f.paceMutex.Unlock()
	interval := f.pace[host]
	if interval == 0 {
		return time.Time{}
	}
	next := time.Now()
	if reserved := f.nextRequest[host]; reserved.After(next) {
		next = reserved
	}
	f.nextRequest[host] = next.Add(interval)
	return next
}

func (f *archiveFetcher) wait(until time.Time) error {
	delay := time.Until(until)
	if delay <= 0 {
		return f.ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-f.ctx.Done():
		return f.ctx.Err()
	case <-timer.C:
		return nil
	}
}

// archiveSiteParsers handle sites whose pages carry no content without
// scripts. Everything else goes through archiveHTMLArticle.
var archiveSiteParsers = map[string]func(*archiveFetcher, *url.URL) (*archiveArticle, error){
	"www.instructables.com": instructablesArticle,
	"instructables.com":     instructablesArticle,
	"hackaday.io":           hackadayArticle,
	"www.printables.com":    printablesArticle,
	"printables.com":        printablesArticle,
	"www.thingiverse.com":   thingiverseArticle,
	"thingiverse.com":       thingiverseArticle,
	"www.youtube.com":       youtubeArticle,
	"youtube.com":           youtubeArticle,
	"m.youtube.com":         youtubeArticle,
	"youtu.be":              youtubeArticle,
}

var (
	hackadayProjectPath = regexp.MustCompile(`^/project/(\d+)`)
	hackadayDetailsRule = &compiledArchiveRule{domain: "hackaday.io", content: cascadia.MustCompile(".post-content")}
)

// hackadayArticle prefers the project's details page: the landing page cuts
// the write-up short behind a "read more" link.
func hackadayArticle(fetcher *archiveFetcher, source *url.URL) (*archiveArticle, error) {
	if match := hackadayProjectPath.FindStringSubmatch(source.Path); match != nil {
		details := source.ResolveReference(&url.URL{Path: "/project/" + match[1] + "/details"})
		if article, err := archiveHTMLArticle(fetcher, details, hackadayDetailsRule); err == nil && len(archiveNodeText(article.Content)) > 400 {
			article.Title, _, _ = strings.Cut(article.Title, " | ")
			return article, nil
		}
	}
	return archiveHTMLArticle(fetcher, source, nil)
}

var archiveFileExtensions = map[string]bool{
	".stl": true, ".3mf": true, ".step": true, ".stp": true, ".obj": true, ".dxf": true, ".pdf": true,
	".scad": true, ".f3d": true, ".fcstd": true, ".zip": true, ".gcode": true, ".ino": true, ".hex": true,
	".uf2": true, ".kicad_pcb": true, ".skp": true,
}

var (
	archiveMainSelector  = cascadia.MustCompile(`article, main, [role="main"], [itemprop="articleBody"], .entry-content, .post-content, .article-content, .post, #content, #main`)
	archiveChromePattern = regexp.MustCompile(`(?i)(^|[-_])(nav|navbar|navigation|menu|sidebar|footer|comments?|respond|share|sharing|social|related|newsletter|subscribe|cookies?|consent|popup|modal|advert|ads?|breadcrumbs?|promo|widget|skip|banner)($|[-_])`)
	archiveKeptAttribute = map[string]bool{"href": true, "src": true, "alt": true, "title": true, "colspan": true, "rowspan": true, "start": true, "lang": true, "dir": true}
)

// archiveReader stores one curated link as a clean page with its images and
// attached files beside it.
func archiveReader(fetcher *archiveFetcher, source archiveSource, output string, site archiveSiteInfo, rule *compiledArchiveRule) ([]archivedPage, error) {
	parser := archiveSiteParsers[strings.ToLower(source.URL.Host)]
	var article *archiveArticle
	var err error
	if parser != nil {
		article, err = parser(fetcher, source.URL)
	} else {
		article, err = archiveHTMLArticle(fetcher, source.URL, rule)
	}
	if err != nil {
		if fetcher.ctx.Err() != nil {
			return nil, fetcher.ctx.Err()
		}
		fallback, waybackErr := archiveWaybackArticle(fetcher, source.URL, rule)
		if waybackErr != nil {
			return nil, fmt.Errorf("%w (%v)", err, waybackErr)
		}
		article = fallback
	}
	appendArchiveSubpages(fetcher, article, rule)
	appendArchiveLinkedGuides(fetcher, article, source.URL)
	repos := archiveRepos(article, source.URL)
	appendArchiveRepoReadmes(fetcher, article, repos, source.URL)
	entry := archiveEntryPath(source.URL)
	pages := []archivedPage{{URL: source.URL.String(), Path: entry, Title: article.Title, HTML: true}}

	files := archiveContentFiles(article)
	for _, anchor := range cascadia.QueryAll(article.Content, cascadia.MustCompile("a[href]")) {
		anchor.Attr = slices.DeleteFunc(anchor.Attr, func(attribute html.Attribute) bool { return attribute.Key == "href" })
	}
	pruneArchiveChrome(article.Content)
	adoptArchiveHeading(article)
	images := 0
	localized := make(map[string]string)
	var localize func(*html.Node)
	localize = func(node *html.Node) {
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.ElementNode && child.Data == "img" {
				local := ""
				if target, err := article.Base.Parse(archiveAttribute(child, "src")); err == nil && (target.Scheme == "http" || target.Scheme == "https") {
					key := target.String()
					if known, ok := localized[key]; ok {
						local = known
					} else if images < maxArchiveImages {
						stored, err := archiveStoreAsset(fetcher, target, source.URL, output, "image/", maxArchiveImageBytes)
						if err != nil && article.Wayback != "" {
							stored, err = archiveStoreAsset(fetcher, archiveWaybackURL(article.Wayback, "im_", target), source.URL, output, "image/", maxArchiveImageBytes)
						}
						if err == nil {
							local = stored
							images++
							pages = append(pages, archivedPage{URL: key, Path: stored})
						}
						localized[key] = local
					}
				}
				if local == "" {
					node.RemoveChild(child)
				} else {
					archiveSetAttribute(child, "src", archiveRelative(entry, local))
				}
			} else {
				localize(child)
			}
			child = next
		}
	}
	localize(article.Content)
	if images == 0 && article.Image != "" {
		lead := &html.Node{Type: html.ElementNode, Data: "img", DataAtom: atom.Img, Attr: []html.Attribute{{Key: "src", Val: article.Image}}}
		holder := archiveContainer()
		holder.AppendChild(lead)
		if localize(holder); holder.FirstChild != nil {
			holder.RemoveChild(lead)
			article.Content.InsertBefore(lead, article.Content.FirstChild)
		}
	}

	videos, err := archiveVideos(fetcher, article, output, entry)
	if err != nil {
		return nil, err
	}
	pages = append(pages, videos...)
	pages[0].Title = article.Title

	stored := make([]archiveStoredFile, 0, len(files))
	var anchors [][]*html.Node
	budget := fetcher.fileBudget
	for _, file := range files {
		if len(stored) >= maxArchiveFiles || budget <= 0 {
			break
		}
		target, err := article.Base.Parse(file.URL)
		if err != nil {
			continue
		}
		local, size, err := archiveStoreFile(fetcher, target, output, file.Name, budget)
		if err != nil && article.Wayback != "" {
			local, size, err = archiveStoreFile(fetcher, archiveWaybackURL(article.Wayback, "id_", target), output, file.Name, budget)
		}
		if err != nil {
			continue
		}
		budget -= size
		name := file.Name
		if name == "" || strings.Contains(name, "://") {
			name = path.Base(target.Path)
		}
		stored = append(stored, archiveStoredFile{Name: path.Base(name), Local: local, Size: size})
		anchors = append(anchors, file.anchors)
	}
	var bundle *archiveStoredFile
	if len(stored) > archiveBundleAfter {
		if packed, err := bundleArchiveFiles(output, entry, stored); err == nil {
			bundle = &packed
			pages = append(pages, archivedPage{URL: source.URL.String(), Path: packed.Local})
		}
	}
	var packedRepos []archiveStoredFile
	for _, repo := range repos {
		if budget <= 0 || fetcher.ctx.Err() != nil {
			break
		}
		packed, err := archiveStoreRepo(fetcher, repo, output, budget)
		if err != nil {
			slog.Info("archive repository skipped", "repo", repo.label(), "err", err)
			continue
		}
		budget -= packed.Size
		pages = append(pages, archivedPage{URL: source.URL.String(), Path: packed.Local})
		packedRepos = append(packedRepos, packed)
		for _, anchor := range repo.anchors {
			archiveSetAttribute(anchor, "href", archiveRelative(entry, packed.Local))
		}
	}
	for index, file := range stored {
		if bundle == nil {
			pages = append(pages, archivedPage{URL: source.URL.String(), Path: file.Local})
		}
		target := file.Local
		if bundle != nil {
			target = bundle.Local
		}
		for _, anchor := range anchors[index] {
			archiveSetAttribute(anchor, "href", archiveRelative(entry, target))
		}
	}
	cleanArchiveContent(article.Content)

	destination := filepath.Join(output, filepath.FromSlash(entry))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nil, err
	}
	page, err := renderArchiveArticle(article, source, archiveFilesBox(entry, stored, bundle, packedRepos), site, entry)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(destination, page, 0o644); err != nil {
		return nil, err
	}
	return pages, nil
}

func archiveHTMLArticle(fetcher *archiveFetcher, source *url.URL, rule *compiledArchiveRule) (*archiveArticle, error) {
	body, contentType, final, err := fetcher.get(source, maxArchiveResponseBytes)
	if err != nil {
		return nil, err
	}
	return archiveParseHTML(fetcher, body, contentType, final, rule)
}

// archiveParseHTML reduces a fetched page to its content; final is the
// address relative links resolve against.
func archiveParseHTML(fetcher *archiveFetcher, body []byte, contentType string, final *url.URL, rule *compiledArchiveRule) (*archiveArticle, error) {
	source := final
	if contentType != "" && !strings.Contains(contentType, "html") {
		name := path.Base(final.Path)
		return &archiveArticle{Base: final, Content: archiveContainer(), Files: []archiveFile{{Name: name, URL: final.String()}}}, nil
	}
	// Pages are written out as UTF-8, so decode older encodings first, as
	// named by the Content-Type header or the page's own meta tag.
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		reader = bytes.NewReader(body)
	}
	document, err := html.Parse(reader)
	if err != nil {
		return nil, err
	}
	meta := archiveMeta(document)
	article := &archiveArticle{Base: final, Title: meta["og:title"], Description: meta["og:description"], Image: meta["og:image"], Author: meta["author"]}
	article.PageTitle = archiveDocumentTitle(document)
	if article.Title == "" || strings.HasPrefix(article.Title, "|") {
		article.Title = article.PageTitle
	}
	if article.Description == "" {
		article.Description = meta["description"]
	}
	if archiveChallengeTitle.MatchString(article.Title) || archiveChallengePage(document) {
		return nil, fmt.Errorf("GET %s: blocked by a browser challenge", source)
	}
	if base := cascadia.Query(document, cascadia.MustCompile("base[href]")); base != nil {
		if resolved, err := final.Parse(archiveAttribute(base, "href")); err == nil {
			article.Base = resolved
		}
	}
	resolveArchiveLazyImages(document)
	article.Links = archiveLinks(document, article.Base)
	archiveMarkVideoEmbeds(fetcher, article, document)
	pruneArchiveElements(document)
	matched := rule != nil && rule.content != nil && cascadia.Query(document, rule.content) != nil
	applyArchiveRule(document, rule)
	selected := archiveBody(document)
	if !matched {
		if main := archiveMainContent(document); main != nil {
			selected = main
		}
	}
	article.Content = archiveContainer()
	if selected != nil {
		for child := selected.FirstChild; child != nil; {
			next := child.NextSibling
			selected.RemoveChild(child)
			article.Content.AppendChild(child)
			child = next
		}
	}
	if final.Host == "github.com" {
		article.Title = strings.TrimPrefix(article.Title, "GitHub - ")
		if parts := strings.Split(strings.Trim(final.Path, "/"), "/"); len(parts) == 2 {
			article.Title = parts[0] + "/" + parts[1]
		}
	}
	return article, nil
}

func archiveContainer() *html.Node {
	return &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
}

// archiveMainContent picks the most specific landmark that still holds most of
// the page's landmark text, or nil when the landmarks miss the real content.
func archiveMainContent(document *html.Node) *html.Node {
	candidates := make([]*html.Node, 0)
	for _, candidate := range cascadia.QueryAll(document, archiveMainSelector) {
		if !archiveChromeNamed(candidate) {
			candidates = append(candidates, candidate)
		}
	}
	largest := 0
	sizes := make([]int, len(candidates))
	for index, candidate := range candidates {
		sizes[index] = len(strings.TrimSpace(archiveNodeText(candidate)))
		largest = max(largest, sizes[index])
	}
	if body := archiveBody(document); body == nil || largest < 1500 && largest*4 < len(strings.TrimSpace(archiveNodeText(body))) {
		return nil
	}
	var selected *html.Node
	smallest := 0
	for index, candidate := range candidates {
		if sizes[index]*10 >= largest*6 && (selected == nil || sizes[index] < smallest) {
			selected, smallest = candidate, sizes[index]
		}
	}
	return selected
}

func archiveChromeNamed(node *html.Node) bool {
	for _, token := range strings.Fields(archiveAttribute(node, "class") + " " + archiveAttribute(node, "id")) {
		if archiveChromePattern.MatchString(token) {
			return true
		}
	}
	return false
}

func archiveMeta(document *html.Node) map[string]string {
	meta := make(map[string]string)
	for _, node := range cascadia.QueryAll(document, cascadia.MustCompile("meta")) {
		key := strings.ToLower(archiveAttribute(node, "property"))
		if key == "" {
			key = strings.ToLower(archiveAttribute(node, "name"))
		}
		if content := strings.TrimSpace(archiveAttribute(node, "content")); key != "" && content != "" && meta[key] == "" {
			meta[key] = content
		}
	}
	return meta
}

// resolveArchiveLazyImages moves deferred image sources into src before the
// sanitizer drops data attributes and srcset. Lazy loaders name the attribute
// freely (data-src, data-lazy-src, data-lzl-src, …), so any data-*src* counts.
func resolveArchiveLazyImages(document *html.Node) {
	for _, image := range cascadia.QueryAll(document, cascadia.MustCompile("img")) {
		source, srcset := "", ""
		for _, attribute := range image.Attr {
			key, value := strings.ToLower(attribute.Key), strings.TrimSpace(attribute.Val)
			if value == "" || strings.HasPrefix(value, "data:") {
				continue
			}
			switch {
			case strings.Contains(key, "srcset"):
				if srcset == "" {
					srcset = archiveSrcsetBest(value)
				}
			case source == "" && (key == "data-original" || key == "data-orig-file" || key == "data-large-file" || strings.HasPrefix(key, "data-") && strings.Contains(key, "src")):
				source = value
			}
		}
		current := strings.TrimSpace(archiveAttribute(image, "src"))
		if source == "" && (current == "" || strings.HasPrefix(current, "data:")) {
			if source = archivePlaceholderSource(current); source == "" {
				source = srcset
			}
		}
		if source != "" {
			archiveSetAttribute(image, "src", source)
		}
	}
}

var archivePlaceholderURL = regexp.MustCompile(`data-u="([^"]+)"`)

// archivePlaceholderSource reads the real address out of an SVG placeholder
// that carries it as data-u, as ShortPixel's lazy loader writes them.
func archivePlaceholderSource(source string) string {
	encoded, found := strings.CutPrefix(source, "data:image/svg+xml;base64,")
	if !found {
		return ""
	}
	placeholder, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	match := archivePlaceholderURL.FindSubmatch(placeholder)
	if match == nil {
		return ""
	}
	address, err := url.QueryUnescape(string(match[1]))
	if err != nil {
		return ""
	}
	return address
}

func archiveSrcsetBest(srcset string) string {
	best, bestWidth := "", 0
	for _, candidate := range strings.Split(srcset, ",") {
		fields := strings.Fields(candidate)
		if len(fields) == 0 {
			continue
		}
		width := 1
		if len(fields) > 1 {
			width, _ = strconv.Atoi(strings.TrimRight(fields[1], "wx"))
		}
		if best == "" || width > bestWidth && width <= 1600 {
			best, bestWidth = fields[0], width
		}
	}
	return best
}

// pruneArchiveChrome drops navigation, forms, and other page furniture that
// has no use in an offline copy.
func pruneArchiveChrome(content *html.Node) {
	total := len(archiveNodeText(content))
	var prune func(*html.Node)
	prune = func(node *html.Node) {
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.ElementNode && archiveChromeElement(child, total) {
				node.RemoveChild(child)
			} else {
				prune(child)
			}
			child = next
		}
	}
	prune(content)
}

func archiveChromeElement(node *html.Node, total int) bool {
	switch node.Data {
	case "nav", "header", "footer", "aside", "form", "button", "input", "select", "textarea", "svg", "canvas", "dialog", "template", "video", "audio", "source", "label":
		return true
	}
	if _, hidden := archiveLookupAttribute(node, "hidden"); hidden || archiveAttribute(node, "aria-hidden") == "true" {
		return true
	}
	return archiveChromeNamed(node) && len(archiveNodeText(node))*10 < total*4
}

// adoptArchiveHeading drops a leading heading that repeats the page title, and
// prefers its wording when the title only adds a site name to it.
func adoptArchiveHeading(article *archiveArticle) {
	heading := cascadia.Query(article.Content, cascadia.MustCompile("h1"))
	if heading == nil || heading.Parent == nil {
		return
	}
	text := strings.Join(strings.Fields(archiveNodeText(heading)), " ")
	title := strings.ToLower(article.Title)
	switch {
	case len(text) >= 4 && strings.Contains(title, strings.ToLower(text)):
		article.Title = text
	case title == "" || !strings.Contains(strings.ToLower(text), title):
		return
	}
	heading.Parent.RemoveChild(heading)
}

// cleanArchiveContent keeps structural attributes only and drops elements left
// empty by earlier pruning.
func cleanArchiveContent(node *html.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && child.Data == "video" {
			child = next
			continue
		}
		if child.Type == html.ElementNode {
			attributes := child.Attr[:0]
			for _, attribute := range child.Attr {
				if archiveKeptAttribute[attribute.Key] {
					attributes = append(attributes, attribute)
				}
			}
			child.Attr = attributes
			cleanArchiveContent(child)
			switch child.Data {
			case "img", "br", "hr", "td", "th":
			default:
				if child.FirstChild == nil {
					node.RemoveChild(child)
				}
			}
		} else if child.Type == html.TextNode && strings.TrimSpace(child.Data) == "" && (child.PrevSibling == nil || next == nil) {
			node.RemoveChild(child)
		}
		child = next
	}
}

type archiveContentFile struct {
	archiveFile
	anchors []*html.Node
}

// archiveContentFiles lists the article's attachments plus linked design
// files, remembering which anchors point at each one.
func archiveContentFiles(article *archiveArticle) []archiveContentFile {
	files := make([]archiveContentFile, 0, len(article.Files))
	index := make(map[string]int)
	for _, file := range article.Files {
		index[file.URL] = len(files)
		files = append(files, archiveContentFile{archiveFile: file})
	}
	for _, anchor := range cascadia.QueryAll(article.Content, cascadia.MustCompile("a[href]")) {
		target, err := article.Base.Parse(archiveAttribute(anchor, "href"))
		if err != nil || target.Scheme != "http" && target.Scheme != "https" || !archiveFileExtensions[strings.ToLower(path.Ext(target.Path))] {
			continue
		}
		target.Fragment = ""
		position, ok := index[target.String()]
		if !ok {
			position = len(files)
			index[target.String()] = position
			files = append(files, archiveContentFile{archiveFile: archiveFile{Name: path.Base(target.Path), URL: target.String()}})
		}
		files[position].anchors = append(files[position].anchors, anchor)
	}
	return files
}

func archiveStoreAsset(fetcher *archiveFetcher, target, source *url.URL, output, wantType string, limit int) (string, error) {
	body, contentType, _, err := fetcher.get(target, limit)
	if err != nil {
		return "", err
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	extension := strings.ToLower(path.Ext(target.Path))
	if !strings.HasPrefix(mediaType, wantType) && !archiveImageExtension(extension) {
		return "", fmt.Errorf("GET %s: unexpected content type %q", target, contentType)
	}
	if len(body) < 512 {
		return "", fmt.Errorf("GET %s: placeholder image", target)
	}
	local := archiveLocalPath(target, "")
	optimized, changed := archiveOptimizeImage(body)
	if changed {
		body, extension = optimized, ""
		mediaType = "image/jpeg"
	}
	if changed || target.Host != source.Host || target.RawQuery != "" || !archiveImageExtension(extension) || strings.ContainsAny(local, `'"&<>% `) {
		sum := sha256.Sum256([]byte(target.String()))
		if !archiveImageExtension(extension) {
			extension = archiveMediaExtension(mediaType)
		}
		local = "_assets/" + hex.EncodeToString(sum[:8]) + extension
	}
	destination := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	return local, os.WriteFile(destination, body, 0o644)
}

// archiveStoreFile saves one attachment of at most budget bytes and reports
// its size.
func archiveStoreFile(fetcher *archiveFetcher, target *url.URL, output, name string, budget int) (string, int, error) {
	body, contentType, _, err := fetcher.get(target, min(maxArchiveResponseBytes, budget))
	if err != nil {
		return "", 0, err
	}
	if strings.Contains(contentType, "text/html") || !archiveFileMatches(body, name) {
		return "", 0, fmt.Errorf("GET %s: not a file", target)
	}
	sum := sha256.Sum256([]byte(target.String()))
	local := "_files/" + hex.EncodeToString(sum[:3]) + "-" + archiveFileName(name)
	destination := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", 0, err
	}
	return local, len(body), os.WriteFile(destination, body, 0o644)
}

func archiveFileName(name string) string {
	cleaned := strings.Map(func(char rune) rune {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '.', char == '-', char == '_':
			return char
		}
		return '_'
	}, path.Base(name))
	if len(cleaned) > 80 {
		cleaned = cleaned[len(cleaned)-80:]
	}
	return strings.TrimLeft(cleaned, ".")
}

func archiveImageExtension(extension string) bool {
	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif":
		return true
	}
	return false
}

func archiveMediaExtension(mediaType string) string {
	switch mediaType {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/avif":
		return ".avif"
	}
	return ".jpg"
}

// archiveEntryPath names a project's page. A link straight to a PDF or other
// file still gets an HTML page, which then offers the stored file.
func archiveEntryPath(source *url.URL) string {
	local := archiveLocalPath(source, "")
	if extension := path.Ext(local); archiveFileExtensions[extension] || archiveImageExtension(extension) {
		local += ".html"
	}
	return local
}

func archiveRelative(from, to string) string {
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(to))
	if err != nil {
		return to
	}
	return filepath.ToSlash(relative)
}

func archiveAttribute(node *html.Node, key string) string {
	value, _ := archiveLookupAttribute(node, key)
	return value
}

func archiveLookupAttribute(node *html.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val, true
		}
	}
	return "", false
}

func archiveSetAttribute(node *html.Node, key, value string) {
	for index := range node.Attr {
		if node.Attr[index].Key == key {
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

// archiveChallengeText is what bot-check interstitials say in place of the
// page; a short page saying it is no article.
var (
	archiveChallengeTitle = regexp.MustCompile(`(?i)^(just a moment|one moment, please|attention required)`)
	archiveChallengeText  = regexp.MustCompile(`(?i)request is being verified|checking your browser|verify you are (a )?human|enable javascript and cookies to continue`)
)

func archiveChallengePage(document *html.Node) bool {
	body := archiveBody(document)
	if body == nil {
		return false
	}
	text := archiveNodeText(body)
	return len(text) < 3000 && archiveChallengeText.MatchString(text)
}

// archiveFileMatches rejects a download whose bytes are not what its name
// says, such as an error page saved under a .zip name.
func archiveFileMatches(body []byte, name string) bool {
	head := body[:min(len(body), 1024)]
	switch strings.ToLower(path.Ext(name)) {
	case ".zip", ".3mf", ".f3d", ".fcstd":
		return bytes.HasPrefix(body, []byte("PK"))
	case ".pdf":
		return bytes.Contains(head, []byte("%PDF"))
	case ".html", ".htm", ".svg":
		return true
	}
	return !strings.HasPrefix(http.DetectContentType(head), "text/html")
}
