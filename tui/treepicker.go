package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PackGroup is a display group containing packs.
type PackGroup struct {
	Name  string // display_group value, e.g. "Maps & Geodata"
	Packs []Pack
}

// Pack is a named bundle of sources.
type Pack struct {
	Name        string
	Description string
	Sources     []PackSource
}

// PackSource is a single recipe inside a pack.
type PackSource struct {
	ID          string
	Type        string // e.g. "zim", "binary", "pmtiles"
	Strategy    string // "download" or "build"
	Description string
	SizeGB      float64
}

// Row kinds for the flattened tree display.
const (
	RowGroup  = iota
	RowPack
	RowItem
	RowAction // optional action row at the bottom
)

// PickerRow is a single row in the flattened tree view.
type PickerRow struct {
	Kind       int
	GroupName  string
	Pack       *Pack
	Source     *PackSource
	GroupPacks []Pack // only set for RowGroup rows
}

// TreePickerConfig configures a TreePicker.
type TreePickerConfig struct {
	Groups      []PackGroup
	CheckedIDs  map[string]bool // initial selection (copied, not mutated)
	FreeGB      float64         // available disk space (0 = unknown)
	ReadOnly    bool            // disable toggling
	ShowAction  bool            // show action row at bottom
	ActionLabel string          // label for action row (default "Continue to review →")
}

// TreePicker is a collapsible tree view with checkboxes for selecting pack sources.
type TreePicker struct {
	Groups          []PackGroup
	CheckedIDs      map[string]bool
	AutoDepIDs      map[string]bool // IDs auto-included as deps
	UserCheckedIDs  map[string]bool // IDs the user explicitly toggled on
	CollapsedGroups map[string]bool
	CollapsedPacks  map[string]bool
	Rows            []PickerRow
	Cursor          int
	ScrollOffset    int
	FreeGB          float64
	ReadOnly        bool
	ShowAction      bool
	ActionLabel     string
	Width           int
	Height          int
	ReserveLines    int // lines reserved for chrome below tree (default 10)
	Theme           Theme
	Keys            KeyMap
}

// NewTreePicker creates a tree picker from the given configuration.
func NewTreePicker(cfg TreePickerConfig) TreePicker {
	tp := TreePicker{
		Groups:          cfg.Groups,
		CheckedIDs:      make(map[string]bool),
		AutoDepIDs:      make(map[string]bool),
		UserCheckedIDs:  make(map[string]bool),
		CollapsedGroups: make(map[string]bool),
		CollapsedPacks:  make(map[string]bool),
		FreeGB:          cfg.FreeGB,
		ReadOnly:        cfg.ReadOnly,
		ShowAction:      cfg.ShowAction,
		ActionLabel:     cfg.ActionLabel,
		Theme:           DefaultTheme(),
		Keys:            DefaultKeyMap(),
	}

	for id, v := range cfg.CheckedIDs {
		if v {
			tp.CheckedIDs[id] = true
			tp.UserCheckedIDs[id] = true
		}
	}

	// Groups start expanded.
	for _, g := range tp.Groups {
		tp.CollapsedGroups[g.Name] = false
	}
	// Packs start collapsed.
	for _, g := range tp.Groups {
		for _, p := range g.Packs {
			tp.CollapsedPacks[p.Name] = true
		}
	}

	tp.RebuildRows()
	return tp
}

// RebuildRows flattens the tree respecting collapsed state.
func (tp *TreePicker) RebuildRows() {
	tp.Rows = nil
	for _, g := range tp.Groups {
		tp.Rows = append(tp.Rows, PickerRow{
			Kind:       RowGroup,
			GroupName:  g.Name,
			GroupPacks: g.Packs,
		})
		if tp.CollapsedGroups[g.Name] {
			continue
		}
		for i := range g.Packs {
			p := &g.Packs[i]
			tp.Rows = append(tp.Rows, PickerRow{
				Kind: RowPack,
				Pack: p,
			})
			if tp.CollapsedPacks[p.Name] {
				continue
			}
			for j := range p.Sources {
				s := &p.Sources[j]
				tp.Rows = append(tp.Rows, PickerRow{
					Kind:   RowItem,
					Source: s,
				})
			}
		}
	}
	if tp.ShowAction {
		tp.Rows = append(tp.Rows, PickerRow{Kind: RowAction})
	}
}

