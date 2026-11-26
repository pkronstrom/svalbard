package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

var archiveURLPattern = regexp.MustCompile(`https?://[^\s<>()\[\]{}"']+`)

const archiveProjectVersion = 8

type archiveSource struct {
	ID   string
	URL  *url.URL
	Page int
	// Category, Label, and Author come from the list that links to the source.
	Category string
	Label    string
	Author   string
}

type archiveListing struct {
	label  string
	author string
}

var (
	archiveListingHeading    = regexp.MustCompile(`^\s*(?:\d{3}\s+)?([A-Z](?:[A-Z &/,'’.+\-]|[23]D)*)`)
	archiveListingCredit     = regexp.MustCompile(`(?s)^(.*?)\s*(?:-\s*|\s)by\s+(.*?)\s*\(via [^)]*\)`)
	archiveListingPageNumber = regexp.MustCompile(`^\s*\d{3}`)
	archiveListingByline     = regexp.MustCompile(`^(.*?)\s*-\s*by\s+([^\n]*)`)
)

// archivePDFListing reads a catalog page laid out as a heading followed by
// numbered entries: "GAMING / CONSOLES 01 Name blurb - by Author (via Site). 02 …".
func archivePDFListing(text string) (string, []archiveListing) {
	match := archiveListingHeading.FindStringSubmatch(text)
	if match == nil {
		return "", nil
	}
	category := strings.Trim(strings.Join(strings.Fields(match[1]), " "), " /&+-.,")
	if len(category) < 3 {
		return "", nil
	}
	items := make([]archiveListing, 0)
	rest := archiveListingPageNumber.ReplaceAllString(text[len(match[0]):], "")
	marker := func(number int) int {
		location := regexp.MustCompile(fmt.Sprintf(`(?:^|[\s.)A-Z])%02d [^\s]`, number)).FindStringIndex(rest)
		if location == nil {
			return -1
		}
		return strings.Index(rest[location[0]:], fmt.Sprintf("%02d ", number)) + location[0]
	}
	start := marker(1)
	for number := 1; start >= 0; number++ {
		rest = rest[start+3:]
		entry := rest
		if start = marker(number + 1); start >= 0 {
			entry = rest[:start]
		}
		item := archiveListing{label: entry}
		if credit := archiveListingCredit.FindStringSubmatch(entry); credit != nil {
			item.label, item.author = credit[1], credit[2]
		} else if credit := archiveListingByline.FindStringSubmatch(entry); credit != nil {
			item.label, item.author = credit[1], credit[2]
		} else if line := strings.Index(entry, "\n"); line >= 0 {
			item.label = entry[:line]
		}
		item.label = strings.Trim(strings.Join(strings.Fields(item.label), " "), " .-")
		item.author = strings.Trim(strings.Join(strings.Fields(item.author), " "), " .")
		items = append(items, item)
	}
	return category, items
}

