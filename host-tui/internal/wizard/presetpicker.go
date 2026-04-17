package wizard

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// presetDoneMsg is sent when the user selects a preset (or Customize).
type presetDoneMsg struct {
	preset PresetOption
}

// presetPickerModel is the Bubble Tea sub-model for preset selection.
type presetPickerModel struct {
	presets      []PresetOption
	regions      []string
	freeGB       float64
	cursor       int
	scrollOffset int
	hasCustom    bool // always true; the Customize entry is appended
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

// newPresetPicker creates a presetPickerModel. presets must already be sorted
// by size (ascending). The cursor defaults to the recommended preset: the
// largest preset where ContentGB <= freeGB.
func newPresetPicker(presets []PresetOption, regions []string, freeGB float64) presetPickerModel {
	cursor := 0
	for i, p := range presets {
		if p.ContentGB <= freeGB {
			cursor = i
		}
	}

	return presetPickerModel{
		presets:   presets,
		regions:   regions,
		freeGB:    freeGB,
		cursor:    cursor,
		hasCustom: true,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

// itemCount returns the total number of selectable items (presets + Customize).
func (m presetPickerModel) itemCount() int {
	n := len(m.presets)
	if m.hasCustom {
		n++
	}
	return n
}

func (m presetPickerModel) maxVisible() int {
	// Reserve lines for header (2) and footer (2) and customize entry (2)
	avail := m.height - 6
	// Each preset takes 2 lines (name + description), plus region headers
	if avail < 4 {
		avail = 4
	}
	return avail / 2 // approximate: 2 lines per item
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

// Init satisfies tea.Model. No initial command is needed.
func (m presetPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the preset picker.
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
			// Customize option is the last item
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

// View renders the preset picker UI.
func (m presetPickerModel) View() string {
	var b strings.Builder

	// Free space header
	if m.freeGB > 0 {
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s free", formatSizeGB(m.freeGB))))
	} else {
		b.WriteString(m.theme.Muted.Render("  Free space unknown"))
	}
	b.WriteString("\n\n")

	maxVis := m.maxVisible()
	endIdx := m.scrollOffset + maxVis
	if endIdx > len(m.presets) {
		endIdx = len(m.presets)
	}

	// Show scroll indicator at top
	if m.scrollOffset > 0 {
		b.WriteString(m.theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	prevRegion := ""
	for i := m.scrollOffset; i < endIdx; i++ {
		p := m.presets[i]

		// Region header
		if p.Region != "" && p.Region != prevRegion {
			if prevRegion != "" {
				b.WriteString("\n")
			}
			b.WriteString(m.theme.Section.Render("  " + p.Region))
			b.WriteString("\n")
			prevRegion = p.Region
		}

		isCursor := i == m.cursor
		tooLarge := m.freeGB > 0 && p.ContentGB > m.freeGB

		// Line 1: cursor + name + size
		caret := "  "
		if isCursor {
			caret = "> "
		}
		size := formatSizeGB(p.ContentGB)
		nameLine := fmt.Sprintf("%s%-20s %8s", caret, p.Name, size)

		if tooLarge && !isCursor {
			nameLine += "  (too large)"
		}

		// Line 2: description (indented)
		descLine := "    " + p.Description

		if isCursor {
			b.WriteString(m.theme.Focus.Render(nameLine))
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render(descLine))
		} else if tooLarge {
			b.WriteString(m.theme.Muted.Render(nameLine))
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render(descLine))
		} else {
			b.WriteString(m.theme.Base.Render(nameLine))
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render(descLine))
		}
		b.WriteString("\n")
	}

	// Show scroll indicator at bottom
	if endIdx < len(m.presets) {
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
	}

	return b.String()
}

// formatSizeGB formats a size in GB to a human-readable string.
// Values < 1 GB are shown as "~N MB", values >= 1 GB as "~N GB".
func formatSizeGB(gb float64) string {
	if gb < 1 {
		mb := gb * 1024
		return fmt.Sprintf("~%.0f MB", math.Round(mb))
	}
	return fmt.Sprintf("~%.0f GB", math.Round(gb))
}
