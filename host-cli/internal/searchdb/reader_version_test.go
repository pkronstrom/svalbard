package searchdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func indexedSource(t *testing.T, db *DB, filename string) Source {
	t.Helper()
	sources, err := db.IndexedSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.Filename == filename {
			return source
		}
	}
	t.Fatalf("source %s not indexed", filename)
	return Source{}
}

func TestReaderVersionRoundTripsAndDefaultsToZero(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	if err := db.ReplaceSource(ctx, Source{Filename: "a.zim", Title: "A", UUID: "u", Size: 1, ModTimeNS: 2, ReaderVersion: 2}, nil); err != nil {
		t.Fatal(err)
	}
	if got := indexedSource(t, db, "a.zim").ReaderVersion; got != 2 {
		t.Fatalf("ReaderVersion = %d, want 2", got)
	}
	if err := db.ReplaceSource(ctx, Source{Filename: "b.zim", Title: "B", UUID: "u", Size: 1, ModTimeNS: 2}, nil); err != nil {
		t.Fatal(err)
	}
	if got := indexedSource(t, db, "b.zim").ReaderVersion; got != 0 {
		t.Fatalf("ReaderVersion = %d, want 0", got)
	}
	if err := db.SetSourceReaderVersion(ctx, "a.zim", 3); err != nil {
		t.Fatal(err)
	}
	if got := indexedSource(t, db, "a.zim").ReaderVersion; got != 3 {
		t.Fatalf("ReaderVersion after set = %d, want 3", got)
	}
	if err := db.SetSourceReaderVersion(ctx, "missing.zim", 3); err == nil {
		t.Fatal("SetSourceReaderVersion of an unknown source succeeded")
	}
}

func TestOpenAddsReaderVersionToAnOldDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE sources (id INTEGER PRIMARY KEY AUTOINCREMENT, filename TEXT UNIQUE NOT NULL, title TEXT NOT NULL, indexed_at TEXT, source_uuid TEXT, source_size INTEGER, source_mtime_ns INTEGER, note TEXT);
		INSERT INTO sources (filename, title, source_uuid, source_size, source_mtime_ns) VALUES ('old.zim', 'Old', 'u', 1, 2);`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := indexedSource(t, db, "old.zim")
	if source.ReaderVersion != 0 || source.UUID != "u" {
		t.Fatalf("old row = %+v, want ReaderVersion 0 and its identity", source)
	}
}

func TestArticleHeadsListsPathsAndTitles(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedSource(t, db, "a.zim", "A", []Article{
		{Path: "/home.html", Title: "{{ title }}", Body: "x"},
		{Path: "/home", Title: "Home", Body: "y"},
	})
	seedSource(t, db, "b.zim", "B", []Article{{Path: "/other", Title: "Other", Body: "z"}})

	heads, err := db.ArticleHeads("a.zim")
	if err != nil {
		t.Fatal(err)
	}
	want := []ArticleHead{{Path: "/home", Title: "Home"}, {Path: "/home.html", Title: "{{ title }}"}}
	if len(heads) != len(want) || heads[0] != want[0] || heads[1] != want[1] {
		t.Fatalf("ArticleHeads = %+v, want %+v", heads, want)
	}
}
