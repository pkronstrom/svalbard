package wizard

import "github.com/pkronstrom/svalbard/tui"

// View renders the wizard UI — left pane shows stage list, right pane shows
// the active sub-model's interactive view.
func (m Model) View() string {
	// Build step navigation list (left pane)
	items := make([]tui.NavItem, len(wizardSteps))
	for i, step := range wizardSteps {
		items[i] = tui.NavItem{
			ID:    step.id,
			Label: step.label,
		}
	}

	nav := tui.NavList{
		Items:    items,
		Selected: int(m.stage),
		Theme:    m.theme,
	}

	// Right pane = active sub-model's view
	var right string
	switch m.stage {
	case stagePath:
		right = m.pathPicker.View()
	case stagePlatforms:
		right = m.platformPicker.View()
	case stagePreset:
		right = m.presetPicker.View()
	case stagePacks:
		right = m.packPicker.View()
	case stageReview:
		right = m.review.View()
	}

	footer := tui.FooterHints(m.keys.Enter, m.keys.Back)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard Init",
		Left:    nav.Render(),
		Right:   right,
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}
