package dashboard

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/tui"
)

// View renders the two-pane dashboard layout.
func (m Model) View() string {
	// Build the navigation list from hostDestinations.
	items := make([]tui.NavItem, len(hostDestinations))
	for i, d := range hostDestinations {
		items[i] = tui.NavItem{
			ID:          d.id,
			Label:       d.label,
			Description: d.desc,
			Separator:   separatorBefore[d.id],
		}
	}

	nav := tui.NavList{
		Items:       items,
		Selected:    m.selected,
		Theme:       m.theme,
		ShowNumbers: true,
	}

	// Build the detail pane for the currently selected destination.
	detail := contextForDestination(hostDestinations[m.selected].id, m)

	// Footer key hints — override Enter label for host context.
	enter := m.keys.Enter
	enter.Label = "Enter: select"
	footer := tui.FooterHints(
		m.keys.MoveUp,
		enter,
		m.keys.Back,
		m.keys.Quit,
	)

	shell := tui.ShellLayout{
		Theme:        m.theme,
		AppName:      "Svalbard",
		Identity:     filepath.Base(m.vaultPath),
		Status:       "vault loaded",
		Left:         nav.Render(),
		Right:        detail.Render(),
		CompactRight: detail.Title,
		Footer:       footer,
		Width:        m.width,
		Height:       m.height,
	}

	return shell.Render()
}
