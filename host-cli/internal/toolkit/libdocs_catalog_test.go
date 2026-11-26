package toolkit

import (
	"io/fs"
	"path"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestRealLibraryDocsPassLint(t *testing.T) {
	if problems := LintLibraryDocs(EmbeddedLibraryDocs(), embeddedRecipes(t)); len(problems) != 0 {
		t.Fatalf("library docs fail the offline check:\n%s", problemsText(problems))
	}
}

// clusterBytes is the allocation unit of a large exFAT stick: every file
// takes at least this much, so the budgets count files as well as bytes.
const clusterBytes = 128 << 10

func allocated(size int64) int64 {
	return (size + clusterBytes - 1) / clusterBytes * clusterBytes
}

// Per library: its quickstart and reference file on the stick, and in the
// archive its one build plus package.json, licence and README (Three.js
// adds the two addons its quickstart names).
const (
	maxLibraryStickFiles   = 2
	maxLibraryArchiveFiles = 8
	maxLibraryBytes        = 6 << 20
)

// Per recipe, measured 2026-11-27 with headroom: files on the stick (the
// archive and the docs) and their allocated size at 128 KB clusters.
var recipeBudgets = map[string]struct {
	files     int
	allocated int64
}{
	"js-core":   {files: 33, allocated: 11 << 20},
	"js-3d-cad": {files: 7, allocated: 2 << 20},
}

// frontmatterAllowance covers the frontmatter apply adds to a quickstart.
const frontmatterAllowance = 512

func TestLibraryBudgets(t *testing.T) {
	docs := EmbeddedLibraryDocs()
	seen := map[string]bool{}
	for _, recipe := range embeddedRecipes(t) {
		if recipe.Type != catalog.LibraryType {
			continue
		}
		seen[recipe.ID] = true
		budget, ok := recipeBudgets[recipe.ID]
		if !ok {
			t.Errorf("%s has no budget in recipeBudgets", recipe.ID)
			continue
		}
		if recipe.Archive == nil || recipe.Archive.Bytes == 0 {
			t.Errorf("%s records no archive size (run build-tools/cmd/js-lock)", recipe.ID)
			continue
		}
		files, size := 1, allocated(recipe.Archive.Bytes)
		for _, lib := range recipe.Libraries {
			stick := 0
			for _, name := range []string{catalog.LibraryDocName(lib.Package), catalog.LibraryReferenceName(lib.Package)} {
				info, err := fs.Stat(docs, path.Join(recipe.ID, name))
				if err != nil {
					continue
				}
				stick++
				size += allocated(info.Size() + frontmatterAllowance)
			}
			files += stick
			if stick > maxLibraryStickFiles {
				t.Errorf("%s/%s puts %d files on the stick, budget %d", recipe.ID, lib.Package, stick, maxLibraryStickFiles)
			}
			for _, source := range recipe.Bundle {
				if catalog.TopLevelPackage(source) != lib.Package {
					continue
				}
				if source.Extracted == nil {
					t.Errorf("%s/%s records no extracted size", recipe.ID, lib.Package)
				} else if source.Extracted.Files > maxLibraryArchiveFiles || source.Extracted.Bytes > maxLibraryBytes {
					t.Errorf("%s/%s puts %d files, %d bytes in the archive; budget %d files, %d bytes",
						recipe.ID, lib.Package, source.Extracted.Files, source.Extracted.Bytes, maxLibraryArchiveFiles, maxLibraryBytes)
				}
			}
		}
		t.Logf("%s: %d files on the stick, %.2f MB allocated (archive %d files, %d bytes)", recipe.ID, files, float64(size)/(1<<20), recipe.Archive.Files, recipe.Archive.Bytes)
		if files > budget.files || size > budget.allocated {
			t.Errorf("%s: %d files, %d bytes allocated on the stick; budget %d files, %d bytes", recipe.ID, files, size, budget.files, budget.allocated)
		}
	}
	for id := range recipeBudgets {
		if !seen[id] {
			t.Errorf("recipeBudgets names %s, which is not a js-lib recipe", id)
		}
	}
}
