// Package wizard implements the `svalbard init` guided setup wizard.
// It presents a step-based flow with a navigation list on the left
// and contextual detail on the right, using the shared tui design system.
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// wizardSteps defines the ordered stages of the init wizard (spec section 7).
var wizardSteps = []struct{ id, label string }{
	{"path", "Vault Path"},
	{"preset", "Choose Preset"},
	{"adjust", "Adjust Contents"},
	{"review", "Review Plan"},
	{"apply", "Apply"},
}

// Model is the Bubble Tea model for the init wizard.
type Model struct {
	pathValue   string // prefilled or user-entered vault path
	currentStep int
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap
}

// New creates a new wizard Model. If prefillPath is non-empty the vault path
// step is pre-populated with that value.
func New(prefillPath string) Model {
	return Model{
		pathValue:   prefillPath,
		currentStep: 0,
		theme:       tui.DefaultTheme(),
		keys:        tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the wizard model.
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

		case m.keys.Enter.Matches(msg):
			if m.currentStep < len(wizardSteps)-1 {
				m.currentStep++
			}
			return m, nil

		case m.keys.Back.Matches(msg):
			if m.currentStep > 0 {
				m.currentStep--
				return m, nil
			}
			return m, tea.Quit
		}
	}

	return m, nil
}
