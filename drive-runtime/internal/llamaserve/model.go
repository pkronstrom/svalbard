package llamaserve

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveModel returns the path to a GGUF model. If selected is non-empty it is
// tried as an absolute path first, then relative to driveRoot/models. Otherwise
// the lexically-first *.gguf in driveRoot/models is returned (macOS resource
// forks "._*" are skipped).
func ResolveModel(driveRoot, selected string) (string, error) {
	if selected != "" {
		// Try as-is first (absolute path), then resolve relative to models dir.
		if info, err := os.Stat(selected); err == nil && !info.IsDir() {
			return selected, nil
		}
		path := filepath.Join(driveRoot, "models", selected)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("model not found: %s", selected)
	}

	models, err := ChatModels(driveRoot)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no GGUF model found in models/")
	}
	return filepath.Join(driveRoot, "models", models[0]), nil
}

// ChatModels lists the chat model files in driveRoot/models, sorted. Embedding
// models and drafters live in subdirectories and are not chat models; macOS
// resource forks "._*" are skipped.
func ChatModels(driveRoot string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(driveRoot, "models", "*.gguf"))
	if err != nil {
		return nil, err
	}
	var models []string
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "._") {
			continue
		}
		models = append(models, base)
	}
	sort.Strings(models)
	return models, nil
}
