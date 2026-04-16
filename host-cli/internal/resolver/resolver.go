// Package resolver resolves download URLs from static values or date-pattern templates.
package resolver

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Resolve returns a concrete download URL. If staticURL is non-empty it is
// returned directly. Otherwise urlPattern is treated as a template containing
// a {date} placeholder: the base directory is fetched via HTTP and the
// filename with the lexicographically latest date match is selected.
func Resolve(staticURL, urlPattern string) (string, error) {
	if staticURL != "" {
		return staticURL, nil
	}
	if urlPattern == "" {
		return "", fmt.Errorf("no URL or pattern")
	}

	// Split at last "/" into base URL and filename pattern.
	lastSlash := strings.LastIndex(urlPattern, "/")
	if lastSlash < 0 {
		return "", fmt.Errorf("pattern %q has no path separator", urlPattern)
	}
	baseURL := urlPattern[:lastSlash]
	filenamePattern := urlPattern[lastSlash+1:]

	// Build regex from the filename pattern, replacing {date} with a capture group.
	datePattern := strings.ReplaceAll(filenamePattern, "{date}", `(\d{4}-\d{2})`)
	fileRe, err := regexp.Compile("^" + datePattern + "$")
	if err != nil {
		return "", fmt.Errorf("compiling pattern: %w", err)
	}

	// Fetch the directory listing.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	// Extract all href values.
	hrefRe := regexp.MustCompile(`href="([^"]+)"`)
	matches := hrefRe.FindAllStringSubmatch(string(body), -1)

	// Match hrefs against the filename pattern and collect (date, filename) pairs.
	type entry struct {
		date     string
		filename string
	}
	var entries []entry
	for _, m := range matches {
		href := m[1]
		sub := fileRe.FindStringSubmatch(href)
		if sub == nil {
			continue
		}
		entries = append(entries, entry{date: sub[1], filename: href})
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no files matching pattern %q in %s", filenamePattern, baseURL)
	}

	// Sort by date string lexicographically and pick the latest.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date < entries[j].date
	})

	latest := entries[len(entries)-1]
	return baseURL + "/" + latest.filename, nil
}
