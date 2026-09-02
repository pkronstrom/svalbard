package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type archiveSource struct {
	ID   string
	URL  *url.URL
	Page int
}

type archiveProject struct {
	ID         string `json:"id"`
	SourceURL  string `json:"source_url"`
	SourcePage int    `json:"source_page,omitempty"`
	EntryPath  string `json:"entry_path"`
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
		for index := 0; index < annotations.Len(); index++ {
			source, err := archiveURL(annotations.Index(index).Key("A").Key("URI").Text())
			if err != nil || seen[source.String()] {
				continue
			}
			seen[source.String()] = true
			sources = append(sources, newArchiveSource(source, page))
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

func writeArchiveIndex(path string, sources []archiveSource) error {
	var page strings.Builder
	page.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Archive</title></head><body><h1>Archive</h1><ul>")
	for _, source := range sources {
		local := filepath.ToSlash(filepath.Join("projects", source.ID, archiveLocalPath(source.URL, "")))
		fmt.Fprintf(&page, "<li><a href=%q>%s</a></li>", local, source.URL.String())
	}
	page.WriteString("</ul></body></html>")
	return os.WriteFile(path, []byte(page.String()), 0o644)
}

func writeArchiveProject(path string, source archiveSource, entryPath string) error {
	data, err := json.MarshalIndent(archiveProject{
		ID: source.ID, SourceURL: source.URL.String(), SourcePage: source.Page, EntryPath: entryPath,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
