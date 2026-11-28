package catalog

import (
	"github.com/pkronstrom/svalbard/host-cli/internal/weblauncher"
	"testing"
)

func TestStandardPresetsOfferWebLauncher(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipe, ok := cat.RecipeByID("web-launcher")
	if !ok || recipe.Build.Config["package_sha256"] != weblauncher.Fingerprint() {
		t.Fatalf("launcher recipe missing its bundled version")
	}
	for _, region := range []string{"default", "finland"} {
		for _, size := range []string{"2", "32", "64", "128", "256", "512", "1tb", "2tb"} {
			name := region + "-" + size
			if !resolvedWithDeps(t, cat, name)["web-launcher"] {
				t.Errorf("%s omits web-launcher", name)
			}
		}
	}
}
