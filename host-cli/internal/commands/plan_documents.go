package commands

import (
	"fmt"
	"io"
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
	// Problem is set when the collection cannot be compared here: its
	// definition, or a local folder it reads, is missing on this computer.
	Problem string
}

// BuildPlan is planner.Build plus document collections. The planner compares
// ids only, so a realized collection whose document list, sha256 pins or
// local file sizes no longer match its entries is added to ToDownload here;
// its apply job then fetches and deletes only the difference. Nothing is
// read from the network.
//
// A realized collection that cannot be compared (no definition on this
// computer, or an unreadable local source) is left alone and reported, so the
// installed documents stay usable.
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
				documents = append(documents, DocumentPlan{ID: id, Problem: "definition not found here; updates paused"})
			}
			continue
		}
		if !recipe.IsCollection() {
			continue
		}
		docs, err := catalog.ExpectedDocuments(recipe)
		if len(entries) == 0 {
			// Already queued by id; apply reports a source it cannot read.
			item := DocumentPlan{ID: id, New: true, Diff: apply.DocumentDiff{Added: len(docs)}}
			if err != nil {
				item.Problem = err.Error()
			}
			documents = append(documents, item)
			continue
		}
		if err != nil {
			documents = append(documents, DocumentPlan{ID: id, Problem: err.Error() + "; updates paused"})
			continue
		}
		if diff := apply.DiffDocuments(recipe, docs, entries); diff.Any() {
			plan.ToDownload = append(plan.ToDownload, id)
			documents = append(documents, DocumentPlan{ID: id, Diff: diff})
		}
	}
	return plan, documents
}

// WriteDocumentPlans writes one line per document collection an apply would
// touch or cannot compare.
func WriteDocumentPlans(w io.Writer, documents []DocumentPlan) {
	for _, item := range documents {
		switch {
		case item.Problem != "":
			fmt.Fprintf(w, "%s: %s\n", item.ID, item.Problem)
		case item.New:
			fmt.Fprintf(w, "new %s: %s\n", item.ID, countDocuments(item.Diff.Added))
		default:
			var parts []string
			if n := item.Diff.Added; n > 0 {
				parts = append(parts, "+"+countDocuments(n))
			}
			if n := item.Diff.Removed; n > 0 {
				parts = append(parts, "-"+countDocuments(n))
			}
			if n := item.Diff.Changed; n > 0 {
				parts = append(parts, countDocuments(n)+" changed")
			}
			fmt.Fprintf(w, "update %s: %s\n", item.ID, strings.Join(parts, ", "))
		}
	}
}

func countDocuments(n int) string {
	if n == 1 {
		return "1 document"
	}
	return fmt.Sprintf("%d documents", n)
}
