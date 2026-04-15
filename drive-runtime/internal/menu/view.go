package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	sectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func renderView(m Model) string {
	if m.showingOutput {
		return renderOutputView(m)
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Svalbard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k or arrows: move • /: filter • Enter: launch • q: quit"))
	b.WriteString("\n\n")

	filter := m.filter
	if filter == "" {
		filter = "all actions"
	}
	b.WriteString(fmt.Sprintf("Filter: %s\n\n", filter))

	visible := m.VisibleActions()
	if len(visible) == 0 {
		b.WriteString(helpStyle.Render("No actions match the current filter."))
		b.WriteString("\n")
	} else {
		currentSection := ""
		for idx, action := range visible {
			if action.Section != currentSection {
				currentSection = action.Section
				b.WriteString(sectionStyle.Render(strings.ToUpper(currentSection)))
				b.WriteString("\n")
			}

			line := fmt.Sprintf("  %s", action.Label)
			if idx == m.selected {
				line = selectedStyle.Render("> " + action.Label)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if m.status != "" {
		b.WriteString("\n")
		if m.lastErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("%s: %v", m.status, m.lastErr)))
		} else {
			b.WriteString(statusStyle.Render(m.status))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderOutputView(m Model) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Svalbard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter or Esc: back • q: quit"))
	b.WriteString("\n\n")
	b.WriteString(m.output)
	if !strings.HasSuffix(m.output, "\n") {
		b.WriteString("\n")
	}
	if m.lastErr != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Action failed: %v", m.lastErr)))
		b.WriteString("\n")
	}
	return b.String()
}

func groupSections(actions []config.MenuAction) []string {
	sections := make([]string, 0)
	seen := map[string]bool{}
	for _, action := range actions {
		if seen[action.Section] {
			continue
		}
		seen[action.Section] = true
		sections = append(sections, action.Section)
	}
	return sections
}
