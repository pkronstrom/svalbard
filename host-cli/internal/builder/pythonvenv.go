package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// pythonTarget maps a svalbard platform to what uv needs to provision it
// without running anything on that platform.
type pythonTarget struct {
	uvPlatform string // uv pip --python-platform (wheel tag selection)
	python     string // uv python install request suffix (python-build-standalone build)
	macosMin   string // MACOSX_DEPLOYMENT_TARGET; raises the accepted macOS wheel tag
}

// Linux targets glibc 2.28 (Debian 10+, Raspberry Pi OS bookworm). macOS arm64
// targets 14.0 because angr's native deps only ship macosx_14_0_arm64 wheels.
var pythonTargets = map[string]pythonTarget{
	"linux-x86_64": {uvPlatform: "x86_64-manylinux_2_28", python: "linux-x86_64-gnu"},
	"linux-arm64":  {uvPlatform: "aarch64-manylinux_2_28", python: "linux-aarch64-gnu"},
	"macos-arm64":  {uvPlatform: "aarch64-apple-darwin", python: "macos-aarch64-none", macosMin: "14.0"},
	"macos-x86_64": {uvPlatform: "x86_64-apple-darwin", python: "macos-x86_64-none", macosMin: "13.0"},
}

// buildPythonVenv provisions Python tools for all python-package recipes that
// reference this venv. uv runs in the tools image and cross-targets every
// platform, so nothing is executed for a foreign platform:
//  1. Installs a relocatable python-build-standalone interpreter per platform,
//     rewritten to hold no symlinks or case-only name differences (exFAT)
//  2. Installs each package with `uv pip install --target` into its own dir
//     (both through installTree, which keeps symlinks off the vault and ._
//     files off exFAT), resolving wheels for that platform's tags; a recipe with
//     requirements_from installs the package without dependencies, then the
//     requirements file the package ships
//  3. Generates wrapper scripts in bin/<platform>/ that run the entry point
//     with the drive's interpreter and the package dir on PYTHONPATH
//
// A package that cannot be installed for a platform (no compatible wheels) is
// skipped for that platform with a warning; it fails the build only when no
// platform could install it, or when it has a python_launcher (scripts call
// that on every platform).
func buildPythonVenv(root string, recipe catalog.Item, cat *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Collect python-package recipes that reference this venv AND are in the
	// user's desired set. This prevents installing packages the user didn't select.
	var pkgItems []catalog.Item
	for _, item := range cat.AllRecipes() {
		if item.Type == "python-package" && item.Venv == recipe.ID {
			// Only include if user selected this package (or no filter provided).
			if len(opts.DesiredIDs) > 0 && !opts.DesiredIDs[item.ID] {
				continue
			}
			pkgItems = append(pkgItems, item)
		}
	}

	venvEntry := manifest.RealizedEntry{
		ID:             recipe.ID,
		Type:           recipe.Type,
		Filename:       recipe.ID,
		RelativePath:   "runtime/python",
		SourceStrategy: "build",
	}
	targets := opts.Platforms
	if len(targets) == 0 {
		targets = []string{hostPlatformStr()}
	}
	if len(pkgItems) == 0 {
		for _, platform := range targets {
			if err := upgradePythonCommands(filepath.Join(root, "bin", platform), platform); err != nil {
				return nil, err
			}
		}
		return []manifest.RealizedEntry{venvEntry}, nil
	}

	version := pythonVersionFromSpec(recipe.Python)

	workdir, err := pythonBuildDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(workdir, "uv-cache"), 0o755); err != nil {
		return nil, err
	}

	var entries []manifest.RealizedEntry
	installed := map[string]bool{}
	// A tree of an older layout that cannot be installed again (offline) is
	// kept and the build goes on; after one such failure the others are not
	// tried, since they would fail the same way.
	repairDeferred := false

	for _, platform := range targets {
		target, ok := pythonTargets[platform]
		if !ok {
			if hasPlatformIOPythonPackage(pkgItems) {
				return nil, fmt.Errorf("PlatformIO Python closure is unsupported on requested target %s; reprovision with supported targets", platform)
			}
			slog.Warn("python tools not supported on platform, skipping", "platform", platform)
			continue
		}
		platformDir := filepath.Join(root, "runtime", "python", platform)
		interpreterDir := filepath.Join(platformDir, "python")
		binDir := filepath.Join(root, "bin", platform)
		for _, dir := range []string{binDir, filepath.Join(platformDir, "tools")} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}

		// Phase 1: Install the interpreter for this platform, also over one an
		// older layout left (symlinks, terminfo).
		pythonBin := findInstalledPython(interpreterDir, version)
		repair := pythonBin != "" && !pythonLayoutCurrent(interpreterDir+".layout")
		if pythonBin == "" || (repair && !repairDeferred) {
			slog.Info("installing python", "version", version, "platform", platform)
			emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventProgress, Message: "installing python " + version + " for " + platform})
			request := "cpython-" + version + "-" + target.python
			switch err := installInterpreter(ctx, root, workdir, request, interpreterDir); {
			case err != nil && repair:
				slog.Warn("kept python interpreter of an older layout", "platform", platform, "err", truncate(err.Error(), 300))
				repairDeferred = true
			case err != nil:
				return nil, fmt.Errorf("installing python for %s: %w", platform, err)
			default:
				if pythonBin = findInstalledPython(interpreterDir, version); pythonBin == "" {
					return nil, fmt.Errorf("could not find installed python in %s", interpreterDir)
				}
				if err := os.WriteFile(interpreterDir+".layout", []byte(pythonTreeLayoutStamp), 0o644); err != nil {
					return nil, err
				}
			}
		}
		pythonRel, err := filepath.Rel(platformDir, pythonBin)
		if err != nil {
			return nil, err
		}

		for _, pkg := range pkgItems {
			// Phase 2: Install the package tree, unless an identical one exists.
			// The stamp sits beside the tree, not inside it, so the host never
			// writes into a directory the container created.
			toolDir := filepath.Join(platformDir, "tools", pkg.ID)
			stamp := strings.Join(pkg.Packages, "\n") + "\n"
			if pkg.RequirementsFrom != "" {
				if !filepath.IsLocal(pkg.RequirementsFrom) {
					return nil, fmt.Errorf("%s: requirements_from %q must be a path inside the package tree", pkg.ID, pkg.RequirementsFrom)
				}
				stamp += "requirements_from: " + pkg.RequirementsFrom + "\n"
			}
			stamp += pythonTreeLayoutStamp
			stampPath := toolDir + ".packages"
			existing, stampErr := os.ReadFile(stampPath)
			rebuild := stampErr != nil || string(existing) != stamp
			// The same pins installed in an older layout.
			repair := rebuild && stampErr == nil && stampWithoutLayout(string(existing)) == stampWithoutLayout(stamp)
			if isPlatformIOPythonPackage(pkg) && !platformIOPythonTreeComplete(toolDir, pkg.EntryPoints) {
				rebuild, repair = true, false
			}
			if info, err := os.Stat(toolDir); err != nil || !info.IsDir() {
				repair = false
			}
			if repair && repairDeferred {
				rebuild = false
			}
			if rebuild {
				slog.Info("installing python package", "package", pkg.ID, "platform", platform)
				emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventProgress, Message: "installing " + pkg.ID + " for " + platform})
				var env []string
				if target.macosMin != "" {
					env = append(env, "MACOSX_DEPLOYMENT_TARGET="+target.macosMin)
				}
				install := func(flags ...string) error {
					args := append([]string{"uv", "pip", "install"}, flags...)
					args = append(args,
						"--target", "{dir}",
						"--python-platform", target.uvPlatform,
						"--python-version", version,
						"--cache-dir", containerUVCache,
						"--link-mode", "copy",
					)
					commands := [][]string{append(slices.Clone(args), pkg.Packages...)}
					if pkg.RequirementsFrom != "" {
						// The requirements file only exists once the package
						// that ships it is in the tree.
						commands = [][]string{
							append(append(slices.Clone(args), "--no-deps"), pkg.Packages...),
							append(args, "-r", "{dir}/"+filepath.ToSlash(pkg.RequirementsFrom)),
						}
					}
					return installTree(ctx, root, workdir, env, commands, nil, toolDir)
				}
				// Wheels only first: with builds allowed uv picks the newest
				// version even when only its sdist exists, and a native sdist
				// cannot be built for a foreign platform. Retry with builds for
				// pure-python sdists (uv rejects built wheels that don't match).
				err := install("--no-build")
				if err != nil {
					err = install()
				}
				switch {
				case err != nil && repair:
					slog.Warn("kept python package tree of an older layout", "package", pkg.ID, "platform", platform, "err", truncate(err.Error(), 300))
					repairDeferred = true
				case err != nil:
					if isPlatformIOPythonPackage(pkg) {
						return nil, fmt.Errorf("PlatformIO Python package %s could not be installed for required target %s: %w", pkg.ID, platform, err)
					}
					if pkg.PythonLauncher != "" {
						// Skills call the launcher on whichever platform the
						// drive runs; a gap would leave them broken there.
						return nil, fmt.Errorf("python package %s has no compatible wheels for %s: %w", pkg.ID, platform, err)
					}
					slog.Warn("python package unavailable for platform, skipping", "package", pkg.ID, "platform", platform, "err", truncate(err.Error(), 300))
					emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventLog, Message: fmt.Sprintf("skipping %s for %s: no compatible wheels", pkg.ID, platform)})
					for _, ep := range pkg.EntryPoints {
						_ = os.Remove(filepath.Join(binDir, ep))
					}
					continue
				default:
					if err := os.WriteFile(stampPath, []byte(stamp), 0o644); err != nil {
						return nil, err
					}
				}
			}

			// Phase 3: Generate wrapper scripts. Console scripts written by
			// `--target` carry the build container's shebang, so run them
			// through the drive's interpreter instead.
			for _, ep := range pkg.EntryPoints {
				wrapper := filepath.Join(binDir, ep)
				script := fmt.Sprintf(
					"#!/bin/sh\nDRIVE=\"$(cd \"$(dirname \"$0\")/../..\" && pwd)\"\nPY=\"$DRIVE/runtime/python/%s\"\n%sPYTHONPATH=\"$PY/tools/%s\" PYTHONNOUSERSITE=1 exec \"$PY/%s\" \"$PY/tools/%s/bin/%s\" \"$@\"\n",
					platform, bytecodeLines(platform), pkg.ID, filepath.ToSlash(pythonRel), pkg.ID, ep,
				)
				if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
					return nil, fmt.Errorf("writing wrapper for %s: %w", ep, err)
				}
			}
			if pkg.PythonLauncher != "" {
				launcher := filepath.Join(binDir, pkg.PythonLauncher)
				script := fmt.Sprintf(
					"#!/bin/sh\nunset PYTHONHOME\nDRIVE=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/../..\" && pwd)\"\nPY=\"$DRIVE/runtime/python/%s\"\n%sPYTHONPATH=\"$PY/tools/%s\" PYTHONNOUSERSITE=1 exec \"$PY/%s\" \"$@\"\n",
					platform, bytecodeLines(platform), pkg.ID, filepath.ToSlash(pythonRel),
				)
				if err := os.WriteFile(launcher, []byte(script), 0o755); err != nil {
					return nil, fmt.Errorf("writing launcher %s: %w", pkg.PythonLauncher, err)
				}
				// WriteFile keeps the mode of an existing file.
				if err := os.Chmod(launcher, 0o755); err != nil {
					return nil, err
				}
			}

			installed[pkg.ID] = true
			entries = append(entries, manifest.RealizedEntry{
				ID:             pkg.ID,
				Type:           pkg.Type,
				Filename:       pkg.ID,
				RelativePath:   filepath.Join("runtime", "python", platform, "tools", pkg.ID),
				SizeBytes:      dirSize(toolDir),
				SourceStrategy: "build",
			})
		}
	}

	for _, pkg := range pkgItems {
		if !installed[pkg.ID] {
			return nil, fmt.Errorf("python package %s could not be installed for any of %v", pkg.ID, targets)
		}
	}
	for _, platform := range targets {
		if err := upgradePythonCommands(filepath.Join(root, "bin", platform), platform); err != nil {
			return nil, err
		}
	}

	if repairDeferred {
		message := "Python trees use an older layout; the next apply with network will rebuild them"
		slog.Warn(message)
		emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventLog, Message: message})
	}

	// Builds before the cache moved to the computer left it on the vault.
	for _, legacy := range []string{filepath.Join(root, ".staging", "uv-cache"), filepath.Join(root, ".staging", "build", recipe.ID)} {
		if err := os.RemoveAll(legacy); err != nil {
			slog.Warn("could not remove old python build files", "path", legacy, "err", err)
		}
	}
	// and the folders around them, when nothing else uses them.
	_ = os.Remove(filepath.Join(root, ".staging", "build"))
	_ = os.Remove(filepath.Join(root, ".staging"))

	venvEntry.SizeBytes = dirSize(filepath.Join(root, "runtime", "python"))
	return append(entries, venvEntry), nil
}

