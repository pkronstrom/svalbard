package dashboard

import (
	"github.com/pkronstrom/svalbard/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// Destination IDs used for routing and detail pane content.
const (
	destStatus   = "status"
	destBrowse   = "browse"
	destPlan     = "plan"
	destImport   = "import"
	destIndex    = "index"
	destNewVault = "new-vault"
)

type destination struct{ id, label, desc string }

var hostDestinations = []destination{
	{destStatus, "Status", "vault health & sync state"},
	{destBrowse, "Browse", "explore & select content"},
	{destPlan, "Plan", "preview pending changes"},
	{destImport, "Import", "local files, URLs, YouTube"},
	{destIndex, "Index", "keyword & semantic search"},
	{destNewVault, "New Vault", "init wizard for another vault"},
}

// separatorBefore lists destination IDs that should have a separator above them.
var separatorBefore = map[string]bool{destNewVault: true}

// NewVaultMsg is sent when the user selects "New Vault" from the dashboard.
type NewVaultMsg struct{}

// SelectMsg is sent when the user selects a dashboard destination.
type SelectMsg struct {
	ID string
}

// Model is the host-side vault dashboard — the main screen shown when
// svalbard resolves a vault. It uses the shared tui/ components for a
// two-pane layout.
type Model struct {
	vaultPath  string
	selected   int
	width      int
	height     int
	theme      tui.Theme
	keys       tui.KeyMap
	statusMsg  string // transient status message shown in detail pane
}

// New creates a new dashboard Model for the given vault path.
func New(vaultPath string) Model {
	return Model{
		vaultPath: vaultPath,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and returns the updated model and any command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SelectMsg:
		m.statusMsg = msg.ID + ": not yet implemented"
		return m, nil

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
			if m.selected < len(hostDestinations)-1 {
				m.selected++
				m.statusMsg = ""
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.selected > 0 {
				m.selected--
				m.statusMsg = ""
			}
			return m, nil
		case m.keys.Enter.Matches(msg):
			return m, m.selectCurrent()
		default:
			if idx, ok := tui.NumberKeyIndex(msg, len(hostDestinations)); ok {
				m.selected = idx
				return m, nil
			}
		}
	}

	return m, nil
}

// selectCurrent returns a command for the currently selected destination.
func (m Model) selectCurrent() tea.Cmd {
	dest := hostDestinations[m.selected]
	switch dest.id {
	case destNewVault:
		return func() tea.Msg { return NewVaultMsg{} }
	case destStatus:
		// Status is right-pane only, no sub-screen.
		return nil
	default:
		return func() tea.Msg { return SelectMsg{ID: dest.id} }
	}
}
