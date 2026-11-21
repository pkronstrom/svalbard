package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func newPythonTestCatalog(t *testing.T) (*catalog.Catalog, catalog.Item) {
	t.Helper()
	recipes := fstest.MapFS{
		"svalbard-python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nbuild:\n  family: python-venv\n")},
		"sqlmap.yaml":          {Data: []byte("id: sqlmap\ntype: python-package\nvenv: svalbard-python\npackages: [sqlmap==1.10.9]\nentry_points: [sqlmap]\n")},
		"angr.yaml":            {Data: []byte("id: angr\ntype: python-package\nvenv: svalbard-python\npackages: [angr==10.0.1]\nentry_points: [angr]\n")},
	}
	cat, err := catalog.LoadFromFS(recipes, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	return cat, recipe
}

// fakeUV emulates uv inside the tools container: python installs create a
// python-build-standalone layout, pip installs create a --target tree unless
// the package/platform pair is listed in unavailable. angr needs an sdist
// build, so its wheels-only attempt fails.
func fakeUV(t *testing.T, unavailable map[string]bool) *[][]string {
	needsBuild := map[string]bool{"angr": true}
	t.Helper()
	var calls [][]string
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool != "env" {
			t.Fatalf("tool = %q, want env", tool)
		}
		calls = append(calls, args)
		uvAt := slices.Index(args, "uv")
		uvArgs := args[uvAt+1:]
		flag := func(name string) string { return uvArgs[slices.Index(uvArgs, name)+1] }
		switch strings.Join(uvArgs[:2], " ") {
		case "python install":
			request := uvArgs[len(uvArgs)-1]
			build := filepath.Join(flag("--install-dir"), strings.Replace(request, "3.12", "3.12.14", 1))
			if err := os.MkdirAll(filepath.Join(build, "bin"), 0o755); err != nil {
				return "", err
			}
			if err := os.Symlink(build, filepath.Join(flag("--install-dir"), request)); err != nil {
				return "", err
			}
			return "", os.WriteFile(filepath.Join(build, "bin", "python3.12"), nil, 0o755)
		case "pip install":
			pkg := strings.Split(uvArgs[len(uvArgs)-1], "==")[0]
			if unavailable[pkg+"/"+flag("--python-platform")] || (needsBuild[pkg] && slices.Contains(uvArgs, "--no-build")) {
				return "", errors.New("no solution found")
			}
			target := flag("--target")
			if err := os.MkdirAll(filepath.Join(target, "bin"), 0o755); err != nil {
				return "", err
			}
			return "", os.WriteFile(filepath.Join(target, "bin", pkg), []byte("#!/usr/local/bin/python\n"), 0o755)
		}
		t.Fatalf("unexpected uv call %v", uvArgs)
		return "", nil
	}
	t.Cleanup(func() { runToolCommand = previous })
	return &calls
}

func TestBuildPythonVenvCrossInstallsPackagesPerPlatform(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	calls := fakeUV(t, map[string]bool{"angr/x86_64-apple-darwin": true})

	entries, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"linux-arm64", "macos-arm64", "macos-x86_64"}})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, entry := range entries {
		got = append(got, entry.ID+":"+entry.RelativePath)
	}
	want := []string{
		"angr:runtime/python/linux-arm64/tools/angr", "sqlmap:runtime/python/linux-arm64/tools/sqlmap",
		"angr:runtime/python/macos-arm64/tools/angr", "sqlmap:runtime/python/macos-arm64/tools/sqlmap",
		"sqlmap:runtime/python/macos-x86_64/tools/sqlmap",
		"svalbard-python:runtime/python",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}

	wrapper, err := os.ReadFile(filepath.Join(root, "bin", "macos-arm64", "sqlmap"))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		`PY="$DRIVE/runtime/python/macos-arm64"`,
		`PYTHONPATH="$PY/tools/sqlmap"`,
		`exec "$PY/python/cpython-3.12.14-macos-aarch64-none/bin/python3.12" "$PY/tools/sqlmap/bin/sqlmap" "$@"`,
	} {
		if !strings.Contains(string(wrapper), part) {
			t.Errorf("wrapper missing %q:\n%s", part, wrapper)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "macos-x86_64", "angr")); !os.IsNotExist(err) {
		t.Errorf("angr wrapper written for platform without wheels: %v", err)
	}

	var sawMacArm bool
	for _, call := range *calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "--python-platform aarch64-apple-darwin") {
			sawMacArm = true
			if call[0] != "MACOSX_DEPLOYMENT_TARGET=14.0" {
				t.Errorf("macos-arm64 install without deployment target: %v", call)
			}
		}
		if strings.Contains(joined, "aarch64-manylinux_2_28") && strings.HasPrefix(call[0], "MACOSX") {
			t.Errorf("linux install got macOS env: %v", call)
		}
	}
	if !sawMacArm {
		t.Error("no pip install for aarch64-apple-darwin")
	}
	var sqlmapInstalls []string
	for _, call := range *calls {
		if slices.Contains(call, "sqlmap==1.10.9") && slices.Contains(call, "x86_64-apple-darwin") {
			sqlmapInstalls = append(sqlmapInstalls, strings.Join(call, " "))
		}
	}
	if len(sqlmapInstalls) != 1 || !strings.Contains(sqlmapInstalls[0], "pip install --no-build --target") {
		t.Errorf("sqlmap should install wheels-only in one attempt: %v", sqlmapInstalls)
	}

	// A rebuild with unchanged pins reuses interpreters and package trees.
	*calls = nil
	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"linux-arm64", "macos-arm64"}}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("rebuild ran uv: %v", *calls)
	}
}

func TestBuildPythonVenvFailsWhenPackageInstallsNowhere(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	fakeUV(t, map[string]bool{"angr/x86_64-apple-darwin": true})

	_, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"macos-x86_64"}, DesiredIDs: map[string]bool{"angr": true}})
	if err == nil || !strings.Contains(err.Error(), "angr could not be installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildPythonVenvPlatformIOPackageFailsForAnyRequestedTarget(t *testing.T) {
	root := t.TempDir()
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: 3.12\nstrategy: build\nbuild: {family: python-venv}\n")},
		"pio.yaml":    {Data: []byte("id: platformio-core\ntype: python-package\nvenv: svalbard-python\npackages: [platformio==6.2.0]\nentry_points: [pio, platformio]\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	fakeUV(t, map[string]bool{"platformio/x86_64-apple-darwin": true})
	_, err = buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"macos-arm64", "macos-x86_64"}, DesiredIDs: map[string]bool{"platformio-core": true}})
	if err == nil || !strings.Contains(err.Error(), "required target macos-x86_64") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlatformIOPythonTreeCompleteRequiresEntryPoints(t *testing.T) {
	dir := t.TempDir()
	if platformIOPythonTreeComplete(dir, []string{"pio"}) {
		t.Fatal("missing entry point accepted")
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "pio"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !platformIOPythonTreeComplete(dir, []string{"pio"}) {
		t.Fatal("complete PlatformIO tree rejected")
	}
}

func TestPythonVersionFromSpec(t *testing.T) {
	for spec, want := range map[string]string{"3.12": "3.12", ">=3.11": "3.11", "3.12.4": "3.12", "": "3.12"} {
		if got := pythonVersionFromSpec(spec); got != want {
			t.Errorf("pythonVersionFromSpec(%q) = %q, want %q", spec, got, want)
		}
	}
}
