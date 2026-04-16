package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const svalbardDir = ".svalbard"
const actionsFile = "actions.json"

// TypeDirs maps content types to their destination subdirectories.
var TypeDirs = map[string]string{
	"zim":     "zim",
	"pmtiles": "maps",
	"pdf":     "books",
	"epub":    "books",
	"gguf":    "models",
	"binary":  "bin",
	"app":     "apps",
}

// runtimeConfig matches the drive-runtime RuntimeConfig JSON structure.
type runtimeConfig struct {
	Version int         `json:"version"`
	Preset  string      `json:"preset"`
	Groups  []menuGroup `json:"groups"`
}

type menuGroup struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Order       int        `json:"order"`
	Items       []menuItem `json:"items"`
}

type menuItem struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Order       int        `json:"order"`
	Action      actionSpec `json:"action"`
}

type actionSpec struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type builtinConfig struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
}

// builtinAction constructs an actionSpec for a builtin action.
func builtinAction(name string, args map[string]string) actionSpec {
	cfg := builtinConfig{Name: name, Args: args}
	data, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return actionSpec{
		Type:   "builtin",
		Config: data,
	}
}

// humanize converts an ID like "wikipedia-en-nopic" to "Wikipedia En Nopic".
func humanize(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// Generate creates the .svalbard/actions.json runtime config under root.
// entries is the list of realized manifest entries; presetName is recorded in
// the config for the drive-runtime to identify which preset was applied.
func Generate(root string, entries []manifest.RealizedEntry, presetName string) error {
	dir := filepath.Join(root, svalbardDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var libraryItems []menuItem
	var mapsItems []menuItem

	for i, e := range entries {
		order := (i + 1) * 100
		switch e.Type {
		case "zim":
			libraryItems = append(libraryItems, menuItem{
				ID:          "browse-" + e.ID,
				Label:       humanize(e.ID),
				Description: "Browse the " + humanize(e.ID) + " archive.",
				Order:       order,
				Action:      builtinAction("browse", map[string]string{"zim": e.Filename}),
			})
		case "pmtiles":
			mapsItems = append(mapsItems, menuItem{
				ID:          "map-" + e.ID,
				Label:       humanize(e.ID),
				Description: "View the " + humanize(e.ID) + " map.",
				Order:       order,
				Action:      builtinAction("maps", nil),
			})
		}
	}

	var groups []menuGroup

	if len(libraryItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "library",
			Label:       "Library",
			Description: "Browse packaged offline archives and documents.",
			Order:       200,
			Items:       libraryItems,
		})
	}

	if len(mapsItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "maps",
			Label:       "Maps",
			Description: "View offline map layers.",
			Order:       300,
			Items:       mapsItems,
		})
	}

	// Tools group is always present.
	groups = append(groups, menuGroup{
		ID:          "tools",
		Label:       "Tools",
		Description: "System utilities and diagnostics.",
		Order:       900,
		Items: []menuItem{
			{
				ID:          "inspect-drive",
				Label:       "Inspect Drive",
				Description: "Inspect the drive contents and configuration.",
				Order:       100,
				Action:      builtinAction("inspect", nil),
			},
			{
				ID:          "verify-checksums",
				Label:       "Verify Checksums",
				Description: "Verify file integrity using checksums.",
				Order:       200,
				Action:      builtinAction("verify", nil),
			},
		},
	})

	// Sort groups by order.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Order < groups[j].Order
	})

	cfg := runtimeConfig{
		Version: 2,
		Preset:  presetName,
		Groups:  groups,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, actionsFile), data, 0o644)
}
