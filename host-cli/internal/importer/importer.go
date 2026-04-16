package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportLocalFile copies a local file into the workspace library and returns
// a local: prefixed identifier for it.
//
// If outputName is non-empty it is used as the base name for the id slug;
// otherwise the source filename (without extension) is used.
func ImportLocalFile(workspace string, source string, outputName string) (string, error) {
	baseName := outputName
	if baseName == "" {
		ext := filepath.Ext(source)
		baseName = strings.TrimSuffix(filepath.Base(source), ext)
	}

	slug := toSlug(baseName)
	id := "local:" + slug

	libDir := filepath.Join(workspace, "library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return "", fmt.Errorf("creating library directory: %w", err)
	}

	srcFile, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	destPath := filepath.Join(libDir, filepath.Base(source))
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("creating destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", fmt.Errorf("copying file: %w", err)
	}

	return id, nil
}

// toSlug converts a name to a URL-friendly slug: lowercase, spaces become
// hyphens, leading/trailing hyphens trimmed.
func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Trim(s, "-")
	return s
}
