package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// DocumentPlan says what the next apply does to one document collection.
type DocumentPlan struct {
	ID string
	// New is set for a collection the vault does not hold yet.
	New  bool
	Diff apply.DocumentDiff
	// Problem is set when the collection cannot be compared here: neither
	// this computer nor the vault has a usable definition for it.
	Problem string
}

// BuildPlan is planner.Build plus document collections. The planner compares
// ids only, so a realized collection whose document list, sha256 pins or
// local file sizes no longer match its entries is added to ToDownload here;
// its apply job then fetches and deletes only the difference. Nothing is
// read from the network.
//
// A realized collection without a usable definition is left alone and
// reported, so its installed documents stay usable. So are the installed
// documents of a local source that cannot be read on this computer.
func BuildPlan(m manifest.Manifest, cat *catalog.Catalog) (planner.Plan, []DocumentPlan) {
	plan := planner.Build(m)
	realized := make(map[string][]manifest.RealizedEntry)
	for _, entry := range m.Realized.Entries {
		realized[entry.ID] = append(realized[entry.ID], entry)
	}

	var documents []DocumentPlan
	seen := make(map[string]bool, len(m.Desired.Items))
	for _, id := range m.Desired.Items {
		if seen[id] {
			continue
		}
		seen[id] = true
		entries := realized[id]
		recipe, ok := cat.RecipeByID(id)
		if !ok {
			if len(entries) > 0 && apply.IsCollectionEntry(entries[0]) {
				documents = append(documents, DocumentPlan{ID: id, Problem: "definition not found here or on the vault; updates paused"})
			}
			continue
		}
		if !recipe.IsCollection() {
			continue
		}
		set, err := catalog.ExpectedDocuments(recipe)
		switch {
		case err != nil && len(entries) == 0:
			// Already queued by id; apply reports the same problem.
			documents = append(documents, DocumentPlan{ID: id, New: true, Problem: err.Error()})
		case err != nil:
			documents = append(documents, DocumentPlan{ID: id, Problem: err.Error() + "; updates paused"})
		default:
			diff := apply.DiffDocuments(recipe, set, entries)
			if len(entries) > 0 && diff.Any() {
				plan.ToDownload = append(plan.ToDownload, id)
			}
			if len(entries) == 0 || diff.Any() || len(diff.Kept)+len(diff.Missing) > 0 {
				documents = append(documents, DocumentPlan{ID: id, New: len(entries) == 0, Diff: diff})
			}
		}
	}
	return plan, documents
}

// WriteDocumentPlans writes what an apply would do to each document
// collection, one line per collection it would change and one per document
// or source it cannot read on this computer.
func WriteDocumentPlans(w io.Writer, documents []DocumentPlan) {
	for _, item := range documents {
		diff := item.Diff
		switch {
		case item.Problem != "":
			fmt.Fprintf(w, "%s: %s\n", item.ID, item.Problem)
			continue
		case item.New:
			fmt.Fprintf(w, "new %s: %s\n", item.ID, countDocuments(diff.Added))
		case diff.Added+diff.Removed+diff.Changed > 0:
			var parts []string
			if n := diff.Added; n > 0 {
				parts = append(parts, "+"+countDocuments(n))
			}
			if n := diff.Removed; n > 0 {
				parts = append(parts, "-"+countDocuments(n))
			}
			if n := diff.Changed; n > 0 {
				parts = append(parts, countDocuments(n)+" changed")
			}
			fmt.Fprintf(w, "update %s: %s\n", item.ID, strings.Join(parts, ", "))
		}
		WriteDocumentNotes(w, item)
	}
}

// WriteDocumentNotes writes one line for each installed document that is kept
// because its local source cannot be read on this computer, and one for each
// such source the vault holds nothing from.
func WriteDocumentNotes(w io.Writer, item DocumentPlan) {
	for _, kept := range item.Diff.Kept {
		fmt.Fprintf(w, "%s: kept %s: cannot read %s (%s)\n", item.ID, kept.RelPath, kept.Source.Path, kept.Source.Reason)
	}
	for _, source := range item.Diff.Missing {
		fmt.Fprintf(w, "%s: cannot read %s (%s)\n", item.ID, source.Path, source.Reason)
	}
}

// WriteVaultDocumentNotes writes the notes of every collection of the vault
// at root, for a command that is about to apply rather than show a plan.
func WriteVaultDocumentNotes(w io.Writer, root string, cat *catalog.Catalog) {
	m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		return // the apply reports a manifest it cannot load
	}
	_, documents := BuildPlan(m, cat)
	for _, item := range documents {
		if item.Problem != "" {
			fmt.Fprintf(w, "%s: %s\n", item.ID, item.Problem)
			continue
		}
		WriteDocumentNotes(w, item)
	}
}

func countDocuments(n int) string {
	if n == 1 {
		return "1 document"
	}
	return fmt.Sprintf("%d documents", n)
}
