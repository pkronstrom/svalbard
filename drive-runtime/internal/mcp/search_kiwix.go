package mcp

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func (c *SearchCapability) searchArchive(ctx context.Context, book, query, detail string, limit int) (ActionResult, error) {
	port, err := c.ensureKiwix(book)
	if err != nil {
		return ActionResult{}, fmt.Errorf("%s is installed and not in the local index yet. Built-in search unavailable: %w", book, err)
	}
	hits, err := search.KiwixKeyword(ctx, port, []string{book}, query, limit)
	if err != nil {
		return ActionResult{}, fmt.Errorf("%s is installed and not in the local index yet. Built-in search unavailable: %w", book, err)
	}
	items := make([]searchResultItem, 0, len(hits))
	for _, hit := range hits {
		item := searchResultItem{Source: book, Path: hit.Path, Title: hit.Title}
		if detail != "link" {
			item.Snippet = hit.Snippet
		}
		if detail == "full" {
			if page, _, err := c.fetchPage(ctx, book, strings.TrimLeft(hit.Path, "/")); err == nil {
				item.Body = articleOpening(page.Body, c.read.Opening)
			}
		}
		items = append(items, item)
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
