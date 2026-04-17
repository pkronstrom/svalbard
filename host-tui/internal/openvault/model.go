package openvault

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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
	input  string
	errMsg string
	width  int
	height int
	theme  tui.Theme
	keys   tui.KeyMap
}

// New creates an open-vault model.
func New() Model {
	return Model{
		theme: tui.DefaultTheme(),
		keys:  tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the open-vault screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit

		case m.keys.Back.Matches(msg):
			return m, func() tea.Msg { return BackMsg{} }

		case m.keys.Quit.Matches(msg):
			// q in this screen acts as back, not quit, since user is typing.
			// But only if input is empty — otherwise it's a character.
			if m.input == "" {
				return m, func() tea.Msg { return BackMsg{} }
			}
			// Fall through to rune handling below.
			m.input += string(msg.Runes)
			m.errMsg = ""
			return m, nil

		case m.keys.Enter.Matches(msg):
			return m.validate()

		case msg.Type == tea.KeyBackspace:
			if len(m.input) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.input)
				m.input = m.input[:len(m.input)-size]
				m.errMsg = ""
			}
			return m, nil

		case msg.Type == tea.KeyRunes:
			m.input += string(msg.Runes)
			m.errMsg = ""
			return m, nil
		}
	}
	return m, nil
}

// validate checks that the input path contains a manifest.yaml.
func (m Model) validate() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.input)
	if path == "" {
		m.errMsg = "Path cannot be empty."
		return m, nil
	}

	// Expand ~ to home directory.
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Clean the path.
	path = filepath.Clean(path)

	// Check for manifest.yaml inside the path.
	manifest := filepath.Join(path, "manifest.yaml")
	if _, err := os.Stat(manifest); err != nil {
		if os.IsNotExist(err) {
			m.errMsg = "No manifest.yaml found at that path."
		} else {
			m.errMsg = "Cannot access path: " + err.Error()
		}
		return m, nil
	}

	finalPath := path
	return m, func() tea.Msg { return DoneMsg{Path: finalPath} }
}

// View renders the open-vault screen.
func (m Model) View() string {
	var body strings.Builder

	body.WriteString(m.theme.Section.Render("Open Vault"))
	body.WriteString("\n\n")
	body.WriteString(m.theme.Muted.Render("Enter the path to an existing Svalbard vault directory."))
	body.WriteString("\n\n")

	// Input line with blinking cursor block.
	body.WriteString(m.theme.Base.Render("  Path: "))
	body.WriteString(m.theme.Focus.Render(m.input))
	body.WriteString(m.theme.Focus.Render("\u2588")) // block cursor
	body.WriteString("\n")

	// Error message.
	if m.errMsg != "" {
		body.WriteString("\n")
		body.WriteString(m.theme.Error.Render("  " + m.errMsg))
		body.WriteString("\n")
	}

	// Footer.
	footer := tui.FooterHints(
		m.keys.Enter,
		m.keys.Back,
	)

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
