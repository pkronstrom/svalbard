package tui

import (
	"strings"
)

// StepStatus represents the state of a progress step.
type StepStatus int

const (
	// StepPending indicates the step has not started.
	StepPending StepStatus = iota
	// StepActive indicates the step is currently running.
	StepActive
	// StepDone indicates the step completed successfully.
	StepDone
	// StepFailed indicates the step encountered an error.
	StepFailed
)

// ProgressStep represents a single step in a multi-step progress view.
type ProgressStep struct {
	Label  string
	Status StepStatus
	Error  string // only relevant when Status == StepFailed
}

// ProgressView renders a multi-step progress screen for long-running
// operations such as host apply, imports, indexing, or drive runtime actions.
type ProgressView struct {
	Theme Theme
	Title string
	Steps []ProgressStep
	Log   string // optional expandable log content
}

// Render produces the string representation of the progress view.
func (v ProgressView) Render() string {
	var b strings.Builder

	// Title
	if v.Title != "" {
		b.WriteString(v.Theme.Title.Render(v.Title))
		b.WriteString("\n\n")
	}

	// Steps
	for i, step := range v.Steps {
		var icon, label string
		switch step.Status {
		case StepPending:
			icon = v.Theme.Muted.Render("[    ]")
			label = v.Theme.Muted.Render(step.Label)
		case StepActive:
			icon = v.Theme.Focus.Render("[....]")
			label = v.Theme.Focus.Render(step.Label)
		case StepDone:
			icon = v.Theme.Success.Render("[done]")
			label = v.Theme.Success.Render(step.Label)
		case StepFailed:
			icon = v.Theme.Danger.Render("[FAIL]")
			label = v.Theme.Danger.Render(step.Label)
		}

		b.WriteString(icon + " " + label)

		// Error line for failed steps
		if step.Status == StepFailed && step.Error != "" {
			b.WriteString("\n")
			b.WriteString("       " + v.Theme.Error.Render(step.Error))
		}

		// Newline between steps (not after last)
		if i < len(v.Steps)-1 {
			b.WriteString("\n")
		}
	}

	// Log
	if v.Log != "" {
		b.WriteString("\n\n")
		b.WriteString(v.Theme.Muted.Render(v.Log))
	}

	return b.String()
}
