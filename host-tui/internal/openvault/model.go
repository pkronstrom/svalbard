package openvault

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// DoneMsg is sent when the user submits a valid vault path.
type DoneMsg struct {
	Path string
}

// BackMsg is sent when the user cancels.
type BackMsg struct{}

// Model is the Bubble Tea model for the open-vault screen.
type Model struct {
	picker filepicker.Model
	errMsg string
	width  int
	height int
	theme  tui.Theme
	keys   tui.KeyMap
}

// New creates an open-vault model with a directory-only file picker.
func New() Model {
	fp := filepicker.New()
	fp.CurrentDirectory, _ = os.UserHomeDir()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.Cursor = ">"

	// Remove esc from the filepicker's Back binding so we can handle it
	// ourselves for screen-level back navigation. Keep h/backspace/left
	// for navigating to the parent directory.
	fp.KeyMap.Back = key.NewBinding(
		key.WithKeys("h", "backspace", "left"),
		key.WithHelp("h", "back"),
	)

	return Model{
		picker: fp,
		theme:  tui.DefaultTheme(),
		keys:   tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model and kicks off the initial directory read.
func (m Model) Init() tea.Cmd {
	return m.picker.Init()
}

// Update handles messages for the open-vault screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve lines for shell chrome: top bar(1) + blank(1) + header(2) + blank(1) + footer(1) + blank(1)
		pickerHeight := msg.Height - 7
		if pickerHeight < 4 {
			pickerHeight = 4
		}
		m.picker.SetHeight(pickerHeight)

	case tea.KeyMsg:
		// Intercept esc and q before the filepicker sees them.
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Back.Matches(msg):
			return m, func() tea.Msg { return BackMsg{} }
		case m.keys.Quit.Matches(msg):
			return m, func() tea.Msg { return BackMsg{} }
		}
	}

	// Forward to filepicker.
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	// Check if user selected a directory.
	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		return m, m.validate(path)
	}

	return m, cmd
}

// validate checks that the selected path contains a manifest.yaml.
func (m Model) validate(path string) tea.Cmd {
	manifest := filepath.Join(path, "manifest.yaml")
	if _, err := os.Stat(manifest); err != nil {
		// Not a vault — clear selection, stay in picker. We can't set errMsg
		// from a cmd, so we show it inline next update. For now, the picker
		// just descends into the directory which is fine — user can keep browsing.
		return nil
	}
	finalPath := path
	return func() tea.Msg { return DoneMsg{Path: finalPath} }
}

// View renders the open-vault screen.
func (m Model) View() string {
	var body strings.Builder

	body.WriteString(m.theme.Muted.Render("Select a directory containing manifest.yaml"))
	body.WriteString("\n")
	body.WriteString(m.theme.Section.Render(m.picker.CurrentDirectory))
	body.WriteString("\n\n")
	body.WriteString(m.picker.View())

	if m.errMsg != "" {
		body.WriteString("\n")
		body.WriteString(m.theme.Error.Render("  " + m.errMsg))
	}

	footer := "Enter: select | h/←: parent | Esc: back"

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "open vault",
		Left:    body.String(),
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}
