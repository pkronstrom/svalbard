package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

// IndexProgress reports per-file indexing progress.
type IndexProgress struct {
	File    string // ZIM filename, or a document's path below the vault root
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

// IndexVault scans the vault's ZIM files and documents and builds a SQLite
// FTS5 search index.
// Progress is reported per-file via onProgress. Text output goes to w.
func IndexVault(root string, force bool, w io.Writer, onProgress func(IndexProgress)) error {
	return IndexVaultContext(context.Background(), root, force, w, onProgress)
}

// IndexVaultContext scans ZIM files and documents and reconciles their
// searchable articles. It preserves prior sources until each replacement has
// been fully extracted and its identity has been rechecked.
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
	zimFiles, documents, missing, err := scanSources(root)
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
		// Sources already indexed from a directory that is now gone are a
		// lost mount, not a removal: pruning them would empty the index.
		if dir := sourceDir(source.Filename); missing[dir] {
			return fmt.Errorf("the index holds sources from %s/, which is missing; not pruning them", dir)
		}
	}

	notify := func(p IndexProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	present := make(map[string]bool, len(zimFiles)+len(documents))
	for _, doc := range documents {
		present[doc] = true
	}
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
	if err := indexDocuments(ctx, root, documents, force, db, indexedByFilename, w, notify); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := db.PruneSources(ctx, present); err != nil {
		return fmt.Errorf("pruning removed sources: %w", err)
	}
	if len(zimFiles) == 0 && len(documents) == 0 {
		fmt.Fprintln(w, "No ZIM files or documents found")
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

// scanSources lists the vault's ZIM files and documents. A source directory
// that does not exist counts as empty only when the manifest records nothing
// installed there; otherwise it is an error, so an unmounted or half-copied
// vault is never read as "everything was removed". zim/ keeps its older,
// stricter rule and also needs a readable v2 manifest to be absent; books/
// did not exist before documents were indexed, so a vault without a manifest
// simply has none. The directories that are missing and accepted as empty are
// returned for the caller to check against what the index already holds.
func scanSources(root string) (zimFiles, documents []string, missing map[string]bool, err error) {
	missing = map[string]bool{}
	m, manifestErr := manifest.Load(filepath.Join(root, "manifest.yaml"))
	recorded := func(dir string) bool {
		for _, entry := range m.Realized.Entries {
			if strings.HasPrefix(filepath.ToSlash(entry.RelativePath), dir+"/") {
				return true
			}
		}
		return false
	}

	zimFiles, err = ScanZIMFiles(root)
	if errors.Is(err, fs.ErrNotExist) && manifestErr == nil && m.Version == 2 && !recorded("zim") {
		missing["zim"], err = true, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	documents, err = ScanDocuments(root)
	if errors.Is(err, fs.ErrNotExist) && !(manifestErr == nil && recorded(documentsDir)) {
		missing[documentsDir], err = true, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return zimFiles, documents, missing, nil
}

// sourceDir names the vault directory an indexed source was read from.
func sourceDir(filename string) string {
	if IsDocumentSource(filename) {
		return documentsDir
	}
	return "zim"
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
