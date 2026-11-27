package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func testSearch(t *testing.T, endpoint string, transport transportFunc) *Search {
	t.Helper()
	search, err := New(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	search.client.Transport = transport
	return search
}

func TestDuckDuckGoResults(t *testing.T) {
	body := fixture(t, "duckduckgo-results.html")
	search := testSearch(t, "", func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "html.duckduckgo.com" || req.URL.Path != "/html/" ||
			req.URL.Query().Get("q") != "snow & ice" || len(req.URL.Query()) != 1 {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		return response(http.StatusOK, body), nil
	})
	if got := search.Provider(); got != "DuckDuckGo" {
		t.Fatalf("provider = %q", got)
	}
	got, err := search.Query(t.Context(), "  snow & ice  ", 0)
	want := []Result{
		{Title: "Snow & ice", URL: "https://example.org/svalbard?id=1", Snippet: "An Arctic overview."},
		{Title: "Forecast", URL: "https://example.net/snow"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Query returned %#v, %v, want %#v", got, err, want)
	}
}

func TestSearXNGResults(t *testing.T) {
	body := fixture(t, "searxng-results.json")
	search := testSearch(t, "http://[::1]:8080/searx/search", func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Scheme != "http" || req.URL.Host != "[::1]:8080" || req.URL.Path != "/searx/search" ||
			req.URL.Query().Get("q") != "snow & ice" || req.URL.Query().Get("format") != "json" || len(req.URL.Query()) != 2 {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		return response(http.StatusOK, body), nil
	})
	if got := search.Provider(); got != "SearXNG" {
		t.Fatalf("provider = %q", got)
	}
	got, err := search.Query(t.Context(), "snow & ice", 5)
	want := []Result{
		{Title: "Glacier research", URL: "https://example.org/research", Snippet: "A & B observations from the Arctic."},
		{Title: "Ice forecast", URL: "http://example.net/forecast"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Query returned %#v, %v, want %#v", got, err, want)
	}
	data, err := json.Marshal(got[0])
	if err != nil || !strings.Contains(string(data), `"title":`) || !strings.Contains(string(data), `"url":`) || !strings.Contains(string(data), `"snippet":`) {
		t.Fatalf("result JSON = %s, %v", data, err)
	}
}

func TestProviderUnavailable(t *testing.T) {
	challenge := fixture(t, "duckduckgo-challenge.html")
	const jsonEndpoint = "https://searx.example.org/search"
	for _, test := range []struct {
		name, endpoint, body, detail string
		status                       int
	}{
		{name: "challenge status", status: http.StatusAccepted, detail: "challenge"},
		{name: "challenge markup", status: http.StatusOK, body: challenge, detail: "challenge"},
		{name: "rate limited", status: http.StatusTooManyRequests, detail: "429"},
		{name: "provider forbidden", endpoint: jsonEndpoint, status: http.StatusForbidden, detail: "403"},
		{name: "unknown HTML", status: http.StatusOK, body: "<div>query-private-content</div>"},
		{name: "JSON expected", endpoint: jsonEndpoint, status: http.StatusOK, body: "<h1>query-private-content</h1>"},
		{name: "missing results", endpoint: jsonEndpoint, status: http.StatusOK, body: `{"query":"query-private-content"}`},
		{name: "engines failed", endpoint: jsonEndpoint, status: http.StatusOK, body: `{"results":[],"unresponsive_engines":[["engine","timeout"]]}`},
		{name: "oversized body", status: http.StatusOK, body: strings.Repeat("x", 1<<20+1), detail: "large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			search := testSearch(t, test.endpoint, func(*http.Request) (*http.Response, error) {
				return response(test.status, test.body), nil
			})
			got, err := search.Query(t.Context(), "query-private-content", 0)
			if len(got) != 0 || !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), test.detail) || strings.Contains(err.Error(), "query-private-content") {
				t.Fatalf("Query returned %#v, %v", got, err)
			}
		})
	}
}

func TestProviderExplicitNoResults(t *testing.T) {
	for _, test := range []struct{ endpoint, body string }{
		{"", `<div class="result result--no-result"><div class="no-results">No results.</div></div>`},
		{"https://searx.example.org", `{"results":[],"unresponsive_engines":[]}`},
	} {
		search := testSearch(t, test.endpoint, func(req *http.Request) (*http.Response, error) {
			if test.endpoint != "" && req.URL.Path != "/search" {
				t.Fatalf("default search path = %q", req.URL.Path)
			}
			return response(http.StatusOK, test.body), nil
		})
		got, err := search.Query(t.Context(), "known empty", 1)
		if got == nil || len(got) != 0 || err != nil {
			t.Fatalf("Query returned %#v, %v", got, err)
		}
	}
}

func TestSearXNGEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{
		"http://localhost:8080/search", "http://127.0.0.1/search", "http://[::1]/search",
		"https://searx.example.org", "https://searx.example.org/custom/search",
	} {
		if _, err := New(endpoint); err != nil {
			t.Errorf("New(%q): %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{
		"http://192.168.1.1/search", "http://searx.example.org/search", "http://127.1/search",
		"https://alice:secret@searx.example.org/search", "https://searx.example.org/search#fragment",
		"https://searx.example.org/search?token=secret", "https://searx.example.org/search?",
		"https://searx.example.org:99999/search", "//searx.example.org/search", "https:///search",
		"ftp://searx.example.org/search", "https://[searx.example.org]/search",
	} {
		if _, err := New(endpoint); err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("invalid endpoint got error %v", err)
		}
	}
}

func TestQueryOmitsUnsafeCitationLinks(t *testing.T) {
	for _, address := range []string{
		"http://localhost", "http://127.1", "http://10.0.0.1",
		"http://[::ffff:192.168.0.1]", "http://[fe80::1%25eth0]",
		"http://0x7f.0.0.1", "http://example.org.localhost.",
		"http://example.org。localhost", "https://[example.org]",
		"https://login:secret@example.org/", "file:///etc/hosts",
	} {
		t.Run(address, func(t *testing.T) {
			data, err := json.Marshal(map[string]any{
				"results": []Result{{Title: "Unsafe", URL: address}},
			})
			if err != nil {
				t.Fatal(err)
			}
			search := testSearch(t, "https://searx.example.org", func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, string(data)), nil
			})
			if got, err := search.Query(t.Context(), "snow", 0); len(got) > 0 || !errors.Is(err, ErrUnavailable) {
				t.Fatalf("Query returned %#v, %v", got, err)
			}
		})
	}
}

func TestCrossOriginRedirectRefused(t *testing.T) {
	calls := 0
	search := testSearch(t, "https://searx.example.org/search", func(req *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Fatalf("redirected to %s", req.URL)
		}
		res := response(http.StatusFound, "")
		res.Header.Set("Location", "https://another.example.org/search")
		return res, nil
	})
	_, err := search.Query(t.Context(), "query-private-content", 0)
	if !errors.Is(err, ErrUnavailable) || calls != 1 || strings.Contains(err.Error(), "query-private-content") {
		t.Fatalf("calls = %d, error = %v", calls, err)
	}
}

func TestQueryBounds(t *testing.T) {
	rows := make([]map[string]string, 8)
	for i := range rows {
		rows[i] = map[string]string{
			"title": strings.Repeat("界", 250), "content": strings.Repeat("野", 250),
			"url": fmt.Sprintf("https://example.org/%d", i),
		}
	}
	rows[0]["url"] += strings.Repeat("x", 1024)
	body, err := json.Marshal(map[string]any{"results": rows})
	if err != nil {
		t.Fatal(err)
	}
	search := testSearch(t, "https://searx.example.org", func(req *http.Request) (*http.Response, error) {
		if _, ok := req.Context().Deadline(); !ok {
			t.Fatal("request has no deadline")
		}
		return response(http.StatusOK, string(body)), nil
	})
	for _, test := range []struct{ limit, count int }{{0, 3}, {1, 1}, {99, 5}} {
		got, err := search.Query(t.Context(), "snow", test.limit)
		if len(got) != test.count || err != nil {
			t.Fatalf("Query limit %d returned %d results, %v", test.limit, len(got), err)
		}
		for _, item := range got {
			if len(item.Title) > 160 || len(item.Snippet) > 320 || len(item.URL) > 1024 ||
				!utf8.ValidString(item.Title) || !utf8.ValidString(item.Snippet) {
				t.Fatalf("result exceeds output bounds: %#v", item)
			}
		}
	}
}

func TestQueryRejectsEmptyAndOverlongInput(t *testing.T) {
	search := testSearch(t, "", func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s", req.URL)
		return nil, nil
	})
	for _, query := range []string{"  ", strings.Repeat("x", 513)} {
		if _, err := search.Query(t.Context(), query, 0); err == nil {
			t.Error("invalid query accepted")
		}
	}
}

func TestQueryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	search := testSearch(t, "", func(req *http.Request) (*http.Response, error) {
		cancel()
		return nil, req.Context().Err()
	})
	if _, err := search.Query(ctx, "query-private-content", 0); !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "query-private-content") {
		t.Fatalf("Query error = %v", err)
	}
}

func TestCancelledQueryDoesNotContactProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	search := testSearch(t, "", func(req *http.Request) (*http.Response, error) {
		t.Fatal("cancelled search contacted the provider")
		return nil, req.Context().Err()
	})
	if _, err := search.Query(ctx, "query-private-content", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Query error = %v", err)
	}
}

func TestTransportErrorDoesNotLeakQuery(t *testing.T) {
	search := testSearch(t, "", func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("failed to GET %s", req.URL)
	})
	if _, err := search.Query(t.Context(), "query-private-content", 0); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "query-private-content") {
		t.Fatalf("Query error = %v", err)
	}
}
