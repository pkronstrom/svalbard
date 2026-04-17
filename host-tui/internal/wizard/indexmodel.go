package wizard

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// IndexFunc runs keyword indexing with progress reporting.
type IndexFunc func(ctx context.Context, vaultPath string, onProgress func(file, status, detail string)) error

// wizardIndexDoneMsg is sent when indexing completes.
type wizardIndexDoneMsg struct{}

type indexStep struct {
	file   string
	status string
	detail string
}

type indexStartedMsg struct{ ch <-chan indexStep }
type indexTickMsg struct {
	step indexStep
	done bool
}

// wizardIndexModel runs keyword indexing within the wizard.
type wizardIndexModel struct {
	vaultPath string
	runIndex  IndexFunc
	steps     []indexStep
	ch        <-chan indexStep
	started   bool
	finished  bool
	cancel    context.CancelFunc
	width     int
	height    int
	theme     tui.Theme
	keys      tui.KeyMap
}

func newWizardIndex(vaultPath string, runIndex IndexFunc) wizardIndexModel {
	return wizardIndexModel{
		vaultPath: vaultPath,
		runIndex:  runIndex,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m wizardIndexModel) Init() tea.Cmd {
	return m.startIndex()
}

func (m wizardIndexModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case indexStartedMsg:
		m.ch = msg.ch
		m.started = true
		return m, waitForIndexEvent(m.ch)

	case indexTickMsg:
		if msg.done {
			m.finished = true
			return m, nil
		}
		m.updateStep(msg.step)
		return m, waitForIndexEvent(m.ch)

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.finished && m.keys.Enter.Matches(msg) {
			return m, func() tea.Msg { return wizardIndexDoneMsg{} }
		}
		// Allow skipping with Esc
		if m.keys.Back.Matches(msg) {
			if m.cancel != nil {
				m.cancel()
			}
			return m, func() tea.Msg { return wizardIndexDoneMsg{} }
		}
	}
	return m, nil
}

func (m *wizardIndexModel) updateStep(s indexStep) {
	for i := range m.steps {
		if m.steps[i].file == s.file {
			m.steps[i].status = s.status
			m.steps[i].detail = s.detail
			return
		}
	}
	m.steps = append(m.steps, s)
}

func (m wizardIndexModel) startIndex() tea.Cmd {
	runIndex := m.runIndex
	vaultPath := m.vaultPath
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	return func() tea.Msg {
		ch := make(chan indexStep, 16)
		go func() {
			defer close(ch)
			if err := runIndex(ctx, vaultPath, func(file, status, detail string) {
				ch <- indexStep{file: file, status: status, detail: detail}
			}); err != nil {
				ch <- indexStep{file: "error", status: tui.StatusFailed, detail: err.Error()}
			}
		}()
		return indexStartedMsg{ch: ch}
	}
}

func waitForIndexEvent(ch <-chan indexStep) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return indexTickMsg{step: ev, done: !ok}
	}
}

func (m wizardIndexModel) View() string {
	if !m.started {
		return m.theme.Muted.Render("  Preparing to index...")
	}

	steps := make([]tui.ProgressStep, len(m.steps))
	for i, s := range m.steps {
		label := s.file
		if s.detail != "" {
			label = s.detail
		}
		steps[i] = tui.ProgressStep{
			ID:     s.file,
			Label:  label,
			Status: s.status,
		}
	}

	maxVis := m.height - 10
	if maxVis < 4 {
		maxVis = 4
	}
	pv := tui.ProgressView{
		Theme:        m.theme,
		Steps:        steps,
		MaxVisible:   maxVis,
		ScrollToTail: true,
	}

	var b strings.Builder
	b.WriteString(m.theme.Section.Render("Building search index"))
	b.WriteString("\n\n")
	b.WriteString(pv.Render())
	b.WriteString("\n")

	if m.finished {
		done := 0
		for _, s := range steps {
			if s.Status == tui.StatusDone {
				done++
			}
		}
		b.WriteString(m.theme.Success.Render(fmt.Sprintf("  %d files indexed", done)))
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Enter: continue to dashboard"))
	} else {
		b.WriteString(pv.RenderSummary())
		b.WriteString("\n")
		b.WriteString(m.theme.Help.Render("  Esc: skip indexing"))
	}

	return b.String()
}
