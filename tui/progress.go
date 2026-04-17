package tui

import (
	"fmt"
	"strings"
)

// ProgressStep represents a single step in a multi-step progress view.
type ProgressStep struct {
	ID         string // unique identifier
	Label      string // display label (falls back to ID if empty)
	Status     string // StatusDone, StatusActive, StatusFailed, StatusIndexing, StatusSkip, or ""
	Step       string // current build step name (e.g. "wget", "warc2zim")
	Downloaded int64  // bytes downloaded so far
	Total      int64  // total bytes (-1 if unknown)
	Error      string // error message (only for StatusFailed)
}

// ProgressView renders a multi-step progress screen for long-running
// operations such as apply, imports, indexing, or drive runtime actions.
type ProgressView struct {
	Theme        Theme
	Steps        []ProgressStep
	MaxVisible   int // max rows to show (0 = show all)
	ScrollToTail bool // if true, scroll to show latest steps instead of first
}

// StatusIcon returns the themed icon string for a status value.
func StatusIcon(status string, theme Theme) string {
	switch status {
	case StatusDone:
		return theme.Success.Render("✓")
	case StatusActive:
		return theme.Warning.Render("↓")
	case StatusFailed:
		return theme.Danger.Render("✗")
	case StatusIndexing:
		return theme.Warning.Render("·")
	case StatusSkip:
		return theme.Muted.Render("–")
	default:
		return theme.Muted.Render(" ")
	}
}

// Render produces the string representation of the progress view.
func (v ProgressView) Render() string {
	var b strings.Builder

	maxVis := v.MaxVisible
	if maxVis <= 0 || maxVis > len(v.Steps) {
		maxVis = len(v.Steps)
	}

	start := 0
	if v.ScrollToTail && len(v.Steps) > maxVis {
		start = len(v.Steps) - maxVis
	}
	end := start + maxVis
	if end > len(v.Steps) {
		end = len(v.Steps)
	}

	if start > 0 {
		b.WriteString(v.Theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		step := v.Steps[i]
		symbol := StatusIcon(step.Status, v.Theme)

		label := step.Label
		if label == "" {
			label = step.ID
		}

		// Append progress info for active items.
		if step.Status == StatusActive {
			if step.Downloaded > 0 {
				if step.Total > 0 {
					pct := int(float64(step.Downloaded) / float64(step.Total) * 100)
					label += fmt.Sprintf("  %s/%s  %d%%",
						FormatBytes(step.Downloaded), FormatBytes(step.Total), pct)
				} else {
					label += fmt.Sprintf("  %s", FormatBytes(step.Downloaded))
				}
			} else if step.Step != "" {
				label += fmt.Sprintf("  %s", step.Step)
			}
		}

		// Append inline error (truncated).
		if step.Error != "" {
			errMsg := step.Error
			if len(errMsg) > 60 {
				errMsg = errMsg[:60] + "..."
			}
			label += "  " + errMsg
		}

		// Style the label based on status.
		var styledLabel string
		switch step.Status {
		case StatusDone:
			styledLabel = v.Theme.Base.Render(label)
		case StatusActive:
			styledLabel = v.Theme.Base.Render(label)
		case StatusFailed:
			styledLabel = v.Theme.Danger.Render(label)
		case StatusIndexing:
			styledLabel = v.Theme.Base.Render(label)
		default:
			styledLabel = v.Theme.Muted.Render(label)
		}

		b.WriteString(fmt.Sprintf("  %s  %s\n", symbol, styledLabel))
	}

	if end < len(v.Steps) {
		b.WriteString(v.Theme.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}

	return b.String()
}

// RenderSummary returns a summary line like "3/11 done  2 active  1 failed".
func (v ProgressView) RenderSummary() string {
	done, active, failed := 0, 0, 0
	for _, s := range v.Steps {
		switch s.Status {
		case StatusDone:
			done++
		case StatusActive:
			active++
		case StatusFailed:
			failed++
		}
	}
	total := len(v.Steps)
	summary := fmt.Sprintf("%d/%d done", done, total)
	if active > 0 {
		summary += fmt.Sprintf("  %d active", active)
	}
	result := v.Theme.Muted.Render(summary)
	if failed > 0 {
		result += "  " + v.Theme.Danger.Render(fmt.Sprintf("%d failed", failed))
	}
	return result
}
