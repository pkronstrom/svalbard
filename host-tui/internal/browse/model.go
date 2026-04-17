package browse

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
	"github.com/pkronstrom/svalbard/tui"
)

// BackMsg is sent when the user exits the browse screen without saving.
type BackMsg struct{}

// SavedMsg is sent when the user saves changes and exits.
type SavedMsg struct{}

// Config holds everything the browse screen needs.
// Callbacks are passed directly to avoid circular imports with the hosttui package.
type Config struct {
	PackGroups   []wizard.PackGroup
	Presets      []wizard.PresetOption
	DesiredItems []string
	FreeGB       float64
	SaveDesired  func(ids []string) error // nil for read-only mode
}

// Row kinds for the flattened tree display.
const (
	rowGroup = iota
	rowPack
	rowItem
)

// pickerRow is a single row in the flattened tree view.
type pickerRow struct {
	kind       int
	groupName  string
	pack       *wizard.Pack
	source     *wizard.PackSource
	groupPacks []wizard.Pack // only set for rowGroup rows
}

// Model is the Bubble Tea model for the browse screen.
type Model struct {
	groups     []wizard.PackGroup
	presets    []wizard.PresetOption
	presetIdx  int // current preset cycle index (-1 = custom)
	checkedIDs map[string]bool
	initialIDs map[string]bool // snapshot at entry for dirty check
	readOnly   bool
	saveFunc   func(ids []string) error

	// Tree state
	collapsedGroups map[string]bool
	collapsedPacks  map[string]bool
	rows            []pickerRow
	cursor          int
	scrollOffset    int
	freeGB          float64

	// Save prompt
	showSavePrompt bool
	saveChoice     int // 0=yes, 1=no

	// Layout
	width  int
	height int
	theme  tui.Theme
	keys   tui.KeyMap
}

