package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("124"))
	sectionStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("180"))
	selectedStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	descriptionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	statusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	errorStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("131"))
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedRowStyle    = lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("255"))
	selectedMutedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("252"))
	selectedSpacerStyle = lipgloss.NewStyle().Background(lipgloss.Color("240"))
	numberStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedNumberStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
)

const (
	menuGutter     = ""
	menuCaretSpace = "  "
	menuCaret      = "> "
	menuSubIndent  = "  "
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

	title := "⌕ Search"
	modeLabel := fmt.Sprintf("[%s]", strings.ToUpper(string(m.searchMode[:1]))+string(m.searchMode[1:]))
	b.WriteString(titleStyle.Render(title))
	b.WriteString(" ")
	b.WriteString(sectionStyle.Render(modeLabel))
	b.WriteString("\n")

	if m.searchInfo.SemanticEnabled {
		b.WriteString(descriptionStyle.Render("semantic available"))
		b.WriteString("\n")
	}

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
	b.WriteString("\n")

	cursor := ""
	if !m.searchResultsFocus {
		cursor = "█"
	}
	b.WriteString(fmt.Sprintf("> %s%s\n\n", m.searchQuery, cursor))

	shown := len(m.searchResults)
	b.WriteString(sectionStyle.Render(fmt.Sprintf("Results (%d shown, max 20)", shown)))
	b.WriteString("\n")
	if shown == 0 {
		b.WriteString(helpStyle.Render("No results yet. Type a query and press Enter."))
		b.WriteString("\n")
		return b.String()
	}

	for idx, result := range m.searchResults {
		line := renderSearchResultLine(idx+1, result.Filename, result.Title, idx == m.searchSelected)
		b.WriteString(line)
		b.WriteString("\n")
		if result.Snippet != "" {
			snippet := renderSearchResultSnippet(result.Snippet, idx == m.searchSelected)
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
	if selected.Snippet != "" {
		b.WriteString(descriptionStyle.Render("  Snippet: " + selected.Snippet))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter search/open • Tab mode • j/k move • Esc clear/back"))
	b.WriteString("\n")

	return b.String()
}

func sourceColor(filename string) lipgloss.Color {
	palette := []lipgloss.Color{
		lipgloss.Color("124"),
		lipgloss.Color("173"),
		lipgloss.Color("143"),
		lipgloss.Color("109"),
	}
	var sum int
	for _, b := range []byte(strings.ToLower(filename)) {
		sum += int(b)
	}
	return palette[sum%len(palette)]
}

func renderResultNumber(index int, filename string, selected bool) string {
	style := numberStyle.Foreground(sourceColor(filename))
	if selected {
		style = selectedNumberStyle.Foreground(sourceColor(filename)).Background(lipgloss.Color("240"))
	}
	return style.Render(fmt.Sprintf("%02d", index))
}

func renderSearchResultLine(index int, filename, title string, selected bool) string {
	number := renderResultNumber(index, filename, selected)
	if !selected {
		return fmt.Sprintf("%s  %s", number, title)
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		number,
		selectedSpacerStyle.Render("  "),
		selectedRowStyle.Render(title),
	)
}

func renderSearchResultSnippet(snippet string, selected bool) string {
	value := "    " + snippet
	if selected {
		return selectedMutedStyle.Render(value)
	}
	return descriptionStyle.Render(value)
}

func renderTopLevelView(b *strings.Builder, m Model) {
	b.WriteString(titleStyle.Render("Svalbard"))
	b.WriteString("\n")
	b.WriteString("\n")

	visible := m.VisibleGroups()
	if len(visible) == 0 {
		b.WriteString(helpStyle.Render("No groups match the current filter."))
		b.WriteString("\n")
		return
	}

	for idx, group := range visible {
		label := menuRow(displayGroupLabel(group.ID, group.Label), false)
		if idx == m.groupSelected {
			label = selectedStyle.Render(menuRow(displayGroupLabel(group.ID, group.Label), true))
		}
		b.WriteString(label)
		b.WriteString("\n")
	}

	if group, ok := m.SelectedGroup(); ok {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Selected"))
		b.WriteString("\n")
		b.WriteString(descriptionStyle.Render(group.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k or arrows: move • Enter: open • Esc/q: quit"))
	b.WriteString("\n")
}

func renderGroupView(b *strings.Builder, m Model) {
	group, ok := m.CurrentGroup()
	if !ok {
		renderTopLevelView(b, m)
		return
	}

	b.WriteString(titleStyle.Render("Svalbard / " + group.Label))
	b.WriteString("\n")
	b.WriteString(descriptionStyle.Render(group.Description))
	b.WriteString("\n")
	b.WriteString("\n")

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

		label := menuRow(item.Label, false)
		if item.Subheader != "" {
			label = menuIndentedRow(item.Label, false)
		}
		if idx == m.itemSelected {
			if item.Subheader != "" {
				label = selectedStyle.Render(menuIndentedRow(item.Label, true))
			} else {
				label = selectedStyle.Render(menuRow(item.Label, true))
			}
		}
		b.WriteString(label)
		b.WriteString("\n")
	}

	if item, ok := m.SelectedItem(); ok {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Selected"))
		b.WriteString("\n")
		b.WriteString(descriptionStyle.Render(item.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k or arrows: move • Enter: launch • Esc/q: back"))
	b.WriteString("\n")
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

func displayGroupLabel(id, label string) string {
	if id == "search" {
		return "⌕ " + label
	}
	return label
}

func menuRow(label string, selected bool) string {
	prefix := menuCaretSpace
	if selected {
		prefix = menuCaret
	}
	return prefix + label
}

func menuIndentedRow(label string, selected bool) string {
	return menuSubIndent + menuRow(label, selected)
}