// UpdateResult indicates what happened during Update.
type UpdateResult int

const (
	UpdateNone    UpdateResult = iota // key not handled
	UpdateNav                         // navigation only
	UpdateToggled                     // selection changed
)

// Update handles navigation and toggle keys. Returns true if a key was handled.
// Does NOT handle Enter on RowAction or Esc/q — callers handle those.
func (tp *TreePicker) Update(msg tea.KeyMsg) bool {
	return tp.UpdateWithResult(msg) != UpdateNone
}

// UpdateWithResult handles navigation and toggle keys, returning what happened.
// Does NOT handle Enter on RowAction or Esc/q — callers handle those.
func (tp *TreePicker) UpdateWithResult(msg tea.KeyMsg) UpdateResult {
	switch {
	case tp.Keys.MoveUp.Matches(msg):
		if tp.Cursor > 0 {
			tp.Cursor--
		}
		tp.EnsureVisible()
		return UpdateNav

	case tp.Keys.MoveDown.Matches(msg):
		if tp.Cursor < len(tp.Rows)-1 {
			tp.Cursor++
		}
		tp.EnsureVisible()
		return UpdateNav

	case tp.Keys.Toggle.Matches(msg):
		if !tp.ReadOnly {
			tp.ToggleAtCursor()
		}
		return UpdateToggled

	case tp.Keys.Enter.Matches(msg):
		if tp.Cursor >= 0 && tp.Cursor < len(tp.Rows) && tp.Rows[tp.Cursor].Kind == RowAction {
			return UpdateNone // let caller handle action
		}
		tp.ExpandCollapseAtCursor()
		return UpdateNav

	case msg.Type == tea.KeyRight:
		tp.ExpandAtCursor()
		return UpdateNav

	case msg.Type == tea.KeyLeft:
		tp.CollapseAtCursor()
		return UpdateNav
	}
	return UpdateNone
}

// IsAutoDep returns true if the given ID is an auto-included dep that was not
// manually selected by the user.
func (tp *TreePicker) IsAutoDep(id string) bool {
	return tp.AutoDepIDs[id] && !tp.UserCheckedIDs[id]
}

// CursorRow returns the row at the current cursor, or nil.
func (tp *TreePicker) CursorRow() *PickerRow {
	if tp.Cursor >= 0 && tp.Cursor < len(tp.Rows) {
		return &tp.Rows[tp.Cursor]
	}
	return nil
}

// ToggleAtCursor toggles the selection at the current cursor position.
func (tp *TreePicker) ToggleAtCursor() {
	if tp.Cursor < 0 || tp.Cursor >= len(tp.Rows) {
		return
	}
	row := tp.Rows[tp.Cursor]
	switch row.Kind {
	case RowGroup:
		allChecked := true
		for _, p := range row.GroupPacks {
			for _, s := range p.Sources {
				if !tp.CheckedIDs[s.ID] {
					allChecked = false
					break
				}
			}
			if !allChecked {
				break
			}
		}
		for _, p := range row.GroupPacks {
			for _, s := range p.Sources {
				if allChecked {
					delete(tp.CheckedIDs, s.ID)
					delete(tp.UserCheckedIDs, s.ID)
				} else {
					tp.CheckedIDs[s.ID] = true
					tp.UserCheckedIDs[s.ID] = true
				}
			}
		}

	case RowPack:
		pack := row.Pack
		checked, total := PackCheckState(pack, tp.CheckedIDs)
		if checked == total && total > 0 {
			for _, s := range pack.Sources {
				delete(tp.CheckedIDs, s.ID)
				delete(tp.UserCheckedIDs, s.ID)
			}
		} else {
			for _, s := range pack.Sources {
				tp.CheckedIDs[s.ID] = true
				tp.UserCheckedIDs[s.ID] = true
			}
		}

	case RowItem:
		src := row.Source
		if tp.IsAutoDep(src.ID) {
			return
		}
		if tp.CheckedIDs[src.ID] {
			delete(tp.CheckedIDs, src.ID)
			delete(tp.UserCheckedIDs, src.ID)
		} else {
			tp.CheckedIDs[src.ID] = true
			tp.UserCheckedIDs[src.ID] = true
		}
	}
}

