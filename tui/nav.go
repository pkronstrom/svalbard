package tui

import "strings"

const (
	caretSpace = "  "
	caret      = "> "
	subIndent  = "  "
)

// MaxVisibleRows returns how many rows fit in a viewport of the given height
// after reserving `reserve` lines for surrounding chrome, never dropping below
// floor.
func MaxVisibleRows(height, reserve, floor int) int {
	v := height - reserve
	if v < floor {
		return floor
	}
	return v
}

// NavItem represents a single entry in a navigation list.
type NavItem struct {
	ID          string
	Label       string
	Description string // short description shown alongside label
	Subheader   string // optional — groups items under a section header
	Separator   bool   // render a separator line before this item
}

// NavList is a navigable list of items used in the left pane of dashboards and pickers.
type NavList struct {
	Items       []NavItem
	Selected    int
	Theme       Theme
	ShowNumbers bool // render 1-9 number prefixes for shortcut keys
	Width       int  // available width for rendering (0 = use default separator)
}

// MoveDown increments Selected, clamping to bounds.
func (nl *NavList) MoveDown() {
	nl.Clamp()
	if nl.Selected < len(nl.Items)-1 {
		nl.Selected++
	}
}

// MoveUp decrements Selected, clamping to bounds.
func (nl *NavList) MoveUp() {
	nl.Clamp()
	if nl.Selected > 0 {
		nl.Selected--
	}
}

// Clamp ensures Selected is within bounds.
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
			sepWidth := nl.Width - 4
			if sepWidth < 6 {
				sepWidth = 6
			}
			b.WriteString(nl.Theme.Muted.Render("  " + strings.Repeat("─", sepWidth)))
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
		if i == nl.Selected {
			line.WriteString(nl.Theme.Selected.Render(prefix + label))
		} else {
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
