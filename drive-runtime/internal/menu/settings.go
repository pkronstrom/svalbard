package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

// settingsAction is the built-in action of the menu's Settings entry. Like
// search it is a screen of the menu itself, not a process to start.
const settingsAction = "settings"

type settingsMode int

const (
	settingsBrowse settingsMode = iota
	settingsPick                // choosing among a choice setting's values
	settingsEdit                // typing a number, text or secret
)

// settingsScreen is the Settings screen: one row per setting of the schema,
// every change saved as it is made.
type settingsScreen struct {
	store *settings.Store
	// loadErr is why the values file could not be read. The screen then shows
	// the defaults and the store refuses every change.
	loadErr error
	rows    []settings.Setting
	cursor  int
	offset  int // first list line shown when the list is taller than the terminal
	mode    settingsMode

	pickCursor int
	pickOffset int
	input      textinput.Model
	saveErr    error
}

func (m *Model) openSettings() {
	store, err := settings.Open(m.driveRoot)
	input := textinput.New()
	input.Prompt = "> "
	input.TextStyle = m.theme.Focus
	input.Cursor.Style = m.theme.Focus
	input.Cursor.SetMode(cursor.CursorStatic)
	screen := settingsScreen{store: store, loadErr: err, input: input}
	for _, group := range store.Groups() {
		screen.rows = append(screen.rows, group.Settings...)
	}
	m.settings = screen
	m.settingsActive = true
	m.inGroup = false
	m.activeGroup = ""
	m.itemSelected = 0
	m.status = ""
	m.lastErr = nil
}

func (s settingsScreen) selected() (settings.Setting, bool) {
	if s.cursor < 0 || s.cursor >= len(s.rows) {
		return settings.Setting{}, false
	}
	return s.rows[s.cursor], true
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	setting, ok := s.selected()

	switch s.mode {
	case settingsPick:
		switch msg.String() {
		case "up", "k":
			if s.pickCursor > 0 {
				s.pickCursor--
			}
		case "down", "j":
			if s.pickCursor < len(setting.Choices)-1 {
				s.pickCursor++
			}
		case "enter":
			if s.pickCursor < len(setting.Choices) {
				s.saveErr = s.store.Set(setting.Key(), setting.Choices[s.pickCursor])
			}
			s.mode = settingsBrowse
		case "esc", "q":
			s.mode = settingsBrowse
		}
		m.scrollSettings()
		return m, nil
	case settingsEdit:
		switch msg.String() {
		case "enter":
			// A value that does not parse keeps the edit line open on it.
			if s.saveErr = s.store.Set(setting.Key(), s.input.Value()); s.saveErr == nil {
				s.mode = settingsBrowse
				s.input.Blur()
			}
		case "esc":
			s.saveErr = nil
			s.mode = settingsBrowse
			s.input.Blur()
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return m, cmd
		}
		m.scrollSettings()
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		m.settingsActive = false
		m.settings = settingsScreen{}
		return m, nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
			s.saveErr = nil
		}
	case "down", "j":
		if s.cursor < len(s.rows)-1 {
			s.cursor++
			s.saveErr = nil
		}
	case "r":
		if ok {
			s.saveErr = s.store.Reset(setting.Key())
		}
	case "enter", " ":
		if !ok {
			break
		}
		switch setting.Type {
		case settings.TypeBool:
			s.saveErr = s.store.Set(setting.Key(), setting.Format(!s.store.Bool(setting.Key())))
		case settings.TypeChoice:
			s.mode = settingsPick
			s.saveErr = nil
			s.pickOffset = 0
			s.pickCursor = max(0, indexOf(setting.Choices, s.store.String(setting.Key())))
		default:
			s.mode = settingsEdit
			s.saveErr = nil
			s.input.EchoMode = textinput.EchoNormal
			if setting.Type == settings.TypeSecret {
				s.input.EchoMode = textinput.EchoPassword
				s.input.EchoCharacter = '•'
			}
			s.input.SetValue(setting.Format(s.store.Value(setting.Key())))
			s.input.CursorEnd()
			s.input.Focus()
		}
	}
	m.scrollSettings()
	return m, nil
}

