package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// Messages emitted by the pack picker.
type packDoneMsg struct {
	selectedIDs map[string]bool
}
type packCancelMsg struct{}

// Row kinds for the flattened tree display.
const (
	rowGroup = iota
	rowPack
	rowItem
	rowAction // "Continue to review →" at the bottom
)

// pickerRow is a single row in the flattened tree view.
type pickerRow struct {
	kind       int
	groupName  string
	pack       *Pack
	source     *PackSource
	groupPacks []Pack // only set for rowGroup rows
}

// packPickerModel is the Bubble Tea model for the pack picker sub-screen.
type packPickerModel struct {
	groups          []PackGroup
	checkedIDs      map[string]bool
	collapsedGroups map[string]bool
	collapsedPacks  map[string]bool
	rows            []pickerRow
	cursor          int
	scrollOffset    int
	freeGB          float64
	width           int
	height          int
	theme           tui.Theme
	keys            tui.KeyMap
}

// newPackPicker creates a pack picker model.
// Groups start expanded, packs start collapsed.
// checked may be nil; it is copied (not mutated).
func newPackPicker(groups []PackGroup, checked map[string]bool, freeGB float64) packPickerModel {
	m := packPickerModel{
		groups:          groups,
		checkedIDs:      make(map[string]bool),
		collapsedGroups: make(map[string]bool),
		collapsedPacks:  make(map[string]bool),
		freeGB:          freeGB,
		theme:           tui.DefaultTheme(),
		keys:            tui.DefaultKeyMap(),
	}

	// Copy checked map.
	for id, v := range checked {
		if v {
			m.checkedIDs[id] = true
		}
	}

	// Groups start expanded (false = not collapsed).
	for _, g := range groups {
		m.collapsedGroups[g.Name] = false
	}

	// Packs start collapsed.
	for _, g := range groups {
		for _, p := range g.Packs {
			m.collapsedPacks[p.Name] = true
		}
	}

	m.rebuildRows()

	// Set cursor to first selectable row (skip nothing — all rows are selectable).
	if len(m.rows) > 0 {
		m.cursor = 0
	}

	return m
}

// rebuildRows flattens the tree respecting collapsed state.
func (m *packPickerModel) rebuildRows() {
	m.rows = nil
	for _, g := range m.groups {
		m.rows = append(m.rows, pickerRow{
			kind:       rowGroup,
			groupName:  g.Name,
			groupPacks: g.Packs,
		})
		if m.collapsedGroups[g.Name] {
			continue
		}
		for i := range g.Packs {
			p := &g.Packs[i]
			m.rows = append(m.rows, pickerRow{
				kind: rowPack,
				pack: p,
			})
			if m.collapsedPacks[p.Name] {
				continue
			}
			for j := range p.Sources {
				s := &p.Sources[j]
				m.rows = append(m.rows, pickerRow{
					kind:   rowItem,
					source: s,
				})
			}
		}
	}
	// Action row at the bottom
	m.rows = append(m.rows, pickerRow{kind: rowAction})
}

// totalCheckedGB returns the total size of all checked sources (dedup by ID).
func (m *packPickerModel) totalCheckedGB() float64 {
	seen := make(map[string]bool)
	total := 0.0
	for _, g := range m.groups {
		for _, p := range g.Packs {
			for _, s := range p.Sources {
				if m.checkedIDs[s.ID] && !seen[s.ID] {
					seen[s.ID] = true
					total += s.SizeGB
				}
			}
		}
	}
	return total
}

// packCheckState returns the count of checked and total sources for a pack.
func packCheckState(pack *Pack, checked map[string]bool) (int, int) {
	c := 0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			c++
		}
	}
	return c, len(pack.Sources)
}

