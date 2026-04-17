package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pkronstrom/svalbard/tui"
)

const (
	menuGutter     = ""
	menuCaretSpace = "  "
	menuCaret      = "> "
	menuSubIndent  = "  "
)

func renderView(m Model) string {
	if m.paletteActive {
		return m.paletteModel.View()
	}
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
			b.WriteString(m.theme.Error.Render(fmt.Sprintf("%s: %v", m.status, m.lastErr)))
		} else {
			b.WriteString(m.theme.Status.Render(m.status))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderSearchView(m Model) string {
	var b strings.Builder

	title := "⌕ Search"
	modeLabel := fmt.Sprintf("[%s]", strings.ToUpper(string(m.searchMode[:1]))+string(m.searchMode[1:]))
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString(" ")
	b.WriteString(m.theme.Section.Render(modeLabel))
	b.WriteString("\n")

	if m.searchInfo.SemanticEnabled {
		b.WriteString(m.theme.Muted.Render("semantic available"))
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
		b.WriteString(m.theme.Error.Render(status + ": " + m.searchErr.Error()))
	} else {
		b.WriteString(m.theme.Muted.Render(status))
	}
	b.WriteString("\n")

	cursor := ""
	if !m.searchResultsFocus {
		cursor = "█"
	}
	b.WriteString(fmt.Sprintf("> %s%s\n\n", m.searchQuery, cursor))

	shown := len(m.searchResults)
	b.WriteString(m.theme.Section.Render(fmt.Sprintf("Results (%d shown, max 20)", shown)))
	b.WriteString("\n")
	if shown == 0 {
		b.WriteString(m.theme.Help.Render("No results yet. Type a query and press Enter."))
		b.WriteString("\n")
		return b.String()
	}

	maxVis := m.searchMaxVisible()
	start := m.searchScrollOffset
	end := start + maxVis
	if end > len(m.searchResults) {
		end = len(m.searchResults)
	}
	if start > 0 {
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  ↑ %d more", start)))
		b.WriteString("\n")
	}
	for idx := start; idx < end; idx++ {
		result := m.searchResults[idx]
		line := renderSearchResultLine(m, idx+1, result.Filename, result.Title, idx == m.searchSelected)
		b.WriteString(line)
		b.WriteString("\n")
	}
	if end < len(m.searchResults) {
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  ↓ %d more", len(m.searchResults)-end)))
		b.WriteString("\n")
	}

	if m.searchSelected >= len(m.searchResults) {
		return b.String()
	}
	selected := m.searchResults[m.searchSelected]
	b.WriteString("\n")
	b.WriteString(m.theme.Section.Render("Selected Result"))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  Source: " + selected.Filename))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  Path:   " + selected.Path))
	b.WriteString("\n")
	if selected.Snippet != "" {
		b.WriteString(m.theme.Muted.Render("  Snippet: " + selected.Snippet))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("Enter search/open • Tab mode • j/k move • Esc clear/back"))
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

func renderResultNumber(m Model, index int, filename string, selected bool) string {
	style := m.theme.Muted.Foreground(sourceColor(filename))
	if selected {
		style = m.theme.Selected.Foreground(sourceColor(filename)).Background(m.theme.SelectedRow.GetBackground())
	}
	return style.Render(fmt.Sprintf("%02d", index))
}

func renderSearchResultLine(m Model, index int, filename, title string, selected bool) string {
	number := renderResultNumber(m, index, filename, selected)
	if !selected {
		return fmt.Sprintf("%s  %s", number, title)
	}
	selectedSpacerStyle := lipgloss.NewStyle().Background(m.theme.SelectedRow.GetBackground())
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		number,
		selectedSpacerStyle.Render("  "),
		m.theme.SelectedRow.Render(title),
	)
}

func renderSearchResultSnippet(m Model, snippet string, selected bool) string {
	value := "    " + snippet
	if selected {
		return m.theme.SelectedMuted.Render(value)
	}
	return m.theme.Muted.Render(value)
}

