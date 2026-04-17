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
	id     string
	status string // "", "active", "done", "failed"
	err    string
}

type applyStartedMsg struct {
	ch <-chan applyEvent
}

type applyEvent struct {
	id     string
	status string
}

type applyTickMsg struct {
	event applyEvent
	done  bool
}

// wizardApplyModel runs the apply process within the wizard.
type wizardApplyModel struct {
	vaultPath string
	itemIDs   []string
	runApply  ApplyFunc
	steps     []applyStep
	ch        <-chan applyEvent
	started   bool
	finished  bool
	errMsg    string
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
		itemIDs:   itemIDs,
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
		// Update step status
		for i := range m.steps {
			if m.steps[i].id == msg.event.id {
				m.steps[i].status = msg.event.status
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
		ch := make(chan applyEvent, 16)
		go func() {
			defer close(ch)
			_ = runApply(vaultPath, func(id, status string) {
				ch <- applyEvent{id: id, status: status}
			})
		}()
		return applyStartedMsg{ch: ch}
	}
}

func waitForEvent(ch <-chan applyEvent) tea.Cmd {
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

	// Show steps with status
	maxVis := m.height - 10
	if maxVis < 3 {
		maxVis = 3
	}

	// Find a good scroll offset — show the active item
	offset := 0
	for i, s := range m.steps {
		if s.status == "active" {
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
	for _, s := range m.steps {
		if s.status == "done" {
			doneCount++
		}
		if s.status == "failed" {
			failCount++
		}
	}

	for i := offset; i < end; i++ {
		s := m.steps[i]
		var symbol string
		switch s.status {
		case "done":
			symbol = m.theme.Success.Render("✓")
		case "active":
			symbol = m.theme.Focus.Render("·")
		case "failed":
			symbol = m.theme.Danger.Render("✗")
		default:
			symbol = m.theme.Muted.Render(" ")
		}

		line := fmt.Sprintf("  %s  %s", symbol, s.id)
		switch s.status {
		case "active":
			b.WriteString(m.theme.Base.Render(line))
		case "done":
			b.WriteString(m.theme.Muted.Render(line))
		case "failed":
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
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %d / %d", doneCount, len(m.steps))))
	}

	return b.String()
}
