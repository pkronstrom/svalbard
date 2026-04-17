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
	{destStatus, "Status", "health & sync"},
	{destBrowse, "Browse", "select content"},
	{destPlan, "Plan", "pending changes"},
	{destImport, "Import", "files & URLs"},
	{destIndex, "Index", "search indexes"},
	{destNewVault, "New Vault", "init wizard"},
}

// separatorBefore lists destination IDs that should have a separator above them.
var separatorBefore = map[string]bool{destNewVault: true}

// NewVaultMsg is sent when the user selects "New Vault" from the dashboard.
type NewVaultMsg struct{}

// SelectMsg is sent when the user selects a dashboard destination.
type SelectMsg struct {
	ID string
}

// StatusData holds live vault status for the right-pane preview.
type StatusData struct {
	PresetName    string
	DesiredCount  int
	RealizedCount int
	PendingCount  int
	DiskUsedGB    float64
	DiskFreeGB    float64
	LastApplied   string
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
	status     *StatusData
	loadStatus func() (StatusData, error)
	showHelp   bool
}

// Config holds optional configuration for the dashboard.
type Config struct {
	LoadStatus func() (StatusData, error)
}

// New creates a new dashboard Model for the given vault path.
func New(vaultPath string, opts ...Config) Model {
	m := Model{
		vaultPath: vaultPath,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
	if len(opts) > 0 && opts[0].LoadStatus != nil {
		m.loadStatus = opts[0].LoadStatus
	}
	return m
}

type statusLoadedMsg struct{ data StatusData }
type statusErrMsg struct{ err error }

// Init loads vault status if a loader is configured.
func (m Model) Init() tea.Cmd {
	if m.loadStatus == nil {
		return nil
	}
	loader := m.loadStatus
	return func() tea.Msg {
		data, err := loader()
		if err != nil {
			return statusErrMsg{err: err}
		}
		return statusLoadedMsg{data: data}
	}
}

// Update handles incoming messages and returns the updated model and any command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusLoadedMsg:
		m.status = &msg.data
		return m, nil

	case statusErrMsg:
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Help overlay intercepts all keys
		if m.showHelp {
			if m.keys.Help.Matches(msg) || m.keys.Back.Matches(msg) || m.keys.Quit.Matches(msg) {
				m.showHelp = false
			}
			return m, nil
		}

		switch {
		case m.keys.Help.Matches(msg):
			m.showHelp = true
			return m, nil
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Quit.Matches(msg):
			return m, tea.Quit
		case m.keys.Back.Matches(msg):
			return m, tea.Quit
		case m.keys.MoveDown.Matches(msg):
			if m.selected < len(hostDestinations)-1 {
				m.selected++
				}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.selected > 0 {
				m.selected--
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