type archiveProject struct {
	Version    int            `json:"version"`
	ID         string         `json:"id"`
	SourceURL  string         `json:"source_url"`
	SourcePage int            `json:"source_page,omitempty"`
	EntryPath  string         `json:"entry_path"`
	Title      string         `json:"title,omitempty"`
	Thumbnail  string         `json:"thumbnail,omitempty"`
	Pages      []archivedPage `json:"pages"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
}

func archiveSources(ctx context.Context, build *catalog.BuildSpec, workdir string) ([]archiveSource, error) {
	switch build.Config["source_format"] {
	case "", "html":
		source, err := archiveURL(build.SourceURL)
		if err != nil {
			return nil, err
		}
		return []archiveSource{newArchiveSource(source, 0)}, nil
	case "pdf-links":
		path := filepath.Join(workdir, "source.pdf")
		if err := stepDownload(ctx, build.SourceURL, path); err != nil {
			return nil, err
		}
		sources, err := archivePDFSources(path)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return nil, fmt.Errorf("no HTTP URLs extracted from PDF")
		}
		return sources, nil
	default:
		return nil, fmt.Errorf("unsupported source_format %q", build.Config["source_format"])
	}
}

func limitArchiveSources(sources []archiveSource, maximum int) []archiveSource {
	if maximum > 0 && len(sources) > maximum {
		return sources[:maximum]
	}
	return sources
}

func archivePDFURLs(path string) ([]*url.URL, error) {
	sources, err := archivePDFSources(path)
	if err != nil {
		return nil, err
	}
	urls := make([]*url.URL, len(sources))
	for index, source := range sources {
		urls[index] = source.URL
	}
	return urls, nil
}

func archivePDFSources(path string) ([]archiveSource, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sources := make([]archiveSource, 0)
	seen := make(map[string]bool)
	for page := 1; page <= reader.NumPage(); page++ {
		annotations := reader.Page(page).V.Key("Annots")
		first := len(sources)
		onPage := make(map[string]bool)
		position := 0
		positions := make([]int, 0)
		for index := 0; index < annotations.Len(); index++ {
			source, err := archiveURL(annotations.Index(index).Key("A").Key("URI").Text())
			if err != nil {
				continue
			}
			if !onPage[source.String()] {
				onPage[source.String()] = true
				position++
			}
			if seen[source.String()] {
				continue
			}
			seen[source.String()] = true
			sources = append(sources, newArchiveSource(source, page))
			positions = append(positions, position)
		}
		if len(sources) == first {
			continue
		}
		text, err := reader.Page(page).GetPlainText(nil)
		if err != nil {
			continue
		}
		category, items := archivePDFListing(text)
		for offset, position := range positions {
			sources[first+offset].Category = category
			if len(items) == len(onPage) {
				sources[first+offset].Label, sources[first+offset].Author = items[position-1].label, items[position-1].author
			}
		}
	}
	if len(sources) > 0 {
		return sources, nil
	}
	text, err := reader.GetPlainText()
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(text)
	if err != nil {
		return nil, err
	}
	for _, source := range archiveURLsFromText(string(contents)) {
		sources = append(sources, newArchiveSource(source, 0))
	}
	return sources, nil
}

func archiveURLsFromText(text string) []*url.URL {
	seen := make(map[string]bool)
	urls := make([]*url.URL, 0)
	for _, raw := range archiveURLPattern.FindAllString(text, -1) {
		source, err := archiveURL(strings.TrimRight(raw, ".,;:!?"))
		if err != nil || seen[source.String()] {
			continue
		}
		seen[source.String()] = true
		urls = append(urls, source)
	}
	return urls
}

func newArchiveSource(source *url.URL, page int) archiveSource {
	sum := sha256.Sum256([]byte(source.String()))
	return archiveSource{
		ID:   archiveHostPath(source.Host) + "-" + hex.EncodeToString(sum[:6]),
		URL:  source,
		Page: page,
	}
}

func archiveHostPath(host string) string {
	return strings.NewReplacer(":", "_", "/", "_").Replace(host)
}

func archiveIndexHref(local string) string {
	return template.HTMLEscapeString(strings.ReplaceAll(local, "'", "%27"))
}

func writeArchiveProject(path string, source archiveSource, entry archivedPage, pages []archivedPage, thumbnail, failure string) error {
	status := "archived"
	if failure != "" {
		status = "unavailable"
	}
	data, err := json.MarshalIndent(archiveProject{
		Version: archiveProjectVersion, ID: source.ID, SourceURL: source.URL.String(), SourcePage: source.Page, EntryPath: entry.Path, Title: entry.Title, Thumbnail: thumbnail, Pages: pages, Status: status, Error: failure,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readArchiveProject(path string) (archiveProject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return archiveProject{}, err
	}
	var project archiveProject
	if err := json.Unmarshal(data, &project); err != nil {
		return archiveProject{}, err
	}
	return project, nil
}
