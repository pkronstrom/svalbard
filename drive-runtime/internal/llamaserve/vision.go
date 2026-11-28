package llamaserve

import (
	"os"
	"path/filepath"
	"strings"
)

// VisionProjector returns the installed companion for a supported model.
func VisionProjector(modelPath string) string {
	name := strings.ToLower(filepath.Base(modelPath))
	var file string
	switch {
	case strings.Contains(name, "inkling-2-mini"):
		file = "mmproj-Inkling-2-Mini-F16.gguf"
	case strings.Contains(name, "qwen3.5-4b"):
		file = "mmproj-Qwen3.5-4B-BF16.gguf"
	case strings.Contains(name, "qwen3.8-27b"):
		file = "mmproj-Qwen3.8-27B-F16.gguf"
	default:
		return ""
	}
	path := filepath.Join(filepath.Dir(modelPath), "vision", file)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return path
	}
	return ""
}
