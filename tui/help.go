package tui

import "strings"

// HelpSection is a group of keybindings shown in the help overlay.
type HelpSection struct {
	Title    string
	Bindings []HelpBinding
}

// HelpBinding is a single key → description pair.
type HelpBinding struct {
	Key  string
	Desc string
}

// RenderHelp renders a full-screen help overlay from the given sections.
func RenderHelp(theme Theme, sections []HelpSection) string {
	var b strings.Builder

	b.WriteString(theme.Section.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	for i, section := range sections {
		if section.Title != "" {
			b.WriteString(theme.Focus.Render(section.Title))
			b.WriteString("\n")
		}

		// Find max key width for alignment
		maxKey := 0
		for _, bind := range section.Bindings {
			if len(bind.Key) > maxKey {
				maxKey = len(bind.Key)
			}
		}

		for _, bind := range section.Bindings {
			key := bind.Key
			for len(key) < maxKey {
				key += " "
			}
			b.WriteString("  " + theme.Selected.Render(key) + "  " + theme.Muted.Render(bind.Desc) + "\n")
		}

		if i < len(sections)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(theme.Help.Render("Press ? or Esc to close"))

	return b.String()
}
