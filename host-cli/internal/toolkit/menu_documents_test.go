package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// menuRow is a generated menu item with its group and decoded action.
type menuRow struct {
	Group     string
	ID        string
	Label     string
	Note      string
	Subheader string
	Desc      string
	Aliases   []string
	Order     int
	Action    string
	Args      map[string]string
}

// generatedMenu writes the menu for entries, as apply does for the recipes
// it knows, and returns its rows in menu order.
func generatedMenu(t *testing.T, entries []manifest.RealizedEntry, recipes map[string]catalog.Item) []menuRow {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, svalbardDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeActionsConfig(root, entries, "test", MenuSpecs(recipes), recipes); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, svalbardDir, actionsFile))
	if err != nil {
		t.Fatal(err)
	}
	var cfg runtimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	var rows []menuRow
	for _, group := range cfg.Groups {
		for _, item := range group.Items {
			row := menuRow{Group: group.ID, ID: item.ID, Label: item.Label, Note: item.Note, Subheader: item.Subheader,
				Desc: item.Description, Aliases: item.Aliases, Order: item.Order}
			if item.Action.Type == "builtin" {
				var action builtinConfig
				if err := json.Unmarshal(item.Action.Config, &action); err != nil {
					t.Fatal(err)
				}
				row.Action, row.Args = action.Name, action.Args
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func rowsByID(rows []menuRow, id string) []menuRow {
	var found []menuRow
	for _, row := range rows {
		if row.ID == id {
			found = append(found, row)
		}
	}
	return found
}

func oneRow(t *testing.T, rows []menuRow, id string) menuRow {
	t.Helper()
	found := rowsByID(rows, id)
	if len(found) != 1 {
		t.Fatalf("menu rows for %q = %+v, want exactly one", id, found)
	}
	return found[0]
}

func TestMenuListsEachDocumentRecipeOnceUnderDocuments(t *testing.T) {
	entries := []manifest.RealizedEntry{
		{ID: "car-manuals", Type: "pdf", Filename: "honda.pdf", RelativePath: "books/car-manuals/honda.pdf"},
		{ID: "car-manuals", Type: "pdf", Filename: "v60.pdf", RelativePath: "books/car-manuals/volvo/v60.pdf"},
		{ID: "car-manuals", Type: "pdf", Filename: "v70.pdf", RelativePath: "books/car-manuals/volvo/v70.pdf"},
		{ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim"},
		{ID: "one-sheet", Type: "pdf", Filename: "sheet.pdf", RelativePath: "books/one-sheet/sheet.pdf"},
		{ID: "who-basic-emergency-care", Type: "pdf", Filename: "WHO_Basic_Emergency_Care.pdf", RelativePath: "books/WHO_Basic_Emergency_Care.pdf"},
		{ID: "army-tm-9-8000", Type: "pdf", Filename: "TM_9-8000.pdf", RelativePath: "books/TM_9-8000.pdf"},
		{ID: "wikem", Type: "zim", Filename: "wikem.zim", RelativePath: "zim/wikem.zim"},
	}
	recipes := map[string]catalog.Item{
		"car-manuals": {ID: "car-manuals", Type: "pdf", Description: "Owner's manuals for the family's cars",
			Documents: []catalog.DocumentSource{{URL: "https://example.org/honda.pdf"}}},
		"who-basic-emergency-care": {ID: "who-basic-emergency-care", Type: "pdf", Description: "WHO Basic Emergency Care — open-access triage",
			Menu: &catalog.MenuSpec{Label: "WHO Basic Emergency Care"}},
		"army-tm-9-8000": {ID: "army-tm-9-8000", Type: "pdf", Description: "US Army TM 9-8000 Principles of Automotive Vehicles (1985) — engines, fuel and brakes"},
		"ifixit": {ID: "ifixit", Type: "zim", Description: "Repair guides"},
		"wikem":  {ID: "wikem", Type: "zim", Description: "Emergency medicine wiki"},
	}
	rows := generatedMenu(t, entries, recipes)

	collection := oneRow(t, rows, "car-manuals")
	want := menuRow{Group: "library", ID: "car-manuals", Label: "Car Manuals", Note: "3 documents", Subheader: "Documents",
		Desc: "Owner's manuals for the family's cars", Aliases: []string{"car-manuals"}, Order: collection.Order,
		Action: "documents", Args: map[string]string{"path": "car-manuals"}}
	if !reflect.DeepEqual(collection, want) {
		t.Errorf("collection row = %+v\nwant %+v", collection, want)
	}

	// A collection of one document is still a folder with a count.
	single := oneRow(t, rows, "one-sheet")
	if single.Note != "1 document" || single.Args["path"] != "one-sheet" || single.Label != "One Sheet" {
		t.Errorf("one-document collection = %+v", single)
	}
	// Without a recipe to describe it, the row says what it opens.
	if single.Desc != "Browse the documents in One Sheet." {
		t.Errorf("description without a recipe = %q", single.Desc)
	}

	// A single-document recipe opens the document itself and has no count.
	who := oneRow(t, rows, "who-basic-emergency-care")
	if who.Action != "documents" || who.Args["path"] != "WHO_Basic_Emergency_Care.pdf" || who.Note != "" {
		t.Errorf("single document row = %+v", who)
	}
	if who.Label != "WHO Basic Emergency Care" || who.Desc != "WHO Basic Emergency Care — open-access triage" {
		t.Errorf("single document label and description = %q, %q", who.Label, who.Desc)
	}

	// Without a menu label, the title the description leads with names the row.
	army := oneRow(t, rows, "army-tm-9-8000")
	if army.Label != "US Army TM 9-8000 Principles of Automotive Vehicles (1985)" || army.Desc != "US Army TM 9-8000 Principles of Automotive Vehicles (1985) — engines, fuel and brakes" {
		t.Errorf("described document label and description = %q, %q", army.Label, army.Desc)
	}

	// The Library reads archives first, then the documents in one block.
	var library []string
	for _, row := range rows {
		if row.Group == "library" {
			library = append(library, row.Subheader+"/"+row.ID)
		}
	}
	wantOrder := []string{"Archives/ifixit", "Archives/wikem", "Documents/car-manuals", "Documents/one-sheet", "Documents/who-basic-emergency-care", "Documents/army-tm-9-8000"}
	if !reflect.DeepEqual(library, wantOrder) {
		t.Errorf("library = %v\nwant    %v", library, wantOrder)
	}

	// Documents alone are enough for Search.
	if len(rowsByID(generatedMenu(t, entries[:3], recipes), "search-all")) != 1 {
		t.Error("a drive with only documents has no Search entry")
	}
}

func TestDocumentRunNameIsTheRecipeIDUnlessTaken(t *testing.T) {
	entries := []manifest.RealizedEntry{
		{ID: "duckdb", Type: "pdf", Filename: "a.pdf", RelativePath: "books/duckdb/a.pdf"},
		{ID: "duckdb-wasm", Type: "app", Filename: "duckdb-wasm", RelativePath: "apps/duckdb-wasm"},
		{ID: "field-manuals", Type: "pdf", Filename: "a.pdf", RelativePath: "books/field-manuals/a.pdf"},
		{ID: "Settings", Type: "pdf", Filename: "a.pdf", RelativePath: "books/Settings/a.pdf"},
		{ID: "wiki", Type: "pdf", Filename: "a.pdf", RelativePath: "books/wiki/a.pdf"},
		{ID: "workspace", Type: "pdf", Filename: "a.pdf", RelativePath: "books/workspace/a.pdf"},
	}
	recipes := map[string]catalog.Item{
		"duckdb-wasm":   {ID: "duckdb-wasm", Type: "app", Menu: &catalog.MenuSpec{Label: "DuckDB", Aliases: []string{"duckdb"}}},
		"field-manuals": {ID: "field-manuals", Type: "pdf", Menu: &catalog.MenuSpec{Aliases: []string{"fm", "search"}}},
	}
	rows := generatedMenu(t, entries, recipes)

	if got := oneRow(t, rows, "field-manuals").Aliases; !reflect.DeepEqual(got, []string{"fm", "field-manuals"}) {
		t.Errorf("aliases = %v, want the recipe's free alias and its id", got)
	}
	// Taken by a recipe's alias, by the runtime, and by built-in entries,
	// one of which this drive does not even show.
	for _, id := range []string{"duckdb", "Settings", "wiki", "workspace"} {
		if got := oneRow(t, rows, id); len(got.Aliases) != 0 || got.Action != "documents" {
			t.Errorf("%s: aliases = %v, action = %q; want a row without a ./run name", id, got.Aliases, got.Action)
		}
	}
	if got := oneRow(t, rows, "duckdb-wasm").Aliases; !reflect.DeepEqual(got, []string{"duckdb"}) {
		t.Errorf("duckdb-wasm aliases = %v", got)
	}
}

func TestMenuHasNoRowForARecipeWithNothingToRun(t *testing.T) {
	entries := []manifest.RealizedEntry{
		{ID: "dufs", Type: "binary", RelativePath: "bin/macos-arm64/dufs.tar.gz"},
		{ID: "dufs", Type: "binary", RelativePath: "bin/linux-x86_64/dufs.tar.gz"},
		{ID: "fimea", Type: "sqlite", Filename: "fimea.sqlite", RelativePath: "data/fimea.sqlite"},
		{ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim"},
		{ID: "kiwix-serve", Type: "binary", RelativePath: "bin/macos-arm64/kiwix-tools.tar.gz"},
		{ID: "opencode", Type: "binary", RelativePath: "bin/macos-arm64/opencode.zip"},
		{ID: "opencode", Type: "binary", RelativePath: "bin/linux-x86_64/opencode.tar.gz"},
		{ID: "unplaced", Type: "pdf", Filename: "unplaced.pdf"},
	}
	recipes := map[string]catalog.Item{
		"dufs":     {ID: "dufs", Type: "binary", Description: "Dufs — HTTP file server"},
		"opencode": {ID: "opencode", Type: "binary", Menu: &catalog.MenuSpec{Group: "local-ai", Subheader: "AI Clients", Label: "OpenCode"}},
	}
	rows := generatedMenu(t, entries, recipes)

	for _, id := range []string{"dufs", "kiwix-serve", "fimea", "unplaced"} {
		if found := rowsByID(rows, id); len(found) != 0 {
			t.Errorf("%s has a menu row with nothing behind it: %+v", id, found)
		}
	}
	// One row for a client installed for two platforms.
	if client := oneRow(t, rows, "opencode"); client.Action != "agent" || client.Args["client"] != "opencode" {
		t.Errorf("opencode row = %+v", client)
	}
	for _, row := range rows {
		if row.Action == "" {
			t.Errorf("row %s/%s has no action", row.Group, row.ID)
		}
	}

	// A database opens in SQLiteViz, so it is listed once that is installed.
	withViewer := append(entries, manifest.RealizedEntry{ID: "sqliteviz", Type: "app", RelativePath: "apps/sqliteviz"})
	if db := oneRow(t, generatedMenu(t, withViewer, recipes), "fimea"); db.Action != "apps" || db.Args["app"] != "sqliteviz" {
		t.Errorf("database row = %+v", db)
	}
}

func TestMenuOffersTheWorkspaceWithOpenWebUI(t *testing.T) {
	model := manifest.RealizedEntry{ID: "qwen-4b", Type: "gguf", Filename: "qwen.gguf"}
	wiki := manifest.RealizedEntry{ID: "silverbullet", Type: "binary"}
	app := manifest.RealizedEntry{ID: "cyberchef", Type: "app", RelativePath: "apps/cyberchef"}

	if found := rowsByID(generatedMenu(t, []manifest.RealizedEntry{model, wiki, app}, nil), "workspace"); len(found) != 0 {
		t.Fatalf("Workspace offered without Open WebUI: %+v", found)
	}

	rows := generatedMenu(t, []manifest.RealizedEntry{
		app, model, wiki,
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/macos-arm64/tools/open-webui"},
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/linux-x86_64/tools/open-webui"},
	}, nil)
	workspace := oneRow(t, rows, "workspace")
	want := menuRow{Group: "apps", ID: "workspace", Label: "Workspace", Note: "pages the assistant saved", Subheader: "Apps",
		Desc: workspace.Desc, Aliases: []string{"workspace"}, Order: workspace.Order, Action: "workspace"}
	if !reflect.DeepEqual(workspace, want) {
		t.Errorf("workspace row = %+v\nwant %+v", workspace, want)
	}
	if workspace.Desc == "" {
		t.Error("workspace row has no description")
	}

	// Wiki, Workspace, then the installed apps.
	var apps []string
	for _, row := range rows {
		if row.Group == "apps" {
			apps = append(apps, row.ID)
		}
	}
	if !reflect.DeepEqual(apps, []string{"silverbullet", "workspace", "cyberchef"}) {
		t.Errorf("apps = %v", apps)
	}
}

func TestMapLibrariesStayOffTheMenu(t *testing.T) {
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	entries := []manifest.RealizedEntry{
		{ID: "maplibre-vendor", Type: "app", RelativePath: "apps/maplibre-vendor"},
		{ID: "mml-map-sheets", Type: "app", RelativePath: "apps/mml-map-sheets"},
	}
	recipes := map[string]catalog.Item{}
	for _, entry := range entries {
		recipe, ok := cat.RecipeByID(entry.ID)
		if !ok {
			t.Fatalf("missing recipe %s", entry.ID)
		}
		recipes[entry.ID] = recipe
	}
	rows := generatedMenu(t, entries, recipes)
	if vendor := rowsByID(rows, "maplibre-vendor"); len(vendor) != 0 {
		t.Errorf("vendor files offered as a standalone app: %+v", vendor)
	}
	if sheets := oneRow(t, rows, "mml-map-sheets"); sheets.Group != "maps" || sheets.Action != "apps" {
		t.Errorf("map sheets must open their page from Maps: %+v", sheets)
	}
}
