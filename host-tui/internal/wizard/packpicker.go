package wizard

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// Messages emitted by the pack picker.
type packDoneMsg struct {
	explicitIDs map[string]bool
	selectedIDs map[string]bool
	missingIDs  []string
}
type packCancelMsg struct {
	explicitIDs map[string]bool
	selectedIDs map[string]bool
	missingIDs  []string
}

type packPickerConfig struct {
	groups      []tui.PackGroup
	explicitIDs map[string]bool
	freeGB      float64
	resolveDeps DepResolver
}

// packPickerModel is the Bubble Tea sub-model for the pack picker sub-screen.
type packPickerModel struct {
	picker      tui.TreePicker
	resolveDeps DepResolver
	missingIDs  []string
	width       int
	height      int
}

// newPackPicker creates a pack picker model.
func newPackPicker(cfg packPickerConfig) packPickerModel {
	tp := tui.NewTreePicker(tui.TreePickerConfig{
		Groups:     packGroupsWithUnavailable(cfg.groups, cfg.explicitIDs),
		CheckedIDs: cfg.explicitIDs,
		FreeGB:     cfg.freeGB,
		ShowAction: true,
	})

	m := packPickerModel{
		picker:      tp,
		resolveDeps: cfg.resolveDeps,
	}
	m.recalcDeps()
	return m
}

func (m *packPickerModel) recalcDeps() {
	missing := m.selectedUnavailableIDs()
	if m.resolveDeps != nil {
		autoDeps, missingIDs := m.resolveDeps(m.picker.UserSelection())
		m.picker.SetAutoDependencies(autoDeps)
		for _, id := range missingIDs {
			missing[id] = true
		}
	}
	m.missingIDs = missingIDSlice(missing)
	m.picker.ReserveLines = 10 + validationLines(m.width, packValidation(*m))
	m.picker.EnsureVisible()
}

// Init satisfies tea.Model.
func (m packPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the pack picker.
func (m packPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.picker.Width = msg.Width
		m.picker.Height = msg.Height
		m.picker.ReserveLines = 10 + validationLines(m.width, packValidation(m))
		m.picker.EnsureVisible()
		return m, nil

	case tea.KeyMsg:
		// Let the shared picker handle navigation/toggle.
		result := m.picker.UpdateWithResult(msg)
		if result == tui.UpdateToggled {
			m.recalcDeps()
		}
		if result != tui.UpdateNone {
			return m, nil
		}

		switch {
		// Enter on action row → done.
		case m.picker.Keys.Enter.Matches(msg):
			if row := m.picker.CursorRow(); row != nil && row.Kind == tui.RowAction {
				done := m.doneMsg()
				return m, func() tea.Msg { return done }
			}

		// 'a' shortcut → done.
		case tui.MatchRune(msg, 'a'):
			done := m.doneMsg()
			return m, func() tea.Msg { return done }

		// Cancel.
		case m.picker.Keys.Quit.Matches(msg):
			cancel := m.cancelMsg()
			return m, func() tea.Msg { return cancel }

		case m.picker.Keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m packPickerModel) doneMsg() packDoneMsg {
	return packDoneMsg{
		explicitIDs: m.picker.UserSelection(),
		selectedIDs: pickerSelection(&m.picker),
		missingIDs:  append([]string(nil), m.missingIDs...),
	}
}

func (m packPickerModel) cancelMsg() packCancelMsg {
	return packCancelMsg{
		explicitIDs: m.picker.UserSelection(),
		selectedIDs: pickerSelection(&m.picker),
		missingIDs:  append([]string(nil), m.missingIDs...),
	}
}

func (m packPickerModel) selectedUnavailableIDs() map[string]bool {
	selected := m.picker.UserSelection()
	missing := make(map[string]bool)
	for _, group := range m.picker.Groups {
		for _, pack := range group.Packs {
			for _, source := range pack.Sources {
				if source.Unavailable && selected[source.ID] {
					missing[source.ID] = true
				}
			}
		}
	}
	return missing
}

func missingIDSlice(missing map[string]bool) []string {
	ids := make([]string, 0, len(missing))
	for id := range missing {
		if missing[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func pickerSelection(picker *tui.TreePicker) map[string]bool {
	ids := picker.CheckedIDSlice()
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	return selected
}

// View renders the pack picker.
func (m packPickerModel) View() string {
	var b strings.Builder
	if validation := packValidation(m); validation != "" {
		b.WriteString(m.picker.Theme.Danger.Render(validationText(m.width, validation)))
		b.WriteString("\n")
	}
	b.WriteString(m.picker.RenderTree())
	b.WriteString("\n")
	b.WriteString(m.picker.RenderSizeSummary())
	b.WriteString("\n")

	b.WriteString(m.picker.Theme.Help.Render(fmt.Sprintf("  %s/%s navigate  %s toggle  %s expand/collapse  a apply  q cancel",
		m.picker.Keys.MoveUp.Key, m.picker.Keys.MoveDown.Key, m.picker.Keys.Toggle.Key, m.picker.Keys.Enter.Key)))
	b.WriteString("\n")

	return b.String()
}

func packValidation(m packPickerModel) string {
	if len(m.missingIDs) == 0 {
		return ""
	}
	return "  Missing catalog recipes: " + strings.Join(m.missingIDs, ", ")
}

func packGroupsWithUnavailable(groups []tui.PackGroup, explicitIDs map[string]bool) []tui.PackGroup {
	copied := make([]tui.PackGroup, len(groups))
	known := make(map[string]bool)
	for i, group := range groups {
		copied[i].Name = group.Name
		copied[i].Packs = make([]tui.Pack, len(group.Packs))
		for j, pack := range group.Packs {
			copied[i].Packs[j] = pack
			copied[i].Packs[j].Sources = append([]tui.PackSource(nil), pack.Sources...)
			for _, source := range pack.Sources {
				known[source.ID] = true
			}
		}
	}

	var unavailable []string
	for id, selected := range explicitIDs {
		if selected && !known[id] {
			unavailable = append(unavailable, id)
		}
	}
	sort.Strings(unavailable)
	if len(unavailable) == 0 {
		return copied
	}

	sources := make([]tui.PackSource, 0, len(unavailable))
	for _, id := range unavailable {
		sources = append(sources, tui.PackSource{ID: id, Description: "Not available in the current catalog", Unavailable: true})
	}
	copied = append(copied, tui.PackGroup{
		Name: "Unavailable",
		Packs: []tui.Pack{{
			Name:        "unavailable-items",
			Description: "Selected recipes unavailable from the current catalog",
			Sources:     sources,
		}},
	})
	return copied
}
