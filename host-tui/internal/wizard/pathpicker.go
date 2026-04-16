package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// pathDoneMsg is sent when the user confirms a vault path selection.
type pathDoneMsg struct {
	path   string
	freeGB float64
}

// pathOption represents a single selectable entry in the path picker list.
type pathOption struct {
	label   string  // display label
	detail  string  // e.g. "50/64 GB"
	path    string  // filesystem path
	freeGB  float64 // free space in GB
	network bool    // true for network volumes
	custom  bool    // true for the "Custom path..." entry
}

// pathPickerModel is the Bubble Tea sub-model for vault path selection.
type pathPickerModel struct {
	options     []pathOption
	cursor      int
	customInput bool   // true when in custom path text entry mode
	inputBuffer string // text typed during custom input mode
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap
}

// newPathPicker creates a pathPickerModel from detected volumes and a home
// volume. If prefill is non-empty it appears as the first option.
func newPathPicker(volumes []Volume, home Volume, prefill string) pathPickerModel {
	var opts []pathOption

	// If prefill is provided, add it as the first option.
	if prefill != "" {
		opts = append(opts, pathOption{
			label:  prefill,
			detail: "",
			path:   prefill,
			freeGB: 0,
		})
	}

	// Add detected volumes.
	for _, v := range volumes {
		detail := fmt.Sprintf("%.0f/%.0f GB", v.FreeGB, v.TotalGB)
		opts = append(opts, pathOption{
			label:   v.Path,
			detail:  detail,
			path:    v.Path,
			freeGB:  v.FreeGB,
			network: v.Network,
		})
	}

	// Add home volume.
	detail := fmt.Sprintf("%.0f GB free", home.FreeGB)
	opts = append(opts, pathOption{
		label:  home.Path,
		detail: detail,
		path:   home.Path,
		freeGB: home.FreeGB,
	})

	// Add custom path entry.
	opts = append(opts, pathOption{
		label:  "Custom path...",
		custom: true,
	})

	return pathPickerModel{
		options: opts,
		cursor:  0,
		theme:   tui.DefaultTheme(),
		keys:    tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command is needed.
func (m pathPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the path picker.
func (m pathPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.customInput {
			return m.updateCustomInput(msg)
		}
		return m.updateList(msg)
	}

	return m, nil
}

// updateList handles key input when in list selection mode.
func (m pathPickerModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.MoveDown.Matches(msg):
		if m.cursor < len(m.options)-1 {
			m.cursor++
		}
		return m, nil

	case m.keys.MoveUp.Matches(msg):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case m.keys.Enter.Matches(msg):
		opt := m.options[m.cursor]
		if opt.custom {
			m.customInput = true
			m.inputBuffer = ""
			return m, nil
		}
		return m, func() tea.Msg {
			return pathDoneMsg{path: opt.path, freeGB: opt.freeGB}
		}
	}

	return m, nil
}

// updateCustomInput handles key input when in custom path text entry mode.
func (m pathPickerModel) updateCustomInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case msg.Type == tea.KeyEscape:
		m.customInput = false
		m.inputBuffer = ""
		return m, nil

	case msg.Type == tea.KeyEnter:
		path := m.inputBuffer
		if path != "" {
			return m, func() tea.Msg {
				return pathDoneMsg{path: path, freeGB: 0}
			}
		}
		return m, nil

	case msg.Type == tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
		return m, nil

	case msg.Type == tea.KeyRunes:
		m.inputBuffer += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

// View renders the path picker UI.
func (m pathPickerModel) View() string {
	if m.customInput {
		return m.viewCustomInput()
	}
	return m.viewList()
}

// viewList renders the numbered list of path options.
func (m pathPickerModel) viewList() string {
	var b strings.Builder

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		if opt.custom {
			line := fmt.Sprintf("%sc) %s", cursor, opt.label)
			if i == m.cursor {
				b.WriteString(m.theme.Focus.Render(line))
			} else {
				b.WriteString(m.theme.Base.Render(line))
			}
		} else {
			networkTag := ""
			if opt.network {
				networkTag = " [network]"
			}

			detailStr := ""
			if opt.detail != "" {
				detailStr = "  " + opt.detail
			}

			line := fmt.Sprintf("%s%d) %s%s%s", cursor, i+1, opt.label, networkTag, detailStr)
			if i == m.cursor {
				b.WriteString(m.theme.Focus.Render(line))
			} else {
				b.WriteString(m.theme.Base.Render(line))
			}
		}

		if i < len(m.options)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// viewCustomInput renders the custom path text entry UI.
func (m pathPickerModel) viewCustomInput() string {
	var b strings.Builder

	b.WriteString("  Enter path: ")
	b.WriteString(m.inputBuffer)
	b.WriteString("\u2588") // block cursor character
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render("  Press Enter to confirm, Esc to cancel"))

	return b.String()
}
