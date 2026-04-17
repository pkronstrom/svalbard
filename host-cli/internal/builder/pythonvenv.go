package builder

import (
	"bytes"
	"fmt"
	"log/slog"
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
func buildPythonVenv(root string, recipe catalog.Item, cat *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	uv := findUV(root)

	// Collect python-package recipes that reference this venv AND are in the
	// user's desired set. This prevents installing packages the user didn't select.
	var pkgItems []catalog.Item
	var allPackages []string
	for _, item := range cat.AllRecipes() {
		if item.Type == "python-package" && item.Venv == recipe.ID {
			// Only include if user selected this package (or no filter provided).
			if len(opts.DesiredIDs) > 0 && !opts.DesiredIDs[item.ID] {
				continue
			}
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

	targets := opts.Platforms
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
		slog.Info("downloading wheels", "platform", platform)
		if err := uv.run(args...); err != nil {
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
	_ = uv.run(args...)

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
			slog.Info("installing python", "version", pythonSpec, "platform", platform)
			if err := uv.run("python", "install", "--install-dir", pythonDir, pythonSpec); err != nil {
				return nil, fmt.Errorf("installing python for %s: %w", platform, err)
			}

			// Find installed python binary.
			installedPython := findInstalledPython(pythonDir)
			if installedPython == "" {
				return nil, fmt.Errorf("could not find installed python in %s", pythonDir)
			}

			// Create base venv.
			if err := uv.run("venv", platformDir, "--python", installedPython); err != nil {
				return nil, fmt.Errorf("creating venv for %s: %w", platform, err)
			}
		}

		// Phase 3: Create per-tool venvs from cache.
		for _, pkg := range pkgItems {
			toolDir := filepath.Join(platformDir, "tools", pkg.ID)
			toolPython := filepath.Join(toolDir, "bin", "python3")

			if _, err := os.Stat(toolPython); os.IsNotExist(err) {
				slog.Info("creating venv", "package", pkg.ID, "platform", platform)

				// Create tool venv using the platform's python.
				if err := uv.run("venv", toolDir, "--python", pythonBin); err != nil {
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
				if err := uv.run(installArgs...); err != nil {
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

// uvRunner abstracts uv execution — either local binary or Docker.
type uvRunner struct {
	local string // path to local uv binary, empty = use Docker
	root  string // vault root (for Docker volume mount)
}

// toContainerPath translates a host absolute path under the vault root
// to the corresponding /vault/... path inside the Docker container.
// Non-vault paths are returned unchanged.
func (u uvRunner) toContainerPath(hostPath string) string {
	if strings.HasPrefix(hostPath, u.root+string(filepath.Separator)) {
		rel, err := filepath.Rel(u.root, hostPath)
		if err == nil {
			return "/vault/" + filepath.ToSlash(rel)
		}
	}
	// Also handle exact match (root itself).
	if hostPath == u.root {
		return "/vault"
	}
	return hostPath
}

// findUV locates uv: drive → PATH → Docker fallback.
func findUV(root string) uvRunner {
	platform := hostPlatformStr()
	drivePath := filepath.Join(root, "bin", platform, "uv")
	if _, err := os.Stat(drivePath); err == nil {
		return uvRunner{local: drivePath, root: root}
	}
	if path, err := exec.LookPath("uv"); err == nil {
		return uvRunner{local: path, root: root}
	}
	// Docker fallback — svalbard-tools container has uv installed.
	return uvRunner{root: root}
}

// run executes a uv subcommand via local binary or Docker.
func (u uvRunner) run(args ...string) error {
	var cmd *exec.Cmd
	if u.local != "" {
		cmd = exec.Command(u.local, args...)
	} else {
		// Translate host paths under vault root to /vault/... for the container.
		translated := make([]string, len(args))
		for i, arg := range args {
			translated[i] = u.toContainerPath(arg)
		}
		dockerArgs := []string{
			"run", "--rm",
			"-v", u.root + ":/vault",
			DefaultDockerImage,
			"uv",
		}
		dockerArgs = append(dockerArgs, translated...)
		cmd = exec.Command("docker", dockerArgs...)
	}
	var buf bytes.Buffer
	cmd.Stderr = &buf
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		slog.Warn("uv exec failed", "args", args[:min(len(args), 3)], "output", truncate(buf.String(), 200))
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
	switch {
	case arch == "arm64" && osName == "linux":
		uvArch = "aarch64" // Linux wheel tags use aarch64
	case arch == "arm64" && osName == "macos":
		uvArch = "arm64" // macOS wheel tags use arm64
	case arch == "x86_64":
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
