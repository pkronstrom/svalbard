package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/websearch"
)

func TestWebSearchRequiresAnExplicitCapability(t *testing.T) {
	root := t.TempDir()
	web, err := websearch.New("")
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []DriveOptions{{}, {Stdio: true}, {Stdio: true, WebSearch: web}} {
		server := NewDriveServer(root, opts)
		t.Cleanup(func() { _ = server.Close() })
		found := false
		for _, tool := range server.Tools() {
			if tool.Name != "web_search" {
				continue
			}
			found = true
			if hint := tool.Annotations.OpenWorldHint; hint == nil || !*hint {
				t.Error("web_search must declare its network access")
			}
		}
		if found != (opts.WebSearch != nil) {
			t.Errorf("web_search advertised=%v, want %v", found, opts.WebSearch != nil)
		}
	}
}

func TestWebSearchRejectsInvalidSavedProvider(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".svalbard")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"123", "null"} {
		values := `{"ai":{"web_search":true,"web_search_url":` + provider + "}}"
		if err := os.WriteFile(filepath.Join(state, "settings.json"), []byte(values), 0o600); err != nil {
			t.Fatal(err)
		}
		prefs, err := settings.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		search, err := WebSearchForSettings(prefs)
		if err == nil || search != nil {
			t.Errorf("enabled=%v, err=%v with invalid provider %s", search != nil, err, provider)
		}
	}
}

func TestWebSearchSelectsTavilyWhenEnabledAndNoSearXNGURL(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".svalbard"), 0o700); err != nil {
		t.Fatal(err)
	}
	prefs, err := settings.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if search, err := WebSearchForSettings(prefs); err != nil || search != nil {
		t.Fatalf("web search activated without opt-in: %v, %v", search != nil, err)
	}
	if err := prefs.Set(settings.KeyWebSearch, "true"); err != nil {
		t.Fatal(err)
	}
	assertProvider := func(want string) {
		t.Helper()
		search, err := WebSearchForSettings(prefs)
		if err != nil || search == nil {
			t.Fatalf("expected %s, got no web search: %v", want, err)
		}
		if got := search.Provider(); got != want {
			t.Errorf("provider = %q, want %q", got, want)
		}
	}
	assertProvider("Tavily")

	if err := prefs.Set(settings.KeyWebSearchURL, "https://search.example.org"); err != nil {
		t.Fatal(err)
	}
	assertProvider("SearXNG")
	if err := prefs.Set(settings.KeyWebSearchURL, ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAVILY_API_KEY", "")
	assertProvider("DuckDuckGo")
}
