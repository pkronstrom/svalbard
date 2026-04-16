// Package hosttui provides entry points for launching Svalbard TUI screens.
package hosttui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/host-tui/internal/dashboard"
	"github.com/pkronstrom/svalbard/host-tui/internal/vault"
	"github.com/pkronstrom/svalbard/host-tui/internal/welcome"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
)

// RunInteractive launches the appropriate TUI screen based on vault resolution:
// vault found → dashboard, no vault → welcome screen (which can transition to wizard).
// The wizardConfig is used when the user triggers the init wizard from the welcome screen.
func RunInteractive(wizardConfig *WizardConfig) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Resolve(cwd)
	if err != nil {
		return runApp(newAppModel(nil, wizardConfig))
	}
	return runApp(newAppModel(&vaultPath, wizardConfig))
}

// RunInitWizard launches the init wizard TUI with the given config.
func RunInitWizard(config WizardConfig) error {
	return runApp(&appModel{screen: screenWizard, wizard: wizard.New(config), wizardConfig: &config})
}

func runApp(m *appModel) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// screen identifies which TUI screen is active.
type screen int

const (
	screenWelcome screen = iota
	screenDashboard
	screenWizard
)

// appModel is a top-level Bubble Tea model that manages screen transitions.
type appModel struct {
	screen       screen
	welcome      welcome.Model
	dashboard    dashboard.Model
	wizard       wizard.Model
	wizardConfig *WizardConfig // stored for welcome→wizard transition
}

func newAppModel(vaultPath *string, wizardConfig *WizardConfig) *appModel {
	if vaultPath != nil {
		return &appModel{
			screen:       screenDashboard,
			dashboard:    dashboard.New(*vaultPath),
			wizardConfig: wizardConfig,
		}
	}
	return &appModel{
		screen:       screenWelcome,
		welcome:      welcome.New(),
		wizardConfig: wizardConfig,
	}
}

func (m *appModel) Init() tea.Cmd {
	switch m.screen {
	case screenWelcome:
		return m.welcome.Init()
	case screenDashboard:
		return m.dashboard.Init()
	case screenWizard:
		return m.wizard.Init()
	}
	return nil
}

func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case welcome.SelectMsg:
		config := m.defaultWizardConfig()
		switch msg.ID {
		case "init", "preset":
			// Both paths start the wizard at the path picker.
			// Every wizard run needs a vault path selected first.
			m.screen = screenWizard
			m.wizard = wizard.New(config)
			return m, nil
		}

	case wizard.BackMsg:
		m.screen = screenWelcome
		m.welcome = welcome.New()
		return m, nil

	case wizard.DoneMsg:
		// Wizard completed — exit TUI. Caller handles init + apply.
		return m, tea.Quit
	}

	switch m.screen {
	case screenWelcome:
		updated, cmd := m.welcome.Update(msg)
		m.welcome = updated.(welcome.Model)
		return m, cmd
	case screenDashboard:
		updated, cmd := m.dashboard.Update(msg)
		m.dashboard = updated.(dashboard.Model)
		return m, cmd
	case screenWizard:
		updated, cmd := m.wizard.Update(msg)
		m.wizard = updated.(wizard.Model)
		return m, cmd
	}
	return m, nil
}

func (m *appModel) View() string {
	switch m.screen {
	case screenWelcome:
		return m.welcome.View()
	case screenDashboard:
		return m.dashboard.View()
	case screenWizard:
		return m.wizard.View()
	}
	return ""
}

func (m *appModel) defaultWizardConfig() WizardConfig {
	if m.wizardConfig != nil {
		return *m.wizardConfig
	}
	return WizardConfig{}
}
