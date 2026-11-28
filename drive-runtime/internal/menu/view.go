package menu

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/tui"
)

const (
	menuGutter            = ""
	menuCaretSpace        = "  "
	menuCaret             = "> "
	menuSubIndent         = "  "
	searchSnippetMaxLines = 3
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
	width := m.searchViewWidth()

	mode := string(m.searchMode)
	modeLabel := "[" + strings.ToUpper(mode[:1]) + mode[1:] + "]"
	fmt.Fprintln(&b, m.theme.Title.Render("⌕ Search"), m.theme.Section.Render(modeLabel))

	status := m.searchStatus
	if status == "" && m.searchPageQuery == "" {
		status = fmt.Sprintf("%d sources • %d articles", m.searchInfo.SourceCount, m.searchInfo.ArticleCount)
		if m.searchInfo.NativeZIMCount > 0 {
			status = fmt.Sprintf("%d sources • %d embedded indexes • %d SQLite entries",
				m.searchInfo.SourceCount, m.searchInfo.NativeZIMCount, m.searchInfo.ArticleCount)
		}
	}
	if m.searchErr != nil {
		fmt.Fprintln(&b, m.theme.Error.Render(shortSearchText(status+": "+m.searchErr.Error(), width)))
	} else if status != "" {
		fmt.Fprintln(&b, m.theme.Muted.Render(shortSearchText(status, width)))
	}

	fmt.Fprintln(&b, "> "+m.searchQuery.View())
	filterHelp := "Ctrl+F archives"
	if count := len(m.excludedArchives); count > 0 {
		filterHelp += fmt.Sprintf(" (%d excluded)", count)
	}
	if m.searchInfo.HybridEnabled {
		filterHelp += " · Tab mode"
	}
	fmt.Fprintln(&b, m.theme.Help.Render(shortSearchText(filterHelp, width)))
	b.WriteByte('\n')

	help := "Enter search · Esc clear/back"
	shown := len(m.searchResults)
	if shown == 0 {
		if !m.searchLoading {
			fmt.Fprintln(&b, m.theme.Help.Render(shortSearchText(help, width)))
		}
		return b.String()
	}
	start := m.searchScrollOffset
	end := min(start+m.searchMaxVisible(), shown)
	label := fmt.Sprintf("Results %d–%d of %d loaded", start+1, end, shown)
	if m.searchPageQuery != "" && strings.TrimSpace(m.searchQuery.Value()) != m.searchPageQuery {
		label += fmt.Sprintf(" for %q", m.searchPageQuery)
	}
	if m.searchHasMore && m.searchResultsFocus && !m.searchLoading {
		hint := " · M load more"
		b.WriteString(m.theme.Section.Render(shortSearchText(label, max(1, width-lipgloss.Width(hint)))))
		fmt.Fprintln(&b, m.theme.Help.Render(hint))
	} else {
		fmt.Fprintln(&b, m.theme.Section.Render(shortSearchText(label, width)))
	}
	for idx := start; idx < end; idx++ {
		fmt.Fprintln(&b, renderSearchResultLine(m, idx+1, m.searchResults[idx], idx == m.searchSelected))
	}

	selected := m.searchResults[m.searchSelected]
	b.WriteByte('\n')
	fmt.Fprintln(&b, m.theme.Section.Render(shortSearchText(searchResultTitle(selected), width)))
	fmt.Fprintln(&b, m.theme.Muted.Render(shortSearchText("  Source: "+selected.Filename, width)))
	fmt.Fprintln(&b, m.theme.Muted.Render(shortSearchText("  Path:   "+selected.Path, width)))
	renderSearchSnippet(&b, m, selected)

	b.WriteByte('\n')
	if m.searchResultsFocus {
		help = "Enter open · Esc edit query · ↑↓ move · PgUp/PgDn"
	}
	fmt.Fprintln(&b, m.theme.Help.Render(shortSearchText(help, width)))
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

func renderSearchResultLine(m Model, index int, result search.Result, selected bool) string {
	number := fmt.Sprintf("%02d", index)
	width := m.searchViewWidth()
	const separator = " │ "
	room := width - len(number) - 4 - lipgloss.Width(separator)
	sourceWidth := min(30, max(1, room/3))
	source := shortSearchText(searchDisplaySource(result.Filename), sourceWidth)
	titleWidth := max(1, room-sourceWidth)
	title := shortSearchText(strings.Join(strings.Fields(searchResultTitle(result)), " "), titleWidth)
	padding := strings.Repeat(" ", max(0, titleWidth-lipgloss.Width(title)))
	if selected {
		row := "› " + number + "  " + title + padding + separator + source
		return m.theme.SelectedRow.Width(width).Render(shortSearchText(row, width))
	}
	number = lipgloss.NewStyle().Foreground(sourceColor(result.Filename)).Render(number)
	return "  " + number + "  " + title + padding + m.theme.Muted.Render(separator+source)
}

func searchDisplaySource(filename string) string {
	if archive, _, ok := engine.EmbeddedArchive(filename); ok {
		filename = archive
	}
	if !strings.EqualFold(path.Ext(filename), ".zim") {
		return path.Base(filename)
	}
	label := strings.TrimSuffix(filename, path.Ext(filename))
	name, metadata, found := strings.Cut(label, "_")
	if !found {
		return label
	}
	switch name {
	case "wikipedia":
		name = "Wikipedia"
	case "wiktionary":
		name = "Wiktionary"
	case "wikibooks":
		name = "Wikibooks"
	case "gutenberg":
		name = "Project Gutenberg"
	case "solar.lowtechmagazine.com":
		name = "Low-tech Magazine"
	case "100r.co":
		name = "100R"
	case "appropedia":
		name = "Appropedia"
	}
	language, variant, _ := strings.Cut(metadata, "_")
	if len(language) < 2 || len(language) > 3 {
		return label
	}
	collection, _, _ := strings.Cut(variant, "_")
	switch {
	case strings.HasPrefix(collection, "lcc-"):
		name += " " + strings.ToUpper(strings.TrimPrefix(collection, "lcc-"))
	case collection != "" && collection != "all" && !strings.HasPrefix(collection, "20"):
		name += " " + collection
	}
	return name + " (" + strings.ToUpper(language) + ")"
}

func searchResultTitle(result search.Result) string {
	title := strings.TrimSpace(result.Title)
	source, _, _ := strings.Cut(searchDisplaySource(result.Filename), " (")
	if title != "" && !strings.EqualFold(title, source) {
		return title
	}
	if _, heading, ok := strings.Cut(result.ChunkHeader, " > "); ok && heading != "" {
		return heading
	}
	entry := path.Base(result.Path)
	if strings.EqualFold(entry, "index.html") {
		entry = path.Base(path.Dir(result.Path))
	}
	if strings.EqualFold(path.Ext(entry), ".html") {
		entry = strings.TrimSuffix(entry, path.Ext(entry))
	}
	if entry == "." || entry == "/" || entry == "" {
		return title
	}
	return strings.NewReplacer("_", " ", "-", " ").Replace(entry)
}

func (m Model) searchViewWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func renderSearchSnippet(b *strings.Builder, m Model, result search.Result) {
	snippet := ansi.Strip(result.Snippet)
	if !result.Native {
		snippet = strings.NewReplacer("»", "", "«", "").Replace(snippet)
	}
	snippet = strings.Join(strings.Fields(snippet), " ")
	if snippet == "" {
		return
	}
	const label = "  Snippet: "
	width := m.searchViewWidth()
	if width <= len(label) {
		return
	}
	room := width - len(label)
	terms := engine.QueryWords(m.searchPageQuery)
	alternative := search.GrowingQuery(terms)
	lines := strings.Split(ansi.Wrap(snippet, room, ""), "\n")
	lines = visibleSearchSnippet(lines, terms, alternative, room)

	highlightTerms := append(terms, alternative...)
	prefix := label
	for _, line := range lines {
		b.WriteString(m.theme.Muted.Render(prefix))
		renderSearchSnippetLine(b, m, line, highlightTerms)
		b.WriteByte('\n')
		prefix = strings.Repeat(" ", len(label))
	}
}

// visibleSearchSnippet keeps the three lines with the best query coverage,
// favoring the initial context when later lines offer no stronger evidence.
func visibleSearchSnippet(lines, terms, alternative []string, width int) []string {
	if len(lines) <= searchSnippetMaxLines {
		return lines
	}
	bestStart, bestScore := 0, -1
	for start := 0; start+searchSnippetMaxLines <= len(lines); start++ {
		text := strings.Join(lines[start:start+searchSnippetMaxLines], " ")
		score := func(words []string) int {
			matched, nearby := search.PassageEvidence(text, words)
			if nearby {
				matched++
			}
			return matched
		}
		if weight := max(score(terms), score(alternative)); weight > bestScore {
			bestStart, bestScore = start, weight
		}
	}
	end := bestStart + searchSnippetMaxLines
	visible := lines[bestStart:end]
	if bestStart > 0 {
		visible[0] = "…" + shortSearchText(visible[0], width-1)
	}
	if end < len(lines) {
		last := len(visible) - 1
		visible[last] = shortSearchText(visible[last]+"…", width)
	}
	return visible
}

var searchWordPattern = regexp.MustCompile("[" + "\\pL\\pN" + "]+")

func renderSearchSnippetLine(b *strings.Builder, m Model, line string, terms []string) {
	last := 0
	for _, span := range searchWordPattern.FindAllStringIndex(line, -1) {
		word := line[span[0]:span[1]]
		if !slices.ContainsFunc(terms, func(term string) bool { return search.MatchesQueryWord(word, term) }) {
			continue
		}
		b.WriteString(m.theme.Base.Render(line[last:span[0]]))
		b.WriteString(m.theme.Focus.Render(word))
		last = span[1]
	}
	b.WriteString(m.theme.Base.Render(line[last:]))
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
