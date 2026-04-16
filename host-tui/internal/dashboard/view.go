package dashboard

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/tui"
)

// View renders the two-pane dashboard layout.
func (m Model) View() string {
	if m.paletteActive {
		return m.paletteModel.View()
	}

	// Build the navigation list from hostDestinations.
	items := make([]tui.NavItem, len(hostDestinations))
	for i, d := range hostDestinations {
		items[i] = tui.NavItem{
			ID:    d.id,
			Label: d.label,
		}
	}

	nav := tui.NavList{
		Items:    items,
		Selected: m.selected,
		Theme:    m.theme,
	}

	// Build the detail pane for the currently selected destination.
	detail := contextForDestination(hostDestinations[m.selected].id, m)

	// Footer key hints.
	footer := tui.FooterHints(
		m.keys.MoveUp,
		m.keys.Enter,
		m.keys.Back,
		m.keys.Quit,
	)

	shell := tui.ShellLayout{
		Theme:    m.theme,
		AppName:  "Svalbard",
		Identity: filepath.Base(m.vaultPath),
		Left:     nav.Render(),
		Right:    detail.Render(),
		Footer:   footer,
		Width:    m.width,
		Height:   m.height,
	}

	return shell.Render()
}