// bytecodeLines keep Python's compiled modules off the stick, where each is
// one more cluster (up to 8,311 files for Open WebUI): they go to this
// computer's cache dir, where Go's os.UserCacheDir has it, and are not written
// without a HOME. A prefix or a no-write the caller set (Open WebUI's runtime,
// the embedded shell) is kept.
func bytecodeLines(platform string) string {
	cache := "${XDG_CACHE_HOME:-$HOME/.cache}"
	if strings.HasPrefix(platform, "macos-") {
		cache = "$HOME/Library/Caches"
	}
	return "if [ -z \"${PYTHONPYCACHEPREFIX:-}\" ] && [ -z \"${PYTHONDONTWRITEBYTECODE:-}\" ]; then\n" +
		"  if [ -n \"${HOME:-}\" ]; then export PYTHONPYCACHEPREFIX=\"" + cache + "/" + bytecodeCacheDir + "\"; else export PYTHONDONTWRITEBYTECODE=1; fi\n" +
		"fi\n"
}

// bytecodeCacheDir is where in the user cache dir Python's bytecode goes;
// the drive runtime uses the same for Open WebUI.
const bytecodeCacheDir = "svalbard/pycache"

// pythonTreeLayoutStamp names how installTree lays trees out. The interpreter's
// python.layout and each package's .packages stamp carry it, so trees an older
// layout left are installed again; change it when the layout changes.
const pythonTreeLayoutStamp = "layout: 2 (no symlinks, no terminfo)\n"