func indexOf(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

// scrollSettings keeps the cursor of the list, or of the open picker, inside
// the lines the terminal has room for.
func (m *Model) scrollSettings() {
	layout := m.settingsLayout()
	if m.settings.mode == settingsPick {
		m.settings.pickOffset = layout.offset
	} else {
		m.settings.offset = layout.offset
	}
}

// settingsLayout is one frame of the Settings screen: fixed lines above and
// below a list that scrolls. top ends and bottom begins with a blank line;
// when the list scrolls those two lines carry the "more" marks, so the rows
// keep their place on the screen.
type settingsLayout struct {
	top    []string
	list   []string
	bottom []string
	offset int // first list line shown
	window int // list lines shown; len(list) when everything fits
}

const settingsDescription = "Choices saved on this drive (.svalbard/settings.json)."

func (m Model) settingsLayout() settingsLayout {
	s := m.settings
	width := m.width
	if width <= 0 {
		width = 80
	}

	layout := settingsLayout{top: []string{
		m.theme.Title.Render("Svalbard / Settings"),
		m.theme.Muted.Render(settingsDescription),
	}}
	if s.loadErr != nil {
		for _, line := range wrapText("Nothing can be changed until the file is fixed or deleted: "+s.loadErr.Error(), width) {
			layout.top = append(layout.top, m.theme.Error.Render(line))
		}
	}
	layout.top = append(layout.top, "")

	setting, ok := s.selected()
	// cursorLine is the list line that must stay in view; anchorLine is the
	// group heading above it, kept in view too when it is the line before.
	cursorLine, anchorLine, prevOffset := 0, 0, s.offset
	if s.mode == settingsPick && ok {
		prevOffset = s.pickOffset
		for _, line := range wrapText(setting.Label+": "+setting.Description, width) {
			layout.top = append(layout.top, m.theme.Muted.Render(line))
		}
		layout.top = append(layout.top, "")
		for i, choice := range setting.Choices {
			label := setting.Display(choice)
			if note := setting.ChoiceNotes[choice]; note != "" {
				label += " — " + note
			}
			if i == s.pickCursor {
				layout.list = append(layout.list, "▸ "+m.theme.Selected.Render(label))
			} else {
				layout.list = append(layout.list, "  "+label)
			}
		}
		cursorLine, anchorLine = s.pickCursor, s.pickCursor
		current := setting.Display(s.store.Value(setting.Key()))
		layout.bottom = []string{"", m.theme.Help.Render("↑/↓ select · enter confirm · esc keeps " + current)}
	} else {
		labelWidth := 0
		for _, row := range s.rows {
			labelWidth = max(labelWidth, lipgloss.Width(row.Label))
		}
		labelWidth += 2
		index := 0
		for _, group := range s.store.Groups() {
			layout.list = append(layout.list, m.theme.Section.Render(group.Label))
			for _, row := range group.Settings {
				mark := "  "
				if s.store.Changed(row.Key()) {
					mark = "* "
				}
				text := menuIndentedRow(strings.TrimRight(fmt.Sprintf("%-*s%s%s", labelWidth, row.Label, mark, settingValueText(s.store, row)), " "), index == s.cursor)
				if index == s.cursor {
					text = m.theme.Selected.Render(text)
					cursorLine, anchorLine = len(layout.list), len(layout.list)
					if index == 0 || s.rows[index-1].Owner != row.Owner {
						anchorLine--
					}
				}
				layout.list = append(layout.list, text)
				index++
			}
		}
		if len(s.rows) == 0 {
			layout.list = append(layout.list, m.theme.Help.Render("No settings available."))
		}

		layout.bottom = append(layout.bottom, "", m.theme.Section.Render("Selected"))
		if ok {
			for _, line := range wrapText(setting.Label+": "+setting.Description, width) {
				layout.bottom = append(layout.bottom, m.theme.Muted.Render(line))
			}
			if hint := settingHint(setting); hint != "" && s.mode == settingsBrowse {
				for _, line := range wrapText(hint, width) {
					layout.bottom = append(layout.bottom, m.theme.Muted.Render(line))
				}
			}
			if problem := s.store.Problem(setting.Key()); problem != "" {
				for _, line := range wrapText("Ignored: "+problem, width) {
					layout.bottom = append(layout.bottom, m.theme.Status.Render(line))
				}
			}
		}
		if s.mode == settingsEdit {
			layout.bottom = append(layout.bottom, s.input.View())
		}
		if s.saveErr != nil {
			for _, line := range wrapText("Not saved: "+s.saveErr.Error(), width) {
				layout.bottom = append(layout.bottom, m.theme.Error.Render(line))
			}
		}
		layout.bottom = append(layout.bottom, "")
		if s.mode == settingsEdit {
			layout.bottom = append(layout.bottom, m.theme.Help.Render("enter save · esc cancel"))
		} else {
			layout.bottom = append(layout.bottom,
				m.theme.Help.Render("enter change · r reset to default · esc back"),
				m.theme.Help.Render("* changed from the default"))
		}
	}

	layout.window = len(layout.list)
	if m.height > 0 {
		if room := m.height - len(layout.top) - len(layout.bottom); len(layout.list) > room {
			layout.window = max(1, room)
		}
	}
	layout.offset = min(prevOffset, len(layout.list)-layout.window)
	layout.offset = min(layout.offset, anchorLine)
	layout.offset = max(layout.offset, cursorLine-layout.window+1, 0)
	return layout
}

// settingValueText is the value column of a row. A secret is never spelled
// out on the screen.
func settingValueText(store *settings.Store, setting settings.Setting) string {
	value := store.Value(setting.Key())
	switch {
	case value == "" && (setting.Type == settings.TypeSecret || setting.Type == settings.TypeText):
		return "not set"
	case setting.Type == settings.TypeSecret:
		return "••••••••"
	}
	return setting.Display(value)
}

// settingHint says what the selected setting accepts.
func settingHint(setting settings.Setting) string {
	switch setting.Type {
	case settings.TypeBool:
		return "Choices: on, off"
	case settings.TypeChoice:
		labels := make([]string, len(setting.Choices))
		for i, choice := range setting.Choices {
			labels[i] = setting.Display(choice)
		}
		return "Choices: " + strings.Join(labels, ", ")
	case settings.TypeNumber:
		switch {
		case setting.Min != nil && setting.Max != nil:
			return fmt.Sprintf("A number from %s to %s", setting.Format(*setting.Min), setting.Format(*setting.Max))
		case setting.Min != nil:
			return fmt.Sprintf("A number, %s or more", setting.Format(*setting.Min))
		case setting.Max != nil:
			return fmt.Sprintf("A number, %s or less", setting.Format(*setting.Max))
		}
		return "A number"
	default:
		return ""
	}
}

func renderSettingsView(m Model) string {
	layout := m.settingsLayout()
	end := layout.offset + layout.window
	lines := append([]string(nil), layout.top...)
	if layout.offset > 0 {
		lines[len(lines)-1] = m.theme.Muted.Render(fmt.Sprintf("  ↑ %d more", layout.offset))
	}
	lines = append(lines, layout.list[layout.offset:end]...)
	lines = append(lines, layout.bottom...)
	if end < len(layout.list) {
		lines[len(layout.top)+layout.window] = m.theme.Muted.Render(fmt.Sprintf("  ↓ %d more", len(layout.list)-end))
	}
	// No final newline: it would be one line more than the terminal has when
	// the screen is exactly full, and scroll the title away.
	return strings.Join(lines, "\n")
}

// wrapText breaks text at spaces into lines of at most width cells. A word
// longer than a line is left whole.
func wrapText(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