// ExpandCollapseAtCursor toggles expand/collapse at the current cursor position.
func (tp *TreePicker) ExpandCollapseAtCursor() {
	if tp.Cursor < 0 || tp.Cursor >= len(tp.Rows) {
		return
	}
	row := tp.Rows[tp.Cursor]
	switch row.Kind {
	case RowGroup:
		tp.CollapsedGroups[row.GroupName] = !tp.CollapsedGroups[row.GroupName]
	case RowPack:
		tp.CollapsedPacks[row.Pack.Name] = !tp.CollapsedPacks[row.Pack.Name]
	}
	tp.RebuildRows()
	if tp.Cursor >= len(tp.Rows) {
		tp.Cursor = len(tp.Rows) - 1
	}
}

// ExpandAtCursor expands the group or pack at the cursor.
func (tp *TreePicker) ExpandAtCursor() {
	if tp.Cursor < 0 || tp.Cursor >= len(tp.Rows) {
		return
	}
	row := tp.Rows[tp.Cursor]
	switch row.Kind {
	case RowGroup:
		if tp.CollapsedGroups[row.GroupName] {
			tp.CollapsedGroups[row.GroupName] = false
			tp.RebuildRows()
		}
	case RowPack:
		if tp.CollapsedPacks[row.Pack.Name] {
			tp.CollapsedPacks[row.Pack.Name] = false
			tp.RebuildRows()
		}
	}
}

// CollapseAtCursor collapses the current level or moves to the parent.
func (tp *TreePicker) CollapseAtCursor() {
	if tp.Cursor < 0 || tp.Cursor >= len(tp.Rows) {
		return
	}
	row := tp.Rows[tp.Cursor]
	switch row.Kind {
	case RowGroup:
		if !tp.CollapsedGroups[row.GroupName] {
			tp.CollapsedGroups[row.GroupName] = true
			tp.RebuildRows()
			if tp.Cursor >= len(tp.Rows) {
				tp.Cursor = len(tp.Rows) - 1
			}
		} else {
			// Already collapsed — collapse all groups.
			for name := range tp.CollapsedGroups {
				tp.CollapsedGroups[name] = true
			}
			tp.RebuildRows()
			if tp.Cursor >= len(tp.Rows) {
				tp.Cursor = len(tp.Rows) - 1
			}
			tp.EnsureVisible()
		}
	case RowPack:
		if !tp.CollapsedPacks[row.Pack.Name] {
			tp.CollapsedPacks[row.Pack.Name] = true
			tp.RebuildRows()
			if tp.Cursor >= len(tp.Rows) {
				tp.Cursor = len(tp.Rows) - 1
			}
		} else {
			// Already collapsed — move to parent group.
			for i := tp.Cursor - 1; i >= 0; i-- {
				if tp.Rows[i].Kind == RowGroup {
					tp.Cursor = i
					tp.EnsureVisible()
					return
				}
			}
		}
	case RowItem:
		// Move to parent pack.
		for i := tp.Cursor - 1; i >= 0; i-- {
			if tp.Rows[i].Kind == RowPack {
				tp.Cursor = i
				tp.EnsureVisible()
				return
			}
		}
	}
}

// MaxVisible returns the number of visible rows based on terminal height.
// ReserveLines controls how many lines are reserved for chrome outside the tree
// (header, detail, summary, footer). Defaults to 10 if not set.
func (tp *TreePicker) MaxVisible() int {
	reserve := tp.ReserveLines
	if reserve == 0 {
		reserve = 10
	}
	v := tp.Height - reserve
	if v < 4 {
		v = 4
	}
	return v
}

