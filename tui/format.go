package tui

import (
	"fmt"
	"math"
)

// TypeSymbol returns a small Unicode symbol indicating the recipe type.
func TypeSymbol(t string) string {
	switch t {
	case "zim", "pdf", "epub", "html":
		return "✦"
	case "binary", "toolchain", "app", "sqlite", "python-package", "python-venv":
		return "⚙"
	case "pmtiles", "gpkg":
		return "⊞"
	case "gguf":
		return "∿"
	default:
		return "·"
	}
}

// FormatSizeGB formats a size in GB to a human-readable string with a ~ prefix.
func FormatSizeGB(gb float64) string {
	if gb < 1 {
		mb := gb * 1024
		return fmt.Sprintf("~%.0f MB", math.Round(mb))
	}
	return fmt.Sprintf("~%.0f GB", math.Round(gb))
}

// FormatSize formats a size in GB to a compact human-readable string without a ~ prefix.
func FormatSize(gb float64) string {
	if gb < 1 {
		return fmt.Sprintf("%.0f MB", gb*1024)
	}
	return fmt.Sprintf("%.1f GB", gb)
}
