package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// Messages emitted by the pack picker.
type packDoneMsg struct {
	selectedIDs map[string]bool
}
type packCancelMsg struct{}

// packPickerModel is the Bubble Tea sub-model for the pack picker sub-screen.
type packPickerModel struct {
	picker tui.TreePicker
	width  int
	height int
}

// newPackPicker creates a pack picker model.
func newPackPicker(groups []tui.PackGroup, checked map[string]bool, freeGB float64) packPickerModel {
	tp := tui.NewTreePicker(tui.TreePickerConfig{
		Groups:     groups,
		CheckedIDs: checked,
		FreeGB:     freeGB,
		ShowAction: true,
	})

	return packPickerModel{picker: tp}
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
		return m, nil

	case tea.KeyMsg:
		// Let the shared picker handle navigation/toggle.
		if m.picker.Update(msg) {
			return m, nil
		}

		switch {
		// Enter on action row → done.
		case m.picker.Keys.Enter.Matches(msg):
			if row := m.picker.CursorRow(); row != nil && row.Kind == tui.RowAction {
				selected := make(map[string]bool, len(m.picker.CheckedIDs))
				for id, v := range m.picker.CheckedIDs {
					if v {
						selected[id] = true
					}
				}
				return m, func() tea.Msg { return packDoneMsg{selectedIDs: selected} }
			}

		// 'a' shortcut → done.
		case tui.MatchRune(msg, 'a'):
			selected := make(map[string]bool, len(m.picker.CheckedIDs))
			for id, v := range m.picker.CheckedIDs {
				if v {
					selected[id] = true
				}
			}
			return m, func() tea.Msg { return packDoneMsg{selectedIDs: selected} }

		// Cancel.
		case m.picker.Keys.Quit.Matches(msg):
			return m, func() tea.Msg { return packCancelMsg{} }

		case m.picker.Keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		}
	}
	return m, nil
}


// View renders the pack picker.
func (m packPickerModel) View() string {
	var b strings.Builder

	b.WriteString(m.picker.RenderTree())
	b.WriteString("\n")
	b.WriteString(m.picker.RenderSizeSummary())
	b.WriteString("\n")

	b.WriteString(m.picker.Theme.Help.Render(fmt.Sprintf("  %s/%s navigate  %s toggle  %s expand/collapse  a apply  q cancel",
		m.picker.Keys.MoveUp.Key, m.picker.Keys.MoveDown.Key, m.picker.Keys.Toggle.Key, m.picker.Keys.Enter.Key)))
	b.WriteString("\n")

	return b.String()
}
