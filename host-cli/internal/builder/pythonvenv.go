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
//  1. Installs a relocatable python-build-standalone interpreter per platform
//  2. Installs each package with `uv pip install --target` into its own dir,
//     resolving wheels for that platform's tags; a recipe with
//     requirements_from installs the package without dependencies, then the
//     requirements file the package ships
//  3. Generates wrapper scripts in bin/<platform>/ that run the entry point
//     with the drive's interpreter and the package dir on PYTHONPATH
//
// A package that cannot be installed for a platform (no compatible wheels) is
// skipped for that platform with a warning; it fails the build only when no
// platform could install it. A package or venv named in opts.Rebuild is being
// updated: its trees are reinstalled, and one that fails to reinstall keeps
// its installed tree and fails the build.
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
	if len(pkgItems) == 0 {
		return []manifest.RealizedEntry{venvEntry}, nil
	}

	version := pythonVersionFromSpec(recipe.Python)
	rebuildAll := opts.Rebuild[recipe.ID]
	targets := opts.Platforms
	if len(targets) == 0 {
		targets = []string{hostPlatformStr()}
	}

	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	uvCache := filepath.Join(root, ".staging", "uv-cache")
	for _, dir := range []string{workdir, uvCache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	uv := func(env []string, args ...string) error {
		args = append(append(append([]string{}, env...), "uv"), args...)
		_, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "env", args)
		return err
	}

	var entries []manifest.RealizedEntry
	installedOn := map[string]bool{}

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

		// Phase 1: Install the interpreter for this platform.
		pythonBin := findInstalledPython(interpreterDir, version)
		newInterpreter := pythonBin == ""
		if newInterpreter {
			slog.Info("installing python", "version", version, "platform", platform)
			emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventProgress, Message: "installing python " + version + " for " + platform})
			request := "cpython-" + version + "-" + target.python
			if err := uv(nil, "python", "install", "--no-bin", "--install-dir", interpreterDir, request); err != nil {
				return nil, fmt.Errorf("installing python for %s: %w", platform, err)
			}
			if pythonBin = findInstalledPython(interpreterDir, version); pythonBin == "" {
				return nil, fmt.Errorf("could not find installed python in %s", interpreterDir)
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
			// Trees are built for one interpreter version. A stamp from before
			// the version was recorded matches the interpreter already there.
			legacyStamp := stamp
			stamp += "python: " + version + "\n"
			stampPath := toolDir + ".packages"
			existing, stampErr := os.ReadFile(stampPath)
			installed := stampErr == nil
			current := installed && (string(existing) == stamp || (!newInterpreter && string(existing) == legacyStamp))
			updating := installed && (rebuildAll || opts.Rebuild[pkg.ID])
			rebuild := !current || updating
			if !rebuild && isPlatformIOPythonPackage(pkg) && !platformIOPythonTreeComplete(toolDir, pkg.EntryPoints) {
				rebuild = true
			}
			if rebuild {
				slog.Info("installing python package", "package", pkg.ID, "platform", platform)
				emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventProgress, Message: "installing " + pkg.ID + " for " + platform})
				staging := toolDir + ".partial"
				_ = os.RemoveAll(staging)
				var env []string
				if target.macosMin != "" {
					env = append(env, "MACOSX_DEPLOYMENT_TARGET="+target.macosMin)
				}
				install := func(flags ...string) error {
					args := append([]string{"pip", "install"}, flags...)
					args = append(args,
						"--target", staging,
						"--python-platform", target.uvPlatform,
						"--python-version", version,
						"--cache-dir", uvCache,
					)
					if pkg.RequirementsFrom == "" {
						return uv(env, append(args, pkg.Packages...)...)
					}
					// The requirements file only exists once the package that
					// ships it is in the tree.
					if err := uv(env, append(append(slices.Clone(args), "--no-deps"), pkg.Packages...)...); err != nil {
						return err
					}
					return uv(env, append(args, "-r", filepath.Join(staging, pkg.RequirementsFrom))...)
				}
				// Wheels only first: with builds allowed uv picks the newest
				// version even when only its sdist exists, and a native sdist
				// cannot be built for a foreign platform. Retry with builds for
				// pure-python sdists (uv rejects built wheels that don't match).
				err := install("--no-build")
				if err != nil {
					_ = os.RemoveAll(staging)
					err = install()
				}
				if err != nil {
					_ = os.RemoveAll(staging)
					if updating {
						return nil, fmt.Errorf("updating %s for %s: %w; its installed copy was kept", pkg.ID, platform, err)
					}
					if isPlatformIOPythonPackage(pkg) {
						return nil, fmt.Errorf("PlatformIO Python package %s could not be installed for required target %s: %w", pkg.ID, platform, err)
					}
					slog.Warn("python package unavailable for platform, skipping", "package", pkg.ID, "platform", platform, "err", truncate(err.Error(), 300))
					emit(opts, BuildEvent{RecipeID: recipe.ID, State: EventLog, Message: fmt.Sprintf("skipping %s for %s: no compatible wheels", pkg.ID, platform)})
					for _, ep := range pkg.EntryPoints {
						_ = os.Remove(filepath.Join(binDir, ep))
					}
					continue
				}
				if err := os.RemoveAll(toolDir); err != nil {
					return nil, err
				}
				if err := os.Rename(staging, toolDir); err != nil {
					return nil, err
				}
				if err := os.WriteFile(stampPath, []byte(stamp), 0o644); err != nil {
					return nil, err
				}
			}

			// Phase 3: Generate wrapper scripts. Console scripts written by
			// `--target` carry the build container's shebang, so run them
			// through the drive's interpreter instead.
			for _, ep := range pkg.EntryPoints {
				wrapper := filepath.Join(binDir, ep)
				script := fmt.Sprintf(
					"#!/bin/sh\nDRIVE=\"$(cd \"$(dirname \"$0\")/../..\" && pwd)\"\nPY=\"$DRIVE/runtime/python/%s\"\nPYTHONPATH=\"$PY/tools/%s\" PYTHONNOUSERSITE=1 exec \"$PY/%s\" \"$PY/tools/%s/bin/%s\" \"$@\"\n",
					platform, pkg.ID, filepath.ToSlash(pythonRel), pkg.ID, ep,
				)
				if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
					return nil, fmt.Errorf("writing wrapper for %s: %w", ep, err)
				}
			}

			installedOn[pkg.ID] = true
			entries = append(entries, manifest.RealizedEntry{
				ID:             pkg.ID,
				Type:           pkg.Type,
				Filename:       pkg.ID,
				RelativePath:   filepath.Join("runtime", "python", platform, "tools", pkg.ID),
				SizeBytes:      dirSize(toolDir),
				SourceStrategy: "build",
			})
		}
		if newInterpreter {
			// Every tree on this platform now runs on the new interpreter.
			removeOtherPythons(interpreterDir, version)
		}
	}

	for _, pkg := range pkgItems {
		if !installedOn[pkg.ID] {
			return nil, fmt.Errorf("python package %s could not be installed for any of %v", pkg.ID, targets)
		}
	}

	venvEntry.SizeBytes = dirSize(filepath.Join(root, "runtime", "python"))
	return append(entries, venvEntry), nil
}

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

// removeOtherPythons deletes the interpreters of a uv-managed install dir
// other than version, left behind when the venv's Python version changes.
func removeOtherPythons(installDir, version string) {
	dirs, _ := os.ReadDir(installDir)
	for _, dir := range dirs {
		name := dir.Name()
		if !strings.HasPrefix(name, "cpython-") || strings.HasPrefix(name, "cpython-"+version+".") || strings.HasPrefix(name, "cpython-"+version+"-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(installDir, name)); err != nil {
			slog.Warn("removing an earlier python", "path", name, "error", err)
		}
	}
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
