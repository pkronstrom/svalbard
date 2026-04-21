package importscreen

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// ImportResult is returned after a successful import.
type ImportResult struct {
	ID     string
	SizeGB float64
}

// BackMsg is sent when the user exits the import screen.
type BackMsg struct{}

// importDoneMsg is sent internally when an import completes.
type importDoneMsg struct {
	result ImportResult
}

// importErrMsg is sent internally when an import fails.
type importErrMsg struct {
	err error
}

// importEntry records a completed import in the history.
type importEntry struct {
	id     string
	sizeGB float64
}

// Config holds everything the import screen needs.
type Config struct {
	RunImport func(ctx context.Context, source string) (ImportResult, error)
}

// Model is the Bubble Tea model for the import screen.
type Model struct {
	runImport func(ctx context.Context, source string) (ImportResult, error)
	input     string
	results   []importEntry
	importing bool
	errMsg    string
	width     int
	height    int
	theme     tui.Theme
	keys      tui.KeyMap
}

// New creates an import screen model.
func New(cfg Config) Model {
	return Model{
		runImport: cfg.RunImport,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the import screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case importDoneMsg:
		m.importing = false
		m.results = append(m.results, importEntry{
			id:     msg.result.ID,
			sizeGB: msg.result.SizeGB,
		})
		m.errMsg = ""
		return m, nil

	case importErrMsg:
		m.importing = false
		m.errMsg = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit

		case m.keys.Back.Matches(msg):
			if !m.importing {
				return m, func() tea.Msg { return BackMsg{} }
			}
			return m, nil

		case m.keys.Quit.Matches(msg):
			// q acts as back only when input is empty and not importing.
			if m.input == "" && !m.importing {
				return m, func() tea.Msg { return BackMsg{} }
			}
			if !m.importing {
				m.input += string(msg.Runes)
				m.errMsg = ""
			}
			return m, nil

		case m.keys.Enter.Matches(msg):
			if m.importing || strings.TrimSpace(m.input) == "" {
				return m, nil
			}
			return m.startImport()

		case msg.Type == tea.KeyBackspace:
			if !m.importing && len(m.input) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.input)
				m.input = m.input[:len(m.input)-size]
				m.errMsg = ""
			}
			return m, nil

		case msg.Type == tea.KeyRunes:
			if !m.importing {
				m.input += string(msg.Runes)
				m.errMsg = ""
			}
			return m, nil
		}
	}
	return m, nil
}

// startImport fires the async import command.
func (m Model) startImport() (tea.Model, tea.Cmd) {
	if m.runImport == nil {
		m.errMsg = "import not available"
		return m, nil
	}
	source := strings.TrimSpace(m.input)
	m.importing = true
	m.input = ""
	m.errMsg = ""

	importFn := m.runImport
	cmd := func() tea.Msg {
		result, err := importFn(context.Background(), source)
		if err != nil {
			return importErrMsg{err: err}
		}
		return importDoneMsg{result: result}
	}
	return m, cmd
}

// View renders the import screen.
func (m Model) View() string {
	var body strings.Builder

	body.WriteString(m.theme.Section.Render("Import"))
	body.WriteString("\n\n")
	body.WriteString(m.theme.Muted.Render("Import content into your vault. Supported sources:"))
	body.WriteString("\n")
	body.WriteString(m.theme.Muted.Render("  - Local files     /path/to/file.zim, /path/to/archive.tar"))
	body.WriteString("\n")
	body.WriteString(m.theme.Muted.Render("  - HTTP/HTTPS URLs  https://example.com/content.zim"))
	body.WriteString("\n")
	body.WriteString(m.theme.Muted.Render("  - YouTube links    https://youtube.com/watch?v=..."))
	body.WriteString("\n\n")
	body.WriteString(m.theme.Warning.Render("  Note: URL and YouTube imports require Docker (svalbard install-deps)."))
	body.WriteString("\n\n")

	// Input line.
	if m.importing {
		body.WriteString(m.theme.Base.Render("  Source: "))
		body.WriteString(m.theme.Status.Render("importing..."))
	} else {
		body.WriteString(m.theme.Base.Render("  Source: "))
		body.WriteString(m.theme.Focus.Render(m.input))
		body.WriteString(m.theme.Focus.Render("\u2588")) // block cursor
	}
	body.WriteString("\n")

	// Error message.
	if m.errMsg != "" {
		body.WriteString("\n")
		body.WriteString(m.theme.Error.Render("  " + m.errMsg))
		body.WriteString("\n")
	}

	// Import history.
	if len(m.results) > 0 {
		body.WriteString("\n")
		body.WriteString(m.theme.Section.Render("  History"))
		body.WriteString("\n")
		for _, entry := range m.results {
			line := fmt.Sprintf("    %s  %.2f GB", entry.id, entry.sizeGB)
			body.WriteString(m.theme.Success.Render(line))
			body.WriteString("\n")
		}
	}

	// Footer.
	footer := tui.FooterHints(
		tui.KeyBinding{Key: "enter", Label: "Enter: import"},
		tui.KeyBinding{Key: "esc", Label: "Esc: back"},
	)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "import",
		Left:    body.String(),
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}
