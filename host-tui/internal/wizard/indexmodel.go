// TODO(deps): Before indexing, check if an embedding model is present in the
// vault config. If missing, offer the catalog's embedding models.
// See: docs/superpowers/specs/2026-04-18-recipe-deps-design.md

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

type indexStartedMsg struct {
	ch     <-chan indexStep
	result <-chan indexResultMsg
}
type indexTickMsg struct {
	step indexStep
	done bool
}
type indexResultMsg struct {
	err       error
	cancelled bool
}

// wizardIndexModel runs keyword indexing within the wizard.
type wizardIndexModel struct {
	vaultPath      string
	runIndex       IndexFunc
	steps          []indexStep
	ch             <-chan indexStep
	result         <-chan indexResultMsg
	started        bool
	finished       bool
	terminal       string
	terminalDetail string
	ctx            context.Context
	cancel         context.CancelFunc
	exitIntent     string
	width          int
	height         int
	theme          tui.Theme
	keys           tui.KeyMap
}

func newWizardIndex(vaultPath string, runIndex IndexFunc) wizardIndexModel {
	ctx, cancel := context.WithCancel(context.Background())
	return wizardIndexModel{
		vaultPath: vaultPath,
		runIndex:  runIndex,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
		ctx:       ctx,
		cancel:    cancel,
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
		m.result = msg.result
		m.started = true
		return m, waitForIndexEvent(m.ch)

	case indexTickMsg:
		if msg.done {
			return m, waitForIndexResult(m.result)
		}
		m.updateStep(msg.step)
		return m, waitForIndexEvent(m.ch)

	case indexResultMsg:
		m.finished = true
		switch {
		case msg.err == nil:
			m.terminal = "finished"
		case msg.cancelled:
			m.terminal = "cancelled"
		default:
			m.terminal = "failed"
			m.terminalDetail = msg.err.Error()
		}
		if m.exitIntent == "quit" {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		if m.keys.ForceQuit.Matches(msg) {
			if !m.finished {
				m.exitIntent = "quit"
				m.cancel()
				return m, nil
			}
			return m, tea.Quit
		}
		if m.finished && m.keys.Enter.Matches(msg) {
			return m, func() tea.Msg { return wizardIndexDoneMsg{} }
		}
		// Esc cancels but remains on this screen until the producer closes.
		if m.keys.Back.Matches(msg) && !m.finished {
			m.exitIntent = "back"
			m.cancel()
			return m, nil
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
	return func() tea.Msg {
		ch := make(chan indexStep, 16)
		result := make(chan indexResultMsg, 1)
		go func() {
			defer close(ch)
			err := runIndex(m.ctx, vaultPath, func(file, status, detail string) {
				select {
				case ch <- indexStep{file: file, status: status, detail: detail}:
				case <-m.ctx.Done():
				}
			})
			result <- indexResultMsg{err: err, cancelled: m.ctx.Err() != nil}
		}()
		return indexStartedMsg{ch: ch, result: result}
	}
}

func waitForIndexEvent(ch <-chan indexStep) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return indexTickMsg{step: ev, done: !ok}
	}
}
func waitForIndexResult(result <-chan indexResultMsg) tea.Cmd {
	return func() tea.Msg { return <-result }
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

	maxVis := m.height - 12
	if maxVis < 1 {
		maxVis = 1
	}
	pv := tui.ProgressView{
		Theme:        m.theme,
		Steps:        steps,
		MaxVisible:   maxVis,
		ScrollToTail: true,
	}

	var b strings.Builder
	b.WriteString(m.theme.Section.Render("Building search index"))
	b.WriteString("\n")

	if m.finished {
		switch m.terminal {
		case "cancelled":
			b.WriteString(m.theme.Warning.Render("  Indexing cancelled"))
		case "failed":
			message := "  Indexing failed"
			if m.terminalDetail != "" {
				message += ": " + m.terminalDetail
			}
			b.WriteString(m.theme.Error.Render(message))
		default:
			done := 0
			for _, s := range steps {
				if s.Status == tui.StatusDone {
					done++
				}
			}
			b.WriteString(m.theme.Success.Render(fmt.Sprintf("  %d files indexed", done)))
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Help.Render("  Enter: continue to dashboard"))
	} else {
		b.WriteString(pv.Render())
		b.WriteString("\n")
		b.WriteString(pv.RenderSummary())
		b.WriteString("\n")
		b.WriteString(m.theme.Help.Render("  Esc: cancel indexing"))
	}

	return b.String()
}
