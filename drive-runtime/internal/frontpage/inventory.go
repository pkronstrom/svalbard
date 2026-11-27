package frontpage

import (
	"cmp"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

type inventory struct {
	Drive     string  `json:"drive"`
	Preset    string  `json:"preset,omitempty"`
	Groups    []group `json:"groups"`
	Models    []model `json:"models,omitempty"`
	Workspace string  `json:"workspace"`
	HasSearch bool    `json:"hasSearch"`
}

type group struct {
	Label string  `json:"label"`
	Items []entry `json:"items"`
}

type entry struct {
	Label       string        `json:"label"`
	Description string        `json:"description,omitempty"`
	Subheader   string        `json:"subheader,omitempty"`
	Note        string        `json:"note,omitempty"`
	Kind        string        `json:"kind"`
	Icon        string        `json:"icon"`
	Service     string        `json:"service,omitempty"`
	Command     string        `json:"command,omitempty"`
	RecipeID    string        `json:"recipeId,omitempty"`
	Summary     string        `json:"summary,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Image       *previewImage `json:"image,omitempty"`
}

type model struct {
	Label string `json:"label"`
	File  string `json:"file"`
}

type serviceSpec struct {
	name   string
	target string
}

type savedPage struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func buildInventory(root string, cfg config.RuntimeConfig) (inventory, map[string]serviceSpec) {
	inv := inventory{
		Drive: filepath.Base(root), Preset: cfg.Preset,
		Groups: []group{},
	}
	services := make(map[string]serviceSpec)
	keys := make(map[serviceSpec]string)
	keyFor := func(spec serviceSpec) string {
		if key := keys[spec]; key != "" {
			return key
		}
		key := strconv.Itoa(len(keys) + 1)
		keys[spec], services[key] = key, spec
		return key
	}
	inv.Workspace = keyFor(serviceSpec{name: "workspace"})
	metadata, _ := mcp.LoadMetadata(root)
	var usesModels bool
	modelLabels := make(map[string]string)

	groups := slices.Clone(cfg.Groups)
	slices.SortStableFunc(groups, func(a, b config.MenuGroup) int { return cmp.Compare(a.Order, b.Order) })
	for _, g := range groups {
		section := group{Label: g.Label}
		items := slices.Clone(g.Items)
		slices.SortStableFunc(items, func(a, b config.MenuItem) int { return cmp.Compare(a.Order, b.Order) })
		for _, item := range items {
			view := terminalEntry(item)
			addRecipePreview(&view, item.ID, metadata.Recipes)
			if item.Action.Type != "" && item.Action.Type != "builtin" {
				section.Items = append(section.Items, view)
				continue
			}
			action, err := item.Action.DecodeBuiltin()
			if err != nil {
				section.Items = append(section.Items, view)
				continue
			}
			view.Icon = actionIcon(action.Name)
			switch action.Name {
			case "web":
				continue
			case "search":
				view.Kind = "search"
				inv.HasSearch = true
			case "settings":
				view.Command = "./run settings"
			case "chat":
				modelLabels[filepath.Base(action.Args["model"])] = item.Label
			case "open-webui":
				usesModels = true
				fallthrough
			case "browse", "apps", "documents", "maps", "wiki", "workspace", "inspect", "serve-all", "share", "mcp-serve":
				view.Kind = "launch"
				view.Service = keyFor(specFor(action))
			}
			if view.Kind != "terminal" {
				view.Command = ""
			}
			section.Items = append(section.Items, view)
		}
		if len(section.Items) > 0 {
			inv.Groups = append(inv.Groups, section)
		}
	}
	if usesModels {
		files, _ := llamaserve.ChatModels(root)
		for _, file := range files {
			label := modelLabels[file]
			if label == "" {
				label = strings.TrimSuffix(file, filepath.Ext(file))
			}
			inv.Models = append(inv.Models, model{Label: label, File: file})
		}
	}
	return inv, services
}

func specFor(action config.BuiltinActionConfig) serviceSpec {
	spec := serviceSpec{name: action.Name}
	switch spec.name {
	case "browse":
		spec.target = action.Args["zim"]
	case "apps":
		spec.target = action.Args["app"]
	case "documents":
		spec.target = action.Args["path"]
	}
	return spec
}

func actionIcon(name string) string {
	switch name {
	case "browse", "documents", "search":
		return name
	case "maps", "wiki", "workspace":
		return name
	case "open-webui":
		return "chat"
	case "apps":
		return "apps"
	case "share", "serve-all", "mcp-serve":
		return "share"
	case "inspect", "verify", "settings":
		return "settings"
	default:
		return "terminal"
	}
}

func terminalEntry(item config.MenuItem) entry {
	view := entry{
		Label: item.Label, Description: item.Description, Subheader: item.Subheader,
		Note: item.Note, Kind: "terminal", Icon: "terminal", Command: "./run",
	}
	if len(item.Aliases) > 0 {
		alias := item.Aliases[0]
		if strings.ContainsFunc(alias, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_:.+", r))
		}) {
			alias = "'" + strings.ReplaceAll(alias, "'", "'\"'\"'") + "'"
		}
		view.Command += " " + alias
	}
	return view
}

func savedPages(root string) ([]savedPage, error) {
	pages := []savedPage{}
	folder, err := os.OpenRoot(filepath.Join(root, workspace.DirName))
	if os.IsNotExist(err) {
		return pages, nil
	}
	if err != nil {
		return nil, err
	}
	defer folder.Close()
	err = fs.WalkDir(folder.FS(), ".", func(rel string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel != "." && strings.HasPrefix(file.Name(), ".") {
			if file.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(path.Ext(rel))
		if file.Type().IsRegular() && (ext == ".html" || ext == ".htm") {
			label := strings.TrimSuffix(file.Name(), path.Ext(rel))
			if strings.EqualFold(label, "index") && path.Dir(rel) != "." {
				label = path.Base(path.Dir(rel))
			}
			label = strings.NewReplacer("-", " ", "_", " ").Replace(label)
			pages = append(pages, savedPage{Name: label, Path: rel})
		}
		return nil
	})
	return pages, err
}
