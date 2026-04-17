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

// IndexProgress reports per-file indexing progress.
type IndexProgress struct {
	File    string // ZIM filename
	Status  string // "extracting", "skip", "done", "failed"
	Detail  string // e.g. "3864 articles"
	Current int64  // articles indexed so far for this file
	Total   int64  // total articles in this file (0 if unknown)
}

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
// Progress is reported per-file via onProgress. Text output goes to w.
func IndexVault(root string, force bool, w io.Writer, onProgress func(IndexProgress)) error {
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

	notify := func(p IndexProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	for _, zf := range zimFiles {
		if !force && indexedSet[zf] {
			notify(IndexProgress{File: zf, Status: "skip", Detail: "already indexed"})
			continue
		}
		if force && indexedSet[zf] {
			// Re-index: remove old articles first.
			if sourceID, err := db.UpsertSource(zf, zf); err == nil {
				_ = db.DeleteSourceArticles(sourceID)
			}
		}

		notify(IndexProgress{File: zf, Status: "extracting", Detail: "extracting articles..."})

		articles, title, err := zimext.ExtractArticles(filepath.Join(root, "zim", zf))
		if err != nil {
			notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
			return fmt.Errorf("extracting articles from %s: %w", zf, err)
		}
		if title == "" {
			title = strings.TrimSuffix(zf, filepath.Ext(zf))
		}

		notify(IndexProgress{
			File: zf, Status: "extracting",
			Detail:  fmt.Sprintf("inserting %d articles...", len(articles)),
			Current: int64(len(articles)), Total: int64(len(articles)),
		})

		sourceID, err := db.UpsertSource(zf, title)
		if err != nil {
			return fmt.Errorf("upserting source %s: %w", zf, err)
		}

		if err := db.InsertArticles(sourceID, articles); err != nil {
			return fmt.Errorf("inserting articles for %s: %w", zf, err)
		}

		notify(IndexProgress{
			File: zf, Status: "done",
			Detail:  fmt.Sprintf("%d articles", len(articles)),
			Current: int64(len(articles)), Total: int64(len(articles)),
		})
	}

	// Store metadata timestamp.
	if err := db.SetMeta("indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting indexed_at metadata: %w", err)
	}

	return nil
}
