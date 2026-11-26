package toolkit

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// The guard: nothing in the catalog can be installed without being reachable
// from the drive's menu. Installing a recipe must add a menu row of its own
// with an action the runtime runs, or the built-in row named for its type in
// reachedThrough, unless the recipe is listed below with the reason it has
// none. A new recipe or type that adds no row fails here until it gets a menu
// entry or a line in one of these lists. Rows a recipe merely switches on for
// the whole drive (Search, Serve Everything, Share) do not count.

// reachedThrough names the built-in row that stands for every recipe of a
// type, by row id.
var reachedThrough = map[string]string{
	"toolchain": "embedded-shell",
}

// noEntryYet marks what a user would want to reach and cannot yet: a gap,
// not a decision. Everything else on the lists is support for another entry.
const noEntryYet = "no menu entry yet"

// supportOnlyTypes are the recipe types that never get a row of their own.
var supportOnlyTypes = map[string]string{
	"gguf-embed":  "embedding model: Search uses it for semantic results",
	"gguf-mtp":    "draft model: speeds up the chat model it is paired with",
	"python-venv": "the Python environment that Python packages run in",
	"skills":      "procedures for the AI clients, which find them with skill_search",
	"js-lib":      "browser libraries the AI clients' HTML skills pack into pages with svalbard-drive html pack",
	"dataset":     noEntryYet, // word lists and templates for the command-line tools
	"gpkg":        "map layers in data/, which the AI clients query with query_sql and the geodata skill exports",
	"sqlite":      noEntryYet, // gets a row only when the SQLiteViz app is installed too, and nothing installs it for it
}

// supportOnlyRecipes are the recipes without a row whose type does have
// entries, by reason.
var supportOnlyRecipes = map[string][]string{
	"server that the Library's archive entries start":      {"kiwix-serve"},
	"server that the Local AI entries start":               {"llama-server"},
	"builds the Python environment during apply":           {"uv"},
	"PlatformIO itself: the Embedded Dev Shell runs it":    {"platformio-core", "espidf-python"},
	"Python libraries the AI clients' document skills run": {"py-docs"},
	"dependency of the map viewer, which has its own rows": {"go-pmtiles", "dufs"},
	// Command-line tools: launching them from the menu is a later phase.
	noEntryYet: {
		"7z", "age", "caido", "chisel", "duckdb", "feroxbuster", "ffuf", "fzf", "gobuster", "httpx", "jq", "katana",
		"ligolo-ng-agent", "ligolo-ng-proxy", "linpeas", "naabu", "nmap-static", "nuclei", "ripgrep", "rustscan",
		"sqlite3", "subfinder", "toybox", "winpeas", "yq", "zstd",
		"angr", "binwalk", "pwntools", "sqlmap", "volatility3",
	},
}

// installedEntries is what apply records for a recipe installed on every
// supported platform. Checksums are left out on purpose: with one, every
// recipe would add the Verify Checksums row and pass the guard through it.
func installedEntries(item catalog.Item) []manifest.RealizedEntry {
	dir := TypeDirs[item.Type]
	entry := func(rel string) manifest.RealizedEntry {
		return manifest.RealizedEntry{ID: item.ID, Type: item.Type, Filename: path.Base(rel), RelativePath: filepath.FromSlash(rel)}
	}
	var entries []manifest.RealizedEntry
	switch {
	case len(item.Platforms) > 0:
		for _, platform := range supportedPlatforms {
			entries = append(entries, entry(path.Join(dir, platform, item.ID+".tar.gz")))
		}
	case item.Type == "python-package" || item.Type == "python-venv":
		for _, platform := range supportedPlatforms {
			entries = append(entries, entry(path.Join(dir, platform, "tools", item.ID)))
		}
	case item.Type == "app":
		entries = append(entries, entry(path.Join(dir, item.ID)))
	case item.Filename != "":
		entries = append(entries, entry(path.Join(dir, item.Filename)))
	default:
		entries = append(entries, entry(path.Join(dir, item.ID+"."+item.Type)))
	}
	return entries
}

// runtimeActions reads the built-in actions the drive runtime dispatches,
// each with its argument names and whether the argument is required, from
// the runtime's source: the two modules cannot import each other.
func runtimeActions(t *testing.T) map[string]map[string]bool {
	t.Helper()
	dir, err := resolveDriveRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(dir, "internal", "actions", "actions.go"))
	if err != nil {
		t.Fatal(err)
	}
	spec := regexp.MustCompile(`(?m)^\s*\{actionID: "([^"]+)", subcommand: \w+(.*)\},$`)
	argument := regexp.MustCompile(`\{name: "([^"]+)"(, required: true)?`)
	actions := make(map[string]map[string]bool)
	for _, match := range spec.FindAllStringSubmatch(string(source), -1) {
		args := make(map[string]bool)
		for _, arg := range argument.FindAllStringSubmatch(match[2], -1) {
			args[arg[1]] = arg[2] != ""
		}
		actions[match[1]] = args
	}
	if len(actions) < 10 || actions["apps"]["app"] != true || len(actions["wiki"]) != 0 {
		t.Fatalf("could not read the runtime's actions from %s: %v", dir, actions)
	}
	// A screen of the menu itself (drive-runtime/internal/menu), not a
	// dispatched action.
	actions["settings"] = map[string]bool{}
	return actions
}

