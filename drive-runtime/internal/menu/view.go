package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	sectionStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	selectedStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	descriptionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedRowStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	numberStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedNumberStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
)

func renderView(m Model) string {
	if m.searchActive {
		return renderSearchView(m)
	}
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

func renderSearchView(m Model) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Svalbard / Search"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: search/open • Tab: toggle mode • j/k: move • Esc: clear/back"))
	b.WriteString("\n\n")

	modeLine := fmt.Sprintf("Mode: %s", m.searchMode)
	if m.searchInfo.SemanticEnabled {
		modeLine += " • semantic available"
	}
	b.WriteString(sectionStyle.Render(modeLine))
	b.WriteString("\n")

	status := m.searchStatus
	if status == "" {
		status = fmt.Sprintf("%d sources • %d articles", m.searchInfo.SourceCount, m.searchInfo.ArticleCount)
	}
	if m.searchLoading {
		status = "Working... " + status
	}
	if m.searchErr != nil {
		b.WriteString(errorStyle.Render(status + ": " + m.searchErr.Error()))
	} else {
		b.WriteString(descriptionStyle.Render(status))
	}
	b.WriteString("\n\n")

	cursor := ""
	if !m.searchResultsFocus {
		cursor = "█"
	}
	b.WriteString(fmt.Sprintf("Query: %s%s\n\n", m.searchQuery, cursor))

	shown := len(m.searchResults)
	b.WriteString(sectionStyle.Render(fmt.Sprintf("Results (%d shown, max 20)", shown)))
	b.WriteString("\n")
	if shown == 0 {
		b.WriteString(helpStyle.Render("No results yet. Type a query and press Enter."))
		b.WriteString("\n")
		return b.String()
	}

	for idx, result := range m.searchResults {
		number := numberStyle.Render(fmt.Sprintf("%02d", idx+1))
		if idx == m.searchSelected {
			number = selectedNumberStyle.Render(fmt.Sprintf("%02d", idx+1))
		}
		line := fmt.Sprintf("%s  %s %s", number, sourceDot(result.Filename), result.Title)
		if idx == m.searchSelected {
			line = selectedRowStyle.Render(" " + line + " ")
		}
		b.WriteString(line)
		b.WriteString("\n")
		if result.Snippet != "" {
			snippet := descriptionStyle.Render("      " + result.Snippet)
			if idx == m.searchSelected {
				snippet = selectedRowStyle.Render(" " + snippet + " ")
			}
			b.WriteString(snippet)
			b.WriteString("\n")
		}
	}

	selected := m.searchResults[m.searchSelected]
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Selected Result"))
	b.WriteString("\n")
	b.WriteString(descriptionStyle.Render("  Source: " + selected.Filename))
	b.WriteString("\n")
	b.WriteString(descriptionStyle.Render("  Path:   " + selected.Path))
	b.WriteString("\n")
	b.WriteString(descriptionStyle.Render("  Mode:   " + string(m.searchMode)))
	b.WriteString("\n")
	if selected.Snippet != "" {
		b.WriteString(descriptionStyle.Render("  Snippet: " + selected.Snippet))
		b.WriteString("\n")
	}

	return b.String()
}

func sourceDot(filename string) string {
	palette := []lipgloss.Color{
		lipgloss.Color("2"),
		lipgloss.Color("4"),
		lipgloss.Color("5"),
		lipgloss.Color("6"),
		lipgloss.Color("11"),
		lipgloss.Color("12"),
		lipgloss.Color("13"),
		lipgloss.Color("14"),
	}
	var sum int
	for _, b := range []byte(strings.ToLower(filename)) {
		sum += int(b)
	}
	color := palette[sum%len(palette)]
	return lipgloss.NewStyle().Foreground(color).Render("●")
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
	b.WriteString(helpStyle.Render("j/k or arrows: move • /: filter • Enter: launch • Esc/q: back"))
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
