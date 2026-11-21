// Command size-drift compares each recipe's size_gb with the Content-Length of
// its upstream download and lists recipes that drifted past a threshold. It
// needs the network, so it is not part of `make verify`.
//
// Usage: go run ./cmd/size-drift [-threshold 0.3] [-min-gb 0.01]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/resolver"
)

type result struct {
	ID       string
	Declared float64
	ActualGB float64
	Drifted  bool
	Err      error
}

func main() {
	threshold := flag.Float64("threshold", 0.3, "report recipes whose size differs by more than this fraction")
	minGB := flag.Float64("min-gb", 0.01, "ignore drift smaller than this many GB")
	flag.Parse()

	cat, err := catalog.LoadCatalog()
	if err != nil {
		fmt.Fprintln(os.Stderr, "loading catalog:", err)
		os.Exit(2)
	}
	var items []catalog.Item
	for _, item := range cat.AllRecipes() {
		// Build recipes produce their artifact locally; python packages have no single URL.
		if item.SizeGB > 0 && item.Build == nil && (item.URL != "" || item.URLPattern != "" || len(item.Platforms) > 0) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	client := &http.Client{Timeout: 30 * time.Second}
	results := make([]result, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, item := range items {
		wg.Add(1)
		go func(i int, item catalog.Item) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = checkRecipe(context.Background(), client, item, *threshold, *minGB)
		}(i, item)
	}
	wg.Wait()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RECIPE\tSIZE_GB\tUPSTREAM_GB\tNOTE")
	drifted, unknown := 0, 0
	for _, r := range results {
		switch {
		case r.Err != nil:
			unknown++
			fmt.Fprintf(w, "%s\t%.3f\t?\t%v\n", r.ID, r.Declared, r.Err)
		case r.Drifted:
			drifted++
			fmt.Fprintf(w, "%s\t%.3f\t%.3f\tdrift %+.0f%%\n", r.ID, r.Declared, r.ActualGB, (r.ActualGB/r.Declared-1)*100)
		}
	}
	w.Flush()
	fmt.Printf("\n%d recipes checked, %d drifted past %.0f%% and %.2f GB, %d unknown\n", len(results), drifted, *threshold*100, *minGB, unknown)
	if drifted > 0 {
		os.Exit(1)
	}
}

// checkRecipe measures the recipe's download. A multi-platform recipe's
// size_gb is one platform's download (catalog.Item.SizeGBFor counts it per
// platform), so it is compared with the largest asset; sizes are decimal GB.
// Drift must exceed both the fraction and minGB.
func checkRecipe(ctx context.Context, client *http.Client, item catalog.Item, threshold, minGB float64) result {
	r := result{ID: item.ID, Declared: item.SizeGB}
	var urls []string
	if len(item.Platforms) > 0 {
		for _, url := range item.Platforms {
			urls = append(urls, url)
		}
	} else {
		url, err := resolver.Resolve(item.URL, item.URLPattern)
		if err != nil {
			r.Err = err
			return r
		}
		urls = []string{url}
	}
	var largest int64
	for _, url := range urls {
		size, err := contentLength(ctx, client, url)
		if err != nil {
			r.Err = err
			return r
		}
		largest = max(largest, size)
	}
	r.ActualGB = float64(largest) / 1e9
	ratio := r.ActualGB / r.Declared
	r.Drifted = (ratio > 1+threshold || ratio < 1-threshold) && math.Abs(r.ActualGB-r.Declared) >= minGB
	return r
}

func contentLength(ctx context.Context, client *http.Client, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD %s", resp.Status)
	}
	if resp.ContentLength <= 0 {
		return 0, errors.New("no Content-Length")
	}
	return resp.ContentLength, nil
}
