// Package welcome implements the screen shown when svalbard-tui is launched
// but no vault is found in the current directory tree.
package welcome

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/tui"
)

// welcomeDestination pairs an ID with a human-readable label shown in the nav list.
type welcomeDestination struct {
	id    string
	label string
}

var welcomeDestinations = []welcomeDestination{
	{"init", "Init Vault"},
	{"preset", "Choose Preset"},
}

// Model is the Bubble Tea model for the welcome / no-vault-found screen.
type Model struct {
	selected int
	width    int
	height   int
	theme    tui.Theme
	keys     tui.KeyMap
}

// New creates a welcome Model with the default theme and key map.
func New() Model {
	return Model{
		theme: tui.DefaultTheme(),
		keys:  tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and returns the updated model.
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
		case m.keys.Quit.Matches(msg):
			return m, tea.Quit
		case m.keys.Back.Matches(msg):
			return m, tea.Quit
		case m.keys.MoveDown.Matches(msg):
			if m.selected < len(welcomeDestinations)-1 {
				m.selected++
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case m.keys.Enter.Matches(msg):
			// Placeholder — no-op until wired in Task 3.5
			return m, nil
		}
	}

	return m, nil
}

// View renders the welcome screen using the shared tui layout primitives.
func (m Model) View() string {
	// Build navigation list from destinations
	items := make([]tui.NavItem, len(welcomeDestinations))
	for i, d := range welcomeDestinations {
		items[i] = tui.NavItem{ID: d.id, Label: d.label}
	}

	nav := tui.NavList{
		Items:    items,
		Selected: m.selected,
		Theme:    m.theme,
	}

	detail := tui.DetailPane{
		Theme: m.theme,
		Title: "Welcome",
		Body:  "No vault found in the current directory.\nCreate a new vault or browse presets to get started.",
	}

	footer := tui.FooterHints(
		m.keys.MoveUp,
		m.keys.Enter,
		m.keys.Quit,
	)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "no vault",
		Left:    nav.Render(),
		Right:   detail.Render(),
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

// Selected returns the current selection index (for testing).
func (m Model) Selected() int {
	return m.selected
}
