// Package websearch returns public search links and snippets from one configured provider.
package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const (
	duckDuckGo    = "DuckDuckGo"
	searXNG       = "SearXNG"
	tavily        = "Tavily"
	defaultLimit  = 3
	maxLimit      = 5
	maxQueryBytes = 512
	maxBodyBytes  = 1 << 20
	maxURLBytes   = 1024
	searchTimeout = 8 * time.Second
)

// ErrUnavailable reports a provider or transport failure.
var ErrUnavailable = errors.New("search unavailable")

type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type Search struct {
	client   *http.Client
	endpoint url.URL
	provider string
	apiKey   string
}

func newSearch(provider string, endpoint url.URL) *Search {
	return &Search{
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		endpoint: endpoint,
		provider: provider,
	}
}

// New selects DuckDuckGo HTML or an operator's SearXNG search endpoint.
func New(endpoint string) (*Search, error) {
	if endpoint == "" {
		return newSearch(duckDuckGo, url.URL{Scheme: "https", Host: "html.duckduckgo.com", Path: "/html/"}), nil
	}

	address := strings.TrimSpace(endpoint)
	target, err := parseWebURL(address)
	if err != nil || target.RawQuery != "" || target.ForceQuery || strings.Contains(address, "#") ||
		(target.Scheme == "http" && !isLoopback(target.Hostname())) {
		return nil, errors.New("SearXNG endpoint must be an HTTPS URL or an HTTP loopback URL without credentials, query, or fragment")
	}
	if target.Path == "" || target.Path == "/" {
		target.Path = "/search"
		target.RawPath = ""
	}
	return newSearch(searXNG, *target), nil
}

// NewTavily uses the fixed Tavily endpoint so the key is never sent to a
// user-configured URL.
func NewTavily(apiKey string) *Search {
	search := newSearch(tavily, url.URL{Scheme: "https", Host: "api.tavily.com", Path: "/search"})
	search.apiKey = apiKey
	return search
}

func (s *Search) Provider() string { return s.provider }

// Query performs one bounded request to the configured provider.
func (s *Search) Query(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > maxQueryBytes || !utf8.ValidString(query) {
		return nil, fmt.Errorf("web search query must contain 1 to %d bytes of UTF-8 text", maxQueryBytes)
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	if ctx.Err() != nil {
		return nil, s.requestFailed(ctx, "request cancelled")
	}
	req, err := s.newRequest(ctx, query, limit)
	if err != nil {
		return nil, s.unavailable("invalid request")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, s.requestFailed(ctx, "request failed")
	}
	defer resp.Body.Close()
	if s.provider == duckDuckGo && resp.StatusCode == http.StatusAccepted {
		return nil, s.unavailable("provider returned a bot challenge")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, s.unavailable(fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, s.requestFailed(ctx, "could not read response")
	}
	if len(body) > maxBodyBytes {
		return nil, s.unavailable("response too large")
	}

	var results []Result
	if s.provider == duckDuckGo {
		results, err = parseDuckDuckGo(body, limit)
	} else {
		results, err = parseJSONResults(body, limit)
	}
	if err != nil {
		return nil, s.unavailable(err.Error())
	}
	return results, nil
}

func (s *Search) newRequest(ctx context.Context, query string, limit int) (*http.Request, error) {
	target := s.endpoint
	if s.provider == tavily {
		payload, err := json.Marshal(struct {
			Query       string `json:"query"`
			SearchDepth string `json:"search_depth"`
			MaxResults  int    `json:"max_results"`
		}{Query: query, SearchDepth: "basic", MaxResults: limit})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req, nil
	}
	params := url.Values{"q": {query}}
	accept := "text/html"
	if s.provider == searXNG {
		params.Set("format", "json")
		accept = "application/json"
	}
	target.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	return req, nil
}

func (s *Search) requestFailed(ctx context.Context, detail string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s %w: %w", s.provider, ErrUnavailable, err)
	}
	return s.unavailable(detail)
}

func (s *Search) unavailable(detail string) error {
	return fmt.Errorf("%s %w (%s)", s.provider, ErrUnavailable, detail)
}

func parseWebURL(address string) (*url.URL, error) {
	invalid := errors.New("invalid web URL")
	if strings.ContainsFunc(address, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return nil, invalid
	}
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || strings.HasSuffix(parsed.Host, ":") {
		return nil, invalid
	}
	if port := parsed.Port(); port != "" {
		if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
			return nil, invalid
		}
	}
	return parsed, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Zone() == "" && addr.Unmap().IsLoopback()
}

var reservedResultNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("3fff::/20"),
}

// These are citation links. Check for obvious local addresses without resolving
// hostnames or connecting to any search result.
func publicResultHost(host string) bool {
	if addr, err := netip.ParseAddr(host); err == nil {
		ip := addr.Unmap()
		return addr.Zone() == "" && ip.IsGlobalUnicast() && !ip.IsPrivate() &&
			!slices.ContainsFunc(reservedResultNetworks, func(network netip.Prefix) bool { return network.Contains(ip) })
	}
	host, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	dot := strings.LastIndexByte(host, '.')
	if dot < 0 {
		return false
	}
	tld := host[dot+1:]
	switch tld {
	case "localhost", "local", "internal", "test", "invalid", "example":
		return false
	}
	// Browsers also accept shortened, octal and hexadecimal IPv4 addresses.
	return host != "home.arpa" && !strings.HasSuffix(host, ".home.arpa") && strings.Trim(tld, "0123456789") != "" && !strings.HasPrefix(tld, "0x")
}

func addResult(results []Result, title, address, snippet string) []Result {
	if len(address) > maxURLBytes {
		return results
	}
	target, err := parseWebURL(address)
	if err != nil || !publicResultHost(target.Hostname()) {
		return results
	}
	address = target.String()
	title = cleanText(title, 160)
	if len(address) > maxURLBytes || title == "" || slices.ContainsFunc(results, func(result Result) bool { return result.URL == address }) {
		return results
	}
	return append(results, Result{Title: title, URL: address, Snippet: cleanText(snippet, 320)})
}

func cleanText(value string, limit int) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	value = strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	if len(value) <= limit {
		return value
	}
	for !utf8.RuneStart(value[limit]) {
		limit--
	}
	return strings.TrimSpace(value[:limit])
}