// Init satisfies tea.Model.
func (m packPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the pack picker.
func (m packPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		// Navigate up
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureVisible()
			return m, nil

		// Navigate down
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.ensureVisible()
			return m, nil

		// Toggle selection
		case m.keys.Toggle.Matches(msg):
			m.toggleAtCursor()
			return m, nil

		// Enter: expand/collapse on groups/packs, or confirm on action row
		case m.keys.Enter.Matches(msg):
			if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].kind == rowAction {
				selected := make(map[string]bool, len(m.checkedIDs))
				for id, v := range m.checkedIDs {
					if v {
						selected[id] = true
					}
				}
				return m, func() tea.Msg { return packDoneMsg{selectedIDs: selected} }
			}
			m.expandCollapseAtCursor()
			return m, nil

		case msg.Type == tea.KeyRight || msg.Type == tea.KeyLeft:
			if m.cursor >= 0 && m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				if row.kind == rowGroup || row.kind == rowPack {
					m.expandCollapseAtCursor()
				}
			}
			return m, nil

		// Apply
		case matchRune(msg, 'a'):
			selected := make(map[string]bool, len(m.checkedIDs))
			for id, v := range m.checkedIDs {
				if v {
					selected[id] = true
				}
			}
			return m, func() tea.Msg { return packDoneMsg{selectedIDs: selected} }

		// Cancel
		case m.keys.Quit.Matches(msg):
			return m, func() tea.Msg { return packCancelMsg{} }
		}
	}
	return m, nil
}

// matchRune checks if a key message is a specific rune.
func matchRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}

// toggleAtCursor toggles the selection at the current cursor position.
func (m *packPickerModel) toggleAtCursor() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.cursor]
	switch row.kind {
	case rowGroup:
		// Toggle all sources in all packs of the group.
		allChecked := true
		for _, p := range row.groupPacks {
			for _, s := range p.Sources {
				if !m.checkedIDs[s.ID] {
					allChecked = false
					break
				}
			}
			if !allChecked {
				break
			}
		}
		for _, p := range row.groupPacks {
			for _, s := range p.Sources {
				if allChecked {
					delete(m.checkedIDs, s.ID)
				} else {
					m.checkedIDs[s.ID] = true
				}
			}
		}

	case rowPack:
		pack := row.pack
		checked, total := packCheckState(pack, m.checkedIDs)
		if checked == total && total > 0 {
			// All checked => uncheck all
			for _, s := range pack.Sources {
				delete(m.checkedIDs, s.ID)
			}
		} else {
			// Some or none checked => check all
			for _, s := range pack.Sources {
				m.checkedIDs[s.ID] = true
			}
		}

	case rowItem:
		src := row.source
		if m.checkedIDs[src.ID] {
			delete(m.checkedIDs, src.ID)
		} else {
			m.checkedIDs[src.ID] = true
		}
	}
}

// expandCollapseAtCursor toggles expand/collapse at the current cursor position.
func (m *packPickerModel) expandCollapseAtCursor() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.cursor]
	switch row.kind {
	case rowGroup:
		m.collapsedGroups[row.groupName] = !m.collapsedGroups[row.groupName]
	case rowPack:
		m.collapsedPacks[row.pack.Name] = !m.collapsedPacks[row.pack.Name]
	}
	m.rebuildRows()
	// Clamp cursor after rows change.
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

// maxVisible returns the number of visible rows based on terminal height.
func (m *packPickerModel) maxVisible() int {
	// Reserve: header(2) + detail area(4) + total/free(1) + help(1) + margins(2)
	v := m.height - 10
	if v < 4 {
		v = 4
	}
	return v
}

