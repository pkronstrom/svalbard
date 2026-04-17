package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// buildPythonVenv provisions Python environments for all python-package recipes
// that reference this venv. For each target platform it:
//  1. Downloads all wheels to a shared cache
//  2. Installs a standalone Python interpreter
//  3. Creates per-tool isolated venvs from the cache
//  4. Generates wrapper scripts in bin/<platform>/
func buildPythonVenv(root string, recipe catalog.Item, cat *catalog.Catalog, platforms []string) ([]manifest.RealizedEntry, error) {
	uv, err := findUV(root)
	if err != nil {
		return nil, err
	}

	// Collect all python-package recipes that reference this venv.
	var pkgItems []catalog.Item
	var allPackages []string
	for _, item := range cat.AllRecipes() {
		if item.Type == "python-package" && item.Venv == recipe.ID {
			pkgItems = append(pkgItems, item)
			allPackages = append(allPackages, item.Packages...)
		}
	}

	if len(allPackages) == 0 {
		// No packages to install — just record the venv as realized.
		return []manifest.RealizedEntry{{
			ID:             recipe.ID,
			Type:           recipe.Type,
			Filename:       recipe.ID,
			RelativePath:   "runtime/python",
			SourceStrategy: "build",
		}}, nil
	}

	pythonSpec := recipe.Python
	if pythonSpec == "" {
		pythonSpec = ">=3.11"
	}

	targets := platforms
	if len(targets) == 0 {
		targets = []string{hostPlatformStr()}
	}

	cacheDir := filepath.Join(root, "runtime", "python", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	// Phase 1: Download wheels to shared cache for all target platforms.
	for _, platform := range targets {
		uvPlatform, uvArch := toUVPlatformArgs(platform)
		args := []string{
			"pip", "download",
			"--dest", cacheDir,
			"--python-version", pythonVersionFromSpec(pythonSpec),
			"--platform", uvPlatform + "_" + uvArch,
		}
		args = append(args, allPackages...)
		fmt.Fprintf(os.Stderr, "  downloading wheels for %s...\n", platform)
		if err := runUV(uv, args...); err != nil {
			return nil, fmt.Errorf("downloading wheels for %s: %w", platform, err)
		}
	}
	// Also download platform-independent wheels (pure Python).
	args := []string{
		"pip", "download",
		"--dest", cacheDir,
		"--python-version", pythonVersionFromSpec(pythonSpec),
		"--platform", "any",
	}
	args = append(args, allPackages...)
	// Best-effort: some packages don't have pure-python wheels.
	_ = runUV(uv, args...)

	var entries []manifest.RealizedEntry

	// Phase 2-4: Per platform.
	for _, platform := range targets {
		platformDir := filepath.Join(root, "runtime", "python", platform)
		pythonDir := filepath.Join(platformDir, ".python")
		binDir := filepath.Join(root, "bin", platform)

		if err := os.MkdirAll(pythonDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return nil, err
		}

		// Phase 2: Install Python interpreter.
		pythonBin := filepath.Join(platformDir, "bin", "python3")
		if _, err := os.Stat(pythonBin); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "  installing python %s for %s...\n", pythonSpec, platform)
			if err := runUV(uv, "python", "install", "--install-dir", pythonDir, pythonSpec); err != nil {
				return nil, fmt.Errorf("installing python for %s: %w", platform, err)
			}

			// Find installed python binary.
			installedPython := findInstalledPython(pythonDir)
			if installedPython == "" {
				return nil, fmt.Errorf("could not find installed python in %s", pythonDir)
			}

			// Create base venv.
			if err := runUV(uv, "venv", platformDir, "--python", installedPython); err != nil {
				return nil, fmt.Errorf("creating venv for %s: %w", platform, err)
			}
		}

		// Phase 3: Create per-tool venvs from cache.
		for _, pkg := range pkgItems {
			toolDir := filepath.Join(platformDir, "tools", pkg.ID)
			toolPython := filepath.Join(toolDir, "bin", "python3")

			if _, err := os.Stat(toolPython); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "  creating venv for %s (%s)...\n", pkg.ID, platform)

				// Create tool venv using the platform's python.
				if err := runUV(uv, "venv", toolDir, "--python", pythonBin); err != nil {
					return nil, fmt.Errorf("creating venv for %s/%s: %w", pkg.ID, platform, err)
				}

				// Install packages from cache (offline).
				installArgs := []string{
					"pip", "install",
					"--python", toolPython,
					"--no-index",
					"--find-links", cacheDir,
				}
				installArgs = append(installArgs, pkg.Packages...)
				if err := runUV(uv, installArgs...); err != nil {
					return nil, fmt.Errorf("installing %s for %s: %w", pkg.ID, platform, err)
				}
			}

			// Phase 4: Generate wrapper scripts.
			for _, ep := range pkg.EntryPoints {
				wrapper := filepath.Join(binDir, ep)
				script := fmt.Sprintf(
					"#!/bin/sh\nDRIVE=\"$(cd \"$(dirname \"$0\")/../..\" && pwd)\"\nexec \"$DRIVE/runtime/python/%s/tools/%s/bin/%s\" \"$@\"\n",
					platform, pkg.ID, ep,
				)
				if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
					return nil, fmt.Errorf("writing wrapper for %s: %w", ep, err)
				}
			}

			// Record python-package as realized.
			toolSize := dirSize(toolDir)
			entries = append(entries, manifest.RealizedEntry{
				ID:             pkg.ID,
				Type:           pkg.Type,
				Filename:       pkg.ID,
				RelativePath:   filepath.Join("runtime", "python", platform, "tools", pkg.ID),
				SizeBytes:      toolSize,
				SourceStrategy: "build",
			})
		}
	}

	// Record the venv itself as realized.
	venvSize := dirSize(filepath.Join(root, "runtime", "python"))
	entries = append(entries, manifest.RealizedEntry{
		ID:             recipe.ID,
		Type:           recipe.Type,
		Filename:       recipe.ID,
		RelativePath:   "runtime/python",
		SizeBytes:      venvSize,
		SourceStrategy: "build",
	})

	return entries, nil
}

