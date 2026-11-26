package builder

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

// TestBuildPythonVenvE2E builds the real python-package recipes with Docker
// against PyPI. Opt-in: SVALBARD_PYTHON_E2E=<vault dir>, optionally
// SVALBARD_PYTHON_E2E_PLATFORMS and SVALBARD_PYTHON_E2E_PACKAGES (comma lists).
func TestBuildPythonVenvE2E(t *testing.T) {
	root := os.Getenv("SVALBARD_PYTHON_E2E")
	if root == "" {
		t.Skip("set SVALBARD_PYTHON_E2E to a vault dir to run")
	}
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	desired := map[string]bool{}
	for _, id := range strings.Split(envOr("SVALBARD_PYTHON_E2E_PACKAGES", "sqlmap,pwntools"), ",") {
		desired[id] = true
	}
	platforms := strings.Split(envOr("SVALBARD_PYTHON_E2E_PLATFORMS", hostPlatformStr()), ",")
	entries, err := buildPythonVenv(root, recipe, cat, Options{Platforms: platforms, DesiredIDs: desired})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Logf("%s %s %d MB", entry.ID, entry.RelativePath, entry.SizeBytes>>20)
	}
	// Run what was built for this computer: the interpreter binary and its
	// python3 (a copy or a script, never a symlink).
	if !slices.Contains(platforms, hostPlatformStr()) {
		return
	}
	bin := findInstalledPython(filepath.Join(root, "runtime", "python", hostPlatformStr(), "python"), pythonVersionFromSpec(recipe.Python))
	for _, python := range []string{bin, filepath.Join(filepath.Dir(bin), "python3")} {
		out, err := exec.Command(python, "-c", "import ssl, sqlite3, curses, sys; print(sys.version)").CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v\n%s", python, err, out)
		}
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
