// Package hosttui provides entry points for launching Svalbard TUI screens.
package hosttui

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/host-tui/internal/browse"
	"github.com/pkronstrom/svalbard/host-tui/internal/dashboard"
	"github.com/pkronstrom/svalbard/host-tui/internal/importscreen"
	"github.com/pkronstrom/svalbard/host-tui/internal/index"
	"github.com/pkronstrom/svalbard/host-tui/internal/openvault"
	"github.com/pkronstrom/svalbard/host-tui/internal/plan"
	"github.com/pkronstrom/svalbard/host-tui/internal/vault"
	"github.com/pkronstrom/svalbard/host-tui/internal/welcome"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
)

// RunInteractive launches the appropriate TUI screen based on vault resolution:
// explicit vault path → dashboard, auto-detect from CWD → dashboard, otherwise → welcome screen.
func RunInteractive(vaultFlag string, wizardConfig *WizardConfig, deps *DashboardDeps) error {
	if vaultFlag != "" {
		return runApp(newAppModel(&vaultFlag, wizardConfig, deps))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return runApp(newAppModel(nil, wizardConfig, deps))
	}

	vaultPath, err := vault.Resolve(cwd)
	if err != nil {
		return runApp(newAppModel(nil, wizardConfig, deps))
	}
	return runApp(newAppModel(&vaultPath, wizardConfig, deps))
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
	screenBrowse
	screenPlan
	screenImport
	screenIndex
	screenOpenVault
)

// appModel is a top-level Bubble Tea model that manages screen transitions.
type appModel struct {
	screen       screen
	prevScreen   screen // where to return when wizard/sub-screen emits BackMsg
	vaultPath    string
	deps         *DashboardDeps
	wizardConfig *WizardConfig
	width, height int

	welcome   welcome.Model
	dashboard dashboard.Model
	wizard    wizard.Model
	browse    browse.Model
	planScr   plan.Model
	importScr importscreen.Model
	indexScr  index.Model
	openVault openvault.Model
}

func newAppModel(vaultPath *string, wizardConfig *WizardConfig, deps *DashboardDeps) *appModel {
	m := &appModel{
		wizardConfig: wizardConfig,
		deps:         deps,
	}
	if vaultPath != nil {
		m.screen = screenDashboard
		m.vaultPath = *vaultPath
		m.dashboard = m.newDashboard(*vaultPath)
	} else {
		m.screen = screenWelcome
		m.welcome = welcome.New()
	}
	return m
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
	// Track terminal dimensions for forwarding to new screens.
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsm.Width
		m.height = wsm.Height
	}

	switch msg := msg.(type) {

	// --- Welcome screen messages ---
	case welcome.SelectMsg:
		switch msg.ID {
		case "new-vault":
			m.prevScreen = screenWelcome
			m.screen = screenWizard
			m.wizard = wizard.New(m.defaultWizardConfig())
			return m, m.sendSize()
		case "open-vault":
			m.screen = screenOpenVault
			m.openVault = openvault.New()
			return m, tea.Batch(m.openVault.Init(), m.sendSize())
		case "browse":
			m.prevScreen = screenWelcome
			m.screen = screenBrowse
			m.browse = m.newBrowse(true) // read-only
			return m, m.sendSize()
		}

	// --- Dashboard messages ---
	case dashboard.NewVaultMsg:
		m.prevScreen = screenDashboard
		m.screen = screenWizard
		m.wizard = wizard.New(m.defaultWizardConfig())
		return m, m.sendSize()

	case dashboard.SelectMsg:
		switch msg.ID {
		case "browse":
			m.prevScreen = screenDashboard
			m.screen = screenBrowse
			m.browse = m.newBrowse(false)
			return m, m.sendSize()
		case "plan":
			m.screen = screenPlan
			m.planScr = m.newPlan()
			return m, m.sendSize()
		case "import":
			m.screen = screenImport
			m.importScr = m.newImport()
			return m, m.sendSize()
		case "index":
			m.screen = screenIndex
			m.indexScr = m.newIndex()
			return m, m.sendSize()
		}

	// --- Wizard messages ---
	case wizard.BackMsg:
		m.screen = m.prevScreen
		if m.prevScreen != screenDashboard {
			m.screen = screenWelcome
			m.welcome = welcome.New()
		}
		return m, m.sendSize()

	case wizard.DoneMsg:
		r := msg.Result
		m.vaultPath = r.VaultPath
		// Rebuild deps for the new vault
		if m.deps != nil && m.deps.RebuildForVault != nil {
			m.deps = m.deps.RebuildForVault(r.VaultPath)
		}
		// Transition to dashboard
		m.screen = screenDashboard
		m.dashboard = m.newDashboard(r.VaultPath)
		return m, tea.Batch(m.dashboard.Init(), m.sendSize())

	// --- Browse messages ---
	case browse.BackMsg:
		return m, m.returnFromBrowse()

	case browse.SavedMsg:
		return m, m.returnFromBrowse()

	// --- Plan messages ---
	case plan.BackMsg:
		m.screen = screenDashboard
		m.dashboard = m.newDashboard(m.vaultPath)
		return m, tea.Batch(m.dashboard.Init(), m.sendSize())

	case plan.BrowseMsg:
		m.screen = screenBrowse
		m.browse = m.newBrowse(false)
		return m, m.sendSize()

	// --- Import messages ---
	case importscreen.BackMsg:
		m.screen = screenDashboard
		return m, m.sendSize()

	// --- Index messages ---
	case index.BackMsg:
		m.screen = screenDashboard
		return m, m.sendSize()

	// --- Open Vault messages ---
	case openvault.DoneMsg:
		m.vaultPath = msg.Path
		if m.deps != nil && m.deps.RebuildForVault != nil {
			m.deps = m.deps.RebuildForVault(msg.Path)
		}
		m.screen = screenDashboard
		m.dashboard = m.newDashboard(msg.Path)
		return m, tea.Batch(m.dashboard.Init(), m.sendSize())

	case openvault.BackMsg:
		m.screen = screenWelcome
		m.welcome = welcome.New()
		return m, m.sendSize()
	}

	// Forward to active screen
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
	case screenBrowse:
		updated, cmd := m.browse.Update(msg)
		m.browse = updated.(browse.Model)
		return m, cmd
	case screenPlan:
		updated, cmd := m.planScr.Update(msg)
		m.planScr = updated.(plan.Model)
		return m, cmd
	case screenImport:
		updated, cmd := m.importScr.Update(msg)
		m.importScr = updated.(importscreen.Model)
		return m, cmd
	case screenIndex:
		updated, cmd := m.indexScr.Update(msg)
		m.indexScr = updated.(index.Model)
		return m, cmd
	case screenOpenVault:
		updated, cmd := m.openVault.Update(msg)
		m.openVault = updated.(openvault.Model)
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
	case screenBrowse:
		return m.browse.View()
	case screenPlan:
		return m.planScr.View()
	case screenImport:
		return m.importScr.View()
	case screenIndex:
		return m.indexScr.View()
	case screenOpenVault:
		return m.openVault.View()
	}
	return ""
}

