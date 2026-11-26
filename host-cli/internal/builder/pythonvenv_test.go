package builder

import (
	"context"
	"encoding/json"
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

// fakeUV emulates the tools container. Each install records its uv commands
// (environment first, {dir} unreplaced) and archives its result: an
// interpreter install archives a python-build-standalone install dir (alias
// link, uv's dot files, a terminfo pair that differs only in case, python3
// linking a large binary); pip installs fill a --target dir unless the
// package/platform pair is listed in unavailable. angr needs an sdist build,
// so its wheels-only attempt fails. A --no-deps install ships
// requirements-slim.txt in the tree, and a -r install needs its file to
// exist. The unpack step extracts the rewritten archive. The build dir moves
// to a temporary dir.
func fakeUV(t *testing.T, unavailable map[string]bool) *[][]string {
	needsBuild := map[string]bool{"angr": true}
	t.Helper()
	buildDir := t.TempDir()
	previousDir := pythonBuildDir
	pythonBuildDir = func() (string, error) { return buildDir, nil }
	var calls [][]string
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool != "python3" || len(args) != 4 {
			t.Fatalf("unexpected tool call %s %v", tool, args)
		}
		switch args[1] {
		case containerUnpackScript:
			return "", extractTestTar(args[2], args[3])
		case containerInstallScript:
		default:
			t.Fatalf("unexpected script %q", args[1])
		}
		var spec struct {
			Env      []string
			Commands [][]string
		}
		if err := json.Unmarshal([]byte(args[2]), &spec); err != nil {
			t.Fatal(err)
		}
		archive, dir := args[3], t.TempDir()
		for _, command := range spec.Commands {
			calls = append(calls, append(slices.Clone(spec.Env), command...))
			uvArgs := make([]string, len(command)-1)
			for i, arg := range command[1:] {
				uvArgs[i] = strings.ReplaceAll(arg, "{dir}", dir)
			}
			flag := func(name string) string { return uvArgs[slices.Index(uvArgs, name)+1] }
			switch strings.Join(uvArgs[:2], " ") {
			case "python install":
				request := uvArgs[len(uvArgs)-1]
				build := strings.Replace(request, "3.12", "3.12.14", 1)
				return "", writeTestTar(archive, []testTarEntry{
					{name: "./", dir: true},
					{name: "./.lock"},
					{name: "./.temp/", dir: true},
					{name: "./" + request, link: "/tmp/tmp.x/" + build},
					{name: "./" + build + "/", dir: true},
					{name: "./" + build + "/bin/", dir: true},
					{name: "./" + build + "/bin/python3.12", data: make([]byte, treeLinkCopyLimit+1), mode: 0o755},
					{name: "./" + build + "/bin/python3", link: "python3.12"},
					{name: "./" + build + "/share/terminfo/h/hp2621", data: []byte("ti")},
					{name: "./" + build + "/share/terminfo/2/2621A", link: "../h/hp2621"},
					{name: "./" + build + "/share/terminfo/2/2621a", link: "../h/hp2621"},
				})
			case "pip install":
				if at := slices.Index(uvArgs, "-r"); at >= 0 {
					if _, err := os.Stat(uvArgs[at+1]); err != nil {
						return "", err
					}
					if err := os.WriteFile(filepath.Join(flag("--target"), "deps-installed"), nil, 0o644); err != nil {
						return "", err
					}
					continue
				}
				pkg := strings.Split(uvArgs[len(uvArgs)-1], "==")[0]
				if unavailable[pkg+"/"+flag("--python-platform")] || (needsBuild[pkg] && slices.Contains(uvArgs, "--no-build")) {
					return "", errors.New("no solution found")
				}
				target := flag("--target")
				if err := os.MkdirAll(filepath.Join(target, "bin"), 0o755); err != nil {
					return "", err
				}
				if slices.Contains(uvArgs, "--no-deps") {
					if err := os.WriteFile(filepath.Join(target, "requirements-slim.txt"), []byte("dep==1.0\n"), 0o644); err != nil {
						return "", err
					}
				}
				if err := os.WriteFile(filepath.Join(target, "bin", pkg), []byte("#!/usr/local/bin/python\n"), 0o755); err != nil {
					return "", err
				}
			default:
				t.Fatalf("unexpected uv call %v", uvArgs)
			}
		}
		return "", tarTestDir(dir, archive)
	}
	t.Cleanup(func() {
		runToolCommand = previous
		pythonBuildDir = previousDir
	})
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

func TestBuildPythonVenvInstallsShippedRequirements(t *testing.T) {
	root := t.TempDir()
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nbuild: {family: python-venv}\n")},
		"slim.yaml":   {Data: []byte("id: slimapp\ntype: python-package\nvenv: svalbard-python\npackages: [slimapp==1.0]\nrequirements_from: requirements-slim.txt\nentry_points: [slimapp]\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	calls := fakeUV(t, nil)

	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"linux-arm64"}}); err != nil {
		t.Fatal(err)
	}

	var installs []string
	for _, call := range *calls {
		if joined := strings.Join(call, " "); strings.Contains(joined, "pip install") {
			installs = append(installs, joined)
		}
	}
	toolDir := filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "slimapp")
	if len(installs) != 2 {
		t.Fatalf("pip installs = %v, want package then requirements", installs)
	}
	if !strings.Contains(installs[0], "pip install --no-build --target") || !strings.HasSuffix(installs[0], "--no-deps slimapp==1.0") {
		t.Errorf("package install = %q, want wheels-only without dependencies", installs[0])
	}
	if !strings.HasSuffix(installs[1], "-r {dir}/requirements-slim.txt") || strings.Contains(installs[1], "--no-deps") || !strings.Contains(installs[1], "--no-build") {
		t.Errorf("requirements install = %q, want the shipped file with dependencies", installs[1])
	}
	if _, err := os.Stat(filepath.Join(toolDir, "deps-installed")); err != nil {
		t.Errorf("requirements were not installed into the package tree: %v", err)
	}
	stamp, err := os.ReadFile(toolDir + ".packages")
	if err != nil {
		t.Fatal(err)
	}
	if string(stamp) != "slimapp==1.0\nrequirements_from: requirements-slim.txt\n" {
		t.Errorf("stamp = %q", stamp)
	}

	*calls = nil
	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"linux-arm64"}}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("rebuild ran uv: %v", *calls)
	}
}

