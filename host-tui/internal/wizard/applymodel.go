package wizard

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// wizardApplyDoneMsg is sent when the apply completes.
type wizardApplyDoneMsg struct{ success bool }

type applyStep struct {
	id         string
	status     string // tui.Status* constants
	downloaded int64
	total      int64
	err        string
}

type applyStartedMsg struct {
	ch     <-chan ApplyEvent
	result <-chan applyResultMsg
}

type applyTickMsg struct {
	event ApplyEvent
	done  bool
}
type applyResultMsg struct {
	err      error
	outcomes []ApplyEvent
}

// wizardApplyModel runs the apply process within the wizard.
type wizardApplyModel struct {
	vaultPath string
	runApply  ApplyFunc
	steps     []applyStep
	ch        <-chan ApplyEvent
	result    <-chan applyResultMsg
	started   bool
	finished  bool
	err       string
	globalErr string
	// notices are apply's messages about the vault as a whole, whose event
	// id is no step: shown under the steps, each once.
	notices    []string
	stopping   bool
	exitIntent string
	ctx        context.Context
	cancel     context.CancelFunc
	width      int
	height     int
	theme      tui.Theme
	keys       tui.KeyMap
}

func newWizardApply(vaultPath string, itemIDs []string, runApply ApplyFunc) wizardApplyModel {
	ctx, cancel := context.WithCancel(context.Background())
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
		ctx:       ctx,
		cancel:    cancel,
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
		m.result = msg.result
		m.started = true
		return m, waitForEvent(m.ch)

	case applyTickMsg:
		if msg.done {
			return m, waitForResult(m.result)
		}
		if msg.event.ID == "" && msg.event.Error != "" {
			m.globalErr = msg.event.Error
			return m, waitForEvent(m.ch)
		}
		matched := false
		for i := range m.steps {
			if m.steps[i].id == msg.event.ID {
				m.steps[i].status = msg.event.Status
				m.steps[i].downloaded = msg.event.Downloaded
				m.steps[i].total = msg.event.Total
				if msg.event.Error != "" {
					m.steps[i].err = msg.event.Error
				}
				matched = true
				break
			}
		}
		if !matched && msg.event.Message != "" && !slices.Contains(m.notices, msg.event.Message) {
			m.notices = append(m.notices, msg.event.Message)
		}
		return m, waitForEvent(m.ch)

	case applyResultMsg:
		m.finished = true
		for _, event := range msg.outcomes {
			for i := range m.steps {
				if m.steps[i].id == event.ID {
					m.steps[i].status = event.Status
					m.steps[i].downloaded = event.Downloaded
					m.steps[i].total = event.Total
					m.steps[i].err = event.Error
				}
			}
		}
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		if m.err == "" {
			m.err = m.globalErr
		}
		if m.exitIntent == "quit" {
			return m, tea.Quit
		}
		if m.exitIntent == "back" {
			return m, func() tea.Msg { return wizardApplyDoneMsg{success: false} }
		}
		return m, nil

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) || m.keys.Back.Matches(msg) {
			if !m.finished && !m.stopping {
				m.stopping = true
				if m.keys.ForceQuit.Matches(msg) {
					m.exitIntent = "quit"
				} else {
					m.exitIntent = "back"
				}
				m.cancel()
				return m, nil
			}
			if m.finished && m.keys.ForceQuit.Matches(msg) {
				return m, tea.Quit
			}
		}
		if m.finished && m.keys.Enter.Matches(msg) {
			return m, func() tea.Msg { return wizardApplyDoneMsg{success: m.success()} }
		}
	}
	return m, nil
}

func (m wizardApplyModel) startApply() tea.Cmd {
	runApply := m.runApply
	vaultPath := m.vaultPath
	return func() tea.Msg {
		ch := make(chan ApplyEvent, 16)
		result := make(chan applyResultMsg, 1)
		outcomes := make(map[string]ApplyEvent)
		var outcomesMu sync.Mutex
		go func() {
			defer close(ch)
			err := runApply(m.ctx, vaultPath, func(ev ApplyEvent) {
				if ev.ID != "" {
					outcomesMu.Lock()
					outcomes[ev.ID] = ev
					outcomesMu.Unlock()
				}
				select {
				case ch <- ev:
				case <-m.ctx.Done():
				}
			})
			if err != nil {
				select {
				case ch <- ApplyEvent{Status: tui.StatusFailed, Error: err.Error()}:
				case <-m.ctx.Done():
				}
			}
			outcomesMu.Lock()
			snapshot := make([]ApplyEvent, 0, len(outcomes))
			for _, event := range outcomes {
				snapshot = append(snapshot, event)
			}
			outcomesMu.Unlock()
			result <- applyResultMsg{err: err, outcomes: snapshot}
		}()
		return applyStartedMsg{ch: ch, result: result}
	}
}

func waitForResult(result <-chan applyResultMsg) tea.Cmd {
	return func() tea.Msg { return <-result }
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
	for _, notice := range m.notices {
		b.WriteString(m.theme.Help.Render("  Note: " + notice))
		b.WriteString("\n")
	}

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
		pending := len(pv.Steps) - done - failed
		if !m.success() {
			b.WriteString(m.theme.Warning.Render(fmt.Sprintf("  Completed: %d, Failed: %d, Pending: %d", done, failed, pending)))
			b.WriteString("\n")
			b.WriteString(m.theme.Warning.Render("  " + m.err + "\n  Reopen this vault and apply the remaining plan to resume."))
		} else {
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("  Done: %d items applied", done)))
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Enter: continue"))
	} else if m.stopping {
		b.WriteString(m.theme.Warning.Render("Stopping… saving progress"))
	} else {
		b.WriteString(pv.RenderSummary())
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Esc: stop and keep what is downloaded"))
	}

	return b.String()
}

func (m wizardApplyModel) success() bool {
	if m.err != "" || m.globalErr != "" {
		return false
	}
	for _, step := range m.steps {
		if step.status != tui.StatusDone {
			return false
		}
	}
	return true
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
