package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PaletteCloseMsg is sent when the palette is dismissed.
type PaletteCloseMsg struct{}

// PaletteSelectMsg is sent when a palette entry is selected.
type PaletteSelectMsg struct {
	Entry       PaletteEntry
	FreeformArg string
}

// PaletteModel is the Bubble Tea model for the command palette overlay.
// It accepts typed input, shows filtered results, and dispatches selection
// messages back to the parent model.
type PaletteModel struct {
	palette  Palette
	query    string
	filtered []PaletteResult
	selected int
	theme    Theme
}

// NewPaletteModel creates a new PaletteModel pre-populated with the given
// entries and theme. The initial filtered list is the result of Match("").
func NewPaletteModel(entries []PaletteEntry, theme Theme) PaletteModel {
	p := Palette{Entries: entries}
	return PaletteModel{
		palette:  p,
		query:    "",
		filtered: p.Match(""),
		selected: 0,
		theme:    theme,
	}
}

// Init implements tea.Model. It returns nil (no initial command).
func (m PaletteModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles keyboard input for the palette.
func (m PaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return PaletteCloseMsg{} }

		case "enter":
			if len(m.filtered) > 0 {
				entry := m.filtered[m.selected]
				return m, func() tea.Msg {
					return PaletteSelectMsg{
						Entry:       entry.PaletteEntry,
						FreeformArg: entry.FreeformArg,
					}
				}
			}
			return m, nil

		case "up":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case "down":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, nil

		case "backspace":
			if len(m.query) > 0 {
				// Remove last rune
				runes := []rune(m.query)
				m.query = string(runes[:len(runes)-1])
				m.filtered = m.palette.Match(m.query)
				m.selected = 0
			}
			return m, nil

		default:
			// Handle printable rune input
			if msg.Type == tea.KeyRunes {
				m.query += string(msg.Runes)
				m.filtered = m.palette.Match(m.query)
				m.selected = 0
			}
			return m, nil
		}
	}

	return m, nil
}

// View implements tea.Model. It renders the command palette overlay.
func (m PaletteModel) View() string {
	var b strings.Builder

	// Section header
	b.WriteString(m.theme.Section.Render("Command Palette"))
	b.WriteString("\n")

	// Input line with cursor block
	b.WriteString(m.theme.Focus.Render("> " + m.query + "\u2588"))
	b.WriteString("\n")

	// Blank line
	b.WriteString("\n")

	// Filtered results list
	if len(m.filtered) == 0 {
		if m.query != "" {
			b.WriteString(m.theme.Muted.Render("No matches."))
			b.WriteString("\n")
		}
	} else {
		for i, r := range m.filtered {
			if i == m.selected {
				b.WriteString(m.theme.Selected.Render("> " + r.Label))
			} else {
				b.WriteString("  " + r.Label)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("Enter: select | Esc: close | ↑/↓: move"))

	return b.String()
}