// stampWithoutLayout drops a package stamp's last line when it names a layout.
func stampWithoutLayout(stamp string) string {
	body := strings.TrimSuffix(stamp, "\n")
	at := strings.LastIndex(body, "\n") + 1
	if strings.HasPrefix(body[at:], "layout: ") {
		return stamp[:at]
	}
	return stamp
}

func pythonLayoutCurrent(stampPath string) bool {
	data, err := os.ReadFile(stampPath)
	return err == nil && string(data) == pythonTreeLayoutStamp
}

// PythonTreesNeedRepair reports whether a vault holds Python trees for one of
// platforms in an older layout than this builder writes, the uv cache older
// builds left on it, or commands that write bytecode onto the stick, so apply
// rebuilds the venv even when nothing else asks for it. A stamp without its
// tree (a removed package) is not a tree. Rewriting the commands alone needs
// neither the network nor Docker.
func PythonTreesNeedRepair(root string, platforms []string) bool {
	if _, err := os.Stat(filepath.Join(root, ".staging", "uv-cache")); err == nil {
		return true
	}
	isDir := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}
	for _, platform := range platforms {
		platformDir := filepath.Join(root, "runtime", "python", platform)
		if isDir(filepath.Join(platformDir, "python")) && !pythonLayoutCurrent(filepath.Join(platformDir, "python.layout")) {
			return true
		}
		stamps, _ := filepath.Glob(filepath.Join(platformDir, "tools", "*.packages"))
		for _, stamp := range stamps {
			data, err := os.ReadFile(stamp)
			if isDir(strings.TrimSuffix(stamp, ".packages")) && (err != nil || !strings.HasSuffix(string(data), pythonTreeLayoutStamp)) {
				return true
			}
		}
		if pythonCommandsWriteBytecode(filepath.Join(root, "bin", platform)) {
			return true
		}
	}
	return false
}

