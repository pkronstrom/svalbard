package commands

import (
	"context"
	"errors"
	"fmt"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	runConfiguredKeyword  = indexVaultUnlocked
	runConfiguredSemantic = indexSemanticUnlocked
)

// RunConfiguredIndex runs the manifest-selected indexing strategy after a
// durable apply. It owns one writer lock across keyword and semantic stages.
func RunConfiguredIndex(ctx context.Context, root, strategy string, w io.Writer, keywordProgress func(IndexProgress), semanticProgress func(SemanticProgress)) error {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	switch strategy {
	case "", "full-text", "keyword":
		strategy = "keyword"
	case "none":
		return nil
	case "semantic", "full":
	default:
		return fmt.Errorf("unknown index strategy %q", strategy)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := acquireIndexWriterLock(root)
	if err != nil {
		return err
	}
	defer release()
	// A fresh tools-only vault has no archive directory, no documents and no
	// search database, and gets none. Existing indexes and recorded archives
	// or documents still require a readable source directory, so a missing
	// mount/directory cannot silently prune their data.
	if _, statErr := os.Stat(filepath.Join(root, "zim")); os.IsNotExist(statErr) {
		m, manifestErr := manifest.Load(filepath.Join(root, "manifest.yaml"))
		hasSources := false
		for _, entry := range m.Realized.Entries {
			hasSources = hasSources || entry.Type == "zim" || strings.HasPrefix(filepath.ToSlash(entry.RelativePath), documentsDir+"/")
		}
		documents, _ := ScanDocuments(root)
		if _, dbErr := os.Stat(filepath.Join(root, "data", "search.db")); manifestErr == nil && m.Version == 2 && !hasSources && len(documents) == 0 && os.IsNotExist(dbErr) {
			fmt.Fprintln(w, "No ZIM archives or documents to index.")
			return ctx.Err()
		}
	}
	if err := runConfiguredKeyword(ctx, root, false, w, keywordProgress); err != nil {
		return err
	}
	if strategy != "semantic" && strategy != "full" {
		return nil
	}
	// The keyword stage has just accepted the source directories, so a
	// missing one here is an empty one.
	files, err := ScanZIMFiles(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	documents, err := ScanDocuments(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if len(files) == 0 && len(documents) == 0 {
		return nil
	}
	if err := runConfiguredSemantic(ctx, root, false, w, semanticProgress); err != nil {
		return fmt.Errorf("semantic indexing after keyword indexing: %w", err)
	}
	return nil
}
