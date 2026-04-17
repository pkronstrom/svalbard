package browse

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// BackMsg is sent when the user exits the browse screen without saving.
type BackMsg struct{}

// SavedMsg is sent when the user saves changes and exits.
type SavedMsg struct{}

// Config holds everything the browse screen needs.
type Config struct {
	PackGroups   []tui.PackGroup
	Presets      []PresetOption
	DesiredItems []string
	FreeGB       float64
	SaveDesired  func(ids []string) error // nil for read-only mode
}

// PresetOption mirrors wizard.PresetOption for Browse's preset cycling.
type PresetOption struct {
	Name      string
	SourceIDs []string
}

// Model is the Bubble Tea model for the browse screen.
type Model struct {
	picker     tui.TreePicker
	presets    []PresetOption
	presetIdx  int // current preset cycle index (-1 = custom)
	initialIDs map[string]bool
	readOnly   bool
	saveFunc   func(ids []string) error

	// Save prompt
	showSavePrompt bool
	saveChoice     int

	width  int
	height int
	keys   tui.KeyMap
}

// New creates a browse model from the given configuration.
func New(cfg Config) Model {
	checkedIDs := make(map[string]bool)
	initialIDs := make(map[string]bool)
	for _, id := range cfg.DesiredItems {
		checkedIDs[id] = true
		initialIDs[id] = true
	}

	tp := tui.NewTreePicker(tui.TreePickerConfig{
		Groups:     cfg.PackGroups,
		CheckedIDs: checkedIDs,
		FreeGB:     cfg.FreeGB,
		ReadOnly:   cfg.SaveDesired == nil,
	})

	var presets []PresetOption
	for _, p := range cfg.Presets {
		presets = append(presets, PresetOption{Name: p.Name, SourceIDs: p.SourceIDs})
	}

	return Model{
		picker:     tp,
		presets:    presets,
		presetIdx:  -1,
		initialIDs: initialIDs,
		readOnly:   cfg.SaveDesired == nil,
		saveFunc:   cfg.SaveDesired,
		keys:       tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the browse screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.picker.Width = msg.Width
		m.picker.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.showSavePrompt {
			return m.updateSavePrompt(msg)
		}

		// Let tree picker handle navigation/toggle first.
		if m.picker.Update(msg) {
			return m, nil
		}

		switch {
		case matchRune(msg, 'p'):
			if !m.readOnly && len(m.presets) > 0 {
				m.cyclePreset()
			}
			return m, nil

		case m.keys.Quit.Matches(msg), m.keys.Back.Matches(msg):
			if !m.readOnly && m.picker.IsDirty(m.initialIDs) {
				m.showSavePrompt = true
				m.saveChoice = 0
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateSavePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchRune(msg, 'y'):
		if err := m.saveFunc(m.picker.CheckedIDSlice()); err != nil {
			m.showSavePrompt = false
			return m, nil
		}
		return m, func() tea.Msg { return SavedMsg{} }

	case matchRune(msg, 'n'):
		return m, func() tea.Msg { return BackMsg{} }

	case m.keys.Back.Matches(msg):
		m.showSavePrompt = false
		return m, nil

	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) cyclePreset() {
	m.presetIdx = (m.presetIdx + 1) % len(m.presets)
	preset := m.presets[m.presetIdx]
	m.picker.CheckedIDs = make(map[string]bool)
	for _, id := range preset.SourceIDs {
		m.picker.CheckedIDs[id] = true
	}
}

// View renders the browse screen.
func (m Model) View() string {
	tree := m.picker.RenderTree()
	tree += "\n"
	tree += m.picker.RenderSizeSummary()

	if m.showSavePrompt {
		tree += "\n\n"
		tree += m.picker.Theme.Warning.Render("  Save changes before leaving?")
		tree += "\n"
		tree += m.picker.Theme.Base.Render("  y = save   n = discard   esc = cancel")
	}

	detail := m.picker.RenderDetail()

	totalGB := m.picker.TotalCheckedGB()
	header := fmt.Sprintf("Browse  %d selected  %.1f GB", m.picker.TotalCheckedCount(), totalGB)

	var footerParts []string
	footerParts = append(footerParts,
		fmt.Sprintf("%s/%s navigate", m.keys.MoveUp.Key, m.keys.MoveDown.Key),
		fmt.Sprintf("%s toggle", m.keys.Toggle.Key),
		fmt.Sprintf("%s expand/collapse", m.keys.Enter.Key),
	)
	if !m.readOnly && len(m.presets) > 0 {
		footerParts = append(footerParts, "p preset")
	}
	footerParts = append(footerParts, "esc back")
	footer := strings.Join(footerParts, "  ")

	shell := tui.ShellLayout{
		Theme:   m.picker.Theme,
		AppName: "Svalbard",
		Status:  header,
		Left:    detail,
		Right:   tree,
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

func matchRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}
