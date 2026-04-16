package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

// ScanZIMFiles returns a sorted list of .zim filenames (not full paths)
// found in root/zim/.
func ScanZIMFiles(root string) ([]string, error) {
	zimDir := filepath.Join(root, "zim")
	entries, err := os.ReadDir(zimDir)
	if err != nil {
		return nil, fmt.Errorf("scanning zim directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".zim") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// IndexVault scans ZIM files in the vault and builds a SQLite FTS5 search index.
//
// For each ZIM file not yet indexed, it extracts searchable articles from the
// archive and stores them in the search database. Progress is written to w.
func IndexVault(root string, w io.Writer) error {
	zimFiles, err := ScanZIMFiles(root)
	if err != nil {
		return err
	}

	if len(zimFiles) == 0 {
		fmt.Fprintln(w, "No ZIM files found")
		return nil
	}

	// Ensure data directory exists.
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "search.db")
	db, err := searchdb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening search database: %w", err)
	}
	defer db.Close()

	// Determine which files are already indexed.
	indexed, err := db.IndexedFilenames()
	if err != nil {
		return fmt.Errorf("reading indexed filenames: %w", err)
	}
	indexedSet := make(map[string]bool, len(indexed))
	for _, fn := range indexed {
		indexedSet[fn] = true
	}

	for _, zf := range zimFiles {
		if indexedSet[zf] {
			fmt.Fprintf(w, "  skip %s (already indexed)\n", zf)
			continue
		}

		fmt.Fprintf(w, "  indexing %s ...\n", zf)

		articles, title, err := zimext.ExtractArticles(filepath.Join(root, "zim", zf))
		if err != nil {
			return fmt.Errorf("extracting articles from %s: %w", zf, err)
		}
		if title == "" {
			title = strings.TrimSuffix(zf, filepath.Ext(zf))
		}

		sourceID, err := db.UpsertSource(zf, title)
		if err != nil {
			return fmt.Errorf("upserting source %s: %w", zf, err)
		}

		if err := db.InsertArticles(sourceID, articles); err != nil {
			return fmt.Errorf("inserting articles for %s: %w", zf, err)
		}

		fmt.Fprintf(w, "  indexed %s (%d article)\n", zf, len(articles))
	}

	// Store metadata timestamp.
	if err := db.SetMeta("indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting indexed_at metadata: %w", err)
	}

	sc, ac, err := db.Stats()
	if err != nil {
		return fmt.Errorf("reading stats: %w", err)
	}
	fmt.Fprintf(w, "Index complete: %d source(s), %d article(s)\n", sc, ac)
	return nil
}
