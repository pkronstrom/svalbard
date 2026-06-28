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

	pattern := filepath.Join(driveRoot, "models", "*.gguf")
	models, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	filtered := models[:0]
	for _, model := range models {
		base := filepath.Base(model)
		if strings.HasPrefix(base, "._") {
			continue
		}
		filtered = append(filtered, model)
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no GGUF model found in models/")
	}
	return filtered[0], nil
}
