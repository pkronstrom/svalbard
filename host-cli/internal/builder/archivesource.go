package builder

import (
	"context"
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

func archiveSources(ctx context.Context, build *catalog.BuildSpec, workdir string) ([]*url.URL, error) {
	switch build.Config["source_format"] {
	case "", "html":
		source, err := archiveURL(build.SourceURL)
		if err != nil {
			return nil, err
		}
		return []*url.URL{source}, nil
	case "pdf-links":
		path := filepath.Join(workdir, "source.pdf")
		if err := stepDownload(ctx, build.SourceURL, path); err != nil {
			return nil, err
		}
		sources, err := archivePDFURLs(path)
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

func archivePDFURLs(path string) ([]*url.URL, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	text, err := reader.GetPlainText()
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(text)
	if err != nil {
		return nil, err
	}
	return archiveURLsFromText(string(contents)), nil
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

func archiveHostPath(host string) string {
	return strings.NewReplacer(":", "_", "/", "_").Replace(host)
}

func writeArchiveIndex(path string, sources []*url.URL) error {
	var page strings.Builder
	page.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Archive</title></head><body><h1>Archive</h1><ul>")
	for _, source := range sources {
		local := filepath.ToSlash(filepath.Join(archiveHostPath(source.Host), archiveLocalPath(source, "")))
		fmt.Fprintf(&page, "<li><a href=%q>%s</a></li>", local, source.String())
	}
	page.WriteString("</ul></body></html>")
	return os.WriteFile(path, []byte(page.String()), 0o644)
}
