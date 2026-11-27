package search

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

// MergeZIMKeywords adds native full-text hits to the SQLite result list.
// covered indicates that this scope has native sources, even if Kiwix fails.
// The caller can still use the returned SQLite hits with a coverage warning.
func MergeZIMKeywords(ctx context.Context, eng *engine.Engine, start func(context.Context) (int, error), query, scope string, limit int, local []Result) (results []Result, covered bool, err error) {
	words := engine.QueryWords(query)
	if limit <= 0 || len(words) == 0 {
		return local, false, nil
	}
	groups, err := eng.NativeZIMGroups(ctx, scope)
	if err != nil || len(groups) == 0 {
		return local, false, err
	}
	port, err := start(ctx)
	if err != nil {
		return local, true, err
	}

	// libkiwix searches archives of the same language together. Issue one
	// request per group and keep list order independent of response timing.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	lists := make([][]Result, len(groups)+1)
	lists[0] = local
	failures := make([]error, len(groups))
	slots := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, books := range groups {
		wg.Go(func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				failures[i] = ctx.Err()
				return
			}
			lists[i+1], failures[i] = KiwixKeyword(ctx, port, books, strings.Join(words, " "), min(limit*4, 200))
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return local, true, err
	}
	if !slices.ContainsFunc(lists[1:], func(hits []Result) bool { return len(hits) > 0 }) {
		return local, true, errors.Join(failures...)
	}
	return blendKeywordResults(lists, words, scope, limit), true, errors.Join(failures...)
}

// KiwixKeyword searches the selected books served by a loopback Kiwix process.
// Book names are the ZIM filenames without their extension.
func KiwixKeyword(ctx context.Context, port int, books []string, query string, limit int) ([]Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	endpoint := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/search"}
	endpoint.RawQuery = url.Values{
		"books.name": books,
		"pattern":    {query},
		"format":     {"xml"},
		"pageLength": {fmt.Sprint(limit)},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kiwix search for %s returned HTTP %d", books[0], resp.StatusCode)
	}
	var rss struct {
		XMLName xml.Name `xml:"rss"`
		Items   []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rss); err != nil {
		return nil, fmt.Errorf("decoding Kiwix result: %w", err)
	}
	var results []Result
	for _, item := range rss.Items {
		link, err := url.Parse(item.Link)
		if err != nil {
			continue
		}
		content, ok := strings.CutPrefix(link.Path, "/content/")
		book, path, split := strings.Cut(content, "/")
		if !ok || !split || path == "" || !slices.Contains(books, book) {
			continue
		}
		r := Result{Filename: book + ".zim", Path: "/" + path, Title: strings.TrimSpace(item.Title), Native: true}
		if page, err := ExtractText(item.Description); err == nil {
			r.Snippet = strings.Join(strings.Fields(page.Body), " ")
			if text := []rune(r.Snippet); len(text) > 320 {
				r.Snippet = string(text[:320]) + "…"
			}
		}
		results = append(results, r)
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

func blendKeywordResults(lists [][]Result, words []string, scope string, limit int) []Result {
	type candidate struct {
		hit    Result
		weight float64
		source string
	}
	var candidates []candidate
	seen := make(map[[2]string]bool)
	query := strings.ToLower(strings.Join(words, " "))
	for _, list := range lists {
		for rank, hit := range list {
			key := [2]string{hit.Filename, strings.TrimLeft(hit.Path, "/")}
			if seen[key] {
				continue
			}
			seen[key] = true
			// Xapian and SQLite scores use different units. Compare rank
			// within each list and give extra weight to matches in the title.
			titleWords := engine.QueryWords(hit.Title)
			score := 1 / float64(4+rank)
			for _, word := range words {
				if slices.ContainsFunc(titleWords, func(titleWord string) bool { return strings.EqualFold(titleWord, word) }) {
					score += 0.1 / float64(len(words))
				}
			}
			if strings.ToLower(strings.Join(titleWords, " ")) == query {
				score++
			}
			source, _, embedded := engine.EmbeddedArchive(hit.Filename)
			if !embedded {
				source = hit.Filename
			}
			candidates = append(candidates, candidate{hit, score, source})
		}
	}
	counts := make(map[string]int)
	var results []Result
	for len(results) < limit && len(candidates) > 0 {
		weight := func(c candidate) float64 {
			if scope == "" && strings.HasSuffix(strings.ToLower(c.source), ".zim") {
				return c.weight / (1 + 0.6*float64(counts[c.source]))
			}
			return c.weight
		}
		best := 0
		for i := 1; i < len(candidates); i++ {
			a, b := candidates[i], candidates[best]
			switch {
			case a.hit.Partial != b.hit.Partial:
				if !a.hit.Partial {
					best = i
				}
			case a.hit.Partial && len(a.hit.Matched) != len(b.hit.Matched):
				if len(a.hit.Matched) > len(b.hit.Matched) {
					best = i
				}
			case weight(a) > weight(b):
				best = i
			}
		}
		selected := candidates[best]
		counts[selected.source]++
		results = append(results, selected.hit)
		candidates = slices.Delete(candidates, best, best+1)
	}
	return results
}
