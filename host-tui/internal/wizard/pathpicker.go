package wizard

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
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
	customInput bool // true when browsing for a custom path
	picker      filepicker.Model
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

func newFilePicker() filepicker.Model {
	fp := filepicker.New()
	fp.CurrentDirectory, _ = os.Getwd()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.Cursor = ">"

	// Remove esc from the filepicker's Back binding so we handle it
	// ourselves to exit the file browser back to the list.
	fp.KeyMap.Back = key.NewBinding(
		key.WithKeys("h", "backspace", "left"),
		key.WithHelp("h", "back"),
	)

	return fp
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
		if m.customInput {
			pickerHeight := msg.Height - 7
			if pickerHeight < 4 {
				pickerHeight = 4
			}
			m.picker.SetHeight(pickerHeight)
		}
		return m, nil

	case tea.KeyMsg:
		if m.customInput {
			return m.updateFilePicker(msg)
		}
		return m.updateList(msg)
	}

	if m.customInput {
		// Forward non-key messages (readDirMsg etc.) to the filepicker.
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
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
			m.picker = newFilePicker()
			pickerHeight := m.height - 7
			if pickerHeight < 4 {
				pickerHeight = 4
			}
			m.picker.SetHeight(pickerHeight)
			return m, m.picker.Init()
		}
		return m, func() tea.Msg {
			return pathDoneMsg{path: opt.path, freeGB: opt.freeGB}
		}
	}

	return m, nil
}

// updateFilePicker handles key input when browsing for a custom path.
func (m pathPickerModel) updateFilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.Back.Matches(msg):
		// Esc exits the file browser back to the list.
		m.customInput = false
		return m, nil
	}

	// Forward to filepicker.
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	// Check if user selected a directory.
	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		return m, func() tea.Msg {
			return pathDoneMsg{path: path, freeGB: 0}
		}
	}

	return m, cmd
}

// View renders the path picker UI.
func (m pathPickerModel) View() string {
	if m.customInput {
		return m.viewFilePicker()
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

// viewFilePicker renders the file picker for custom path selection.
func (m pathPickerModel) viewFilePicker() string {
	var b strings.Builder

	b.WriteString(m.theme.Muted.Render("Select a directory for the new vault"))
	b.WriteString("\n")
	b.WriteString(m.theme.Section.Render(m.picker.CurrentDirectory))
	b.WriteString("\n\n")
	b.WriteString(m.picker.View())
	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("  enter select  h/← parent  esc cancel"))

	return b.String()
}
