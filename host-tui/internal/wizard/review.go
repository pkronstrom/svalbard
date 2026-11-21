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
	Unavailable bool
}

// reviewConfirmMsg is sent when the user confirms the review.
type reviewConfirmMsg struct{}

// reviewBackMsg is sent when the user navigates back from the review.
type reviewBackMsg struct{}

// reviewModel is the Bubble Tea sub-model for the review/confirmation screen.
type reviewModel struct {
	vaultPath    string
	items        []ReviewItem
	missingIDs   []string
	freeGB       float64
	cursor       int
	scrollOffset int
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
	errMsg       string
}

func newReviewModel(vaultPath string, items []ReviewItem, missingIDs []string, freeGB float64) reviewModel {
	// Sort items by ID for consistent display.
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	return reviewModel{
		vaultPath:  vaultPath,
		items:      items,
		missingIDs: append([]string(nil), missingIDs...),
		freeGB:     freeGB,
		theme:      tui.DefaultTheme(),
		keys:       tui.DefaultKeyMap(),
	}
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
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
	avail := m.height - 13 - validationLines(m.width, reviewValidation(m))
	if avail < 1 {
		avail = 1
	}
	return avail
}

func reviewValidation(m reviewModel) string {
	message := ""
	if len(m.missingIDs) > 0 {
		message = "  Missing catalog recipes: " + strings.Join(m.missingIDs, ", ")
	}
	if m.errMsg != "" {
		if message != "" {
			message += "\n"
		}
		message += "  " + m.errMsg
	}
	return message
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
	if validation := reviewValidation(m); validation != "" {
		b.WriteString(m.theme.Danger.Render(validationText(m.width, validation)))
		b.WriteString("\n")
	}

	// Header
	totalGB := m.totalSizeGB()
	b.WriteString(fmt.Sprintf("  Target: %s\n", m.vaultPath))
	label := tui.FormatSizeGB(totalGB)
	if m.hasUnavailable() {
		label = "known-size subtotal " + label
	}
	b.WriteString(fmt.Sprintf("  Items:  %d sources, %s", len(m.items), label))
	if m.freeGB > 0 {
		b.WriteString(fmt.Sprintf(" / %s free", tui.FormatSizeGB(m.freeGB)))
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

		sym := tui.TypeSymbol(item.Type)
		size := tui.FormatSizeGB(item.SizeGB)
		if item.Unavailable {
			size = "unknown size"
		}
		line := fmt.Sprintf("  %s%s %s  %s", prefix, sym, item.ID, size)

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
		size := tui.FormatSizeGB(item.SizeGB)
		if item.Unavailable {
			size = "unknown size"
		}
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s %s · %s · %s", tui.TypeSymbol(item.Type), item.ID, item.Type, size)))
		if item.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render("  " + item.Description))
		}
	}
	b.WriteString("\n\n")
	// Footer
	b.WriteString(m.theme.Help.Render("  j/k scroll  enter confirm  esc back"))

	return b.String()
}

func (m reviewModel) totalSizeGB() float64 {
	var total float64
	for _, item := range m.items {
		if !item.Unavailable {
			total += item.SizeGB
		}
	}
	return total
}

func (m reviewModel) hasUnavailable() bool {
	for _, item := range m.items {
		if item.Unavailable {
			return true
		}
	}
	return false
}
