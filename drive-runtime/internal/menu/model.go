package menu

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

type actionFinishedMsg struct {
	err error
}

type actionOutputMsg struct {
	output string
	err    error
}

type Model struct {
	cfg       config.RuntimeConfig
	driveRoot string
	runner    actions.Runner

	groupSelected int
	itemSelected  int
	activeGroup   string
	inGroup       bool

	filter        string
	filtering     bool
	width         int
	height        int
	status        string
	lastErr       error
	showingOutput bool
	output        string
}

func NewModel(cfg config.RuntimeConfig, driveRoot string) Model {
	return Model{
		cfg:       cfg,
		driveRoot: driveRoot,
		runner:    actions.NewRunner(driveRoot),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case actionFinishedMsg:
		m.lastErr = msg.err
		if msg.err != nil {
			m.status = "Action failed"
		} else {
			m.status = "Action finished"
		}
		return m, nil
	case actionOutputMsg:
		m.lastErr = msg.err
		if msg.err != nil {
			m.status = "Action failed"
		} else {
			m.status = ""
		}
		m.output = msg.output
		m.showingOutput = true
		return m, nil
	case tea.KeyMsg:
		if m.showingOutput {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter", "esc":
				m.showingOutput = false
				m.output = ""
				m.status = ""
				m.lastErr = nil
				return m, nil
			}
			return m, nil
		}

		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filter = ""
				m.filtering = false
				m.clampSelection()
			case "enter":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.clampSelection()
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.filter += msg.String()
					m.clampSelection()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.MoveUp()
			return m, nil
		case "down", "j":
			m.MoveDown()
			return m, nil
		case "/":
			m.filtering = true
			return m, nil
		case "esc":
			if m.inGroup {
				m.inGroup = false
				m.activeGroup = ""
				m.itemSelected = 0
				m.filter = ""
				m.filtering = false
				return m, nil
			}
			m.filter = ""
			m.groupSelected = 0
			return m, nil
		case "enter":
			if !m.inGroup {
				group, ok := m.SelectedGroup()
				if !ok {
					return m, nil
				}
				m.inGroup = true
				m.activeGroup = group.ID
				m.itemSelected = 0
				m.filter = ""
				m.filtering = false
				return m, nil
			}

			item, ok := m.SelectedItem()
			if !ok {
				return m, nil
			}
			resolved, err := m.runner.Resolve(item.Action)
			if err != nil {
				m.lastErr = err
				m.status = "Action failed"
				return m, nil
			}
			if resolved.Mode == actions.ModeCaptureOutput {
				return m, func() tea.Msg {
					err := resolved.Cmd.Run()
					output := ""
					if resolved.Cmd.Stdout != nil {
						if buf, ok := resolved.Cmd.Stdout.(interface{ String() string }); ok {
							output = buf.String()
						}
					}
					if resolved.Cmd.Stderr != nil {
						if buf, ok := resolved.Cmd.Stderr.(interface{ String() string }); ok {
							output += buf.String()
						}
					}
					return actionOutputMsg{output: output, err: err}
				}
			}
			return m, tea.ExecProcess(resolved.Cmd, func(err error) tea.Msg {
				return actionFinishedMsg{err: err}
			})
		}
	}

	return m, nil
}

func (m Model) View() string {
	return renderView(m)
}

func (m *Model) SetFilter(value string) {
	m.filter = value
	m.clampSelection()
}

func (m *Model) SetSelected(index int) {
	if m.inGroup {
		m.itemSelected = index
	} else {
		m.groupSelected = index
	}
	m.clampSelection()
}

func (m Model) SelectedIndex() int {
	if m.inGroup {
		return m.itemSelected
	}
	return m.groupSelected
}

func (m *Model) MoveDown() {
	visibleCount := len(m.visibleEntries())
	if visibleCount == 0 {
		m.SetSelected(0)
		return
	}
	if m.SelectedIndex() < visibleCount-1 {
		m.SetSelected(m.SelectedIndex() + 1)
	}
}

func (m *Model) MoveUp() {
	if m.SelectedIndex() > 0 {
		m.SetSelected(m.SelectedIndex() - 1)
	}
}

func (m Model) SelectedGroup() (config.MenuGroup, bool) {
	visible := m.VisibleGroups()
	if len(visible) == 0 {
		return config.MenuGroup{}, false
	}
	if m.groupSelected < 0 || m.groupSelected >= len(visible) {
		return config.MenuGroup{}, false
	}
	return visible[m.groupSelected], true
}

func (m Model) SelectedItem() (config.MenuItem, bool) {
	visible := m.VisibleItems()
	if len(visible) == 0 {
		return config.MenuItem{}, false
	}
	if m.itemSelected < 0 || m.itemSelected >= len(visible) {
		return config.MenuItem{}, false
	}
	return visible[m.itemSelected], true
}

func (m *Model) clampSelection() {
	visibleCount := len(m.visibleEntries())
	if visibleCount == 0 {
		if m.inGroup {
			m.itemSelected = 0
		} else {
			m.groupSelected = 0
		}
		return
	}
	if m.inGroup {
		if m.itemSelected >= visibleCount {
			m.itemSelected = visibleCount - 1
		}
		if m.itemSelected < 0 {
			m.itemSelected = 0
		}
		return
	}
	if m.groupSelected >= visibleCount {
		m.groupSelected = visibleCount - 1
	}
	if m.groupSelected < 0 {
		m.groupSelected = 0
	}
}

func (m Model) CurrentGroup() (config.MenuGroup, bool) {
	for _, group := range m.cfg.Groups {
		if group.ID == m.activeGroup {
			return group, true
		}
	}
	return config.MenuGroup{}, false
}

func (m Model) VisibleGroups() []config.MenuGroup {
	if strings.TrimSpace(m.filter) == "" {
		return append([]config.MenuGroup(nil), m.cfg.Groups...)
	}

	needle := strings.ToLower(strings.TrimSpace(m.filter))
	visible := make([]config.MenuGroup, 0, len(m.cfg.Groups))
	for _, group := range m.cfg.Groups {
		if strings.Contains(strings.ToLower(group.Label), needle) ||
			strings.Contains(strings.ToLower(group.Description), needle) ||
			strings.Contains(strings.ToLower(group.ID), needle) {
			visible = append(visible, group)
		}
	}
	return visible
}

func (m Model) VisibleItems() []config.MenuItem {
	group, ok := m.CurrentGroup()
	if !ok {
		return []config.MenuItem{}
	}
	if strings.TrimSpace(m.filter) == "" {
		return append([]config.MenuItem(nil), group.Items...)
	}

	needle := strings.ToLower(strings.TrimSpace(m.filter))
	visible := make([]config.MenuItem, 0, len(group.Items))
	for _, item := range group.Items {
		if strings.Contains(strings.ToLower(item.Label), needle) ||
			strings.Contains(strings.ToLower(item.Description), needle) ||
			strings.Contains(strings.ToLower(item.Subheader), needle) ||
			strings.Contains(strings.ToLower(item.ID), needle) {
			visible = append(visible, item)
		}
	}
	return visible
}

func (m Model) visibleEntries() []string {
	if m.inGroup {
		items := m.VisibleItems()
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.ID)
		}
		return result
	}

	groups := m.VisibleGroups()
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.ID)
	}
	return result
}
