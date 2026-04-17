package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// presetDoneMsg is sent when the user selects a preset (or Customize).
type presetDoneMsg struct {
	preset PresetOption
}

// presetCancelMsg is sent when the user navigates back from the preset picker.
type presetCancelMsg struct{}

// presetPickerModel is the Bubble Tea sub-model for preset selection.
type presetPickerModel struct {
	presets      []PresetOption
	regions      []string
	freeGB       float64
	cursor       int
	recommended  int // index of recommended preset (largest that fits)
	scrollOffset int
	hasCustom    bool
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

func newPresetPicker(presets []PresetOption, regions []string, freeGB float64) presetPickerModel {
	recommended := 0
	for i, p := range presets {
		if p.ContentGB <= freeGB {
			recommended = i
		}
	}

	return presetPickerModel{
		presets:      presets,
		regions:      regions,
		freeGB:       freeGB,
		cursor:       recommended,
		recommended:  recommended,
		scrollOffset: 0, // always start at top
		hasCustom:    true,
		theme:        tui.DefaultTheme(),
		keys:         tui.DefaultKeyMap(),
	}
}

func (m presetPickerModel) itemCount() int {
	n := len(m.presets)
	if m.hasCustom {
		n++
	}
	return n
}

// maxVisible returns how many preset lines fit in the scroll area.
// Each preset is 1 line. Region headers and other chrome are accounted for.
func (m presetPickerModel) maxVisible() int {
	// Shell chrome: top bar(1) + blanks(2) + footer(1) = 4
	// Own chrome: free space header(2) + customize(3) + detail area(4) = 9
	avail := m.height - 13
	if avail < 3 {
		avail = 3
	}
	return avail
}

func (m *presetPickerModel) ensureVisible() {
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

func (m presetPickerModel) Init() tea.Cmd { return nil }

func (m presetPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Back.Matches(msg), m.keys.Quit.Matches(msg):
			return m, func() tea.Msg { return presetCancelMsg{} }
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < m.itemCount()-1 {
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
		case m.keys.Enter.Matches(msg):
			if m.cursor >= len(m.presets) {
				return m, func() tea.Msg {
					return presetDoneMsg{preset: PresetOption{}}
				}
			}
			selected := m.presets[m.cursor]
			return m, func() tea.Msg {
				return presetDoneMsg{preset: selected}
			}
		}
	}
	return m, nil
}

func (m presetPickerModel) View() string {
	var b strings.Builder

	// Free space header
	if m.freeGB > 0 {
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s free", tui.FormatSizeGB(m.freeGB))))
	} else {
		b.WriteString(m.theme.Muted.Render("  Free space unknown"))
	}
	b.WriteString("\n\n")

	// Determine visible range
	maxVis := m.maxVisible()
	totalPresets := len(m.presets)

	startIdx := m.scrollOffset
	endIdx := startIdx + maxVis
	if endIdx > totalPresets {
		endIdx = totalPresets
	}

	// Scroll indicator top
	if startIdx > 0 {
		b.WriteString(m.theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Preset list — one line per item
	prevRegion := ""
	// Track region of items before the visible window for proper headers
	if startIdx > 0 {
		prevRegion = m.presets[startIdx-1].Region
	}

	for i := startIdx; i < endIdx; i++ {
		p := m.presets[i]

		// Region header when region changes
		if p.Region != "" && p.Region != prevRegion {
			if i > startIdx {
				b.WriteString("\n")
			}
			b.WriteString(m.theme.Section.Render("  " + p.Region))
			b.WriteString("\n")
			prevRegion = p.Region
		}

		isCursor := i == m.cursor
		tooLarge := m.freeGB > 0 && p.ContentGB > m.freeGB
		isRecommended := i == m.recommended

		// Build the line: caret + name + size + markers
		caret := "  "
		if isCursor {
			caret = "> "
		}

		size := tui.FormatSizeGB(p.ContentGB)
		line := fmt.Sprintf("%s%-20s %8s", caret, p.Name, size)

		if isRecommended && !isCursor {
			line += "  *"
		}
		if tooLarge && !isCursor {
			line += "  (too large)"
		}

		switch {
		case isCursor:
			b.WriteString(m.theme.Focus.Render(line))
		case tooLarge:
			b.WriteString(m.theme.Muted.Render(line))
		default:
			b.WriteString(m.theme.Base.Render(line))
		}
		b.WriteString("\n")
	}

	// Scroll indicator bottom
	if endIdx < totalPresets {
		b.WriteString(m.theme.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}

	// Customize option
	if m.hasCustom {
		b.WriteString("\n")
		customIdx := len(m.presets)
		caret := "  "
		if m.cursor == customIdx {
			caret = "> "
		}
		line := caret + "Customize — browse all content"
		if m.cursor == customIdx {
			b.WriteString(m.theme.Focus.Render(line))
		} else {
			b.WriteString(m.theme.Base.Render(line))
		}
		b.WriteString("\n")
	}

	// Description area for selected preset
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  ─────────────────────────────────"))
	b.WriteString("\n")
	if m.cursor < len(m.presets) {
		desc := m.presets[m.cursor].Description
		if desc != "" {
			b.WriteString(m.theme.Muted.Render("  " + desc))
		}
	} else {
		b.WriteString(m.theme.Muted.Render("  Browse the full content catalog and pick individual items."))
	}
	b.WriteString("\n\n")
	b.WriteString(m.theme.Help.Render("  j/k navigate  enter select  esc back"))

	return b.String()
}

