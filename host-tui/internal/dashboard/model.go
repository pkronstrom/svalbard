package dashboard

import (
	"github.com/pkronstrom/svalbard/tui"

	tea "github.com/charmbracelet/bubbletea"
)

var hostDestinations = []struct{ id, label string }{
	{"overview", "Overview"},
	{"add", "Add Content"},
	{"remove", "Remove Content"},
	{"import", "Import"},
	{"plan", "Plan"},
	{"apply", "Apply"},
	{"presets", "Presets"},
}

// Model is the host-side vault dashboard — the main screen shown when
// svalbard resolves a vault. It uses the shared tui/ components for a
// two-pane layout.
type Model struct {
	vaultPath     string
	selected      int
	width         int
	height        int
	theme         tui.Theme
	keys          tui.KeyMap
	paletteActive bool
	paletteModel  tui.PaletteModel
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
	case tui.PaletteCloseMsg:
		m.paletteActive = false
		return m, nil

	case tui.PaletteSelectMsg:
		m.paletteActive = false
		// Navigate to the selected destination by finding its index.
		for i, d := range hostDestinations {
			if d.id == msg.Entry.ID {
				m.selected = i
				break
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// When the palette is active, delegate all key messages to it.
		if m.paletteActive {
			updated, cmd := m.paletteModel.Update(msg)
			m.paletteModel = updated.(tui.PaletteModel)
			return m, cmd
		}

		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Quit.Matches(msg):
			return m, tea.Quit
		case m.keys.Palette.Matches(msg):
			m.paletteActive = true
			m.paletteModel = tui.NewPaletteModel(buildHostPaletteEntries(), m.theme)
			return m, nil
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
		case m.keys.Back.Matches(msg):
			return m, tea.Quit
		case m.keys.Enter.Matches(msg):
			// Placeholder — no-op for now.
			return m, nil
		}
	}

	return m, nil
}

// buildHostPaletteEntries creates palette entries from the host destinations.
func buildHostPaletteEntries() []tui.PaletteEntry {
	entries := make([]tui.PaletteEntry, len(hostDestinations))
	for i, d := range hostDestinations {
		entry := tui.PaletteEntry{
			ID:    d.id,
			Label: d.label,
		}
		// Import destination accepts freeform input.
		if d.id == "import" {
			entry.Verbs = []string{"import"}
			entry.AcceptsFreeform = true
		}
		entries[i] = entry
	}
	return entries
}
