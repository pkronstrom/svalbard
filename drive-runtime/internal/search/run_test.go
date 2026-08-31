package search

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestMain(m *testing.M) {
	if os.Getenv("SVALBARD_TEST_KIWIX") == "1" {
		if pidFile := os.Getenv("SVALBARD_TEST_KIWIX_PID_FILE"); pidFile != "" {
			if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
				os.Exit(1)
			}
		}
		for i, arg := range os.Args {
			if arg == "--port" && i+1 < len(os.Args) {
				if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})); err != nil {
					os.Exit(1)
				}
			}
		}
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestRunUsesInitialQueryAndPreservesTerminalCommands(t *testing.T) {
	root := newSearchDrive(t)

	tests := []struct {
		name         string
		initialQuery string
		input        string
		want         []string
	}{
		{
			name:         "initial query",
			initialQuery: "water",
			input:        "q\n",
			want:         []string{"Searching (keyword): water", "1. [wiki] Water purification"},
		},
		{
			name:  "keyword mode command",
			input: "/fts\nwater\nq\n",
			want:  []string{"Switched to keyword search", "Searching (keyword): water"},
		},
		{
			name:  "semantic mode command",
			input: "/sem\nwater\nq\n",
			want:  []string{"Switched to keyword search", "Searching (keyword): water"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := Run(context.Background(), strings.NewReader(tc.input), &stdout, root, tc.initialQuery, nil); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("Run() output = %q, want %q", stdout.String(), want)
				}
			}
		})
	}
}

func TestRunPromptsAgainAfterEmptyResultChoice(t *testing.T) {
	root := newSearchDrive(t)
	var stdout bytes.Buffer

	if err := Run(context.Background(), strings.NewReader("water\n\nq\n"), &stdout, root, "", nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if prompts := strings.Count(stdout.String(), "[keyword] Search"); prompts != 2 {
		t.Fatalf("Run() prompts = %d, want 2; output = %q", prompts, stdout.String())
	}
}

func TestRunOpensNumberedResultThroughSession(t *testing.T) {
	root := newSearchDrive(t)
	writeTestKiwix(t, root)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_TEST_KIWIX", "1")

	var opened string
	var stdout bytes.Buffer
	err := Run(context.Background(), strings.NewReader("water\n1\nq\n"), &stdout, root, "", func(url string) error {
		opened = url
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(opened, "/content/wiki/Water_purification") {
		t.Errorf("opened URL = %q", opened)
	}
	if !strings.Contains(stdout.String(), "Opening result") {
		t.Errorf("Run() output = %q, want opening confirmation", stdout.String())
	}
}

func TestRunClosesSessionAfterInputError(t *testing.T) {
	root := newSearchDrive(t)
	writeTestKiwix(t, root)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(root, "kiwix.pid")
	t.Setenv("SVALBARD_TEST_KIWIX", "1")
	t.Setenv("SVALBARD_TEST_KIWIX_PID_FILE", pidFile)

	err := Run(context.Background(), strings.NewReader("water\n1\n"), io.Discard, root, "", func(string) error { return nil })
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Run() error = %v, want EOF", err)
	}
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read Kiwix PID: %v", err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatalf("parse Kiwix PID: %v", err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("Kiwix process %d remains after Run() error: %v", pid, err)
	}
}

func TestSessionFallsBackToKeywordWhenHybridUnavailable(t *testing.T) {
	root := newSearchDrive(t)
	session, err := NewSession(root, nil)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	response, err := session.Search(context.Background(), ModeHybrid, "water", 20)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if response.EffectiveMode != ModeKeyword {
		t.Errorf("EffectiveMode = %q, want %q", response.EffectiveMode, ModeKeyword)
	}
	if response.Status != "Hybrid unavailable, fell back to keyword" {
		t.Errorf("Status = %q", response.Status)
	}
	if len(response.Results) != 1 || response.Results[0].Title != "Water purification" {
		t.Errorf("Results = %+v, want water result", response.Results)
	}
}

func TestSessionOpensResultAndClosesKiwix(t *testing.T) {
	root := newSearchDrive(t)
	writeTestKiwix(t, root)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_TEST_KIWIX", "1")

	var opened string
	session, err := NewSession(root, func(url string) error {
		opened = url
		return nil
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	response, err := session.Search(context.Background(), ModeKeyword, "water", 20)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if err := session.OpenResult(response.Results[0]); err != nil {
		t.Fatalf("OpenResult() error = %v", err)
	}
	if !strings.Contains(opened, "/content/wiki/Water_purification") {
		t.Errorf("opened URL = %q", opened)
	}
	server := session.kiwixServer
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if server == nil {
		t.Fatal("OpenResult() did not start Kiwix")
	}
	if server.ProcessState == nil {
		t.Fatal("Close() did not wait for the Kiwix process")
	}
}

func newSearchDrive(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dataDir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const schema = `
		CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL);
		CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER NOT NULL, path TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '');
		CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id');
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create search schema: %v", err)
	}
	source, err := db.Exec("INSERT INTO sources (filename) VALUES ('wiki.zim')")
	if err != nil {
		t.Fatal(err)
	}
	sourceID, err := source.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	article, err := db.Exec("INSERT INTO articles (source_id, path, title, body) VALUES (?, '/Water_purification', 'Water purification', 'Water is essential.')", sourceID)
	if err != nil {
		t.Fatal(err)
	}
	articleID, err := article.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO articles_fts (rowid, title, body) VALUES (?, 'Water purification', 'Water is essential.')", articleID); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTestKiwix(t *testing.T, root string) {
	t.Helper()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(os.Args[0], filepath.Join(binDir, "kiwix-serve")); err != nil {
		t.Fatalf("symlink test Kiwix: %v", err)
	}
}
