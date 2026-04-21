// Package wizard implements the `svalbard init` guided setup wizard.
// It orchestrates sub-models (path picker, preset picker, pack picker, review)
// using a nested Bubble Tea model approach. Each stage is a self-contained
// tea.Model that sends done-messages when complete.
package wizard

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

var wizardSteps = []struct{ id, label string }{
	{"path", "Vault Path"},
	{"platforms", "Platforms"},
	{"preset", "Choose Preset"},
	{"packs", "Pack Picker"},
	{"review", "Review"},
	{"apply", "Apply"},
	{"index", "Build Index"},
}

// BackMsg is sent when the user navigates back from the first wizard step.
type BackMsg struct{}

// stage tracks which sub-model is active.
type stage int

const (
	stagePath stage = iota
	stagePlatforms
	stagePreset
	stagePacks
	stageReview
	stageApply
	stageIndex
)

// Model is the Bubble Tea model for the init wizard.
type Model struct {
	config WizardConfig
	stage  stage
	width  int
	height int
	theme  tui.Theme
	keys   tui.KeyMap

	// Sub-models (created lazily per stage)
	pathPicker       pathPickerModel
	platformPicker   platformPickerModel
	presetPicker     presetPickerModel
	packPicker       packPickerModel
	review           reviewModel
	applyModel       wizardApplyModel
	indexModel       wizardIndexModel

	// Accumulated state across stages
	vaultPath     string
	freeGB        float64
	hostPlatforms []string
	checkedIDs    map[string]bool
	presetName    string
	region        string
}

// New creates a new wizard Model with the given config.
func New(config WizardConfig) Model {
	m := Model{
		config:     config,
		stage:      stagePath,
		theme:      tui.DefaultTheme(),
		keys:       tui.DefaultKeyMap(),
		checkedIDs: make(map[string]bool),
	}

	// Always start at path picker — every wizard run needs a vault path
	m.pathPicker = newPathPicker(config.Volumes, config.HomeVolume, config.PrefillPath)

	return m
}

// Init satisfies tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the wizard orchestrator.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to active sub-model
		return m.forwardToActive(msg)

	// Sub-model done messages drive stage transitions
	case pathDoneMsg:
		m.vaultPath = msg.path
		m.freeGB = msg.freeGB
		m.stage = stagePlatforms
		m.platformPicker = newPlatformPicker()
		return m.sizeActiveModel()

	case platformDoneMsg:
		m.hostPlatforms = msg.platforms
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m.sizeActiveModel()

	case presetCancelMsg:
		m.stage = stagePlatforms
		m.platformPicker = newPlatformPicker()
		return m.sizeActiveModel()

	case presetDoneMsg:
		m.presetName = msg.preset.Name
		m.region = msg.preset.Region
		m.checkedIDs = make(map[string]bool)
		for _, id := range msg.preset.SourceIDs {
			m.checkedIDs[id] = true
		}
		m.stage = stagePacks
		m.packPicker = newPackPicker(packPickerConfig{
			groups:      m.config.PackGroups,
			checkedIDs:  m.checkedIDs,
			freeGB:      m.freeGB,
			resolveDeps: m.config.ResolveDeps,
		})
		return m.sizeActiveModel()

	case packDoneMsg:
		m.checkedIDs = msg.selectedIDs
		m.stage = stageReview
		m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), m.freeGB)
		return m.sizeActiveModel()

	case packCancelMsg:
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m.sizeActiveModel()

	case reviewConfirmMsg:
		ids := m.selectedIDList()
		// Init the vault if callback is available
		if m.config.InitVault != nil {
			if err := m.config.InitVault(m.vaultPath, ids, m.presetName, m.region, m.hostPlatforms); err != nil {
				m.stage = stageReview
				m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), m.freeGB)
				m.review.errMsg = fmt.Sprintf("Init failed: %v", err)
				return m.sizeActiveModel()
			}
		}
		// Transition to apply stage
		if m.config.RunApply != nil {
			m.stage = stageApply
			m.applyModel = newWizardApply(m.vaultPath, ids, m.config.RunApply)
			sized, sizeCmd := m.sizeActiveModel()
			m = sized.(Model)
			return m, tea.Batch(m.applyModel.Init(), sizeCmd)
		}
		// No apply callback — just exit with result
		return m, m.doneCmd()

	case wizardApplyDoneMsg:
		if m.config.RunIndex != nil {
			m.stage = stageIndex
			m.indexModel = newWizardIndex(m.vaultPath, m.config.RunIndex)
			sized, sizeCmd := m.sizeActiveModel()
			m = sized.(Model)
			return m, tea.Batch(m.indexModel.Init(), sizeCmd)
		}
		return m, m.doneCmd()

	case wizardIndexDoneMsg:
		return m, m.doneCmd()

	case reviewBackMsg:
		m.stage = stagePacks
		m.packPicker = newPackPicker(packPickerConfig{
			groups:      m.config.PackGroups,
			checkedIDs:  m.checkedIDs,
			freeGB:      m.freeGB,
			resolveDeps: m.config.ResolveDeps,
		})
		return m.sizeActiveModel()

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) {
			return m, tea.Quit
		}
		// Back from path picker goes to welcome screen
		if m.stage == stagePath && (m.keys.Back.Matches(msg) || m.keys.Quit.Matches(msg)) {
			return m, func() tea.Msg { return BackMsg{} }
		}
		// Back from platform picker goes to path picker
		if m.stage == stagePlatforms && (m.keys.Back.Matches(msg) || m.keys.Quit.Matches(msg)) {
			m.stage = stagePath
			m.pathPicker = newPathPicker(m.config.Volumes, m.config.HomeVolume, m.vaultPath)
			return m.sizeActiveModel()
		}
	}

	// Forward all other messages to the active sub-model
	return m.forwardToActive(msg)
}

