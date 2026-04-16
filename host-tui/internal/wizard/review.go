package wizard

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// ReviewItem is a resolved item for the review screen.
type ReviewItem struct {
	ID          string
	Type        string
	SizeGB      float64
	Description string
}

// reviewConfirmMsg is sent when the user confirms the review.
type reviewConfirmMsg struct{}

// reviewBackMsg is sent when the user navigates back from the review.
type reviewBackMsg struct{}

// reviewModel is the Bubble Tea sub-model for the review/confirmation screen.
type reviewModel struct {
	vaultPath    string
	items        []ReviewItem
	freeGB       float64
	scrollOffset int
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

// newReviewModel creates a reviewModel with the given vault path, items, and free space.
func newReviewModel(vaultPath string, items []ReviewItem, freeGB float64) reviewModel {
	return reviewModel{
		vaultPath:    vaultPath,
		items:        items,
		freeGB:       freeGB,
		scrollOffset: 0,
		theme:        tui.DefaultTheme(),
		keys:         tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command is needed.
func (m reviewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the review model.
func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit

		case m.keys.Enter.Matches(msg):
			return m, func() tea.Msg { return reviewConfirmMsg{} }

		case m.keys.Back.Matches(msg):
			return m, func() tea.Msg { return reviewBackMsg{} }

		case m.keys.MoveDown.Matches(msg):
			maxOffset := m.maxScrollOffset()
			if m.scrollOffset < maxOffset {
				m.scrollOffset++
			}
			return m, nil

		case m.keys.MoveUp.Matches(msg):
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the review screen.
func (m reviewModel) View() string {
	var b strings.Builder

	// Header: target and summary
	totalGB := m.totalSizeGB()
	b.WriteString(fmt.Sprintf("  Target: %s\n", m.vaultPath))
	b.WriteString(fmt.Sprintf("  Items:  %d sources, %.1f GB / %.0f GB free\n", len(m.items), totalGB, m.freeGB))
	b.WriteString("\n")

	// Group items by type
	groups := m.groupByType()
	keys := sortedKeys(groups)

	// Build all item lines
	var lines []string
	for _, key := range keys {
		for _, item := range groups[key] {
			line := fmt.Sprintf("  %-20s %-10s %8s  %s",
				item.ID,
				strings.ToUpper(item.Type),
				formatSizeGB(item.SizeGB),
				item.Description,
			)
			lines = append(lines, line)
		}
	}

	// Apply scrolling
	visibleLines := m.visibleLineCount()
	start := m.scrollOffset
	if start > len(lines) {
		start = len(lines)
	}
	end := start + visibleLines
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(lines) > visibleLines && m.scrollOffset < m.maxScrollOffset() {
		b.WriteString(m.theme.Muted.Render("  ... scroll for more"))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  Enter to confirm  |  Esc to go back"))

	return b.String()
}

// totalSizeGB returns the sum of all item sizes.
func (m reviewModel) totalSizeGB() float64 {
	var total float64
	for _, item := range m.items {
		total += item.SizeGB
	}
	return total
}

// groupByType groups items by their Type field.
func (m reviewModel) groupByType() map[string][]ReviewItem {
	groups := make(map[string][]ReviewItem)
	for _, item := range m.items {
		groups[item.Type] = append(groups[item.Type], item)
	}
	// Sort items within each group alphabetically by ID
	for key := range groups {
		sort.Slice(groups[key], func(i, j int) bool {
			return groups[key][i].ID < groups[key][j].ID
		})
	}
	return groups
}

// visibleLineCount returns the number of item lines that fit in the viewport.
// Reserves space for header (3 lines), footer (2 lines), and scroll hint (1 line).
func (m reviewModel) visibleLineCount() int {
	if m.height <= 0 {
		return 20 // sensible default when height is unknown
	}
	available := m.height - 6 // 3 header + 2 footer + 1 buffer
	if available < 1 {
		return 1
	}
	return available
}

// maxScrollOffset returns the maximum scroll offset based on item count and visible lines.
func (m reviewModel) maxScrollOffset() int {
	totalLines := m.totalItemLines()
	visible := m.visibleLineCount()
	if totalLines <= visible {
		return 0
	}
	return totalLines - visible
}

// totalItemLines returns the total number of rendered item lines.
func (m reviewModel) totalItemLines() int {
	groups := m.groupByType()
	count := 0
	for _, items := range groups {
		count += len(items)
	}
	return count
}

// sortedKeys returns the sorted keys of a map[string][]ReviewItem.
func sortedKeys(m map[string][]ReviewItem) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatSizeGB is defined in presetpicker.go
