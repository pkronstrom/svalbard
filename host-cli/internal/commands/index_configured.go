package commands

import (
	"context"
	"fmt"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"io"
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
	// A fresh tools-only vault has no archive directory or search database.
	// Existing indexes and recorded archives still require a readable source
	// directory, so a missing mount/directory cannot silently prune their data.
	if _, statErr := os.Stat(filepath.Join(root, "zim")); os.IsNotExist(statErr) {
		m, manifestErr := manifest.Load(filepath.Join(root, "manifest.yaml"))
		hasArchives := false
		for _, entry := range m.Realized.Entries {
			hasArchives = hasArchives || entry.Type == "zim"
		}
		if _, dbErr := os.Stat(filepath.Join(root, "data", "search.db")); manifestErr == nil && m.Version == 2 && !hasArchives && os.IsNotExist(dbErr) {
			fmt.Fprintln(w, "No ZIM archives to index.")
			return ctx.Err()
		}
	}
	if err := runConfiguredKeyword(ctx, root, false, w, keywordProgress); err != nil {
		return err
	}
	if strategy != "semantic" && strategy != "full" {
		return nil
	}
	files, err := ScanZIMFiles(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	if err := runConfiguredSemantic(ctx, root, false, w, semanticProgress); err != nil {
		return fmt.Errorf("semantic indexing after keyword indexing: %w", err)
	}
	return nil
}
