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
	var b strings.Builder

	if !m.started {
		b.WriteString(m.theme.Muted.Render("  Preparing to apply..."))
		return b.String()
	}

	b.WriteString(m.theme.Section.Render("Downloading & applying"))
	b.WriteString("\n\n")

	maxVis := m.height - 10
	if maxVis < 3 {
		maxVis = 3
	}

	// Scroll to show active items.
	offset := 0
	for i, s := range m.steps {
		if s.status == tui.StatusActive {
			offset = i - maxVis/2
			break
		}
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + maxVis
	if end > len(m.steps) {
		end = len(m.steps)
	}

	if offset > 0 {
		b.WriteString(m.theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	doneCount := 0
	failCount := 0
	activeCount := 0
	for _, s := range m.steps {
		switch s.status {
		case tui.StatusDone:
			doneCount++
		case tui.StatusFailed:
			failCount++
		case tui.StatusActive:
			activeCount++
		}
	}

	for i := offset; i < end; i++ {
		s := m.steps[i]
		var symbol string
		switch s.status {
		case tui.StatusDone:
			symbol = m.theme.Success.Render("✓")
		case tui.StatusActive:
			symbol = m.theme.Focus.Render("↓")
		case tui.StatusFailed:
			symbol = m.theme.Danger.Render("✗")
		default:
			symbol = m.theme.Muted.Render(" ")
		}

		label := s.id
		if s.status == tui.StatusActive && s.downloaded > 0 {
			if s.total > 0 {
				pct := int(float64(s.downloaded) / float64(s.total) * 100)
				label += fmt.Sprintf("  %s/%s  %d%%",
					tui.FormatBytes(s.downloaded), tui.FormatBytes(s.total), pct)
			} else {
				label += fmt.Sprintf("  %s", tui.FormatBytes(s.downloaded))
			}
		}
		if s.err != "" {
			errMsg := s.err
			if len(errMsg) > 50 {
				errMsg = errMsg[:50] + "..."
			}
			label += "  " + errMsg
		}

		line := fmt.Sprintf("  %s  %s", symbol, label)
		switch s.status {
		case tui.StatusActive:
			b.WriteString(m.theme.Base.Render(line))
		case tui.StatusFailed:
			b.WriteString(m.theme.Danger.Render(line))
		default:
			b.WriteString(m.theme.Muted.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.steps) {
		b.WriteString(m.theme.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.finished {
		if failCount > 0 {
			b.WriteString(m.theme.Warning.Render(fmt.Sprintf("  Done: %d completed, %d failed", doneCount, failCount)))
		} else {
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("  Done: %d items applied", doneCount)))
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Enter: continue to dashboard"))
	} else {
		summary := fmt.Sprintf("  %d/%d done", doneCount, len(m.steps))
		if activeCount > 0 {
			summary += fmt.Sprintf("  %d active", activeCount)
		}
		b.WriteString(m.theme.Muted.Render(summary))
	}

	return b.String()
}

