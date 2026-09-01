package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

func buildZIMCompact(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.SourceURL == "" {
		return nil, fmt.Errorf("zim-compact %s: source_url required", recipe.ID)
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	extracted := filepath.Join(workdir, "extracted")
	source := filepath.Join(workdir, "source.zim")
	redirects := filepath.Join(workdir, "redirects.tsv")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(source); os.IsNotExist(err) {
		if err := stepDownload(ctx, recipe.Build.SourceURL, source); err != nil {
			return nil, err
		}
	}
	width := configInt(recipe.Build.Config, "width", 200)
	quality := configInt(recipe.Build.Config, "quality", 40)
	stdout, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "zim-compact", []string{
		fmt.Sprintf("--width=%d", width), fmt.Sprintf("--quality=%d", quality),
		"--redirects=" + redirects, source, extracted,
	})
	if err != nil {
		return nil, err
	}
	metadata := parseToolMetadata(stdout)
	outputName := recipe.Build.Output
	if outputName == "" {
		outputName = recipe.ID + ".zim"
	}
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], outputName)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	args := []string{
		"--welcome=" + metadataValue(metadata, "main_page", "index"),
		"--language=" + metadataValue(metadata, "language", "eng"),
		"--title=" + metadataValue(metadata, "title", "Wikipedia (compact)"),
		"--description=" + metadataValue(metadata, "description", "Wikipedia with resized images"),
		"--creator=" + metadataValue(metadata, "creator", "Wikipedia contributors"),
		"--publisher=Svalbard", "--name=" + recipe.ID, "--withoutFTIndex",
		"--redirects=" + redirects, fmt.Sprintf("--threads=%d", runtime.NumCPU()), extracted, output,
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "zimwriterfs", args); err != nil {
		return nil, err
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "zimcheck", []string{output}); err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: outputName,
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], outputName),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

func parseToolMetadata(output string) map[string]string {
	metadata := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return metadata
}

func metadataValue(metadata map[string]string, key, fallback string) string {
	if value := metadata[key]; value != "" {
		return value
	}
	return fallback
}

func configInt(config map[string]string, key string, fallback int) int {
	value, err := strconv.Atoi(config[key])
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
