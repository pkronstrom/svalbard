package cli

import (
	"io"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	hosttui "github.com/pkronstrom/svalbard/host-tui"
)

func TestSizeEstimatesCountPerPlatformRecipesOncePerVaultPlatform(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cat, err := catalog.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	kiwix, ok := cat.RecipeByID("kiwix-serve")
	if !ok || len(kiwix.Platforms) < 2 {
		t.Fatal("kiwix-serve fixture needs a platforms map")
	}
	vault := t.TempDir()
	if err := commands.InitVaultWithOptions(vault, []string{"kiwix-serve"}, "", "", []string{"macos-arm64", "linux-x86_64"}); err != nil {
		t.Fatal(err)
	}
	sourceSize := func(groups []hosttui.PackGroup) float64 {
		for _, g := range groups {
			for _, p := range g.Packs {
				for _, s := range p.Sources {
					if s.ID == "kiwix-serve" {
						return s.SizeGB
					}
				}
			}
		}
		t.Fatal("kiwix-serve missing from pack groups")
		return 0
	}

	cfg, err := buildWizardConfig(vault, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got := sourceSize(cfg.PackGroups); got != kiwix.SizeGB {
		t.Errorf("wizard before the platform step: %v GB, want one platform's %v", got, kiwix.SizeGB)
	}
	_, groups := cfg.ForPlatforms([]string{"macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64"})
	if got, want := sourceSize(groups), 4*kiwix.SizeGB; got != want {
		t.Errorf("wizard with four platforms: %v GB, want %v", got, want)
	}

	deps := buildDashboardDeps(vault, &cfg)
	want := 2 * kiwix.SizeGB
	if got := sourceSize(deps.PackGroups); got != want {
		t.Errorf("browse for a two-platform vault: %v GB, want %v", got, want)
	}
	plan, err := deps.LoadPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.DownloadGB != want {
		t.Errorf("plan download for a two-platform vault: %v GB, want %v", plan.DownloadGB, want)
	}
}