func renderTopLevelView(b *strings.Builder, m Model) {
	visible := m.VisibleGroups()
	keys := tui.DefaultKeyMap()
	footer := tui.FooterHints(keys.MoveUp, keys.Enter, keys.Back, keys.Quit)

	if len(visible) == 0 {
		shell := tui.ShellLayout{
			Theme:    m.theme,
			AppName:  "Svalbard",
			Identity: m.cfg.Preset,
			Left:     m.theme.Help.Render("No groups match the current filter."),
			Right:    "",
			Footer:   footer,
			Width:    m.width,
			Height:   m.height,
		}
		b.WriteString(shell.Render())
		return
	}

	// Build NavList from visible groups
	items := make([]tui.NavItem, len(visible))
	for i, group := range visible {
		items[i] = tui.NavItem{
			ID:    group.ID,
			Label: displayGroupLabel(group.ID, group.Label),
		}
	}
	nav := tui.NavList{
		Items:    items,
		Selected: m.groupSelected,
		Theme:    m.theme,
	}

	// Build DetailPane for the selected group
	var detail tui.DetailPane
	if group, ok := m.SelectedGroup(); ok {
		detail = contextForGroup(group, m.theme)
	} else {
		detail = tui.DetailPane{Theme: m.theme}
	}

	shell := tui.ShellLayout{
		Theme:    m.theme,
		AppName:  "Svalbard",
		Identity: m.cfg.Preset,
		Left:     nav.Render(),
		Right:    detail.Render(),
		Footer:   footer,
		Width:    m.width,
		Height:   m.height,
	}
	b.WriteString(shell.Render())
}

func renderGroupView(b *strings.Builder, m Model) {
	group, ok := m.CurrentGroup()
	if !ok {
		renderTopLevelView(b, m)
		return
	}

	b.WriteString(m.theme.Title.Render("Svalbard / " + group.Label))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render(group.Description))
	b.WriteString("\n")
	b.WriteString("\n")

	visible := m.VisibleItems()
	if len(visible) == 0 {
		b.WriteString(m.theme.Help.Render("No items match the current filter."))
		b.WriteString("\n")
		return
	}

	currentSubheader := ""
	for idx, item := range visible {
		if item.Subheader != "" && item.Subheader != currentSubheader {
			currentSubheader = item.Subheader
			b.WriteString(m.theme.Section.Render(currentSubheader))
			b.WriteString("\n")
		}

		label := menuRow(item.Label, false)
		if item.Subheader != "" {
			label = menuIndentedRow(item.Label, false)
		}
		if idx == m.itemSelected {
			if item.Subheader != "" {
				label = m.theme.Selected.Render(menuIndentedRow(item.Label, true))
			} else {
				label = m.theme.Selected.Render(menuRow(item.Label, true))
			}
		}
		b.WriteString(label)
		b.WriteString("\n")
	}

	if item, ok := m.SelectedItem(); ok {
		b.WriteString("\n")
		b.WriteString(m.theme.Section.Render("Selected"))
		b.WriteString("\n")
		b.WriteString(m.theme.Muted.Render(item.Description))
		b.WriteString("\n")
	}

	keys := tui.DefaultKeyMap()
	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render(tui.FooterHints(keys.MoveUp, keys.Enter, keys.Back, keys.Quit)))
	b.WriteString("\n")
}

func renderOutputView(m Model) string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render("Svalbard"))
	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("Enter or Esc: back • q: quit"))
	b.WriteString("\n\n")
	b.WriteString(m.output)
	if !strings.HasSuffix(m.output, "\n") {
		b.WriteString("\n")
	}
	if m.lastErr != nil {
		b.WriteString("\n")
		b.WriteString(m.theme.Error.Render(fmt.Sprintf("Action failed: %v", m.lastErr)))
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
