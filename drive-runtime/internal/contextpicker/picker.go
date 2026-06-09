// Package contextpicker shows a small launch-time menu for choosing the
// llama-server context window, defaulting to the host-adaptive "Auto" value.
package contextpicker

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
)

var selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

type choice struct {
	label  string
	tokens int
}

func choicesFor(autoCtx int) []choice {
	return []choice{
		{fmt.Sprintf("Auto — %s  (recommended)", fmtTokens(autoCtx)), autoCtx},
		{fmt.Sprintf("Fast — %s  (snappier, less memory)", fmtTokens(llamaserve.CtxFast)), llamaserve.CtxFast},
		{fmt.Sprintf("Balanced — %s", fmtTokens(llamaserve.CtxBalanced)), llamaserve.CtxBalanced},
		{fmt.Sprintf("Max — %s  (may be slow / high memory)", fmtTokens(llamaserve.CtxMax)), llamaserve.CtxMax},
	}
}

func fmtTokens(t int) string {
	if t%1024 == 0 {
		return fmt.Sprintf("%dK", t/1024)
	}
	return fmt.Sprintf("%d", t)
}

// Pick shows the context menu and returns the chosen token count. On a
// non-interactive stdin (piped, no TTY) or any error it returns autoCtx
// silently.
func Pick(autoCtx int) int {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return autoCtx
	}
	res, err := tea.NewProgram(newModel(autoCtx)).Run()
	if err != nil {
		return autoCtx
	}
	if m, ok := res.(model); ok {
		return m.selected
	}
	return autoCtx
}

type model struct {
	choices  []choice
	cursor   int
	selected int
	done     bool
}

func newModel(autoCtx int) model {
	return model{choices: choicesFor(autoCtx), selected: autoCtx}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.choices)-1 {
			m.cursor++
		}
	case "enter":
		m.selected = m.choices[m.cursor].tokens
		m.done = true
		return m, tea.Quit
	case "esc", "q", "ctrl+c":
		// Keep the Auto default (selected is already autoCtx).
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	b.WriteString("Context window:\n")
	for i, c := range m.choices {
		if i == m.cursor {
			b.WriteString("▸ " + selectedStyle.Render(c.label) + "\n")
		} else {
			b.WriteString("  " + c.label + "\n")
		}
	}
	b.WriteString("\n↑/↓ select · enter confirm · esc keeps Auto\n")
	return b.String()
}
