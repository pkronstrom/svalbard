// Package wizard implements the `svalbard init` guided setup wizard.
// It orchestrates sub-models (path picker, preset picker, pack picker, review)
// using a nested Bubble Tea model approach. Each stage is a self-contained
// tea.Model that sends done-messages when complete.
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

var wizardSteps = []struct{ id, label string }{
	{"path", "Vault Path"},
	{"preset", "Choose Preset"},
	{"packs", "Pack Picker"},
	{"review", "Review"},
}

// BackMsg is sent when the user navigates back from the first wizard step.
type BackMsg struct{}

// stage tracks which sub-model is active.
type stage int

const (
	stagePath stage = iota
	stagePreset
	stagePacks
	stageReview
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
	pathPicker   pathPickerModel
	presetPicker presetPickerModel
	packPicker   packPickerModel
	review       reviewModel

	// Accumulated state across stages
	vaultPath  string
	freeGB     float64
	checkedIDs map[string]bool
	presetName string
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

	if config.StartAtStep > 0 && config.StartAtStep <= int(stageReview) {
		m.stage = stage(config.StartAtStep)
	}

	// Initialize path picker
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
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m, nil

	case presetDoneMsg:
		m.presetName = msg.preset.Name
		m.checkedIDs = make(map[string]bool)
		for _, id := range msg.preset.SourceIDs {
			m.checkedIDs[id] = true
		}
		m.stage = stagePacks
		m.packPicker = newPackPicker(m.config.PackGroups, m.checkedIDs, m.freeGB)
		return m, nil

	case packDoneMsg:
		m.checkedIDs = msg.selectedIDs
		m.stage = stageReview
		m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), m.freeGB)
		return m, nil

	case packCancelMsg:
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m, nil

	case reviewConfirmMsg:
		return m, func() tea.Msg {
			return DoneMsg{Result: WizardResult{
				VaultPath:   m.vaultPath,
				SelectedIDs: m.selectedIDList(),
				PresetName:  m.presetName,
			}}
		}

	case reviewBackMsg:
		m.stage = stagePacks
		m.packPicker = newPackPicker(m.config.PackGroups, m.checkedIDs, m.freeGB)
		return m, nil

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) {
			return m, tea.Quit
		}
		// Back from path picker goes to welcome screen
		if m.stage == stagePath && m.keys.Back.Matches(msg) && !m.pathPicker.customInput {
			return m, func() tea.Msg { return BackMsg{} }
		}
	}

	// Forward all other messages to the active sub-model
	return m.forwardToActive(msg)
}

func (m Model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.stage {
	case stagePath:
		updated, cmd := m.pathPicker.Update(msg)
		m.pathPicker = updated.(pathPickerModel)
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
	return ids
}
