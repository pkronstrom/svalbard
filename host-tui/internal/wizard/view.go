package wizard

import "github.com/pkronstrom/svalbard/tui"

// View renders the wizard UI — left pane shows steps, right pane shows
// context for the current step.
func (m Model) View() string {
	// Build step navigation list
	items := make([]tui.NavItem, len(wizardSteps))
	for i, step := range wizardSteps {
		items[i] = tui.NavItem{
			ID:    step.id,
			Label: step.label,
		}
	}

	nav := tui.NavList{
		Items:    items,
		Selected: m.currentStep,
		Theme:    m.theme,
	}

	detail := contextForStep(m)

	footer := tui.FooterHints(
		m.keys.Enter,
		m.keys.Back,
	)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard Init",
		Left:    nav.Render(),
		Right:   detail.Render(),
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

// contextForStep returns the DetailPane content for the currently active wizard step.
func contextForStep(m Model) tui.DetailPane {
	dp := tui.DetailPane{Theme: m.theme}

	switch wizardSteps[m.currentStep].id {
	case "path":
		dp.Title = "Vault Path"
		if m.pathValue != "" {
			dp.Body = "Path: " + m.pathValue + "\nPress Enter to continue."
		} else {
			dp.Body = "Choose the directory for your vault."
		}

	case "preset":
		dp.Title = "Choose Preset"
		dp.Body = "Select a preset to configure the vault's initial content."

	case "adjust":
		dp.Title = "Adjust Contents"
		dp.Body = "Add or remove items from the preset selection."

	case "review":
		dp.Title = "Review Plan"
		dp.Body = "Review what will be downloaded and configured."

	case "apply":
		dp.Title = "Apply"
		dp.Body = "Execute the setup. Downloads content and builds the vault."
	}

	return dp
}
