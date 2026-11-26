package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const settingsSchemaFile = "settings.schema.json"

// settingsSchema mirrors the schema document in
// drive-runtime/internal/settings. The types are duplicated because host-cli
// and drive-runtime are separate Go modules; keep them in sync.
type settingsSchema struct {
	Version int             `json:"version"`
	Groups  []settingsGroup `json:"groups"`
}

type settingsGroup struct {
	Owner    string         `json:"owner"`
	Label    string         `json:"label"`
	Settings []settingEntry `json:"settings"`
}

type settingEntry struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Default     any      `json:"default"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Choices     []string `json:"choices,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}

// writeSettingsSchema writes the settings declared by the installed recipes,
// one group per recipe, for the drive's Settings screen and `./run settings`.
// The values themselves (.svalbard/settings.json) belong to the user and are
// never touched here. Without any declared setting a stale file from an
// earlier apply is removed; the runtime's built-in settings need no file.
func writeSettingsSchema(root string, entries []manifest.RealizedEntry, recipes map[string]catalog.Item) error {
	path := filepath.Join(root, svalbardDir, settingsSchemaFile)
	schema := settingsSchema{Version: 1}
	seen := make(map[string]bool)
	for _, entry := range entries {
		// A per-platform recipe has one realized entry per platform.
		recipe, ok := recipes[entry.ID]
		if !ok || seen[entry.ID] || len(recipe.Settings) == 0 {
			continue
		}
		seen[entry.ID] = true
		group := settingsGroup{Owner: entry.ID, Label: settingsGroupLabel(recipe)}
		for _, setting := range recipe.Settings {
			group.Settings = append(group.Settings, settingEntry{
				Name: setting.Name, Type: setting.Type, Default: setting.Default,
				Label: setting.Label, Description: setting.Description,
				Choices: setting.Choices, Min: setting.Min, Max: setting.Max,
			})
		}
		schema.Groups = append(schema.Groups, group)
	}
	if len(schema.Groups) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	sort.SliceStable(schema.Groups, func(i, j int) bool {
		return strings.ToLower(schema.Groups[i].Label) < strings.ToLower(schema.Groups[j].Label)
	})

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-schema-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// settingsGroupLabel is the heading a recipe's settings appear under: the
// label of its menu entry, else its ID. A recipe whose menu entry is a fixed
// built-in (Open WebUI) has its label there, not in the recipe.
func settingsGroupLabel(recipe catalog.Item) string {
	if recipe.Menu != nil && recipe.Menu.Label != "" {
		return recipe.Menu.Label
	}
	for _, capability := range builtinCapabilities {
		if capability.ID == recipe.ID {
			return capability.Label
		}
	}
	return recipe.ID
}
