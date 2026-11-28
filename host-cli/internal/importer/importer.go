package importer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportLocalFile copies a local file into the workspace library and returns
// a local: prefixed identifier for it plus the destination path.
//
// If outputName is non-empty it is used as the base name for the id slug;
// otherwise the source filename (without extension) is used.
func ImportLocalFile(workspace string, source string, outputName string) (string, string, error) {
	return importInto(workspace, source, outputName, "library")
}

// ImportZIM installs an archive where the indexer and Kiwix can find it.
func ImportZIM(vault, source, outputName string) (string, string, error) {
	return importInto(vault, source, outputName, "zim")
}

func importInto(workspace, source, outputName, directory string) (string, string, error) {
	baseName := outputName
	if baseName == "" {
		ext := filepath.Ext(source)
		baseName = strings.TrimSuffix(filepath.Base(source), ext)
	}

	slug := toSlug(baseName)
	id := "local:" + slug

	targetDir := filepath.Join(workspace, directory)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating %s directory: %w", directory, err)
	}

	srcFile, err := os.Open(source)
	if err != nil {
		return "", "", fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	destPath := filepath.Join(targetDir, filepath.Base(source))
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return "", "", err
	}
	if destInfo, err := os.Stat(destPath); err == nil && os.SameFile(srcInfo, destInfo) {
		return id, destPath, nil
	}
	destFile, err := os.CreateTemp(targetDir, ".import-")
	if err != nil {
		return "", "", fmt.Errorf("creating destination file: %w", err)
	}
	defer os.Remove(destFile.Name())
	if err := destFile.Chmod(0o644); err != nil {
		destFile.Close()
		return "", "", err
	}
	_, err = io.Copy(destFile, srcFile)
	if err := errors.Join(err, destFile.Close()); err != nil {
		return "", "", fmt.Errorf("copying file: %w", err)
	}
	if err := os.Rename(destFile.Name(), destPath); err != nil {
		return "", "", fmt.Errorf("publishing imported file: %w", err)
	}
	return id, destPath, nil
}

// toSlug converts a name to a URL-friendly slug: lowercase, spaces become
// hyphens, leading/trailing hyphens trimmed.
func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Trim(s, "-")
	return s
}