// pythonCommandsWriteBytecode reports whether binDir holds a Python command
// written before bytecodeLines, which leaves __pycache__ in the tree.
func pythonCommandsWriteBytecode(binDir string) bool {
	return len(oldPythonCommands(binDir)) > 0
}

// oldPythonCommands are the commands in binDir written before bytecodeLines,
// with their content.
func oldPythonCommands(binDir string) map[string]string {
	old := map[string]string{}
	entries, _ := os.ReadDir(binDir)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
			continue
		}
		path := filepath.Join(binDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		script := string(data)
		if strings.HasPrefix(script, "#!/bin/sh\n") && strings.Contains(script, "/runtime/python/") &&
			strings.Contains(script, "\nPYTHONPATH=") && !strings.Contains(script, "PYTHONPYCACHEPREFIX") {
			old[path] = script
		}
	}
	return old
}

// upgradePythonCommands adds bytecodeLines to the commands in binDir the
// build did not write again, such as those of a package the catalog no
// longer has, so none of them writes onto the stick.
func upgradePythonCommands(binDir, platform string) error {
	for path, script := range oldPythonCommands(binDir) {
		script = strings.Replace(script, "\nPYTHONPATH=", "\n"+bytecodeLines(platform)+"PYTHONPATH=", 1)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// pythonBuildDir holds uv's cache and the interpreter archives while building,
// on the computer rather than the vault: a uv cache holds about one file per
// installed file, so on the vault it was a second loose copy of every tree.
// It is the tools container's /work.
var pythonBuildDir = func() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "svalbard", "python-build"), nil
}