// findUV locates the uv binary: first on the drive, then on PATH.
func findUV(root string) (string, error) {
	platform := hostPlatformStr()
	drivePath := filepath.Join(root, "bin", platform, "uv")
	if _, err := os.Stat(drivePath); err == nil {
		return drivePath, nil
	}
	if path, err := exec.LookPath("uv"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("uv not found (install it or include the uv recipe in your preset)")
}

// runUV executes a uv subcommand, forwarding stderr for progress output.
func runUV(uvPath string, args ...string) error {
	cmd := exec.Command(uvPath, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv %s: %w", strings.Join(args[:min(len(args), 3)], " "), err)
	}
	return nil
}

// findInstalledPython searches for the python3 binary in a uv-managed install dir.
func findInstalledPython(installDir string) string {
	var found string
	filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if (base == "python3" || strings.HasPrefix(base, "python3.")) && info.Mode()&0o111 != 0 {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
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

// toUVPlatformArgs converts a svalbard platform string to uv platform/arch args.
// uv expects manylinux/macosx platform tags for wheel downloads.
func toUVPlatformArgs(platform string) (string, string) {
	parts := strings.SplitN(platform, "-", 2)
	osName, arch := parts[0], parts[1]

	var uvPlatform string
	switch osName {
	case "macos":
		uvPlatform = "macosx_11_0"
	case "linux":
		uvPlatform = "manylinux_2_17"
	default:
		uvPlatform = osName
	}

	var uvArch string
	switch arch {
	case "arm64":
		uvArch = "aarch64"
	case "x86_64":
		uvArch = "x86_64"
	default:
		uvArch = arch
	}

	return uvPlatform, uvArch
}

// pythonVersionFromSpec extracts a concrete version from a spec like ">=3.11".
// For download purposes we need a concrete version; default to 3.11.
func pythonVersionFromSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	spec = strings.TrimPrefix(spec, ">=")
	spec = strings.TrimPrefix(spec, "==")
	spec = strings.TrimPrefix(spec, "~=")
	if spec == "" {
		return "3.11"
	}
	return spec
}
