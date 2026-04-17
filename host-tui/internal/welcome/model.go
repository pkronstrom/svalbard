// Package welcome implements the screen shown when svalbard-tui is launched
// but no vault is found in the current directory tree.
package welcome

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/tui"
)

// welcomeDestination pairs an ID with a human-readable label and description.
type welcomeDestination struct {
	id    string
	label string
	desc  string
}

var welcomeDestinations = []welcomeDestination{
	{"new-vault", "New Vault", "setup wizard"},
	{"open-vault", "Open Vault", "existing vault"},
	{"browse", "Browse", "explore catalog"},
}

// SelectMsg is sent when the user activates a welcome destination.
type SelectMsg struct {
	ID string // "new-vault", "open-vault", or "browse"
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
			dest := welcomeDestinations[m.selected]
			return m, func() tea.Msg { return SelectMsg{ID: dest.id} }
		default:
			if idx, ok := tui.NumberKeyIndex(msg, len(welcomeDestinations)); ok {
				m.selected = idx
				return m, nil
			}
		}
	}

	return m, nil
}

// View renders the welcome screen using the shared tui layout primitives.
func (m Model) View() string {
	// Build navigation list from destinations
	items := make([]tui.NavItem, len(welcomeDestinations))
	for i, d := range welcomeDestinations {
		items[i] = tui.NavItem{
			ID:          d.id,
			Label:       d.label,
			Description: d.desc,
		}
	}

	nav := tui.NavList{
		Items:       items,
		Selected:    m.selected,
		Theme:       m.theme,
		ShowNumbers: true,
	}

	detail := welcomeContext(welcomeDestinations[m.selected].id, m.theme)

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

func welcomeContext(id string, theme tui.Theme) tui.DetailPane {
	d := tui.DetailPane{Theme: theme}
	switch id {
	case "new-vault":
		d.Title = "New Vault"
		d.Body = "Create a new offline knowledge vault.\nChoose a storage path, select target platforms,\npick a preset, and customize content."
	case "open-vault":
		d.Title = "Open Vault"
		d.Body = "Point to an existing vault directory.\nThe vault must contain a manifest.yaml file."
	case "browse":
		d.Title = "Browse"
		d.Body = "Explore the available content catalog.\nSee what encyclopedias, maps, tools, and\nreference material can be included in a vault."
	default:
		d.Title = "Welcome"
		d.Body = "No vault found in the current directory.\nCreate a new vault or open an existing one."
	}
	return d
}
