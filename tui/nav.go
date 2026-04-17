package tui

import "strings"

const (
	caretSpace = "  "
	caret      = "> "
	subIndent  = "  "
)

// NavItem represents a single entry in a navigation list.
type NavItem struct {
	ID          string
	Label       string
	Description string // short description shown alongside label
	Subheader   string // optional — groups items under a section header
	Separator   bool   // render a separator line before this item
	Disabled    bool   // visible but not activatable
}

// NavList is a navigable list of items used in the left pane of dashboards and pickers.
type NavList struct {
	Items       []NavItem
	Selected    int
	Theme       Theme
	ShowNumbers bool // render 1-9 number prefixes for shortcut keys
}

// MoveDown increments Selected, clamping to bounds and skipping disabled items.
func (nl *NavList) MoveDown() {
	if len(nl.Items) == 0 {
		return
	}
	for i := nl.Selected + 1; i < len(nl.Items); i++ {
		if !nl.Items[i].Disabled {
			nl.Selected = i
			return
		}
	}
	// No non-disabled item found below; stay put.
}

// MoveUp decrements Selected, clamping to bounds and skipping disabled items.
func (nl *NavList) MoveUp() {
	if len(nl.Items) == 0 {
		return
	}
	for i := nl.Selected - 1; i >= 0; i-- {
		if !nl.Items[i].Disabled {
			nl.Selected = i
			return
		}
	}
	// No non-disabled item found above; stay put.
}

// Clamp ensures Selected is within bounds and not on a disabled item.
func (nl *NavList) Clamp() {
	if len(nl.Items) == 0 {
		nl.Selected = 0
		return
	}
	if nl.Selected < 0 {
		nl.Selected = 0
	}
	if nl.Selected >= len(nl.Items) {
		nl.Selected = len(nl.Items) - 1
	}
	// If clamped to a disabled item, find nearest enabled
	if nl.Items[nl.Selected].Disabled {
		// Try forward first
		for i := nl.Selected; i < len(nl.Items); i++ {
			if !nl.Items[i].Disabled {
				nl.Selected = i
				return
			}
		}
		// Then backward
		for i := nl.Selected; i >= 0; i-- {
			if !nl.Items[i].Disabled {
				nl.Selected = i
				return
			}
		}
	}
}

// SelectedItem returns the currently selected NavItem.
// Returns false if the list is empty.
func (nl *NavList) SelectedItem() (NavItem, bool) {
	if len(nl.Items) == 0 {
		return NavItem{}, false
	}
	nl.Clamp()
	return nl.Items[nl.Selected], true
}

// Render renders the full navigation list as a string.
func (nl *NavList) Render() string {
	if len(nl.Items) == 0 {
		return ""
	}

	var b strings.Builder
	prevSubheader := ""
	hasSubheaders := false
	num := 0 // visible item number for shortcuts

	// Check if any item has a subheader
	for _, item := range nl.Items {
		if item.Subheader != "" {
			hasSubheaders = true
			break
		}
	}

	for i, item := range nl.Items {
		// Separator line before this item
		if item.Separator && i > 0 {
			b.WriteString(nl.Theme.Muted.Render("  ─────────────────────────────"))
			b.WriteString("\n")
		}

		// Subheader grouping
		if item.Subheader != "" && item.Subheader != prevSubheader {
			// Blank line between groups (except before first)
			if prevSubheader != "" {
				b.WriteString("\n")
			}
			b.WriteString(nl.Theme.Section.Render(item.Subheader))
			b.WriteString("\n")
			prevSubheader = item.Subheader
		}

		num++

		// Build the line
		var line strings.Builder

		// Indent items under subheaders
		if hasSubheaders {
			line.WriteString(subIndent)
		}

		// Caret
		if i == nl.Selected {
			line.WriteString(caret)
		} else {
			line.WriteString(caretSpace)
		}

		// Optional number prefix
		prefix := ""
		if nl.ShowNumbers {
			if num <= 9 {
				prefix = string(rune('0'+num)) + "  "
			} else {
				prefix = "   "
			}
		}

		// Label with appropriate style
		label := item.Label
		switch {
		case item.Disabled:
			line.WriteString(nl.Theme.Muted.Render(prefix + label))
		case i == nl.Selected:
			line.WriteString(nl.Theme.Selected.Render(prefix + label))
		default:
			line.WriteString(nl.Theme.Base.Render(prefix + label))
		}

		b.WriteString(line.String())

		// Description on second line, dimmed
		if item.Description != "" {
			descIndent := caretSpace
			if hasSubheaders {
				descIndent = subIndent + descIndent
			}
			if nl.ShowNumbers {
				descIndent += "   "
			}
			b.WriteString("\n" + descIndent + nl.Theme.Muted.Render(item.Description))
		}

		// Newline between items (not after last)
		if i < len(nl.Items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
