package builder

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// buildAppBundle downloads a source URL and extracts the archive into apps/<id>/.
// Handles .zip and .tar.gz archives. The platforms parameter is unused since
// app bundles are typically platform-independent.
func buildAppBundle(root string, recipe catalog.Item, _ *catalog.Catalog, _ []string) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.SourceURL == "" {
		return nil, fmt.Errorf("app-bundle %s: missing build.source_url", recipe.ID)
	}

	destDir := filepath.Join(root, "apps", recipe.ID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	// Skip if already populated.
	entries, _ := os.ReadDir(destDir)
	if len(entries) > 0 {
		size := dirSize(destDir)
		return []manifest.RealizedEntry{{
			ID:             recipe.ID,
			Type:           recipe.Type,
			Filename:       recipe.ID,
			RelativePath:   filepath.Join("apps", recipe.ID),
			SizeBytes:      size,
			SourceStrategy: "build",
		}}, nil
	}

	// Download to a temp file.
	tmpFile, err := os.CreateTemp("", "svalbard-appbundle-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if _, err := downloader.Download(context.Background(), recipe.Build.SourceURL, tmpPath, ""); err != nil {
		return nil, fmt.Errorf("downloading %s: %w", recipe.ID, err)
	}

	// Extract based on file extension.
	url := recipe.Build.SourceURL
	switch {
	case strings.HasSuffix(url, ".zip"):
		if err := extractZipToDir(tmpPath, destDir); err != nil {
			return nil, fmt.Errorf("extracting %s: %w", recipe.ID, err)
		}
	case strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz"):
		if err := extractTarGzToDir(tmpPath, destDir); err != nil {
			return nil, fmt.Errorf("extracting %s: %w", recipe.ID, err)
		}
	default:
		return nil, fmt.Errorf("app-bundle %s: unsupported archive format: %s", recipe.ID, url)
	}

	size := dirSize(destDir)
	return []manifest.RealizedEntry{{
		ID:             recipe.ID,
		Type:           recipe.Type,
		Filename:       recipe.ID,
		RelativePath:   filepath.Join("apps", recipe.ID),
		SizeBytes:      size,
		SourceStrategy: "build",
	}}, nil
}

func extractZipToDir(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target := filepath.Join(destDir, filepath.Clean(file.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue // path traversal protection
		}
		if file.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode()|0o644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(out, src)
		src.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func extractTarGzToDir(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue // path traversal protection
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
