package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// buildPipeline executes a sequence of build steps defined in recipe.Build.Steps.
// Template variables in step fields are resolved against the recipe and runtime context.
func buildPipeline(root string, recipe catalog.Item, _ *catalog.Catalog, _ Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || len(recipe.Build.Steps) == 0 {
		return nil, fmt.Errorf("pipeline %s: no build steps defined", recipe.ID)
	}

	typeDir := toolkit.TypeDirs[recipe.Type]
	workdir, err := os.MkdirTemp("", "svalbard-build-"+recipe.ID+"-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workdir)

	// Compute output path.
	outputDir := filepath.Join(root, typeDir, recipe.ID)
	outputFile := filepath.Join(root, typeDir, recipe.ID+"."+recipe.Type)
	if recipe.Build.Output != "" {
		outputFile = filepath.Join(root, typeDir, recipe.Build.Output)
	}

	// Build template vars from recipe build config + well-known paths.
	vars := buildTemplateVars(root, recipe, workdir, outputDir, outputFile)

	for i, step := range recipe.Build.Steps {
		switch {
		case step.Download != "":
			if err := stepDownload(resolve(step.Download, vars), resolve(step.Dest, vars)); err != nil {
				return nil, fmt.Errorf("step %d (download): %w", i+1, err)
			}
		case step.Extract != "":
			if err := stepExtract(resolve(step.Extract, vars), resolve(step.Dest, vars)); err != nil {
				return nil, fmt.Errorf("step %d (extract): %w", i+1, err)
			}
		case step.Exec != "":
			resolvedArgs := make([]string, len(step.Args))
			for j, arg := range step.Args {
				resolvedArgs[j] = resolve(arg, vars)
			}
			if err := stepExec(root, step.Exec, resolvedArgs, step.DockerImage); err != nil {
				return nil, fmt.Errorf("step %d (exec %s): %w", i+1, step.Exec, err)
			}
		case step.Verify != "":
			if err := stepVerify(resolve(step.Verify, vars), step.NotEmpty, step.MinSize); err != nil {
				return nil, fmt.Errorf("step %d (verify): %w", i+1, err)
			}
		default:
			return nil, fmt.Errorf("step %d: no action specified", i+1)
		}
	}

	// Determine what was produced and record it.
	entry, err := recordOutput(root, recipe, typeDir, outputDir, outputFile)
	if err != nil {
		return nil, err
	}
	return []manifest.RealizedEntry{entry}, nil
}

// buildTemplateVars creates the variable map for template resolution.
func buildTemplateVars(root string, recipe catalog.Item, workdir, outputDir, outputFile string) map[string]string {
	vars := map[string]string{
		"vault":      root,
		"workdir":    workdir,
		"output":     outputFile,
		"output_dir": outputDir,
		"id":         recipe.ID,
		"type":       recipe.Type,
	}
	if recipe.Build != nil {
		vars["source_url"] = recipe.Build.SourceURL
		// Inject arbitrary build config values (bbox, maxzoom, etc.)
		for k, v := range recipe.Build.Config {
			vars[k] = v
		}
	}
	return vars
}

// resolve replaces {var} placeholders in s with values from vars.
func resolve(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// stepDownload fetches a URL to a local path.
func stepDownload(url, dest string) error {
	if url == "" {
		return fmt.Errorf("empty download URL")
	}
	if dest == "" {
		return fmt.Errorf("empty download destination")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	slog.Info("downloading", "file", filepath.Base(dest))
	_, err := downloader.Download(context.Background(), url, dest, "")
	return err
}

// stepExtract unpacks an archive to a destination directory.
func stepExtract(archivePath, destDir string) error {
	if archivePath == "" {
		return fmt.Errorf("empty archive path")
	}
	if destDir == "" {
		return fmt.Errorf("empty extract destination")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	slog.Info("extracting", "file", filepath.Base(archivePath))
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZipToDir(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGzToDir(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

// stepExec finds a tool and runs it with args.
// Resolution order: drive bin/<platform>/ → PATH → Docker.
// dockerImage overrides the default svalbard-tools container when non-empty.
func stepExec(root, tool string, args []string, dockerImage string) error {
	toolPath := findTool(root, tool)
	slog.Info("exec", "tool", tool, "args", args)

	if toolPath != "" {
		cmd := exec.Command(toolPath, args...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
		return cmd.Run()
	}

	// Docker fallback.
	image := DefaultDockerImage
	if dockerImage != "" {
		image = dockerImage
	}
	slog.Info("exec via docker", "tool", tool, "image", image)
	dockerArgs := []string{
		"run", "--rm",
		"-v", root + ":/vault",
		image,
		tool,
	}
	dockerArgs = append(dockerArgs, args...)
	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	return cmd.Run()
}

// stepVerify checks that a path exists and optionally validates size/contents.
func stepVerify(path string, notEmpty bool, minSize int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("output not found: %s", path)
	}
	if info.IsDir() {
		if notEmpty {
			entries, _ := os.ReadDir(path)
			if len(entries) == 0 {
				return fmt.Errorf("output directory is empty: %s", path)
			}
		}
	} else {
		if minSize > 0 && info.Size() < minSize {
			return fmt.Errorf("output too small: %d bytes (min %d): %s", info.Size(), minSize, path)
		}
	}
	return nil
}

// findTool looks for a tool binary on the drive, then on PATH.
func findTool(root, name string) string {
	platform := hostPlatformStr()
	// Check drive bin directory.
	drivePath := filepath.Join(root, "bin", platform, name)
	if _, err := os.Stat(drivePath); err == nil {
		return drivePath
	}
	// Check drive bin/<tool>/<tool> (nested dir pattern).
	nestedPath := filepath.Join(root, "bin", platform, name, name)
	if _, err := os.Stat(nestedPath); err == nil {
		return nestedPath
	}
	// Check system PATH.
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

// recordOutput determines what the pipeline produced and creates a RealizedEntry.
func recordOutput(root string, recipe catalog.Item, typeDir, outputDir, outputFile string) (manifest.RealizedEntry, error) {
	// Check for directory output first (app-bundle style).
	if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(outputDir)
		if len(entries) > 0 {
			size := dirSize(outputDir)
			return manifest.RealizedEntry{
				ID:             recipe.ID,
				Type:           recipe.Type,
				Filename:       recipe.ID,
				RelativePath:   filepath.Join(typeDir, recipe.ID),
				SizeBytes:      size,
				SourceStrategy: "build",
			}, nil
		}
	}

	// Check for file output.
	if info, err := os.Stat(outputFile); err == nil && !info.IsDir() {
		sha256, _ := downloader.ComputeSHA256(outputFile)
		return manifest.RealizedEntry{
			ID:             recipe.ID,
			Type:           recipe.Type,
			Filename:       filepath.Base(outputFile),
			RelativePath:   filepath.Join(typeDir, filepath.Base(outputFile)),
			SizeBytes:      info.Size(),
			ChecksumSHA256: sha256,
			SourceStrategy: "build",
		}, nil
	}

	return manifest.RealizedEntry{}, fmt.Errorf("pipeline %s: no output produced at %s or %s", recipe.ID, outputDir, outputFile)
}