// runnable reports why a row's action cannot run, or "" when it can.
func runnable(row menuRow, actions map[string]map[string]bool) string {
	if row.Action == "" {
		return "has no action"
	}
	args, known := actions[row.Action]
	if !known {
		return "has the action " + row.Action + ", which the runtime does not have"
	}
	for name, required := range args {
		if required && row.Args[name] == "" {
			return "lacks the " + name + " argument of " + row.Action
		}
	}
	for name := range row.Args {
		if _, ok := args[name]; !ok {
			return "passes " + name + " to " + row.Action + ", which does not take it"
		}
	}
	return ""
}

func embeddedRecipes(t *testing.T) []catalog.Item {
	t.Helper()
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipes := slices.DeleteFunc(cat.AllRecipes(), func(recipe catalog.Item) bool {
		// The type-level dependency table sits among the recipe files, and
		// the loader reads it as a recipe without a type.
		return recipe.ID == "dep-defaults" && recipe.Type == ""
	})
	sort.Slice(recipes, func(i, j int) bool { return recipes[i].ID < recipes[j].ID })
	return recipes
}

func TestEveryCatalogRecipeIsReachableFromTheMenu(t *testing.T) {
	actions := runtimeActions(t)
	recipes := embeddedRecipes(t)

	always := make(map[string]bool)
	for _, row := range generatedMenu(t, nil, nil) {
		always[row.ID] = true
	}

	recipeReason := make(map[string]string)
	for reason, ids := range supportOnlyRecipes {
		for _, id := range ids {
			if recipeReason[id] != "" {
				t.Errorf("%s is on the support-only list twice", id)
			}
			recipeReason[id] = reason
		}
	}

	types := make(map[string]bool)
	reachableTypes := make(map[string]bool)
	ids := make(map[string]bool)
	for _, recipe := range recipes {
		types[recipe.Type] = true
		ids[recipe.ID] = true

		// The recipe on a drive that has nothing else.
		reachable := false
		for _, row := range generatedMenu(t, installedEntries(recipe), map[string]catalog.Item{recipe.ID: recipe}) {
			own := row.ID == recipe.ID || row.ID == reachedThrough[recipe.Type]
			if own && !always[row.ID] && runnable(row, actions) == "" {
				reachable = true
			}
		}
		if reachable {
			reachableTypes[recipe.Type] = true
		}

		typeReason, typeListed := supportOnlyTypes[recipe.Type]
		reason, listed := recipeReason[recipe.ID]
		switch {
		case reachable && listed:
			t.Errorf("%s is on the support-only list (%s) but has a menu entry now: take it off", recipe.ID, reason)
		case typeListed && listed:
			t.Errorf("%s is on the support-only list, and so is its whole type %s (%s): drop one", recipe.ID, recipe.Type, typeReason)
		case !reachable && !listed && !typeListed:
			t.Errorf("%s (type %s) can be installed but nothing in the drive's menu reaches it. "+
				"Give it a menu entry, or add it to supportOnlyRecipes (or its type to supportOnlyTypes) with the reason.",
				recipe.ID, recipe.Type)
		}
	}

	for recipeType, reason := range supportOnlyTypes {
		if !types[recipeType] {
			t.Errorf("supportOnlyTypes lists %s, which no recipe has", recipeType)
		}
		if reachableTypes[recipeType] {
			t.Errorf("supportOnlyTypes lists %s (%s) but a recipe of that type has a menu entry now: take it off", recipeType, reason)
		}
	}
	for id := range recipeReason {
		if !ids[id] {
			t.Errorf("supportOnlyRecipes lists %s, which is not in the catalog", id)
		}
	}
	for recipeType := range reachedThrough {
		if !reachableTypes[recipeType] {
			t.Errorf("reachedThrough names a built-in row for %s, but no recipe of that type gets it", recipeType)
		}
	}
}

// A drive with the whole catalog on it: every row runs something, and every
// `./run` name and row id means one row.
func TestFullCatalogMenuHasNoDeadRowsAndUniqueRunNames(t *testing.T) {
	actions := runtimeActions(t)
	var entries []manifest.RealizedEntry
	byID := make(map[string]catalog.Item)
	for _, recipe := range embeddedRecipes(t) {
		entries = append(entries, installedEntries(recipe)...)
		byID[recipe.ID] = recipe
	}
	rows := generatedMenu(t, entries, byID)
	if len(rows) < 100 {
		t.Fatalf("the full catalog gave %d menu rows", len(rows))
	}

	names := make(map[string]string)
	rowIDs := make(map[string]bool)
	for _, row := range rows {
		if problem := runnable(row, actions); problem != "" {
			t.Errorf("menu row %s/%s (%q) %s", row.Group, row.ID, row.Label, problem)
		}
		if rowIDs[row.ID] {
			t.Errorf("two menu rows have the id %s", row.ID)
		}
		rowIDs[row.ID] = true
		for _, alias := range row.Aliases {
			name := strings.ToLower(alias)
			if other, taken := names[name]; taken {
				t.Errorf("./run %s means both %s and %s", name, other, row.ID)
			}
			if slices.Contains(reservedRunNames, name) {
				t.Errorf("./run %s is the runtime's own command, and %s claims it", name, row.ID)
			}
			names[name] = row.ID
		}
	}

	// The names this work promises.
	for name, id := range map[string]string{
		"workspace":                "workspace",
		"who-basic-emergency-care": "who-basic-emergency-care",
		"wiki":                     wikiRecipeID,
		"search":                   "search-all",
	} {
		if names[name] != id {
			t.Errorf("./run %s opens %q, want %q", name, names[name], id)
		}
	}
}
