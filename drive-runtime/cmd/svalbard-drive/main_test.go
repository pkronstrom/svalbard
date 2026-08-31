package main

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestNativeSearchSubcommand(t *testing.T) {
	root := newNativeSearchDrive(t)
	t.Setenv("DRIVE_ROOT", root)

	stdin, writeStdin, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeStdin.WriteString("q\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeStdin.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, readStdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStdin, oldStdout := os.Args, os.Stdin, os.Stdout
	os.Args = []string{"svalbard-drive", "__native-search", "water"}
	os.Stdin = stdin
	os.Stdout = readStdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		_ = stdin.Close()
		_ = readStdout.Close()
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if err := readStdout.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "Searching (keyword): water") || !strings.Contains(string(output), "Water purification") {
		t.Fatalf("native search output = %q", output)
	}
}

func newNativeSearchDrive(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "actions.json"), []byte(`{"version":1,"preset":"test","groups":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
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
		t.Fatalf("create schema: %v", err)
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