// New creates a browse model from the given configuration.
func New(cfg Config) Model {
	m := Model{
		groups:          cfg.PackGroups,
		presets:         cfg.Presets,
		presetIdx:       -1,
		checkedIDs:      make(map[string]bool),
		initialIDs:      make(map[string]bool),
		readOnly:        cfg.SaveDesired == nil,
		saveFunc:        cfg.SaveDesired,
		collapsedGroups: make(map[string]bool),
		collapsedPacks:  make(map[string]bool),
		freeGB:          cfg.FreeGB,
		theme:           tui.DefaultTheme(),
		keys:            tui.DefaultKeyMap(),
	}

	// Populate checked state from desired items.
	for _, id := range cfg.DesiredItems {
		m.checkedIDs[id] = true
		m.initialIDs[id] = true
	}

	// Groups start expanded.
	for _, g := range m.groups {
		m.collapsedGroups[g.Name] = false
	}

	// Packs start collapsed.
	for _, g := range m.groups {
		for _, p := range g.Packs {
			m.collapsedPacks[p.Name] = true
		}
	}

	m.rebuildRows()
	if len(m.rows) > 0 {
		m.cursor = 0
	}

	return m
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the browse screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Save prompt intercepts all keys.
		if m.showSavePrompt {
			return m.updateSavePrompt(msg)
		}

		switch {
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureVisible()
			return m, nil

		case m.keys.MoveDown.Matches(msg):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.ensureVisible()
			return m, nil

		case m.keys.Toggle.Matches(msg):
			if !m.readOnly {
				m.toggleAtCursor()
			}
			return m, nil

		case m.keys.Enter.Matches(msg):
			m.expandCollapseAtCursor()
			return m, nil

		case matchRune(msg, 'p'):
			if !m.readOnly && len(m.presets) > 0 {
				m.cyclePreset()
			}
			return m, nil

		case m.keys.Quit.Matches(msg), m.keys.Back.Matches(msg):
			if !m.readOnly && m.isDirty() {
				m.showSavePrompt = true
				m.saveChoice = 0
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateSavePrompt handles key input while the save prompt is displayed.
func (m Model) updateSavePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchRune(msg, 'y'):
		if err := m.saveFunc(m.checkedIDSlice()); err != nil {
			// On error, dismiss prompt — user sees original screen.
			m.showSavePrompt = false
			return m, nil
		}
		return m, func() tea.Msg { return SavedMsg{} }

	case matchRune(msg, 'n'):
		return m, func() tea.Msg { return BackMsg{} }

	case m.keys.Back.Matches(msg):
		m.showSavePrompt = false
		return m, nil

	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit
	}
	return m, nil
}

// isDirty returns true if the checked state differs from the initial state.
func (m *Model) isDirty() bool {
	if len(m.checkedIDs) != len(m.initialIDs) {
		return true
	}
	for id := range m.checkedIDs {
		if !m.initialIDs[id] {
			return true
		}
	}
	return false
}

// checkedIDSlice returns the checked IDs as a sorted slice.
func (m *Model) checkedIDSlice() []string {
	ids := make([]string, 0, len(m.checkedIDs))
	for id := range m.checkedIDs {
		ids = append(ids, id)
	}
	return ids
}

// cyclePreset advances to the next preset and applies its source IDs.
func (m *Model) cyclePreset() {
	m.presetIdx = (m.presetIdx + 1) % len(m.presets)
	preset := m.presets[m.presetIdx]

	// Clear and apply preset's source IDs.
	m.checkedIDs = make(map[string]bool)
	for _, id := range preset.SourceIDs {
		m.checkedIDs[id] = true
	}
}

// rebuildRows flattens the tree respecting collapsed state.
func (m *Model) rebuildRows() {
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
}

// toggleAtCursor toggles the selection at the current cursor position.
func (m *Model) toggleAtCursor() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.cursor]
	switch row.kind {
	case rowGroup:
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
			for _, s := range pack.Sources {
				delete(m.checkedIDs, s.ID)
			}
		} else {
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
func (m *Model) expandCollapseAtCursor() {
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
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

// maxVisible returns the number of visible rows based on terminal height.
func (m *Model) maxVisible() int {
	v := m.height - 8
	if v < 4 {
		v = 4
	}
	return v
}

// ensureVisible adjusts scrollOffset so the cursor stays in view.
func (m *Model) ensureVisible() {
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

// totalCheckedGB returns the total size of all checked sources (dedup by ID).
func (m *Model) totalCheckedGB() float64 {
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

// totalCheckedCount returns the number of unique checked source IDs.
func (m *Model) totalCheckedCount() int {
	return len(m.checkedIDs)
}

// View renders the browse screen with left-pane nav and right-pane detail.
func (m Model) View() string {
	var nav strings.Builder

	// Left pane: tree rows.
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
				nav.WriteString(m.theme.Selected.Render(prefix + label))
			} else {
				nav.WriteString(m.theme.Section.Render(prefix + label))
			}

		case rowPack:
			pack := row.pack
			checked, total := packCheckState(pack, m.checkedIDs)
			mark := markChar(checked, total)
			size := packCheckedSizeGB(pack, m.checkedIDs)
			label := fmt.Sprintf("  %s%s %s  %s", prefix, mark, pack.Name, tui.FormatSizeGB(size))
			if isCursor {
				nav.WriteString(m.theme.Selected.Render(label))
			} else if checked > 0 {
				nav.WriteString(m.theme.Base.Render(label))
			} else {
				nav.WriteString(m.theme.Muted.Render(label))
			}

		case rowItem:
			src := row.source
			mark := "·"
			if m.checkedIDs[src.ID] {
				mark = "✓"
			}
			line := fmt.Sprintf("      %s%s %s %s  %s", prefix, mark, src.ID, tui.TypeSymbol(src.Type), tui.FormatSizeGB(src.SizeGB))
			if isCursor {
				nav.WriteString(m.theme.Selected.Render(line))
			} else if m.checkedIDs[src.ID] {
				nav.WriteString(m.theme.Base.Render(line))
			} else {
				nav.WriteString(m.theme.Muted.Render(line))
			}
		}
		nav.WriteString("\n")
	}

	nav.WriteString("\n")

	// Size summary at bottom of nav pane.
	totalGB := m.totalCheckedGB()
	if m.freeGB <= 0 {
		nav.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f GB", totalGB)))
	} else if totalGB <= m.freeGB {
		nav.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, m.freeGB)))
		nav.WriteString(m.theme.Success.Render("fits"))
	} else {
		nav.WriteString(m.theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, m.freeGB)))
		nav.WriteString(m.theme.Danger.Render(fmt.Sprintf("%.1f GB over", totalGB-m.freeGB)))
	}

	// Save prompt overlay.
	if m.showSavePrompt {
		nav.WriteString("\n\n")
		nav.WriteString(m.theme.Warning.Render("  Save changes before leaving?"))
		nav.WriteString("\n")
		nav.WriteString(m.theme.Base.Render("  y = save   n = discard   esc = cancel"))
	}

	// Right pane: detail for focused row.
	detail := m.viewDetail()

	// Header.
	header := fmt.Sprintf("Browse  %d selected  %.1f GB", m.totalCheckedCount(), totalGB)

	// Footer hints.
	var footerParts []string
	footerParts = append(footerParts,
		fmt.Sprintf("%s/%s navigate", m.keys.MoveUp.Key, m.keys.MoveDown.Key),
		fmt.Sprintf("%s toggle", m.keys.Toggle.Key),
		fmt.Sprintf("%s expand/collapse", m.keys.Enter.Key),
	)
	if !m.readOnly && len(m.presets) > 0 {
		footerParts = append(footerParts, "p preset")
	}
	footerParts = append(footerParts, "esc back")
	footer := strings.Join(footerParts, "  ")

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  header,
		Left:    nav.String(),
		Right:   detail,
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