// EnsureVisible adjusts ScrollOffset so the cursor stays in view.
func (tp *TreePicker) EnsureVisible() {
	maxVis := tp.MaxVisible()
	if tp.Cursor < tp.ScrollOffset+2 {
		tp.ScrollOffset = tp.Cursor - 2
		if tp.ScrollOffset < 0 {
			tp.ScrollOffset = 0
		}
	}
	if tp.Cursor >= tp.ScrollOffset+maxVis-2 {
		tp.ScrollOffset = tp.Cursor - maxVis + 3
		maxOff := len(tp.Rows) - maxVis
		if maxOff < 0 {
			maxOff = 0
		}
		if tp.ScrollOffset > maxOff {
			tp.ScrollOffset = maxOff
		}
	}
	if tp.ScrollOffset < 0 {
		tp.ScrollOffset = 0
	}
}

// TotalCheckedGB returns the total size of all checked sources.
func (tp *TreePicker) TotalCheckedGB() float64 {
	seen := make(map[string]bool)
	total := 0.0
	for _, g := range tp.Groups {
		for _, p := range g.Packs {
			for _, s := range p.Sources {
				if tp.CheckedIDs[s.ID] && !seen[s.ID] {
					seen[s.ID] = true
					total += s.SizeGB
				}
			}
		}
	}
	return total
}

// TotalCheckedCount returns the number of unique checked source IDs.
func (tp *TreePicker) TotalCheckedCount() int {
	return len(tp.CheckedIDs)
}

// CheckedIDSlice returns the checked IDs as a slice.
func (tp *TreePicker) CheckedIDSlice() []string {
	ids := make([]string, 0, len(tp.CheckedIDs))
	for id := range tp.CheckedIDs {
		ids = append(ids, id)
	}
	return ids
}

// IsDirty returns true if the checked state differs from the given initial state.
func (tp *TreePicker) IsDirty(initialIDs map[string]bool) bool {
	if len(tp.CheckedIDs) != len(initialIDs) {
		return true
	}
	for id := range tp.CheckedIDs {
		if !initialIDs[id] {
			return true
		}
	}
	return false
}

// PackCheckState returns the count of checked and total sources for a pack.
func PackCheckState(pack *Pack, checked map[string]bool) (int, int) {
	c := 0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			c++
		}
	}
	return c, len(pack.Sources)
}

// PackCheckedSizeGB returns the total size of checked sources in a pack.
func PackCheckedSizeGB(pack *Pack, checked map[string]bool) float64 {
	total := 0.0
	for _, s := range pack.Sources {
		if checked[s.ID] {
			total += s.SizeGB
		}
	}
	return total
}

// PackTypeSymbol returns the type symbol for a pack.
func PackTypeSymbol(pack *Pack) string {
	if len(pack.Sources) == 0 {
		return "·"
	}
	first := pack.Sources[0].Type
	for _, s := range pack.Sources[1:] {
		if s.Type != first {
			return "+"
		}
	}
	return TypeSymbol(first)
}

// PackHasBuild returns true if any source in the pack uses the "build" strategy.
func PackHasBuild(pack *Pack) bool {
	for _, s := range pack.Sources {
		if s.Strategy == "build" {
			return true
		}
	}
	return false
}