// sendSize returns a tea.Cmd that emits the stored terminal dimensions.
// Used after screen transitions so the new screen renders correctly.
func (m *appModel) sendSize() tea.Cmd {
	if m.width == 0 && m.height == 0 {
		return nil
	}
	w, h := m.width, m.height
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: w, Height: h}
	}
}

// returnFromBrowse restores the screen the user was on before entering Browse.
func (m *appModel) returnFromBrowse() tea.Cmd {
	if m.prevScreen == screenWelcome {
		m.screen = screenWelcome
		m.welcome = welcome.New()
	} else {
		m.screen = screenDashboard
	}
	return m.sendSize()
}

func (m *appModel) defaultWizardConfig() WizardConfig {
	var cfg WizardConfig
	if m.wizardConfig != nil {
		cfg = *m.wizardConfig
	}
	// Wire init and apply callbacks from deps
	if m.deps != nil {
		if m.deps.InitVault != nil {
			cfg.InitVault = m.deps.InitVault
		}
		if m.deps.RunApply != nil && m.deps.RebuildForVault != nil {
			rebuildForVault := m.deps.RebuildForVault
			cfg.RunApply = func(vaultPath string, onProgress func(wizard.ApplyEvent)) error {
				// Rebuild deps targeting the new vault path, then run apply
				newDeps := rebuildForVault(vaultPath)
				return newDeps.RunApply(context.Background(), func(ev ApplyEvent) {
					onProgress(wizard.ApplyEvent{
						ID: ev.ID, Status: ev.Status,
						Downloaded: ev.Downloaded, Total: ev.Total,
						Error: ev.Error,
					})
				})
			}
		}
	}
	return cfg
}

