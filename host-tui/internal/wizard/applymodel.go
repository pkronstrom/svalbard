package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// wizardApplyDoneMsg is sent when the apply completes.
type wizardApplyDoneMsg struct{}

type applyStep struct {
	id         string
	status     string // tui.Status* constants
	downloaded int64
	total      int64
	err        string
}

type applyStartedMsg struct {
	ch <-chan ApplyEvent
}

type applyTickMsg struct {
	event ApplyEvent
	done  bool
}

// wizardApplyModel runs the apply process within the wizard.
type wizardApplyModel struct {
	vaultPath string
	runApply  ApplyFunc
	steps     []applyStep
	ch        <-chan ApplyEvent
	started   bool
	finished  bool
	width     int
	height    int
	theme     tui.Theme
	keys      tui.KeyMap
}

func newWizardApply(vaultPath string, itemIDs []string, runApply ApplyFunc) wizardApplyModel {
	steps := make([]applyStep, len(itemIDs))
	for i, id := range itemIDs {
		steps[i] = applyStep{id: id}
	}
	return wizardApplyModel{
		vaultPath: vaultPath,
		runApply:  runApply,
		steps:     steps,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m wizardApplyModel) Init() tea.Cmd {
	return m.startApply()
}

func (m wizardApplyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case applyStartedMsg:
		m.ch = msg.ch
		m.started = true
		return m, waitForEvent(m.ch)

	case applyTickMsg:
		if msg.done {
			m.finished = true
			return m, nil
		}
		for i := range m.steps {
			if m.steps[i].id == msg.event.ID {
				m.steps[i].status = msg.event.Status
				m.steps[i].downloaded = msg.event.Downloaded
				m.steps[i].total = msg.event.Total
				if msg.event.Error != "" {
					m.steps[i].err = msg.event.Error
				}
				break
			}
		}
		return m, waitForEvent(m.ch)

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) {
			return m, tea.Quit
		}
		if m.finished && m.keys.Enter.Matches(msg) {
			return m, func() tea.Msg { return wizardApplyDoneMsg{} }
		}
	}
	return m, nil
}

func (m wizardApplyModel) startApply() tea.Cmd {
	runApply := m.runApply
	vaultPath := m.vaultPath
	return func() tea.Msg {
		ch := make(chan ApplyEvent, 16)
		go func() {
			defer close(ch)
			if err := runApply(vaultPath, func(ev ApplyEvent) {
				ch <- ev
			}); err != nil {
				ch <- ApplyEvent{Status: tui.StatusFailed, Error: err.Error()}
			}
		}()
		return applyStartedMsg{ch: ch}
	}
}

func waitForEvent(ch <-chan ApplyEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return applyTickMsg{event: ev, done: !ok}
	}
}

func (m wizardApplyModel) View() string {
	if !m.started {
		return m.theme.Muted.Render("  Preparing to apply...")
	}

	pv := m.progressView()

	var b strings.Builder
	b.WriteString(m.theme.Section.Render("Downloading & applying"))
	b.WriteString("\n\n")
	b.WriteString(pv.Render())
	b.WriteString("\n")

	if m.finished {
		done, failed := 0, 0
		for _, s := range pv.Steps {
			switch s.Status {
			case tui.StatusDone:
				done++
			case tui.StatusFailed:
				failed++
			}
		}
		if failed > 0 {
			b.WriteString(m.theme.Warning.Render(fmt.Sprintf("  Done: %d completed, %d failed", done, failed)))
		} else {
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("  Done: %d items applied", done)))
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Enter: continue"))
	} else {
		b.WriteString(pv.RenderSummary())
	}

	return b.String()
}

func (m wizardApplyModel) progressView() tui.ProgressView {
	steps := make([]tui.ProgressStep, len(m.steps))
	for i, s := range m.steps {
		steps[i] = tui.ProgressStep{
			ID:         s.id,
			Status:     s.status,
			Downloaded: s.downloaded,
			Total:      s.total,
			Error:      s.err,
		}
	}
	maxVis := m.height - 10
	if maxVis < 4 {
		maxVis = 4
	}
	return tui.ProgressView{
		Theme:      m.theme,
		Steps:      steps,
		MaxVisible: maxVis,
	}
}