// RenderTree renders the tree rows as a string.
func (tp *TreePicker) RenderTree() string {
	var b strings.Builder

	maxVis := tp.MaxVisible()
	end := tp.ScrollOffset + maxVis
	if end > len(tp.Rows) {
		end = len(tp.Rows)
	}

	for i := tp.ScrollOffset; i < end; i++ {
		row := tp.Rows[i]
		isCursor := i == tp.Cursor
		prefix := "  "
		if isCursor {
			prefix = "> "
		}

		switch row.Kind {
		case RowGroup:
			label := row.GroupName
			if label == "" {
				label = "Other"
			}
			if isCursor {
				b.WriteString(tp.Theme.Selected.Render(prefix + label))
			} else {
				b.WriteString(tp.Theme.Section.Render(prefix + label))
			}

		case RowPack:
			pack := row.Pack
			checked, total := PackCheckState(pack, tp.CheckedIDs)
			mark := "·"
			if checked == total && total > 0 {
				mark = "✓"
			} else if checked > 0 {
				mark = "~"
			}
			suffix := fmt.Sprintf("%d/%d", checked, total)
			sym := PackTypeSymbol(pack)
			buildTag := ""
			if PackHasBuild(pack) {
				buildTag = " ⚒"
			}
			label := fmt.Sprintf("    %s%s %s %s%s  %s", prefix, mark, pack.Name, sym, buildTag, suffix)
			if isCursor {
				b.WriteString(tp.Theme.Selected.Render(label))
			} else if checked > 0 {
				b.WriteString(tp.Theme.Base.Render(label))
			} else {
				b.WriteString(tp.Theme.Muted.Render(label))
			}

		case RowItem:
			src := row.Source
			isAutoDep := tp.IsAutoDep(src.ID)
			mark := "·"
			if tp.CheckedIDs[src.ID] {
				mark = "✓"
			}
			strat := StrategySymbol(src.Strategy)
			if strat != "" {
				strat = " " + strat
			}
			line := fmt.Sprintf("        %s%s %s %s%s  %s", prefix, mark, src.ID, TypeSymbol(src.Type), strat, FormatSizeGB(src.SizeGB))
			if isAutoDep {
				line += "  ← dep"
			}
			if isCursor {
				if isAutoDep {
					b.WriteString(tp.Theme.SelectedMuted.Render(line))
				} else {
					b.WriteString(tp.Theme.Selected.Render(line))
				}
			} else if tp.CheckedIDs[src.ID] {
				if isAutoDep {
					b.WriteString(tp.Theme.Muted.Render(line))
				} else {
					b.WriteString(tp.Theme.Base.Render(line))
				}
			} else {
				b.WriteString(tp.Theme.Muted.Render(line))
			}

		case RowAction:
			b.WriteString("\n")
			label := tp.ActionLabel
			if label == "" {
				label = "Continue to review →"
			}
			line := prefix + label
			if isCursor {
				b.WriteString(tp.Theme.Success.Render(line))
			} else {
				b.WriteString(tp.Theme.Focus.Render(line))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderDetail renders compact detail lines for the row at the cursor.
func (tp *TreePicker) RenderDetail() string {
	if tp.Cursor < 0 || tp.Cursor >= len(tp.Rows) {
		return ""
	}

	row := tp.Rows[tp.Cursor]

	switch row.Kind {
	case RowGroup:
		packCount := len(row.GroupPacks)
		itemCount := 0
		for _, p := range row.GroupPacks {
			itemCount += len(p.Sources)
		}
		return tp.Theme.Muted.Render(fmt.Sprintf("  %d packs, %d sources", packCount, itemCount))

	case RowPack:
		pack := row.Pack
		checked, total := PackCheckState(pack, tp.CheckedIDs)
		size := PackCheckedSizeGB(pack, tp.CheckedIDs)
		info := tp.Theme.Muted.Render(fmt.Sprintf("  %d/%d selected · %s", checked, total, FormatSizeGB(size)))
		if pack.Description != "" {
			info += "\n" + tp.Theme.Muted.Render("  "+pack.Description)
		}
		return info

	case RowItem:
		src := row.Source
		stratLabel := "download"
		if src.Strategy == "build" {
			stratLabel = "build ⚒"
		}
		info := tp.Theme.Muted.Render(fmt.Sprintf("  %s · %s · %s", src.Type, stratLabel, FormatSizeGB(src.SizeGB)))
		if tp.IsAutoDep(src.ID) {
			info += "\n" + tp.Theme.Muted.Render("  auto-included — needed by another recipe")
		}
		if src.Description != "" {
			info += "\n" + tp.Theme.Muted.Render("  "+src.Description)
		}
		return info
	}

	return ""
}

// RenderSizeSummary renders the total/free size summary line.
func (tp *TreePicker) RenderSizeSummary() string {
	totalGB := tp.TotalCheckedGB()
	if tp.FreeGB <= 0 {
		return tp.Theme.Base.Render(fmt.Sprintf("  Total: %.1f GB", totalGB))
	}
	if totalGB <= tp.FreeGB {
		return tp.Theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, tp.FreeGB)) +
			tp.Theme.Success.Render("fits")
	}
	return tp.Theme.Base.Render(fmt.Sprintf("  Total: %.1f / %.0f GB  ", totalGB, tp.FreeGB)) +
		tp.Theme.Danger.Render(fmt.Sprintf("%.1f GB over", totalGB-tp.FreeGB))
}
