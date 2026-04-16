package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestLoadGroupedActionsConfig(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "actions.json")
	content := `{
  "version": 2,
  "preset": "default-32",
  "groups": [
    {
      "id": "search",
      "label": "Search",
      "description": "Search across indexed archives and documents.",
      "order": 100,
      "items": [
        {
          "id": "search-all-content",
          "label": "Search all content",
          "description": "Query the on-drive search index across packaged sources.",
          "aliases": ["search"],
          "action": {
            "type": "builtin",
            "config": {
              "name": "search",
              "args": {}
            }
          }
        }
      ]
    },
    {
      "id": "library",
      "label": "Library",
      "description": "Browse packaged offline archives and documents.",
      "order": 200,
      "items": [
        {
          "id": "wikipedia-en-nopic",
          "label": "Wikipedia (text only)",
          "description": "Browse the image-free English Wikipedia archive.",
          "subheader": "Archives",
          "action": {
            "type": "builtin",
            "config": {
              "name": "browse",
              "args": {
                "zim": "wikipedia-en-nopic.zim"
              }
            }
          }
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Version, 2; got != want {
		t.Fatalf("Version = %d, want %d", got, want)
	}
	if got, want := cfg.Groups[0].ID, "search"; got != want {
		t.Fatalf("Groups[0].ID = %q, want %q", got, want)
	}
	if got, want := cfg.Groups[1].Items[0].Subheader, "Archives"; got != want {
		t.Fatalf("Groups[1].Items[0].Subheader = %q, want %q", got, want)
	}
	if got, want := cfg.Groups[0].Items[0].Aliases[0], "search"; got != want {
		t.Fatalf("Groups[0].Items[0].Aliases[0] = %q, want %q", got, want)
	}
	builtin, err := cfg.Groups[1].Items[0].Action.DecodeBuiltin()
	if err != nil {
		t.Fatalf("DecodeBuiltin() error = %v", err)
	}
	if got, want := builtin.Args["zim"], "wikipedia-en-nopic.zim"; got != want {
		t.Fatalf("builtin.Args[zim] = %q, want %q", got, want)
	}
}

func TestFindItemByAlias(t *testing.T) {
	cfg := config.RuntimeConfig{
		Groups: []config.MenuGroup{
			{
				ID: "search",
				Items: []config.MenuItem{
					{
						ID:      "search-all-content",
						Aliases: []string{"search"},
						Action:  config.BuiltinAction("search", nil),
					},
				},
			},
		},
	}

	item, ok := cfg.FindItemByAlias("search")
	if !ok {
		t.Fatal("FindItemByAlias() = false, want true")
	}
	if item.ID != "search-all-content" {
		t.Fatalf("item.ID = %q, want %q", item.ID, "search-all-content")
	}
}
