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

// PlanMsg is sent when the user saves and wants to go to the plan screen.
type PlanMsg struct{}

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
		ShowAction:  cfg.SaveDesired != nil,
		ActionLabel: "Save & review plan →",
	})
	tp.ReserveLines = 11 // detail(3) + summary(1) + shell chrome(7)

	return Model{
		picker:     tp,
		presets:    cfg.Presets,
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
		// Enter on action row → save & go to plan.
		case m.keys.Enter.Matches(msg):
			if row := m.picker.CursorRow(); row != nil && row.Kind == tui.RowAction {
				return m, m.saveAndPlan()
			}
			return m, nil

		// 'a' shortcut → save & go to plan.
		case tui.MatchRune(msg, 'a'):
			if !m.readOnly {
				return m, m.saveAndPlan()
			}
			return m, nil

		case tui.MatchRune(msg, 'p'):
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
	case tui.MatchRune(msg, 'y'):
		if err := m.saveFunc(m.picker.CheckedIDSlice()); err != nil {
			m.showSavePrompt = false
			return m, nil
		}
		return m, func() tea.Msg { return SavedMsg{} }

	case tui.MatchRune(msg, 'n'):
		return m, func() tea.Msg { return BackMsg{} }

	case m.keys.Back.Matches(msg):
		m.showSavePrompt = false
		return m, nil

	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) saveAndPlan() tea.Cmd {
	if m.saveFunc != nil {
		if err := m.saveFunc(m.picker.CheckedIDSlice()); err != nil {
			return nil
		}
	}
	return func() tea.Msg { return PlanMsg{} }
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
	totalGB := m.picker.TotalCheckedGB()

	// Reserve extra lines for save prompt so tree shrinks to make room.
	if m.showSavePrompt {
		m.picker.ReserveLines = 14
	} else {
		m.picker.ReserveLines = 11
	}

	var body strings.Builder
	body.WriteString(m.picker.RenderTree())
	body.WriteString("\n")
	body.WriteString(m.picker.RenderSizeSummary())

	// Detail for focused item below the tree.
	detail := m.picker.RenderDetail()
	if detail != "" {
		body.WriteString("\n")
		body.WriteString(m.picker.Theme.Muted.Render("  ───"))
		body.WriteString("\n")
		body.WriteString(detail)
	}

	if m.showSavePrompt {
		body.WriteString("\n\n")
		body.WriteString(m.picker.Theme.Warning.Render("  Save changes before leaving?"))
		body.WriteString("\n")
		body.WriteString(m.picker.Theme.Base.Render("  y = save   n = discard   esc = cancel"))
	}

	header := fmt.Sprintf("Browse  %d selected  %.1f GB", m.picker.TotalCheckedCount(), totalGB)
	if m.presetIdx >= 0 && m.presetIdx < len(m.presets) {
		header += "  preset: " + m.presets[m.presetIdx].Name
	}

	var footerParts []string
	footerParts = append(footerParts,
		fmt.Sprintf("%s/%s navigate", m.keys.MoveUp.Key, m.keys.MoveDown.Key),
		fmt.Sprintf("%s toggle", m.keys.Toggle.Key),
		"←/→ collapse/expand",
	)
	if !m.readOnly {
		footerParts = append(footerParts, "a apply")
		if len(m.presets) > 0 {
			footerParts = append(footerParts, "p preset")
		}
	}
	footerParts = append(footerParts, "esc back")
	footer := strings.Join(footerParts, "  ")

	shell := tui.ShellLayout{
		Theme:   m.picker.Theme,
		AppName: "Svalbard",
		Status:  header,
		Right:   body.String(),
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

