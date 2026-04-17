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
	cursor       int
	scrollOffset int
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

func newReviewModel(vaultPath string, items []ReviewItem, freeGB float64) reviewModel {
	// Sort items by type then ID for consistent display
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type < items[j].Type
		}
		return items[i].ID < items[j].ID
	})

	return reviewModel{
		vaultPath: vaultPath,
		items:     items,
		freeGB:    freeGB,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m reviewModel) Init() tea.Cmd { return nil }

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
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.ensureVisible()
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m reviewModel) maxVisible() int {
	// Shell chrome(4) + header(3) + detail(4) + footer(2) = 13
	avail := m.height - 13
	if avail < 3 {
		avail = 3
	}
	return avail
}

func (m *reviewModel) ensureVisible() {
	maxVis := m.maxVisible()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+maxVis {
		m.scrollOffset = m.cursor - maxVis + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m reviewModel) View() string {
	var b strings.Builder

	// Header
	totalGB := m.totalSizeGB()
	b.WriteString(fmt.Sprintf("  Target: %s\n", m.vaultPath))
	b.WriteString(fmt.Sprintf("  Items:  %d sources, %s", len(m.items), formatSizeGB(totalGB)))
	if m.freeGB > 0 {
		b.WriteString(fmt.Sprintf(" / %s free", formatSizeGB(m.freeGB)))
	}
	b.WriteString("\n\n")

	// Scrollable item list — one line per item: symbol + ID + size
	maxVis := m.maxVisible()
	endIdx := m.scrollOffset + maxVis
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	if m.scrollOffset > 0 {
		b.WriteString(m.theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		item := m.items[i]
		isCursor := i == m.cursor

		prefix := "  "
		if isCursor {
			prefix = "> "
		}

		sym := typeSymbol(item.Type)
		line := fmt.Sprintf("  %s%s %s  %s", prefix, sym, item.ID, formatSizeGB(item.SizeGB))

		if isCursor {
			b.WriteString(m.theme.Selected.Render(line))
		} else {
			b.WriteString(m.theme.Base.Render(line))
		}
		b.WriteString("\n")
	}

	if endIdx < len(m.items) {
		b.WriteString(m.theme.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}

	// Detail area for selected item
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  ─────────────────────────────────"))
	b.WriteString("\n")
	if m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s %s · %s · %s", typeSymbol(item.Type), item.ID, item.Type, formatSizeGB(item.SizeGB))))
		if item.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render("  " + item.Description))
		}
	}
	b.WriteString("\n\n")

	// Footer
	b.WriteString(m.theme.Help.Render("  Enter: confirm  |  Esc: go back  |  j/k: scroll"))

	return b.String()
}

func (m reviewModel) totalSizeGB() float64 {
	var total float64
	for _, item := range m.items {
		total += item.SizeGB
	}
	return total
}