// newDashboard creates a dashboard.Model with status loading wired up.
func (m *appModel) newDashboard(vaultPath string) dashboard.Model {
	var cfg dashboard.Config
	if m.deps != nil && m.deps.LoadStatus != nil {
		loadStatus := m.deps.LoadStatus
		cfg.LoadStatus = func() (dashboard.StatusData, error) {
			vs, err := loadStatus()
			if err != nil {
				return dashboard.StatusData{}, err
			}
			return dashboard.StatusData{
				PresetName:    vs.PresetName,
				DesiredCount:  vs.DesiredCount,
				RealizedCount: vs.RealizedCount,
				PendingCount:  vs.PendingCount,
				DiskUsedGB:    vs.DiskUsedGB,
				DiskFreeGB:    vs.DiskFreeGB,
				LastApplied:   vs.LastApplied,
			}, nil
		}
	}
	return dashboard.New(vaultPath, cfg)
}

// newBrowse creates a browse.Model from the current deps and vault state.
func (m *appModel) newBrowse(readOnly bool) browse.Model {
	cfg := browse.Config{}
	// Prefer deps for catalog data, fall back to wizardConfig
	if m.deps != nil {
		cfg.PackGroups = m.deps.PackGroups
		cfg.Presets = m.deps.Presets
	} else if m.wizardConfig != nil {
		cfg.PackGroups = m.wizardConfig.PackGroups
		cfg.Presets = m.wizardConfig.Presets
	}
	if !readOnly && m.deps != nil {
		if m.deps.LoadStatus != nil {
			if status, err := m.deps.LoadStatus(); err == nil {
				cfg.FreeGB = status.DiskFreeGB
			}
		}
		if m.deps.LoadDesiredItems != nil {
			if items, err := m.deps.LoadDesiredItems(); err == nil {
				cfg.DesiredItems = items
			}
		}
		cfg.SaveDesired = m.deps.SaveDesiredItems
	}
	return browse.New(cfg)
}

// newPlan creates a plan.Model from the current deps.
func (m *appModel) newPlan() plan.Model {
	cfg := plan.Config{}
	if m.deps != nil && m.deps.LoadPlan != nil {
		if summary, err := m.deps.LoadPlan(); err == nil {
			cfg.DownloadGB = summary.DownloadGB
			cfg.RemoveGB = summary.RemoveGB
			cfg.FreeAfterGB = summary.FreeAfterGB
			for _, item := range summary.ToDownload {
				cfg.Items = append(cfg.Items, plan.PlanItem{
					ID: item.ID, Type: item.Type, SizeGB: item.SizeGB,
					Description: item.Description, Action: item.Action,
				})
			}
			for _, item := range summary.ToRemove {
				cfg.Items = append(cfg.Items, plan.PlanItem{
					ID: item.ID, Type: item.Type, SizeGB: item.SizeGB,
					Description: item.Description, Action: item.Action,
				})
			}
		}
		if m.deps.RunApply != nil {
			cfg.RunApply = func(ctx context.Context, onProgress func(plan.ApplyEvent)) error {
				return m.deps.RunApply(ctx, func(ev ApplyEvent) {
					onProgress(plan.ApplyEvent{
						ID: ev.ID, Status: ev.Status,
						Downloaded: ev.Downloaded, Total: ev.Total,
						Error: ev.Error,
					})
				})
			}
		}
	}
	return plan.New(cfg)
}

// newImport creates an importscreen.Model from the current deps.
func (m *appModel) newImport() importscreen.Model {
	cfg := importscreen.Config{}
	if m.deps != nil && m.deps.RunImport != nil {
		cfg.RunImport = func(ctx context.Context, source string) (importscreen.ImportResult, error) {
			result, err := m.deps.RunImport(ctx, source)
			if err != nil {
				return importscreen.ImportResult{}, err
			}
			return importscreen.ImportResult{ID: result.ID, SizeGB: result.SizeGB}, nil
		}
	}
	return importscreen.New(cfg)
}

// newIndex creates an index.Model from the current deps.
func (m *appModel) newIndex() index.Model {
	cfg := index.Config{}
	if m.deps != nil {
		if m.deps.LoadIndexStatus != nil {
			if status, err := m.deps.LoadIndexStatus(); err == nil {
				cfg.Status = index.IndexStatus{
					KeywordEnabled:   status.KeywordEnabled,
					KeywordSources:   status.KeywordSources,
					KeywordArticles:  status.KeywordArticles,
					KeywordLastBuilt: status.KeywordLastBuilt,
					SemanticEnabled:  status.SemanticEnabled,
					SemanticStatus:   status.SemanticStatus,
				}
			}
		}
		if m.deps.RunIndex != nil {
			cfg.RunIndex = func(ctx context.Context, indexType string, onProgress func(index.IndexEvent)) error {
				return m.deps.RunIndex(ctx, indexType, func(ev IndexEvent) {
					onProgress(index.IndexEvent{File: ev.File, Status: ev.Status})
				})
			}
		}
	}
	return index.New(cfg)
}
