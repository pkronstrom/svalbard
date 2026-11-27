package builder

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
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
// exist. unavailable["python"] fails interpreter installs, as offline. The
// unpack step extracts the rewritten archive. The build dir moves
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
				if unavailable["python"] {
					return "", errors.New("failed to download")
				}
				request := uvArgs[len(uvArgs)-1]
				version := strings.Split(request, "-")[1]
				build := strings.Replace(request, version, version+".14", 1)
				return "", writeTestTar(archive, []testTarEntry{
					{name: "./", dir: true},
					{name: "./.lock"},
					{name: "./.temp/", dir: true},
					{name: "./" + request, link: "/tmp/tmp.x/" + build},
					{name: "./" + build + "/", dir: true},
					{name: "./" + build + "/bin/", dir: true},
					{name: "./" + build + "/bin/python" + version, data: make([]byte, treeLinkCopyLimit+1), mode: 0o755},
					{name: "./" + build + "/bin/python3", link: "python" + version},
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
	if string(stamp) != "slimapp==1.0\nrequirements_from: requirements-slim.txt\npython: 3.12\n"+pythonTreeLayoutStamp {
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
			bytecodeLines(platform) +
			"PYTHONPATH=\"$PY/tools/py-docs\" PYTHONNOUSERSITE=1 exec \"$PY/" + pythons[platform] + "\" \"$@\"\n"
		if string(data) != want {
			t.Errorf("%s =\n%s\nwant\n%s", path, data, want)
		}
	}
}

// A Python command from the stick writes its bytecode to this computer's
// cache dir, never into the tree on the stick.
func TestPythonCommandKeepsBytecodeOffTheStick(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newLauncherTestCatalog(t)
	fakeUV(t, nil)
	platforms := []string{"linux-arm64", "macos-arm64"}
	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: platforms}); err != nil {
		t.Fatal(err)
	}
	if PythonTreesNeedRepair(root, platforms) {
		t.Fatal("fresh commands reported as needing repair")
	}
	interpreters := map[string]string{
		"linux-arm64": "python/cpython-3.12.14-linux-aarch64-gnu/bin/python3.12",
		"macos-arm64": "python/cpython-3.12.14-macos-aarch64-none/bin/python3.12",
	}
	for _, platform := range platforms {
		// The interpreter prints what it was given.
		fake := filepath.Join(root, "runtime", "python", platform, interpreters[platform])
		if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"prefix=${PYTHONPYCACHEPREFIX:-} nowrite=${PYTHONDONTWRITEBYTECODE:-}\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		home := map[string]string{"linux-arm64": "/home/u/.cache", "macos-arm64": "/home/u/Library/Caches"}[platform]
		cases := []struct {
			env  []string
			want string
		}{
			{[]string{"HOME=/home/u"}, "prefix=" + home + "/svalbard/pycache nowrite="},
			{[]string{"HOME=/home/u", "PYTHONPYCACHEPREFIX=/set/by/caller"}, "prefix=/set/by/caller nowrite="},
			{[]string{"HOME=/home/u", "PYTHONDONTWRITEBYTECODE=1"}, "prefix= nowrite=1"},
			{nil, "prefix= nowrite=1"},
		}
		if platform == "linux-arm64" {
			cases = append(cases,
				struct {
					env  []string
					want string
				}{[]string{"XDG_CACHE_HOME=/xdg"}, "prefix=/xdg/svalbard/pycache nowrite="},
				struct {
					env  []string
					want string
				}{[]string{"HOME=/home/u", "XDG_CACHE_HOME=relative"}, "prefix= nowrite=1"})
		} else {
			cases = append(cases, struct {
				env  []string
				want string
			}{[]string{"HOME=/tmp/u"}, "prefix= nowrite=1"})
		}
		for _, tc := range cases {
			if platform == "linux-arm64" && len(tc.env) == 1 && tc.env[0] == "HOME=/home/u" {
				tc.env = append(tc.env, "XDG_CACHE_HOME=")
			}
			cmd := exec.Command("/bin/sh", filepath.Join(root, "bin", platform, "py-docs"))
			cmd.Env = append([]string{"PATH=/usr/bin:/bin"}, tc.env...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s %v: %v\n%s", platform, tc.env, err, out)
			}
			if got := strings.TrimSpace(string(out)); got != tc.want {
				t.Errorf("%s %v: %q, want %q", platform, tc.env, got, tc.want)
			}
		}
	}

	// Commands written before this read as needing the venv build again.
	launcher := filepath.Join(root, "bin", "macos-arm64", "py-docs")
	old := "#!/bin/sh\nunset PYTHONHOME\nPY=\"$DRIVE/runtime/python/macos-arm64\"\nPYTHONPATH=\"$PY/tools/py-docs\" exec \"$PY/python\" \"$@\"\n"
	if err := os.WriteFile(launcher, []byte(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if !PythonTreesNeedRepair(root, platforms) {
		t.Fatal("a command that writes bytecode onto the stick was not reported")
	}

	// The build brings every command up to date, also one of a package the
	// catalog no longer has, so the repair is not asked for again.
	withdrawn := filepath.Join(root, "bin", "linux-arm64", "gone")
	if err := os.WriteFile(withdrawn, []byte("#!/bin/sh\nPY=\"$DRIVE/runtime/python/linux-arm64\"\nPYTHONPATH=\"$PY/tools/gone\" exec \"$PY/python\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := buildPythonVenv(root, recipe, cat, Options{Platforms: platforms}); err != nil {
		t.Fatal(err)
	}
	if PythonTreesNeedRepair(root, platforms) {
		t.Fatal("repair still asked for after the build")
	}
	if data, _ := os.ReadFile(withdrawn); !strings.Contains(string(data), bytecodeLines("linux-arm64")+"PYTHONPATH=") {
		t.Fatalf("withdrawn package's command not upgraded:\n%s", data)
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

// An older CLI's trees: an interpreter with a python3 symlink and no layout
// stamp, and a package stamp without the layout line.
func writeOldLayoutTrees(t *testing.T, root string) {
	t.Helper()
	platformDir := filepath.Join(root, "runtime", "python", "linux-arm64")
	bin := filepath.Join(platformDir, "python", "cpython-3.12.14-linux-aarch64-gnu", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "python3.12"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("python3.12", filepath.Join(bin, "python3")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(platformDir, "tools", "sqlmap"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(platformDir, "tools", "sqlmap.packages"), []byte("sqlmap==1.10.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPythonVenvReinstallsTreesOfAnOlderLayout(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	calls := fakeUV(t, nil)
	writeOldLayoutTrees(t, root)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
	if !PythonTreesNeedRepair(root, opts.Platforms) {
		t.Fatal("old layout not detected")
	}

	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	var ran []string
	for _, call := range *calls {
		ran = append(ran, strings.Join(call[1:3], " "))
	}
	if !slices.Equal(ran, []string{"python install", "pip install"}) {
		t.Errorf("uv ran %v, want the interpreter and the package installed again", ran)
	}
	python3 := filepath.Join(root, "runtime", "python", "linux-arm64", "python", "cpython-3.12.14-linux-aarch64-gnu", "bin", "python3")
	if info, err := os.Lstat(python3); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("python3 still a symlink: %v %v", info, err)
	}
	if PythonTreesNeedRepair(root, opts.Platforms) {
		t.Error("repaired vault still needs repair")
	}
	*calls = nil
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("second build ran uv: %v", *calls)
	}
}

func TestPythonTreesNeedRepair(t *testing.T) {
	platforms := []string{"linux-arm64"}
	if root := t.TempDir(); PythonTreesNeedRepair(root, platforms) {
		t.Error("an empty vault needs repair")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".staging", "uv-cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !PythonTreesNeedRepair(root, platforms) {
		t.Error("a uv cache on the vault needs no repair")
	}
	// A platform the vault no longer targets, and a removed package's stamp,
	// never need it: no build would change them.
	root = t.TempDir()
	writeOldLayoutTrees(t, root)
	if PythonTreesNeedRepair(root, []string{"macos-arm64"}) {
		t.Error("a platform the vault does not target needs repair")
	}
	if err := os.WriteFile(filepath.Join(root, "runtime", "python", "linux-arm64", "python.layout"), []byte(pythonTreeLayoutStamp), 0o644); err != nil {
		t.Fatal(err)
	}
	if !PythonTreesNeedRepair(root, platforms) {
		t.Error("a package stamp without the layout needs no repair")
	}
	if err := os.Remove(filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap")); err != nil {
		t.Fatal(err)
	}
	if PythonTreesNeedRepair(root, platforms) {
		t.Error("a removed package's stamp needs repair")
	}
}

func TestBuildPythonVenvKeepsOlderTreesWhenTheirRepairFails(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	// Offline: nothing installs.
	calls := fakeUV(t, map[string]bool{"python": true, "sqlmap/aarch64-manylinux_2_28": true})
	writeOldLayoutTrees(t, root)
	var events []BuildEvent
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}, OnEvent: func(e BuildEvent) { events = append(events, e) }}

	entries, err := buildPythonVenv(root, recipe, cat, opts)
	if err != nil {
		t.Fatalf("a failed repair failed the build: %v", err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	if !slices.Equal(ids, []string{"sqlmap", "svalbard-python"}) {
		t.Errorf("entries = %v, want the kept trees", ids)
	}
	// One attempt: after the interpreter's repair failed, the package's is
	// not tried.
	if len(*calls) != 1 || !slices.Contains((*calls)[0], "python") {
		t.Errorf("uv ran %v, want one interpreter attempt", *calls)
	}
	platformDir := filepath.Join(root, "runtime", "python", "linux-arm64")
	if info, err := os.Lstat(filepath.Join(platformDir, "python", "cpython-3.12.14-linux-aarch64-gnu", "bin", "python3")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("old interpreter not kept: %v %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(platformDir, "python.layout")); !os.IsNotExist(err) {
		t.Errorf("layout stamp written for a kept tree: %v", err)
	}
	if !PythonTreesNeedRepair(root, opts.Platforms) {
		t.Error("kept trees no longer marked for repair")
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "linux-arm64", "sqlmap")); err != nil {
		t.Errorf("kept package lost its wrapper: %v", err)
	}
	var notices []string
	for _, e := range events {
		if e.State == EventLog {
			notices = append(notices, e.Message)
		}
	}
	if !slices.Equal(notices, []string{"Python trees use an older layout; the next apply with network will rebuild them"}) {
		t.Errorf("notices = %q, want one line", notices)
	}
}

func TestBuildPythonVenvRepairOfAPackageAloneFailsSoftly(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	fakeUV(t, map[string]bool{"sqlmap/aarch64-manylinux_2_28": true})
	writeOldLayoutTrees(t, root)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatalf("a failed package repair failed the build: %v", err)
	}
	stamp, _ := os.ReadFile(filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap.packages"))
	if string(stamp) != "sqlmap==1.10.9\n" {
		t.Errorf("stamp = %q, want the old one kept", stamp)
	}
}

func TestBuildPythonVenvStillFailsForMissingTrees(t *testing.T) {
	cat, recipe := newPythonTestCatalog(t)
	fakeUV(t, map[string]bool{"python": true})
	_, err := buildPythonVenv(t.TempDir(), recipe, cat, Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}})
	if err == nil || !strings.Contains(err.Error(), "installing python for linux-arm64") {
		t.Errorf("err = %v, want a missing interpreter to fail", err)
	}

	// A package whose pins changed is a new install, not a repair.
	root := t.TempDir()
	launcherCat, launcherRecipe := newLauncherTestCatalog(t)
	fakeUV(t, map[string]bool{"docpkg/aarch64-manylinux_2_28": true})
	writeOldLayoutTrees(t, root)
	toolDir := filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "py-docs")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolDir+".packages", []byte("docpkg==0.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = buildPythonVenv(root, launcherRecipe, launcherCat, Options{Platforms: []string{"linux-arm64"}})
	if err == nil || !strings.Contains(err.Error(), "py-docs") {
		t.Errorf("err = %v, want changed pins that cannot install to fail", err)
	}
}

// pipInstalls returns the packages uv pip-installed, as package/platform.
func pipInstalls(calls [][]string) []string {
	var got []string
	for _, call := range calls {
		if !slices.Contains(call, "pip") || slices.Contains(call, "-r") {
			continue
		}
		at := slices.Index(call, "--python-platform")
		got = append(got, strings.Split(call[len(call)-1], "==")[0]+"/"+call[at+1])
	}
	return slices.Compact(got)
}

// Stamps the builder wrote before it recorded the interpreter version, and
// before it recorded the layout, are recognised: the first is current beside
// the one interpreter it was built for, the second is reinstalled once.
func TestBuildPythonVenvTrustsStampsFromBeforeVersions(t *testing.T) {
	for _, tc := range []struct {
		name, stamp  string
		otherPython  bool
		wantInstalls []string
	}{
		{name: "layout, no version", stamp: "sqlmap==1.10.9\n" + pythonTreeLayoutStamp},
		{name: "version, no layout", stamp: "sqlmap==1.10.9\npython: 3.12\n", wantInstalls: []string{"sqlmap/aarch64-manylinux_2_28"}},
		{name: "no version beside two interpreters", stamp: "sqlmap==1.10.9\n" + pythonTreeLayoutStamp, otherPython: true, wantInstalls: []string{"sqlmap/aarch64-manylinux_2_28"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cat, recipe := newPythonTestCatalog(t)
			calls := fakeUV(t, nil)
			opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
			if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
				t.Fatal(err)
			}
			platformDir := filepath.Join(root, "runtime", "python", "linux-arm64")
			if err := os.WriteFile(filepath.Join(platformDir, "tools", "sqlmap.packages"), []byte(tc.stamp), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.otherPython {
				if err := os.MkdirAll(filepath.Join(platformDir, "python", "cpython-3.11.9-linux-aarch64-gnu"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			*calls = nil
			if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
				t.Fatal(err)
			}
			if got := pipInstalls(*calls); !slices.Equal(got, tc.wantInstalls) {
				t.Fatalf("installs = %v, want %v", got, tc.wantInstalls)
			}
			if tc.otherPython {
				return
			}
			*calls = nil
			if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
				t.Fatal(err)
			}
			if len(*calls) != 0 || PythonTreesNeedRepair(root, opts.Platforms) {
				t.Fatalf("the vault should be current after one build: %v", *calls)
			}
		})
	}
}

func TestBuildPythonVenvRebuildsRequestedPackageOnly(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	calls := fakeUV(t, nil)
	opts := Options{Platforms: []string{"linux-arm64"}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	*calls = nil
	opts.Rebuild = map[string]bool{"sqlmap": true}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	if got := pipInstalls(*calls); !slices.Equal(got, []string{"sqlmap/aarch64-manylinux_2_28"}) {
		t.Fatalf("an update of sqlmap with unchanged pins should reinstall sqlmap alone, got %v", got)
	}
}

func TestBuildPythonVenvFailedUpdateKeepsInstalledTree(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	unavailable := map[string]bool{}
	fakeUV(t, unavailable)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	unavailable["sqlmap/aarch64-manylinux_2_28"] = true
	opts.Rebuild = map[string]bool{"sqlmap": true}
	_, err := buildPythonVenv(root, recipe, cat, opts)
	if err == nil || !strings.Contains(err.Error(), "installed copy was kept") {
		t.Fatalf("err = %v, want a failed update that keeps the installed copy", err)
	}
	for _, path := range []string{
		filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap", "bin", "sqlmap"),
		filepath.Join(root, "bin", "linux-arm64", "sqlmap"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("failed update removed %s: %v", path, err)
		}
	}
}

func TestBuildPythonVenvVersionChangeRebuildsTreesAndDropsOldPython(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	calls := fakeUV(t, nil)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	*calls = nil
	recipe.Python = "3.13"
	opts.Rebuild = map[string]bool{recipe.ID: true}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	if got := pipInstalls(*calls); !slices.Equal(got, []string{"sqlmap/aarch64-manylinux_2_28"}) {
		t.Fatalf("a new interpreter should reinstall the trees, got %v", got)
	}
	pythons, _ := filepath.Glob(filepath.Join(root, "runtime", "python", "linux-arm64", "python", "cpython-*"))
	for _, dir := range pythons {
		if strings.Contains(filepath.Base(dir), "3.12") {
			t.Errorf("the 3.12 interpreter should be gone: %s", dir)
		}
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "bin", "linux-arm64", "sqlmap"))
	if err != nil || !strings.Contains(string(wrapper), "python3.13") {
		t.Fatalf("wrapper should run python3.13: %s %v", wrapper, err)
	}
}

// An update the user asked for is tried even after an older-layout repair
// failed, and its failure fails the build where the repair's did not.
func TestBuildPythonVenvUpdateIsTriedAfterADeferredRepair(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	unavailable := map[string]bool{"python": true}
	calls := fakeUV(t, unavailable)
	writeOldLayoutTrees(t, root)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}, Rebuild: map[string]bool{"sqlmap": true}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatalf("an update that installed failed the build: %v", err)
	}
	if got := pipInstalls(*calls); !slices.Equal(got, []string{"sqlmap/aarch64-manylinux_2_28"}) {
		t.Fatalf("installs = %v, want the requested update", got)
	}
	stamp, _ := os.ReadFile(filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap.packages"))
	if string(stamp) != "sqlmap==1.10.9\npython: 3.12\n"+pythonTreeLayoutStamp {
		t.Errorf("stamp = %q, want the current layout and version", stamp)
	}

	// The interpreter still has its older layout; the package too again.
	if err := os.WriteFile(filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap.packages"), []byte("sqlmap==1.10.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unavailable["sqlmap/aarch64-manylinux_2_28"] = true
	_, err := buildPythonVenv(root, recipe, cat, opts)
	if err == nil || !strings.Contains(err.Error(), "installed copy was kept") {
		t.Fatalf("err = %v, want the failed update to fail the build", err)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime", "python", "linux-arm64", "tools", "sqlmap", "bin", "sqlmap")); err != nil {
		t.Errorf("failed update lost the installed tree: %v", err)
	}
}

// A new Python version is installed beside the old one, so a tool whose
// update to it fails keeps a tree and an interpreter that still run.
func TestBuildPythonVenvFailedVersionChangeKeepsOldPython(t *testing.T) {
	root := t.TempDir()
	cat, recipe := newPythonTestCatalog(t)
	unavailable := map[string]bool{}
	fakeUV(t, unavailable)
	opts := Options{Platforms: []string{"linux-arm64"}, DesiredIDs: map[string]bool{"sqlmap": true}}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err != nil {
		t.Fatal(err)
	}
	unavailable["sqlmap/aarch64-manylinux_2_28"] = true
	recipe.Python = "3.13"
	opts.Rebuild = map[string]bool{recipe.ID: true}
	if _, err := buildPythonVenv(root, recipe, cat, opts); err == nil || !strings.Contains(err.Error(), "installed copy was kept") {
		t.Fatalf("err = %v, want a failed update", err)
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "bin", "linux-arm64", "sqlmap"))
	if err != nil {
		t.Fatal(err)
	}
	old := "python/cpython-3.12.14-linux-aarch64-gnu/bin/python3.12"
	if !strings.Contains(string(wrapper), old) {
		t.Fatalf("wrapper should still run 3.12:\n%s", wrapper)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime", "python", "linux-arm64", old)); err != nil {
		t.Errorf("the 3.12 interpreter the kept tool runs on is gone: %v", err)
	}
}