// viewDetail renders the right-pane detail preview for the focused row.
func (m Model) viewDetail() string {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}

	var b strings.Builder
	row := m.rows[m.cursor]

	switch row.kind {
	case rowGroup:
		b.WriteString(m.theme.Section.Render(row.groupName))
		b.WriteString("\n\n")
		packCount := len(row.groupPacks)
		itemCount := 0
		for _, p := range row.groupPacks {
			itemCount += len(p.Sources)
		}
		b.WriteString(m.theme.Base.Render(fmt.Sprintf("  %d packs, %d sources", packCount, itemCount)))

	case rowPack:
		pack := row.pack
		b.WriteString(m.theme.Section.Render(pack.Name))
		b.WriteString("\n\n")
		if pack.Description != "" {
			b.WriteString(m.theme.Base.Render("  " + pack.Description))
			b.WriteString("\n\n")
		}
		checked, total := packCheckState(pack, m.checkedIDs)
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %d / %d selected", checked, total)))
		b.WriteString("\n")
		size := packCheckedSizeGB(pack, m.checkedIDs)
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s selected", tui.FormatSizeGB(size))))

	case rowItem:
		src := row.source
		sym := tui.TypeSymbol(src.Type)
		b.WriteString(m.theme.Section.Render(fmt.Sprintf("%s %s", sym, src.ID)))
		b.WriteString("\n\n")
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  Type: %s", src.Type)))
		b.WriteString("\n")
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  Size: %s", tui.FormatSizeGB(src.SizeGB))))
		if src.Description != "" {
			b.WriteString("\n\n")
			b.WriteString(m.theme.Base.Render("  " + src.Description))
		}
		b.WriteString("\n\n")
		if m.checkedIDs[src.ID] {
			b.WriteString(m.theme.Success.Render("  ✓ Selected"))
		} else {
			b.WriteString(m.theme.Muted.Render("  · Not selected"))
		}
	}

	return b.String()
}

// --- helpers ---

// matchRune checks if a key message is a specific rune.
func matchRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}

// packCheckState returns the count of checked and total sources for a pack.
func packCheckState(pack *wizard.Pack, checked map[string]bool) (int, int) {
	c := 0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			c++
		}
	}
	return c, len(pack.Sources)
}

// packCheckedSizeGB returns the total size of checked sources in a pack.
func packCheckedSizeGB(pack *wizard.Pack, checked map[string]bool) float64 {
	total := 0.0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			total += s.SizeGB
		}
	}
	return total
}

// markChar returns a checkbox character based on checked/total counts.
func markChar(checked, total int) string {
	if checked == total && total > 0 {
		return "[x]"
	}
	if checked > 0 {
		return "[-]"
	}
	return "[ ]"
}


