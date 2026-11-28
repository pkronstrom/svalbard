package frontpage

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestDashboardServesInstalledAppsAndReapsThem(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/demo/index.html", "<h1>Bundled demo</h1>")
	writeFile(t, root, "workspace/Expedition One/index.html", "<h1>Agent's app</h1>")
	writeFile(t, root, "workspace/.draft/private.html", "<h1>Hidden draft</h1>")
	writeFile(t, root, "workspace/readme.md", "notes")
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Apps", Items: []config.MenuItem{
		{Label: "Bundled Demo", Action: config.BuiltinAction("apps", map[string]string{"app": "demo"})},
		{Label: "Workspace", Action: config.BuiltinAction("workspace", nil)},
		{Label: "CLI tool", Action: config.ExecAction(config.ExecActionConfig{Executable: "ignored"})},
	}}}}
	client, origin, stop := startDashboard(t, root, cfg)
	if page := requestBody(t, client, origin+"/", http.StatusOK); !strings.Contains(page, "Drive catalog") {
		t.Fatalf("dashboard returned %q", page)
	}
	var inv inventory
	readResponse(t, requestBody(t, client, origin+"/api/inventory", http.StatusOK), &inv)
	if len(inv.Groups) != 1 || len(inv.Groups[0].Items) != 3 {
		t.Fatalf("unexpected inventory: %+v", inv)
	}
	items := inv.Groups[0].Items
	if items[1].Service != inv.Workspace || items[2].Kind != "terminal" || items[2].Service != "" {
		t.Fatalf("unexpected action mapping: %+v", items)
	}

	var pages []savedPage
	readResponse(t, requestBody(t, client, origin+"/api/workspace", http.StatusOK), &pages)
	if len(pages) != 1 || pages[0].Path != "Expedition One/index.html" || pages[0].Name != "Expedition One" {
		t.Fatalf("unexpected saved apps: %+v", pages)
	}
	launch := "/api/services/" + items[0].Service
	postToDashboard(t, client, origin, launch, "https://unrelated.test", "{}", http.StatusForbidden)
	postToDashboard(t, client, origin, launch, origin, `{"app":"another"}`, http.StatusBadRequest)
	postToDashboard(t, client, origin, launch, origin, "{}", http.StatusOK)
	bundled := waitForState(t, client, origin, items[0].Service, "running").Links[0].URL
	if body := requestBody(t, client, bundled, http.StatusOK); !strings.Contains(body, "Bundled demo") {
		t.Fatalf("app response = %q", body)
	}
	var duplicate state
	readResponse(t, postToDashboard(t, client, origin, launch, origin, "{}", http.StatusOK), &duplicate)
	if len(duplicate.Links) != 1 || duplicate.Links[0].URL != bundled {
		t.Fatalf("second launch = %+v, want the existing app", duplicate)
	}
	postToDashboard(t, client, origin, launch+"/stop", origin, "{}", http.StatusOK)
	waitForState(t, client, origin, items[0].Service, "stopped")
	requireClosed(t, client, bundled)

	postToDashboard(t, client, origin, "/api/services/"+inv.Workspace, origin, "{}", http.StatusOK)
	workspace := waitForState(t, client, origin, inv.Workspace, "running").Links[0].URL
	if strings.HasPrefix(workspace, origin+"/") {
		t.Fatal("workspace shares the dashboard origin")
	}
	if body := requestBody(t, client, workspace+"Expedition%20One/index.html", http.StatusOK); !strings.Contains(body, "Agent's app") {
		t.Fatalf("workspace response = %q", body)
	}
	assertReaderFramePolicy(t, client, workspace+"Expedition%20One/index.html", origin)
	requestBody(t, client, workspace+".draft/private.html", http.StatusNotFound)

	stop()
	requireClosed(t, client, workspace)
	requireClosed(t, client, origin)
}

