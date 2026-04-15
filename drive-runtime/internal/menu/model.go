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

type Model struct {
	cfg       config.RuntimeConfig
	driveRoot string
	runner    actions.Runner

	selected  int
	filter    string
	filtering bool
	width     int
	height    int
	status    string
	lastErr   error
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
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filter = ""
				m.filtering = false
				m.selected = 0
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
			m.filter = ""
			m.selected = 0
			return m, nil
		case "enter":
			action, ok := m.SelectedAction()
			if !ok {
				return m, nil
			}
			cmd, err := m.runner.Command(action.Action, action.Args)
			if err != nil {
				m.lastErr = err
				m.status = "Action failed"
				return m, nil
			}
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
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
	m.selected = index
	m.clampSelection()
}

func (m Model) SelectedIndex() int {
	return m.selected
}

func (m *Model) MoveDown() {
	if len(m.VisibleActions()) == 0 {
		m.selected = 0
		return
	}
	if m.selected < len(m.VisibleActions())-1 {
		m.selected++
	}
}

func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m Model) SelectedAction() (config.MenuAction, bool) {
	visible := m.VisibleActions()
	if len(visible) == 0 {
		return config.MenuAction{}, false
	}
	if m.selected < 0 || m.selected >= len(visible) {
		return config.MenuAction{}, false
	}
	return visible[m.selected], true
}

func (m *Model) clampSelection() {
	visible := m.VisibleActions()
	if len(visible) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m Model) VisibleActions() []config.MenuAction {
	if strings.TrimSpace(m.filter) == "" {
		return append([]config.MenuAction(nil), m.cfg.Actions...)
	}

	needle := strings.ToLower(strings.TrimSpace(m.filter))
	visible := make([]config.MenuAction, 0, len(m.cfg.Actions))
	for _, action := range m.cfg.Actions {
		if strings.Contains(strings.ToLower(action.Label), needle) ||
			strings.Contains(strings.ToLower(action.Section), needle) {
			visible = append(visible, action)
		}
	}
	return visible
}
