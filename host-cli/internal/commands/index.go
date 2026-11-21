package commands

import (
	"context"
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
	return IndexVaultContext(context.Background(), root, force, w, onProgress)
}

// IndexVaultContext scans ZIM files and reconciles their searchable articles.
// It preserves prior sources until each replacement has been fully extracted
// and its archive identity has been rechecked.
func IndexVaultContext(ctx context.Context, root string, force bool, w io.Writer, onProgress func(IndexProgress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	releaseLock, err := acquireIndexWriterLock(root)
	if err != nil {
		return err
	}
	defer releaseLock()
	return indexVaultUnlocked(ctx, root, force, w, onProgress)
}

// indexVaultUnlocked runs while the caller owns the vault index writer lock.
func indexVaultUnlocked(ctx context.Context, root string, force bool, w io.Writer, onProgress func(IndexProgress)) error {
	zimFiles, err := ScanZIMFiles(root)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
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

	indexed, err := db.IndexedSources()
	if err != nil {
		return fmt.Errorf("reading indexed sources: %w", err)
	}
	indexedByFilename := make(map[string]searchdb.Source, len(indexed))
	for _, source := range indexed {
		indexedByFilename[source.Filename] = source
	}

	notify := func(p IndexProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	present := make(map[string]bool, len(zimFiles))
	for _, zf := range zimFiles {
		if err := ctx.Err(); err != nil {
			return err
		}
		present[zf] = true
		path := filepath.Join(root, "zim", zf)
		identity, err := archiveSourceIdentity(ctx, path, zf)
		if err != nil {
			notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
			return fmt.Errorf("reading identity for %s: %w", zf, err)
		}
		if !force && indexedByFilename[zf].IdentityCurrent(identity) {
			notify(IndexProgress{File: zf, Status: "skip", Detail: "already indexed"})
			continue
		}

		notify(IndexProgress{File: zf, Status: "extracting", Detail: "extracting articles..."})

		articles, title, uuid, err := zimext.ExtractArticlesContext(ctx, path)
		if err != nil {
			notify(IndexProgress{File: zf, Status: "failed", Detail: err.Error()})
			return fmt.Errorf("extracting articles from %s: %w", zf, err)
		}
		if title == "" {
			title = strings.TrimSuffix(zf, filepath.Ext(zf))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := archiveSourceIdentity(ctx, path, zf)
		if err != nil {
			return fmt.Errorf("rechecking identity for %s: %w", zf, err)
		}
		if current.UUID != uuid || !identity.IdentityCurrent(current) {
			return fmt.Errorf("source %s changed during extraction; retry indexing", zf)
		}

		notify(IndexProgress{
			File: zf, Status: "extracting",
			Detail:  fmt.Sprintf("inserting %d articles...", len(articles)),
			Current: int64(len(articles)), Total: int64(len(articles)),
		})
		if err := ctx.Err(); err != nil {
			return err
		}

		identity.Title = title
		if err := db.ReplaceSource(ctx, identity, articles); err != nil {
			return fmt.Errorf("replacing articles for %s: %w", zf, err)
		}

		notify(IndexProgress{
			File: zf, Status: "done",
			Detail:  fmt.Sprintf("%d articles", len(articles)),
			Current: int64(len(articles)), Total: int64(len(articles)),
		})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := db.PruneSources(ctx, present); err != nil {
		return fmt.Errorf("pruning removed sources: %w", err)
	}
	if len(zimFiles) == 0 {
		fmt.Fprintln(w, "No ZIM files found")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Store metadata timestamp.
	if err := db.SetMetaContext(ctx, "indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting indexed_at metadata: %w", err)
	}

	return nil
}

func archiveSourceIdentity(ctx context.Context, path, filename string) (searchdb.Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return searchdb.Source{}, fmt.Errorf("stat source: %w", err)
	}
	uuid, err := zimext.ArchiveUUID(ctx, path)
	if err != nil {
		return searchdb.Source{}, err
	}
	return searchdb.Source{Filename: filename, UUID: uuid, Size: info.Size(), ModTimeNS: info.ModTime().UnixNano()}, nil
}
