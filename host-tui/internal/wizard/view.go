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
	case stageApply:
		right = m.applyModel.View()
	case stageIndex:
		right = m.indexModel.View()
	}

	var footer string
	switch m.stage {
	case stagePath:
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: confirm"},
			tui.KeyBinding{Key: "j", Label: "j/k: quick select"},
			tui.KeyBinding{Key: "esc", Label: "q/Esc: back"},
		)
	case stagePlatforms:
		footer = tui.FooterHints(
			tui.KeyBinding{Key: " ", Label: "Space: toggle"},
			tui.KeyBinding{Key: "enter", Label: "Enter: next"},
			tui.KeyBinding{Key: "esc", Label: "q/Esc: back"},
		)
	case stagePreset:
		footer = tui.FooterHints(
			m.keys.MoveUp,
			tui.KeyBinding{Key: "enter", Label: "Enter: select"},
			tui.KeyBinding{Key: "esc", Label: "q/Esc: back"},
		)
	case stagePacks:
		footer = tui.FooterHints(
			m.keys.MoveUp,
			tui.KeyBinding{Key: " ", Label: "Space: toggle"},
			tui.KeyBinding{Key: "enter", Label: "Enter: review"},
			tui.KeyBinding{Key: "esc", Label: "Esc: back"},
		)
	case stageReview:
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: confirm"},
			tui.KeyBinding{Key: "esc", Label: "Esc: back"},
		)
	case stageApply, stageIndex:
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: continue"},
		)
	default:
		footer = tui.FooterHints(m.keys.Enter, m.keys.Back)
	}

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