func TestBuildPythonVenvRejectsRequirementsOutsideTree(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nbuild: {family: python-venv}\n")},
		"slim.yaml":   {Data: []byte("id: slimapp\ntype: python-package\nvenv: svalbard-python\npackages: [slimapp==1.0]\nrequirements_from: ../requirements.txt\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	fakeUV(t, nil)

	_, err = buildPythonVenv(t.TempDir(), recipe, cat, Options{Platforms: []string{"linux-arm64"}})
	if err == nil || !strings.Contains(err.Error(), "must be a path inside the package tree") {
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

func newLauncherTestCatalog(t *testing.T) (*catalog.Catalog, catalog.Item) {
	t.Helper()
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nstrategy: build\nbuild: {family: python-venv}\n")},
		"docs.yaml":   {Data: []byte("id: py-docs\ntype: python-package\nvenv: svalbard-python\npackages: [docpkg==1.0]\npython_launcher: py-docs\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := cat.RecipeByID("svalbard-python")
	return cat, recipe
}

func TestPythonLauncherWrapper(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newLauncherTestCatalog(t)
	fakeUV(t, nil)
	platforms := []string{"linux-arm64", "macos-arm64"}
	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: platforms}); err != nil {
		t.Fatal(err)
	}
	pythons := map[string]string{
		"linux-arm64": "python/cpython-3.12.14-linux-aarch64-gnu/bin/python3.12",
		"macos-arm64": "python/cpython-3.12.14-macos-aarch64-none/bin/python3.12",
	}
	for _, platform := range platforms {
		path := filepath.Join(root, "bin", platform, "py-docs")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("%s mode = %v, want 0755", path, info.Mode().Perm())
		}
		data, _ := os.ReadFile(path)
		want := "#!/bin/sh\n" +
			"unset PYTHONHOME\n" +
			"DRIVE=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/../..\" && pwd)\"\n" +
			"PY=\"$DRIVE/runtime/python/" + platform + "\"\n" +
			"PYTHONPATH=\"$PY/tools/py-docs\" PYTHONNOUSERSITE=1 exec \"$PY/" + pythons[platform] + "\" \"$@\"\n"
		if string(data) != want {
			t.Errorf("%s =\n%s\nwant\n%s", path, data, want)
		}
	}
}

func TestPythonLauncherWithoutEntryPoints(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newLauncherTestCatalog(t)
	fakeUV(t, nil)
	entries, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"macos-arm64"}, DesiredIDs: map[string]bool{"py-docs": true}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID+":"+filepath.ToSlash(e.RelativePath))
	}
	want := []string{"py-docs:runtime/python/macos-arm64/tools/py-docs", "svalbard-python:runtime/python"}
	if !slices.Equal(ids, want) {
		t.Fatalf("entries = %v, want %v", ids, want)
	}
}

func TestPythonLauncherFailsWithoutWheels(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newLauncherTestCatalog(t)
	fakeUV(t, map[string]bool{"docpkg/aarch64-manylinux_2_28": true})
	_, err := buildPythonVenv(root, recipe, cat, Options{Platforms: []string{"macos-arm64", "linux-arm64"}})
	if err == nil || !strings.Contains(err.Error(), "py-docs") || !strings.Contains(err.Error(), "linux-arm64") {
		t.Fatalf("err = %v, want a failure naming py-docs and linux-arm64", err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "linux-arm64", "py-docs")); !os.IsNotExist(err) {
		t.Errorf("launcher written for a platform without wheels: %v", err)
	}
}
