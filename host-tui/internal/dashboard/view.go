package dashboard

import (
	"fmt"
	"path/filepath"

	"github.com/pkronstrom/svalbard/tui"
)

var dashboardHelp = []tui.HelpSection{
	{Title: "Navigation", Bindings: []tui.HelpBinding{
		{Key: "j/k", Desc: "Move up/down"},
		{Key: "1-6", Desc: "Jump to item"},
		{Key: "Enter", Desc: "Open selected screen"},
	}},
	{Title: "Actions", Bindings: []tui.HelpBinding{
		{Key: "Esc", Desc: "Back / quit"},
		{Key: "q", Desc: "Quit"},
		{Key: "?", Desc: "Toggle this help"},
		{Key: "Ctrl+C", Desc: "Force quit"},
	}},
}

// View renders the two-pane dashboard layout.
func (m Model) View() string {
	if m.showHelp {
		return tui.RenderHelp(m.theme, dashboardHelp)
	}

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

	// Footer key hints — override labels for host context.
	enter := m.keys.Enter
	enter.Label = "Enter: select"
	help := m.keys.Help
	help.Label = "?: help"
	footer := tui.FooterHints(
		m.keys.MoveUp,
		enter,
		m.keys.Back,
		help,
		m.keys.Quit,
	)

	shell := tui.ShellLayout{
		Theme:        m.theme,
		AppName:      "Svalbard",
		Identity:     filepath.Base(m.vaultPath),
		Status:       m.ambientStatus(),
		Left:         nav.Render(),
		Right:        detail.Render(),
		CompactRight: detail.Title,
		Footer:       footer,
		Width:        m.width,
		Height:       m.height,
	}

	return shell.Render()
}

func (m Model) ambientStatus() string {
	if m.status == nil {
		return ""
	}
	s := m.status
	status := fmt.Sprintf("%d/%d synced", s.RealizedCount, s.DesiredCount)
	if s.DiskFreeGB > 0 {
		status += fmt.Sprintf(" · %.0f GB free", s.DiskFreeGB)
	}
	return status
}
