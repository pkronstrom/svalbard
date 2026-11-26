package toolkit

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// documentsSubheader heads the Library's document entries. Keep in sync with
// drive-runtime/internal/menu, which counts them apart from the archives.
const documentsSubheader = "Documents"

// documentTypes are the recipe types whose files are documents to open.
var documentTypes = map[string]bool{"pdf": true, "epub": true}

// reservedRunNames are the `./run` words the runtime answers before it looks
// at the menu.
var reservedRunNames = []string{"mcp", "settings"}

// MenuSpecs is the menu spec of each recipe as writeActionsConfig takes it:
// a recipe without a menu label is labelled with its description.
func MenuSpecs(recipes map[string]catalog.Item) map[string]catalog.MenuSpec {
	menus := make(map[string]catalog.MenuSpec)
	for id, recipe := range recipes {
		if recipe.Menu != nil {
			spec := *recipe.Menu
			if spec.Label == "" && recipe.Description != "" {
				spec.Label = recipe.Description
			}
			menus[id] = spec
		} else if recipe.Description != "" {
			menus[id] = catalog.MenuSpec{Label: recipe.Description}
		}
	}
	return menus
}

// runNames hands out `./run` names so that each means one entry.
type runNames map[string]bool

// newRunNames starts with the names that are never a recipe's to take: the
// runtime's own, and every built-in entry's, also of a built-in this drive
// does not show, so a name keeps its meaning when the feature is installed.
func newRunNames() runNames {
	names := make(runNames)
	for _, name := range reservedRunNames {
		names[name] = true
	}
	for _, capability := range builtinCapabilities {
		for _, alias := range capability.Aliases {
			names[strings.ToLower(alias)] = true
		}
	}
	return names
}

// claim returns the names that were still free, and takes them.
func (n runNames) claim(names ...string) []string {
	var free []string
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || n[key] {
			continue
		}
		n[key] = true
		free = append(free, name)
	}
	return free
}

// documentItems is one Library entry per installed document recipe. A
// collection, whose files are in books/<id>/, opens a listing of that folder
// and notes how many documents it holds; a single-document recipe opens the
// document. The label is the recipe's menu label or its humanized id, never
// the description: that is the entry's description. `./run <id>` opens the
// entry when no other entry has the name.
func documentItems(entries []manifest.RealizedEntry, recipes map[string]catalog.Item, names runNames) []menuItem {
	type documents struct {
		first      int // index of the recipe's first entry
		count      int
		path       string // below the type directory, slash-separated
		collection bool
	}
	found := make(map[string]*documents)
	var ids []string
	for i, e := range entries {
		if !documentTypes[e.Type] {
			continue
		}
		docs := found[e.ID]
		if docs == nil {
			docs = &documents{first: i}
			found[e.ID] = docs
			ids = append(ids, e.ID)
		}
		docs.count++
		typeDir := TypeDirs[e.Type]
		if rel, err := filepath.Rel(filepath.Join(typeDir, e.ID), e.RelativePath); err == nil && rel != "." && filepath.IsLocal(rel) {
			docs.collection = true
			docs.path = e.ID
		} else if docs.path == "" {
			if rel, err := filepath.Rel(typeDir, e.RelativePath); err == nil && rel != "." && filepath.IsLocal(rel) {
				docs.path = filepath.ToSlash(rel)
			}
		}
	}

	var items []menuItem
	for _, id := range ids {
		docs := found[id]
		if docs.path == "" {
			// Nothing to open: no row.
			continue
		}
		item := menuItem{
			ID:        id,
			Label:     humanize(id),
			Subheader: documentsSubheader,
			// After every archive, whose order is its entry index.
			Order:  (len(entries) + 1 + docs.first) * 100,
			Action: builtinAction("documents", map[string]string{"path": docs.path}),
		}
		if docs.collection {
			item.Note = fmt.Sprintf("%d documents", docs.count)
			if docs.count == 1 {
				item.Note = "1 document"
			}
		}
		var aliases []string
		if recipe, ok := recipes[id]; ok {
			item.Description = recipe.Description
			if menu := recipe.Menu; menu != nil {
				if menu.Label != "" {
					item.Label = menu.Label
				}
				if menu.Description != "" {
					item.Description = menu.Description
				}
				if menu.Order != 0 {
					item.Order = menu.Order
				}
				aliases = menu.Aliases
			}
		}
		if item.Description == "" {
			if docs.collection {
				item.Description = "Browse the documents in " + item.Label + "."
			} else {
				item.Description = "Open " + item.Label + "."
			}
		}
		item.Aliases = names.claim(append(append([]string(nil), aliases...), id)...)
		items = append(items, item)
	}
	return items
}
