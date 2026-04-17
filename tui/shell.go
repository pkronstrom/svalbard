package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// MinTwoPaneWidth is the minimum terminal width for side-by-side panes.
	// Below this, the layout stacks vertically.
	MinTwoPaneWidth = 80

	// LeftFraction is the proportion of width allocated to the left pane
	// in wide (two-pane) mode.
	LeftFraction = 0.25
)

// ShellLayout is the core layout primitive for Svalbard TUI applications.
// It renders a two-pane operator console in wide terminals or stacks
// vertically in narrow ones. Callers provide pre-rendered Left and Right
// pane content; ShellLayout handles only geometry.
type ShellLayout struct {
	Theme        Theme
	AppName      string
	Identity     string // vault name, drive identity
	Status       string // status badge text
	Left         string // pre-rendered left pane content
	Right        string // pre-rendered right pane content
	CompactRight string // 1-2 line summary for narrow mode (falls back to Right if empty)
	Footer       string // key hint line
	Width        int
	Height       int // reserved for future vertical overflow management
}

// Render produces the adaptive layout string.
// Wide mode (Width >= MinTwoPaneWidth) places panes side-by-side.
// Narrow mode (Width < MinTwoPaneWidth) stacks them vertically.
func (s ShellLayout) Render() string {
	// Top bar: AppName + Identity + Status, space-separated
	topBar := s.Theme.Title.Render(s.AppName) +
		" " + s.Theme.Muted.Render(s.Identity) +
		" " + s.Theme.Status.Render(s.Status)

	// Footer
	footer := s.Theme.Help.Render(s.Footer)

	if s.Width >= MinTwoPaneWidth {
		return s.renderWide(topBar, footer)
	}
	return s.renderNarrow(topBar, footer)
}

func (s ShellLayout) renderWide(topBar, footer string) string {
	gutter := 2
	leftWidth := int(float64(s.Width) * LeftFraction)
	rightWidth := s.Width - leftWidth - gutter

	leftStyle := lipgloss.NewStyle().Width(leftWidth)
	rightStyle := lipgloss.NewStyle().Width(rightWidth)

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(s.Left),
		strings.Repeat(" ", gutter),
		rightStyle.Render(s.Right),
	)

	out := lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		"",
		body,
		"",
		footer,
	)

	if s.Width > 0 && s.Height > 0 {
		return lipgloss.Place(s.Width, s.Height, lipgloss.Left, lipgloss.Top, out)
	}
	return out
}

func (s ShellLayout) renderNarrow(topBar, footer string) string {
	summary := s.CompactRight
	if summary == "" {
		summary = s.Right
	}
	rightContent := s.Theme.Muted.Render(summary)

	out := lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		"",
		s.Left,
		"",
		rightContent,
		"",
		footer,
	)

	if s.Width > 0 && s.Height > 0 {
		return lipgloss.Place(s.Width, s.Height, lipgloss.Left, lipgloss.Top, out)
	}
	return out
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripAnsi removes ANSI escape sequences from a string.
// Useful for width calculations and test assertions on styled output.
func StripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
