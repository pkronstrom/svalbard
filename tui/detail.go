package tui

import "strings"

// DetailField represents a single key-value pair shown in the detail pane.
type DetailField struct {
	Label string
	Value string
}

// DetailPane renders contextual summary information for the focused
// navigation item on the right side of the dashboard.
type DetailPane struct {
	Theme  Theme
	Title  string
	Fields []DetailField
	Body   string // optional paragraph below the fields
}

// Render produces the styled string representation of the detail pane.
//
// Layout:
//   - Title (Section style) + newline, if non-empty
//   - Fields as right-padded label/value pairs, if non-empty
//   - Blank line + Body (Muted style), if Body is non-empty and Fields exist
//   - Body (Muted style) directly after title, if Body is non-empty but no Fields
//   - Fallback "No details available." (Muted style) if everything is empty
func (d DetailPane) Render() string {
	hasTitle := d.Title != ""
	hasFields := len(d.Fields) > 0
	hasBody := d.Body != ""

	// Empty state
	if !hasTitle && !hasFields && !hasBody {
		return d.Theme.Muted.Render("No details available.")
	}

	var b strings.Builder

	// Title
	if hasTitle {
		b.WriteString(d.Theme.Section.Render(d.Title))
		b.WriteString("\n")
	}

	// Fields
	if hasFields {
		// Determine the max label width for alignment
		maxLen := 0
		for _, f := range d.Fields {
			if len(f.Label) > maxLen {
				maxLen = len(f.Label)
			}
		}

		for _, f := range d.Fields {
			label := d.Theme.Muted.Render(padRight(f.Label, maxLen))
			value := d.Theme.Base.Render(f.Value)
			b.WriteString(label + "  " + value + "\n")
		}
	}

	// Body
	if hasBody {
		if hasFields {
			b.WriteString("\n")
		}
		b.WriteString(d.Theme.Muted.Render(d.Body))
	}

	return b.String()
}

// padRight pads string s with trailing spaces until it reaches width n.
// If s is already at least n characters, it is returned unchanged.
func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
