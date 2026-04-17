package tui

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"
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

// StrategySymbol returns a small indicator for the acquisition strategy.
// "⚒" for build recipes, "" (empty) for downloads (the common default).
func StrategySymbol(strategy string) string {
	if strategy == "build" {
		return "⚒"
	}
	return ""
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

// FormatBytes formats a byte count to a human-readable string (KB/MB/GB).
func FormatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// MatchRune checks if a key message is a specific rune.
func MatchRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}
