package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
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
}

// pathPickerModel is the Bubble Tea sub-model for vault path selection.
type pathPickerModel struct {
	options []pathOption
	cursor  int
	input   textinput.Model // editable path text input
	errMsg  string          // validation error
	width   int
	height  int
	theme   tui.Theme
	keys    tui.KeyMap
}

// newPathPicker creates a pathPickerModel from detected volumes and a home
// volume. If prefill is non-empty it appears as the default input path,
// otherwise cwd + "/svalbard-vault" is used.
func newPathPicker(volumes []Volume, home Volume, prefill string) pathPickerModel {
	var opts []pathOption

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

	// Default input path.
	defaultPath := prefill
	if defaultPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = home.Path
		}
		defaultPath = filepath.Join(cwd, "svalbard-vault")
	}

	theme := tui.DefaultTheme()
	ti := textinput.New()
	ti.Prompt = ""
	ti.TextStyle = theme.Focus
	ti.Cursor.Style = theme.Focus
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.SetValue(defaultPath)
	ti.CursorEnd()
	ti.Focus()

	return pathPickerModel{
		options: opts,
		cursor:  -1, // -1 = text input focused
		input:   ti,
		theme:   theme,
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
		return m.updateKey(msg)
	}

	return m, nil
}

func (m pathPickerModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.MoveDown.Matches(msg):
		// Only navigate options when input is not focused, or move from input to first option.
		if m.cursor < len(m.options)-1 {
			m.cursor++
			m.input.SetValue(filepath.Join(m.options[m.cursor].path, "svalbard-vault"))
			m.input.CursorEnd()
			m.errMsg = ""
		}
		return m, nil

	case m.keys.MoveUp.Matches(msg):
		if m.cursor > -1 {
			m.cursor--
			if m.cursor >= 0 {
				m.input.SetValue(filepath.Join(m.options[m.cursor].path, "svalbard-vault"))
				m.input.CursorEnd()
			}
			m.errMsg = ""
		}
		return m, nil

	case m.keys.Enter.Matches(msg):
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			m.errMsg = "path cannot be empty"
			return m, nil
		}

		// Validate parent directory exists.
		parent := filepath.Dir(path)
		if info, err := os.Stat(parent); err != nil || !info.IsDir() {
			m.errMsg = fmt.Sprintf("parent directory does not exist: %s", parent)
			return m, nil
		}

		freeGB := 0.0
		if m.cursor >= 0 && m.cursor < len(m.options) {
			freeGB = m.options[m.cursor].freeGB
		}
		return m, func() tea.Msg {
			return pathDoneMsg{path: path, freeGB: freeGB}
		}
	}

	// All other keys — typed characters, backspace, left/right arrows, ctrl+w,
	// home/end, delete, ctrl+u, etc. — go to the textinput widget.
	m.cursor = -1 // back to free-form input
	m.errMsg = ""
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the path picker UI.
func (m pathPickerModel) View() string {
	var b strings.Builder

	b.WriteString(m.theme.Section.Render("Vault path"))
	b.WriteString("\n\n")

	// Quick-select volume options.
	if len(m.options) > 0 {
		b.WriteString(m.theme.Muted.Render("  Quick select:"))
		b.WriteString("\n")
		for i, opt := range m.options {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			networkTag := ""
			if opt.network {
				networkTag = " [network]"
			}

			line := fmt.Sprintf("%s%d) %s%s  %s", cursor, i+1, opt.label, networkTag, opt.detail)
			if i == m.cursor {
				b.WriteString(m.theme.Focus.Render(line))
			} else {
				b.WriteString(m.theme.Base.Render(line))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Text input for path.
	b.WriteString(m.theme.Base.Render("  Path: "))
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Validation error.
	if m.errMsg != "" {
		b.WriteString(m.theme.Error.Render("  " + m.errMsg))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("  Enter: confirm  j/k: quick select  Esc: back"))

	return b.String()
}
