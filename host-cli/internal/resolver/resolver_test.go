package resolver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveStaticURL(t *testing.T) {
	url, err := Resolve("https://example.com/file.zim", "")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://example.com/file.zim" {
		t.Errorf("got %q", url)
	}
}

func TestResolvePatternURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>
<a href="data_2026-01.zim">data_2026-01.zim</a>
<a href="data_2026-03.zim">data_2026-03.zim</a>
<a href="data_2025-12.zim">data_2025-12.zim</a>
<a href="other_file.txt">other_file.txt</a>
</html>`)
	}))
	defer ts.Close()

	url, err := Resolve("", ts.URL+"/data_{date}.zim")
	if err != nil {
		t.Fatal(err)
	}
	want := ts.URL + "/data_2026-03.zim"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestResolvePatternNoMatches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><a href="unrelated.txt">unrelated</a></html>`)
	}))
	defer ts.Close()

	_, err := Resolve("", ts.URL+"/data_{date}.zim")
	if err == nil {
		t.Fatal("expected error for no matches")
	}
}

func TestResolveBothEmpty(t *testing.T) {
	_, err := Resolve("", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveStaticTakesPrecedence(t *testing.T) {
	// Even with a pattern, static URL wins
	url, err := Resolve("https://example.com/direct.zim", "https://example.com/data_{date}.zim")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://example.com/direct.zim" {
		t.Errorf("static should take precedence, got %q", url)
	}
}
