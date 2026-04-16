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
	presets   []PresetOption
	regions   []string
	freeGB    float64
	cursor    int
	hasCustom bool // always true; the Customize entry is appended
	width     int
	height    int
	theme     tui.Theme
	keys      tui.KeyMap
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
			}
			return m, nil

		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
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
	b.WriteString(fmt.Sprintf("  %s free\n\n", formatSizeGB(m.freeGB)))

	for i, p := range m.presets {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		size := formatSizeGB(p.ContentGB)

		// Check if preset exceeds free space
		extra := ""
		if p.ContentGB > m.freeGB {
			needed := p.ContentGB - m.freeGB
			extra = fmt.Sprintf(" (needs %s more)", formatSizeGB(needed))
		}

		line := fmt.Sprintf("%s%d) %-18s %8s  %s%s", cursor, i+1, p.Name, size, p.Description, extra)

		if i == m.cursor {
			b.WriteString(m.theme.Focus.Render(line))
		} else if p.ContentGB > m.freeGB {
			b.WriteString(m.theme.Muted.Render(line))
		} else {
			b.WriteString(m.theme.Base.Render(line))
		}

		b.WriteString("\n")
	}

	// Customize option
	if m.hasCustom {
		cursor := "  "
		customIdx := len(m.presets)
		if m.cursor == customIdx {
			cursor = "> "
		}

		line := fmt.Sprintf("%sc) Customize — browse all content", cursor)
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
