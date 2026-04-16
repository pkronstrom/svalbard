// Package tui provides the shared visual design system for Svalbard TUI applications.
// It defines semantic style roles via a Theme struct backed by Lip Gloss styles.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds the semantic visual roles used across Svalbard TUI apps.
// Each field is a lipgloss.Style that encodes foreground/background colors,
// bold, and other attributes appropriate for the role.
type Theme struct {
	// Semantic roles
	Base    lipgloss.Style
	Focus   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Danger  lipgloss.Style
	Muted   lipgloss.Style

	// Composite styles
	Title        lipgloss.Style
	Section      lipgloss.Style
	Selected     lipgloss.Style
	SelectedRow  lipgloss.Style
	SelectedMuted lipgloss.Style
	Help         lipgloss.Style
	Error        lipgloss.Style
	Status       lipgloss.Style
}

// DefaultTheme returns the Svalbard default palette.
// Colors are chosen to match the existing drive-runtime styles and
// provide a cohesive dark-terminal aesthetic.
func DefaultTheme() Theme {
	return Theme{
		// Semantic roles
		Base:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Focus:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
		Danger:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("131")),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),

		// Composite styles — these match the existing drive-runtime/internal/menu/view.go
		Title:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("124")),
		Section:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("180")),
		Selected:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")),
		SelectedRow:   lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("255")),
		SelectedMuted: lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("252")),
		Help:          lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Error:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("131")),
		Status:        lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
	}
}