// ensureVisible adjusts scrollOffset so the cursor stays in view.
func (m *packPickerModel) ensureVisible() {
	maxVis := m.maxVisible()
	if m.cursor < m.scrollOffset+2 {
		m.scrollOffset = m.cursor - 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	}
	if m.cursor >= m.scrollOffset+maxVis-2 {
		m.scrollOffset = m.cursor - maxVis + 3
		maxOff := len(m.rows) - maxVis
		if maxOff < 0 {
			maxOff = 0
		}
		if m.scrollOffset > maxOff {
			m.scrollOffset = maxOff
		}
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// View renders the pack picker.
func (m packPickerModel) View() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render("Svalbard — Pack Picker"))
	b.WriteString("\n\n")

	maxVis := m.maxVisible()
	end := m.scrollOffset + maxVis
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scrollOffset; i < end; i++ {
		row := m.rows[i]
		isCursor := i == m.cursor
		prefix := "  "
		if isCursor {
			prefix = "> "
		}

		switch row.kind {
		case rowGroup:
			label := row.groupName
			if isCursor {
				b.WriteString(m.theme.Selected.Render(prefix + label))
			} else {
				b.WriteString(m.theme.Section.Render(prefix + label))
			}

		case rowPack:
			pack := row.pack
			checked, total := packCheckState(pack, m.checkedIDs)
			mark := "·"
			if checked == total && total > 0 {
				mark = "✓"
			} else if checked > 0 {
				mark = "~"
			}
			size := packCheckedSizeGB(pack, m.checkedIDs)
			suffix := formatSizeGB(size)
			if checked > 0 && checked < total {
				suffix = "(shared)"
			}
			label := fmt.Sprintf("    %s%s %s  %s", prefix, mark, pack.Name, suffix)
			if isCursor {
				b.WriteString(m.theme.Selected.Render(label))
			} else if checked > 0 {
				b.WriteString(m.theme.Base.Render(label))
			} else {
				b.WriteString(m.theme.Muted.Render(label))
			}

		case rowItem:
			src := row.source
			mark := "·"
			if m.checkedIDs[src.ID] {
				mark = "✓"
			}
			line := fmt.Sprintf("        %s%s %s %s  %s", prefix, mark, typeSymbol(src.Type), src.ID, formatSizeGB(src.SizeGB))
			if isCursor {
				b.WriteString(m.theme.Selected.Render(line))
			} else if m.checkedIDs[src.ID] {
				b.WriteString(m.theme.Base.Render(line))
			} else {
				b.WriteString(m.theme.Muted.Render(line))
			}

		case rowAction:
			b.WriteString("\n")
			line := prefix + "Continue to review →"
			if isCursor {
				b.WriteString(m.theme.Success.Render(line))
			} else {
				b.WriteString(m.theme.Focus.Render(line))
			}
		}
		b.WriteString("\n")
	}

	// Detail area for cursor item
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  ─────────────────────────────────"))
	b.WriteString("\n")
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		switch row.kind {
		case rowGroup:
			total := 0
			for _, p := range row.groupPacks {
				total += len(p.Sources)
			}
			b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s — %d packs, %d items", row.groupName, len(row.groupPacks), total)))
		case rowPack:
			desc := row.pack.Description
			if desc == "" {
				desc = row.pack.Name
			}
			b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s — %d items", desc, len(row.pack.Sources))))
		case rowItem:
			src := row.source
			line := fmt.Sprintf("  %s %s · %s · %s", typeSymbol(src.Type), src.ID, src.Type, formatSizeGB(src.SizeGB))
			b.WriteString(m.theme.Muted.Render(truncate(line, m.paneWidth())))
			if src.Description != "" {
				b.WriteString("\n")
				b.WriteString(m.theme.Muted.Render(truncate("  "+src.Description, m.paneWidth())))
			}
		}
	}
	b.WriteString("\n\n")

	// Footer: total / free with fits/over indicator.
	totalGB := m.totalCheckedGB()
	if m.freeGB <= 0 {
		// Free space unknown (e.g. custom path) — show total only
		b.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f GB", totalGB)))
	} else if totalGB <= m.freeGB {
		b.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, m.freeGB)))
		b.WriteString(m.theme.Success.Render("fits"))
	} else {
		b.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, m.freeGB)))
		b.WriteString(m.theme.Danger.Render(fmt.Sprintf("%.1f GB over", totalGB-m.freeGB)))
	}
	b.WriteString("\n")

	// Help line — derived from key bindings.
	b.WriteString(m.theme.Help.Render(fmt.Sprintf("  %s/%s navigate  %s toggle  %s expand/collapse  a apply  q cancel",
		m.keys.MoveUp.Key, m.keys.MoveDown.Key, m.keys.Toggle.Key, m.keys.Enter.Key)))
	b.WriteString("\n")

	return b.String()
}

// typeSymbol returns a small Unicode symbol indicating the recipe type.
func typeSymbol(t string) string {
	switch t {
	case "zim", "pdf", "epub", "html":
		return "✦"
	case "binary", "toolchain", "app", "sqlite":
		return "⚙"
	case "pmtiles", "gpkg":
		return "⊞"
	case "gguf":
		return "∿"
	default:
		return "·"
	}
}

func (m *packPickerModel) paneWidth() int {
	// Right pane is ~75% of terminal minus gutter
	w := int(float64(m.width)*0.75) - 4
	if w < 40 {
		w = 40
	}
	return w
}

func truncate(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "…"
}

// packCheckedSizeGB returns the total size of checked sources in a pack.
func packCheckedSizeGB(pack *Pack, checked map[string]bool) float64 {
	total := 0.0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			total += s.SizeGB
		}
	}
	return total
}

// formatSizeGB is defined in presetpicker.go — reused here.