// containerUVCache is pythonBuildDir's uv-cache as the tools container sees it.
const containerUVCache = "/work/uv-cache"

func isPlatformIOPythonPackage(pkg catalog.Item) bool {
	return pkg.ID == "platformio-core" || pkg.ID == "espidf-python"
}

func hasPlatformIOPythonPackage(pkgs []catalog.Item) bool {
	for _, pkg := range pkgs {
		if isPlatformIOPythonPackage(pkg) {
			return true
		}
	}
	return false
}

func platformIOPythonTreeComplete(toolDir string, entryPoints []string) bool {
	info, err := os.Stat(toolDir)
	if err != nil || !info.IsDir() {
		return false
	}
	for _, entryPoint := range entryPoints {
		info, err := os.Stat(filepath.Join(toolDir, "bin", entryPoint))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// findInstalledPython returns the python<version> binary of a uv-managed
// install dir, skipping uv's minor-version symlink directories.
func findInstalledPython(installDir, version string) string {
	matches, _ := filepath.Glob(filepath.Join(installDir, "cpython-*", "bin", "python"+version))
	for _, match := range matches {
		info, err := os.Lstat(filepath.Dir(filepath.Dir(match)))
		if err == nil && info.IsDir() {
			return match
		}
	}
	return ""
}

// hostPlatformStr returns the svalbard platform string for the current machine.
func hostPlatformStr() string {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	archName := runtime.GOARCH
	if archName == "amd64" {
		archName = "x86_64"
	}
	return osName + "-" + archName
}

// pythonVersionFromSpec extracts a major.minor version from a spec like
// ">=3.12" or "3.12.4"; uv installs the latest patch release of it.
func pythonVersionFromSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	spec = strings.TrimLeft(spec, ">=~")
	if parts := strings.Split(spec, "."); len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return "3.12"
}
