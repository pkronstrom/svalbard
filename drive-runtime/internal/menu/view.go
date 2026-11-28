package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
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
		if m.archiveFilterOpen {
			return m.renderArchiveFilter()
		}
		return renderSearchView(m)
	}
	if m.settingsActive {
		return renderSettingsView(m)
	}
	if m.showingOutput {
		return renderOutputView(m)
	}

	var b strings.Builder

	if m.inGroup {
		if group, ok := m.CurrentGroup(); ok {
			return m.groupLayout(group).render(m.theme)
		}
	}
	renderTopLevelView(&b, m)

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(m.renderStatus())
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderStatus() string {
	if m.lastErr != nil {
		return m.theme.Error.Render(fmt.Sprintf("%s: %v", m.status, m.lastErr))
	}
	return m.theme.Status.Render(m.status)
}

func renderSearchView(m Model) string {
	var b strings.Builder

	title := "⌕ Search"
	modeLabel := fmt.Sprintf("[%s]", strings.ToUpper(string(m.searchMode[:1]))+string(m.searchMode[1:]))
	b.WriteString(m.theme.Title.Render(title))
	b.WriteString(" ")
	b.WriteString(m.theme.Section.Render(modeLabel))
	b.WriteString("\n")

	if m.searchInfo.HybridEnabled {
		b.WriteString(m.theme.Muted.Render("hybrid search available"))
		b.WriteString("\n")
	}

	status := m.searchStatus
	if status == "" {
		status = fmt.Sprintf("%d sources • %d articles", m.searchInfo.SourceCount, m.searchInfo.ArticleCount)
		if m.searchInfo.NativeZIMCount > 0 {
			status = fmt.Sprintf("%d sources • %d embedded indexes • %d SQLite entries",
				m.searchInfo.SourceCount, m.searchInfo.NativeZIMCount, m.searchInfo.ArticleCount)
		}
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

	b.WriteString("> ")
	b.WriteString(m.searchQuery.View())
	b.WriteString("\n")
	filterHelp := "Ctrl+F archives"
	if count := len(m.excludedArchives); count > 0 {
		filterHelp += fmt.Sprintf(" (%d excluded)", count)
	}
	b.WriteString(m.theme.Help.Render(filterHelp))
	b.WriteString("\n\n")

	shown := len(m.searchResults)
	b.WriteString(m.theme.Section.Render(fmt.Sprintf("Results (%d loaded)", shown)))
	if m.searchHasMore && m.searchResultsFocus && !m.searchLoading {
		b.WriteString(m.theme.Help.Render("  •  M load more"))
	}
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
	width := m.width
	if width <= 0 {
		width = 80
	}
	b.WriteString(m.theme.Muted.Render(shortSearchText("  Source: "+selected.Filename, width)))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render(shortSearchText("  Path:   "+selected.Path, width)))
	b.WriteString("\n")
	if selected.Snippet != "" {
		snippet := strings.Join(strings.Fields(selected.Snippet), " ")
		b.WriteString(m.theme.Muted.Render(shortSearchText("  Snippet: "+snippet, width)))
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
	width := m.width
	if width <= 0 {
		width = 80
	}
	const separator = " │ "
	room := width - lipgloss.Width(number) - 2 - lipgloss.Width(separator)
	sourceWidth := min(36, max(1, room/3))
	if archive, _, ok := engine.EmbeddedArchive(filename); ok {
		filename = archive
	}
	source := shortSearchText(search.SourceLabel(filename), sourceWidth)
	source = m.theme.Muted.Width(sourceWidth).Render(source)
	title = shortSearchText(strings.Join(strings.Fields(title), " "), max(1, room-sourceWidth))
	if selected {
		title = m.theme.SelectedRow.Render(title)
	}
	return number + "  " + source + m.theme.Muted.Render(separator) + title
}

func shortSearchText(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 1 {
		return "…"
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(text) + "…"
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
			Left:     m.theme.Help.Render("No groups available."),
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

func (m *Model) scrollGroup() {
	if group, ok := m.CurrentGroup(); ok {
		m.itemScrollOffset = m.groupLayout(group).offset
	}
}

func (m Model) groupLayout(group config.MenuGroup) menuListLayout {
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	oneLine := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	layout := menuListLayout{top: []string{
		clip.Render(m.theme.Title.Render("Svalbard / " + group.Label)),
		clip.Render(m.theme.Muted.Render(oneLine(group.Description))), "",
	}}
	visible := group.Items
	if len(visible) == 0 {
		layout.list = []string{clip.Render(m.theme.Help.Render("No items available."))}
	}

	noteColumn := menuNoteColumn(visible)
	currentSubheader := ""
	cursorLine, anchorLine := 0, 0
	for idx, item := range visible {
		anchor := len(layout.list)
		if item.Subheader != "" && item.Subheader != currentSubheader {
			currentSubheader = item.Subheader
			layout.list = append(layout.list, clip.Render(m.theme.Section.Render(currentSubheader)))
		}

		selected := idx == m.itemSelected
		row := menuItemRow(item, selected)
		note := menuNote(row, item.Note, noteColumn, m.width)
		if selected {
			row = m.theme.Selected.Render(row)
			cursorLine, anchorLine = len(layout.list), anchor
		}
		layout.list = append(layout.list, clip.Render(row+m.theme.Muted.Render(note)))
	}

	layout.bottom = []string{""}
	if m.itemSelected >= 0 && m.itemSelected < len(visible) {
		layout.bottom = append(layout.bottom, m.theme.Section.Render("Selected"),
			clip.Render(m.theme.Muted.Render(oneLine(visible[m.itemSelected].Description))), "")
	}
	if m.status != "" {
		layout.bottom = append(layout.bottom, clip.Render(strings.ReplaceAll(m.renderStatus(), "\n", " ")))
	}
	keys := tui.DefaultKeyMap()
	footer := clip.Render(m.theme.Help.Render(tui.FooterHints(keys.MoveUp, keys.Enter, keys.Back, keys.Quit)))
	layout.bottom = append(layout.bottom, footer)
	if m.height > 0 && m.height <= len(layout.top)+len(layout.bottom) {
		// When the terminal is very short, keep room for the selected row.
		layout.top, layout.bottom = layout.top[:1], nil
		if m.height >= 3 {
			layout.bottom = []string{footer}
		}
		if m.height >= 5 {
			layout.top = append(layout.top, "")
			layout.bottom = []string{"", footer}
		}
		if m.height == 1 {
			layout.top = nil
		}
	}
	layout.fit(m.height, m.itemScrollOffset, cursorLine, anchorLine)
	return layout
}

// menuListLayout keeps the title and footer fixed while the body scrolls.
// Blank lines adjacent to the body can carry overflow indicators.
type menuListLayout struct {
	top    []string
	list   []string
	bottom []string
	offset int
	window int
}

func (layout *menuListLayout) fit(height, offset, cursor, anchor int) {
	layout.window = len(layout.list)
	if height > 0 {
		layout.window = min(layout.window, max(1, height-len(layout.top)-len(layout.bottom)))
	}
	layout.offset = min(offset, len(layout.list)-layout.window, anchor)
	layout.offset = max(layout.offset, cursor-layout.window+1, 0)
}

func (layout menuListLayout) render(theme tui.Theme) string {
	end := layout.offset + layout.window
	lines := append([]string(nil), layout.top...)
	if layout.offset > 0 && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines[len(lines)-1] = theme.Muted.Render(fmt.Sprintf("  ↑ %d more", layout.offset))
	}
	lines = append(lines, layout.list[layout.offset:end]...)
	lines = append(lines, layout.bottom...)
	if end < len(layout.list) && len(layout.bottom) > 0 && layout.bottom[0] == "" {
		lines[len(layout.top)+layout.window] = theme.Muted.Render(fmt.Sprintf("  ↓ %d more", len(layout.list)-end))
	}
	return strings.Join(lines, "\n")
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

func menuItemRow(item config.MenuItem, selected bool) string {
	if item.Subheader != "" {
		return menuIndentedRow(item.Label, selected)
	}
	return menuRow(item.Label, selected)
}

const (
	menuNoteGap = "   "
	// menuNoteColumnMax keeps one long label from pushing every note to the
	// right edge: a row longer than this carries its note right after it.
	menuNoteColumnMax = 40
)

// menuNoteColumn is the width the rows that carry a note are padded to, so
// the notes of a group line up.
func menuNoteColumn(items []config.MenuItem) int {
	column := 0
	for _, item := range items {
		if item.Note == "" {
			continue
		}
		if width := lipgloss.Width(menuItemRow(item, false)); width <= menuNoteColumnMax {
			column = max(column, width)
		}
	}
	return column
}

// menuNote is a row's note with the padding that puts it in the note column,
// or nothing when the row has no note or the terminal no room for it.
func menuNote(row, note string, column, width int) string {
	if note == "" {
		return ""
	}
	padded := strings.Repeat(" ", max(column-lipgloss.Width(row), 0)) + menuNoteGap + note
	if width > 0 && lipgloss.Width(row)+lipgloss.Width(padded) > width {
		return ""
	}
	return padded
}
