package mcp

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func (c *SearchCapability) searchArchive(ctx context.Context, book, query, detail string, limit int) (ActionResult, error) {
	port, err := c.ensureKiwix(book)
	if err != nil {
		return ActionResult{}, fmt.Errorf("%s is installed and not in the local index yet. Built-in search unavailable: %w", book, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	endpoint := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/search"}
	endpoint.RawQuery = url.Values{
		"books.name": {book},
		"pattern":    {query},
		"format":     {"xml"},
		"pageLength": {fmt.Sprint(limit)},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ActionResult{}, err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return ActionResult{}, fmt.Errorf("built-in search for %s: %w", book, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ActionResult{}, fmt.Errorf("%s is installed and not in the local index yet. Its built-in search returned HTTP %d. Use search_read to browse the archive", book, resp.StatusCode)
	}
	var rss struct {
		XMLName xml.Name `xml:"rss"`
		Items   []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rss); err != nil {
		return ActionResult{}, fmt.Errorf("reading built-in search results for %s: %w", book, err)
	}
	items := make([]searchResultItem, 0, min(limit, len(rss.Items)))
	for _, hit := range rss.Items {
		link, err := url.Parse(hit.Link)
		if err != nil {
			continue
		}
		path, ok := strings.CutPrefix(link.Path, "/content/"+book+"/")
		if !ok || path == "" {
			continue
		}
		item := searchResultItem{Source: book, Path: "/" + path, Title: strings.TrimSpace(hit.Title)}
		if detail != "link" {
			if snippet, err := search.ExtractText(hit.Description); err == nil {
				item.Snippet = opening(strings.Join(strings.Fields(snippet.Body), " "), 320)
			}
		}
		if detail == "full" {
			if page, _, err := c.fetchPage(ctx, book, path); err == nil {
				item.Body = articleOpening(page.Body, c.read.Opening)
			}
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	note := "Used the archive's built-in keyword search."
	if len(items) == 0 {
		note = "No matches in " + book + "'s built-in search. Try a specific subject or a different archive."
	}
	return ActionResult{Data: items, Text: note}, nil
}

func (c *SearchCapability) handleArchiveLink(ctx context.Context, params map[string]any) (ActionResult, error) {
	source, path := strings.TrimSpace(getString(params, "source")), strings.TrimSpace(getString(params, "path"))
	if source == "" || path == "" {
		return ActionResult{}, fmt.Errorf("source and article path are required")
	}
	filename, err := c.resolveSource(ctx, source, true)
	if err != nil {
		return ActionResult{}, err
	}
	if !strings.EqualFold(filepath.Ext(filename), ".zim") {
		return ActionResult{}, fmt.Errorf("search_link serves ZIM articles. Use the file reference returned by search for document pages")
	}
	_, canonical, err := c.readFullPage(ctx, filename, pathCandidates(path))
	if err != nil {
		return ActionResult{}, err
	}
	port, err := c.ensureKiwix(sourceName(filename))
	if err != nil {
		return ActionResult{}, err
	}
	link := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
		Path:   "/content/" + sourceName(filename) + "/" + strings.TrimLeft(canonical, "/"),
	}
	return ActionResult{Data: map[string]string{"url": link.String(), "source": sourceName(filename), "path": canonical}}, nil
}
