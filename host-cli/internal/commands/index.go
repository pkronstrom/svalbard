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
		// On an exFAT stick macOS keeps a "._name.zim" file beside each
		// archive, which is not one.
		if e.IsDir() || strings.HasPrefix(e.Name(), "._") {
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
	sizes := make(map[string]int64, len(zimFiles))
	for _, filename := range zimFiles {
		info, err := os.Stat(filepath.Join(root, "zim", filename))
		if err != nil {
			return fmt.Errorf("stat %s: %w", filename, err)
		}
		sizes[filename] = info.Size()
	}
	sort.SliceStable(zimFiles, func(i, j int) bool { return sizes[zimFiles[i]] < sizes[zimFiles[j]] })
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
	readers := newReaderSet()
	defer readers.Close()
	for _, zf := range zimFiles {
		if err := ctx.Err(); err != nil {
			return err
		}
		names, err := indexZIM(ctx, db, root, zf, force, indexedByFilename, readers, w, notify)
		if err != nil {
			return err
		}
		for _, name := range names {
			present[name] = true
		}
	}
	if err := indexDocuments(ctx, root, documents, force, db, indexedByFilename, readers, w, notify); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := db.PruneSources(ctx, present); err != nil {
		return fmt.Errorf("pruning removed sources: %w", err)
	}
	if len(zimFiles) == 0 && len(documents) == 0 {
		fmt.Fprintln(w, "Search index: no ZIM files or documents to index")
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
// stricter rule and also needs a readable v2 manifest to be absent; the
// document roots did not exist before their kind was indexed, so a vault
// without a manifest simply has none. The directories that are missing and
// accepted as empty are returned for the caller to check against what the
// index already holds.
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
	documents, absent, err := ScanDocumentRoots(root)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, dir := range absent {
		if manifestErr == nil && recorded(dir) {
			return nil, nil, nil, fmt.Errorf("scanning %s directory: the manifest records content installed there: %w", dir, fs.ErrNotExist)
		}
		missing[dir] = true
	}
	return zimFiles, documents, missing, nil
}

// sourceDir names the vault directory an indexed source was read from.
func sourceDir(filename string) string {
	if dir := documentRootOf(filename); dir != "" {
		return dir
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