// sizeActiveModel sends the current terminal dimensions to the active sub-model
// so it renders correctly on its first frame (before a real WindowSizeMsg arrives).
func (m Model) sizeActiveModel() (tea.Model, tea.Cmd) {
	if m.width > 0 && m.height > 0 {
		return m.forwardToActive(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	}
	return m, nil
}

func (m Model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.stage {
	case stagePath:
		updated, cmd := m.pathPicker.Update(msg)
		m.pathPicker = updated.(pathPickerModel)
		return m, cmd
	case stagePlatforms:
		updated, cmd := m.platformPicker.Update(msg)
		m.platformPicker = updated.(platformPickerModel)
		return m, cmd
	case stagePreset:
		updated, cmd := m.presetPicker.Update(msg)
		m.presetPicker = updated.(presetPickerModel)
		return m, cmd
	case stagePacks:
		updated, cmd := m.packPicker.Update(msg)
		m.packPicker = updated.(packPickerModel)
		return m, cmd
	case stageReview:
		updated, cmd := m.review.Update(msg)
		m.review = updated.(reviewModel)
		return m, cmd
	case stageApply:
		updated, cmd := m.applyModel.Update(msg)
		m.applyModel = updated.(wizardApplyModel)
		return m, cmd
	case stageIndex:
		updated, cmd := m.indexModel.Update(msg)
		m.indexModel = updated.(wizardIndexModel)
		return m, cmd
	}
	return m, nil
}

func (m Model) buildReviewItems() []ReviewItem {
	var items []ReviewItem
	seen := make(map[string]bool)
	for _, g := range m.config.PackGroups {
		for _, p := range g.Packs {
			for _, src := range p.Sources {
				if m.checkedIDs[src.ID] && !seen[src.ID] {
					seen[src.ID] = true
					items = append(items, ReviewItem{
						ID:          src.ID,
						Type:        src.Type,
						SizeGB:      src.SizeGB,
						Description: src.Description,
					})
				}
			}
		}
	}
	return items
}

func (m Model) selectedIDList() []string {
	ids := make([]string, 0, len(m.checkedIDs))
	for id := range m.checkedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (m Model) doneCmd() tea.Cmd {
	result := WizardResult{
		VaultPath:     m.vaultPath,
		SelectedIDs:   m.selectedIDList(),
		PresetName:    m.presetName,
		Region:        m.region,
		HostPlatforms: m.hostPlatforms,
	}
	return func() tea.Msg { return DoneMsg{Result: result} }
}
