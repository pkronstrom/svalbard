package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	sectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	descriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func renderView(m Model) string {
	if m.showingOutput {
		return renderOutputView(m)
	}

	var b strings.Builder

	if m.inGroup {
		renderGroupView(&b, m)
	} else {
		renderTopLevelView(&b, m)
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

func renderTopLevelView(b *strings.Builder, m Model) {
	b.WriteString(titleStyle.Render("Svalbard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k or arrows: move • /: filter • Enter: open • q: quit"))
	b.WriteString("\n\n")

	filter := m.filter
	if filter == "" {
		filter = "all groups"
	}
	b.WriteString(fmt.Sprintf("Filter: %s\n\n", filter))

	visible := m.VisibleGroups()
	if len(visible) == 0 {
		b.WriteString(helpStyle.Render("No groups match the current filter."))
		b.WriteString("\n")
		return
	}

	for idx, group := range visible {
		label := "  " + group.Label
		if idx == m.groupSelected {
			label = selectedStyle.Render("> " + group.Label)
		}
		b.WriteString(label)
		b.WriteString("\n")
		b.WriteString(descriptionStyle.Render("    " + group.Description))
		b.WriteString("\n")
	}
}

func renderGroupView(b *strings.Builder, m Model) {
	group, ok := m.CurrentGroup()
	if !ok {
		renderTopLevelView(b, m)
		return
	}

	b.WriteString(titleStyle.Render("Svalbard / " + group.Label))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k or arrows: move • /: filter • Enter: launch • Esc: back • q: quit"))
	b.WriteString("\n\n")
	b.WriteString(descriptionStyle.Render(group.Description))
	b.WriteString("\n\n")

	filter := m.filter
	if filter == "" {
		filter = "all items"
	}
	b.WriteString(fmt.Sprintf("Filter: %s\n\n", filter))

	visible := m.VisibleItems()
	if len(visible) == 0 {
		b.WriteString(helpStyle.Render("No items match the current filter."))
		b.WriteString("\n")
		return
	}

	currentSubheader := ""
	for idx, item := range visible {
		if item.Subheader != "" && item.Subheader != currentSubheader {
			currentSubheader = item.Subheader
			b.WriteString(sectionStyle.Render(currentSubheader))
			b.WriteString("\n")
		}

		label := "  " + item.Label
		if idx == m.itemSelected {
			label = selectedStyle.Render("> " + item.Label)
		}
		b.WriteString(label)
		b.WriteString("\n")
		b.WriteString(descriptionStyle.Render("    " + item.Description))
		b.WriteString("\n")
	}
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