func TestSearchOpensDocumentAtMatchingPage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "books/field manual.pdf", "%PDF-1.7\n")
	if err := os.Mkdir(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL);
		CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER, path TEXT, title TEXT, body TEXT);
		CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id');
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO sources VALUES (1, 'books/field manual.pdf');
		INSERT INTO articles VALUES (1, 1, 'page/3', 'Water filters', 'Safe drinking water.');
		INSERT INTO articles_fts(rowid, title, body) VALUES (1, 'Water filters', 'Safe drinking water.');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Search", Items: []config.MenuItem{
		{Label: "Search all content", Action: config.BuiltinAction("search", nil)},
	}}}}
	client, origin, stop := startDashboard(t, root, cfg)

	var results struct {
		Hits []struct{ Title, Source, Path string }
	}
	readResponse(t, postToDashboard(t, client, origin, "/api/search", origin, `{"query":"water"}`, http.StatusOK), &results)
	if len(results.Hits) != 1 || results.Hits[0].Title != "Water filters" {
		t.Fatalf("search results = %+v", results)
	}
	var destination struct {
		link
		Service string
	}
	hit := results.Hits[0]
	request, err := json.Marshal(map[string]string{"source": hit.Source, "path": hit.Path})
	if err != nil {
		t.Fatal(err)
	}
	readResponse(t, postToDashboard(t, client, origin, "/api/read", origin, string(request), http.StatusOK), &destination)
	if !strings.HasSuffix(destination.URL, "/field%20manual.pdf#page=3") {
		t.Fatalf("result URL = %s", destination.URL)
	}
	if body := requestBody(t, client, destination.URL, http.StatusOK); !strings.HasPrefix(body, "%PDF-1.7") {
		t.Fatalf("document body = %q", body)
	}
	assertReaderFramePolicy(t, client, destination.URL, origin)
	if destination.Service == "" {
		t.Fatal("search reader has no dashboard stop control")
	}
	postToDashboard(t, client, origin, "/api/services/"+destination.Service+"/stop", origin, "{}", http.StatusOK)
	waitForState(t, client, origin, destination.Service, "stopped")
	stop()
	requireClosed(t, client, destination.URL)
}

func startDashboard(t *testing.T, root string, cfg config.RuntimeConfig) (*http.Client, string, func()) {
	t.Helper()
	client, address, stop := startDashboardLink(t, root, cfg)
	origin, _, _ := strings.Cut(address, "/#")
	return client, origin, stop
}

func startDashboardLink(t *testing.T, root string, cfg config.RuntimeConfig) (*http.Client, string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	address, done := make(chan string, 1), make(chan error, 1)
	go func() {
		done <- Run(ctx, io.Discard, root, cfg, func(url string) error { address <- url; return nil })
	}()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("dashboard returned %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("dashboard did not stop")
		}
	}
	t.Cleanup(stop)

	client := &http.Client{Timeout: 3 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	select {
	case url := <-address:
		return client, url, stop
	case err := <-done:
		stopped = true
		t.Fatalf("dashboard failed to start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("dashboard did not start")
	}
	return nil, "", nil
}

func requestBody(t *testing.T, client *http.Client, url string, want int) string {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return responseBody(t, response, want)
}

func postToDashboard(t *testing.T, client *http.Client, origin, path, headerOrigin, body string, want int) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", headerOrigin)
	request.Header.Set("X-Svalbard-Request", "1")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return responseBody(t, response, want)
}

func responseBody(t *testing.T, response *http.Response, want int) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("%s gave %d, want %d. Body: %s", response.Request.URL.Path, response.StatusCode, want, body)
	}
	return string(body)
}

func waitForState(t *testing.T, client *http.Client, origin, key, want string) state {
	t.Helper()
	var snapshot state
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(15 * time.Millisecond) {
		var all map[string]state
		readResponse(t, requestBody(t, client, origin+"/api/services", http.StatusOK), &all)
		snapshot = all[key]
		if snapshot.Status == want {
			return snapshot
		}
		if snapshot.Status == "failed" {
			break
		}
	}
	t.Fatalf("service state = %+v, want %q", snapshot, want)
	return state{}
}

func requireClosed(t *testing.T, client *http.Client, url string) {
	t.Helper()
	if response, err := client.Get(url); err == nil {
		response.Body.Close()
		t.Errorf("%s is still serving (HTTP %d)", url, response.StatusCode)
	}
}

func readResponse(t *testing.T, response string, value any) {
	t.Helper()
	if err := json.Unmarshal([]byte(response), value); err != nil {
		t.Fatalf("invalid response %q: %v", response, err)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	file := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertReaderFramePolicy(t *testing.T, client *http.Client, address, origin string) {
	t.Helper()
	response, err := client.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	policy := response.Header.Get("Content-Security-Policy")
	if !strings.HasSuffix(policy, "frame-ancestors "+origin) {
		t.Fatalf("reader policy = %q; want framing limited to %s", policy, origin)
	}
}
