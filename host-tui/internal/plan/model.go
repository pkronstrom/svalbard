package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkronstrom/svalbard/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Public types — mirrors of hosttui types, defined locally to avoid import cycle.
// ---------------------------------------------------------------------------

// PlanItem is a single entry in a reconciliation plan.
type PlanItem struct {
	ID          string
	Type        string
	SizeGB      float64
	Description string
	Action      string // "download" or "remove"
}

// ApplyEvent reports progress of a single item during apply.
type ApplyEvent struct {
	ID         string
	Status     string // tui.StatusQueued, tui.StatusActive, tui.StatusDone, tui.StatusFailed
	Step       string // current build step (e.g. "wget", "warc2zim")
	Downloaded int64  // bytes downloaded so far
	Total      int64  // total bytes (-1 if unknown)
	Error      string
}

// Config holds everything the plan screen needs from its parent.
type Config struct {
	Items       []PlanItem
	DownloadGB  float64
	RemoveGB    float64
	FreeAfterGB float64
	RunApply    func(ctx context.Context, onProgress func(ApplyEvent)) error // nil if apply not available
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// BackMsg signals the parent to navigate back.
type BackMsg struct{}

// BrowseMsg signals the parent to open the browse screen.
type BrowseMsg struct{}

// Internal messages.
type applyStartedMsg struct{ ch <-chan ApplyEvent }
type applyTickMsg struct {
	event ApplyEvent
	done  bool
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type applyStep struct {
	id         string
	status     string // tui.Status* constants
	step       string // current build step
	err        string
	downloaded int64
	total      int64
}

// Model is the bubbletea model for the Plan + Apply screen.
type Model struct {
	items       []PlanItem
	downloadGB  float64
	removeGB    float64
	freeAfterGB float64
	runApply    func(ctx context.Context, onProgress func(ApplyEvent)) error

	cursor       int
	scrollOffset int

	// Apply sub-state
	applying     bool
	applyItems   []applyStep
	applyCh      <-chan ApplyEvent
	applyDone    bool
	applyErr     string
	applyCancel  context.CancelFunc

	width, height int
	theme         tui.Theme
	keys          tui.KeyMap
}

// New creates a Model from the given Config.
func New(cfg Config) Model {
	return Model{
		items:       cfg.Items,
		downloadGB:  cfg.DownloadGB,
		removeGB:    cfg.RemoveGB,
		freeAfterGB: cfg.FreeAfterGB,
		runApply:    cfg.RunApply,
		theme:       tui.DefaultTheme(),
		keys:        tui.DefaultKeyMap(),
	}
}

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case applyStartedMsg:
		m.applyCh = msg.ch
		return m, waitForApplyEvent(m.applyCh)

	case applyTickMsg:
		if msg.done {
			m.applyDone = true
			return m, nil
		}
		// Global error (empty ID) — mark remaining queued items as failed.
		if msg.event.ID == "" && msg.event.Status == tui.StatusFailed {
			for i := range m.applyItems {
				if m.applyItems[i].status == "" || m.applyItems[i].status == tui.StatusActive {
					m.applyItems[i].status = tui.StatusFailed
					m.applyItems[i].err = msg.event.Error
				}
			}
		} else {
			m.updateApplyStep(msg.event)
		}
		return m, waitForApplyEvent(m.applyCh)

	case tea.KeyMsg:
		if m.applying {
			return m.updateApplying(msg)
		}
		return m.updatePlan(msg)
	}

	return m, nil
}

func (m Model) updatePlan(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.MoveDown.Matches(msg):
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil

	case m.keys.MoveUp.Matches(msg):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case m.keys.Enter.Matches(msg):
		if len(m.items) == 0 || m.runApply == nil {
			return m, nil
		}
		m.applying = true
		m.applyDone = false
		m.applyErr = ""
		m.applyItems = make([]applyStep, len(m.items))
		for i, it := range m.items {
			m.applyItems[i] = applyStep{id: it.ID}
		}
		cmd, cancel := startApply(m.runApply, m.items)
		m.applyCancel = cancel
		return m, cmd

	case m.keys.Back.Matches(msg), m.keys.Quit.Matches(msg):
		return m, func() tea.Msg { return BackMsg{} }
	}

	// "b" for browse
	if msg.String() == "b" {
		return m, func() tea.Msg { return BrowseMsg{} }
	}

	return m, nil
}

func (m Model) updateApplying(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		if m.applyCancel != nil {
			m.applyCancel()
		}
		return m, tea.Quit

	case m.keys.Enter.Matches(msg):
		if m.applyDone {
			return m, func() tea.Msg { return BackMsg{} }
		}
		return m, nil

	case m.keys.Back.Matches(msg):
		if m.applyDone {
			return m, func() tea.Msg { return BackMsg{} }
		}
		// Cancel running apply and go back.
		if m.applyCancel != nil {
			m.applyCancel()
		}
		return m, func() tea.Msg { return BackMsg{} }
	}

	return m, nil
}

func (m *Model) updateApplyStep(ev ApplyEvent) {
	for i := range m.applyItems {
		if m.applyItems[i].id == ev.ID {
			m.applyItems[i].status = ev.Status
			m.applyItems[i].downloaded = ev.Downloaded
			m.applyItems[i].total = ev.Total
			if ev.Step != "" {
				m.applyItems[i].step = ev.Step
			}
			if ev.Error != "" {
				m.applyItems[i].err = ev.Error
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Async helpers
// ---------------------------------------------------------------------------

func startApply(runApply func(ctx context.Context, onProgress func(ApplyEvent)) error, items []PlanItem) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := func() tea.Msg {
		ch := make(chan ApplyEvent, 16)
		go func() {
			defer close(ch)
			if err := runApply(ctx, func(ev ApplyEvent) {
				ch <- ev
			}); err != nil {
				ch <- ApplyEvent{Status: tui.StatusFailed, Error: err.Error()}
			}
		}()
		return applyStartedMsg{ch: ch}
	}
	return cmd, cancel
}

func waitForApplyEvent(ch <-chan ApplyEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return applyTickMsg{event: ev, done: !ok}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the plan or apply screen.
func (m Model) View() string {
	if len(m.items) == 0 {
		return m.viewEmpty()
	}
	if m.applying {
		return m.viewApply()
	}
	return m.viewPlan()
}

func (m Model) viewEmpty() string {
	body := m.theme.Success.Render("Everything in sync.") +
		"\n\n" +
		m.theme.Muted.Render("No pending downloads or removals.")

	footer := tui.FooterHints(
		tui.KeyBinding{Key: "b", Label: "b: browse"},
		m.keys.Back,
	)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "plan",
		Right:   body,
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}

func (m Model) viewPlan() string {
	var b strings.Builder

	b.WriteString(m.theme.Section.Render("Pending changes"))
	b.WriteString("\n\n")

	// Visible height: reserve for header(2) + detail(4) + summary(2) + footer(2) + shell(4)
	visibleHeight := m.height - 14
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Adjust scroll offset to keep cursor visible.
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleHeight {
		m.scrollOffset = m.cursor - visibleHeight + 1
	}

	end := m.scrollOffset + visibleHeight
	if end > len(m.items) {
		end = len(m.items)
	}

	if m.scrollOffset > 0 {
		b.WriteString(m.theme.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}

	for i := m.scrollOffset; i < end; i++ {
		it := m.items[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		sym := actionSymbol(it.Action)
		line := fmt.Sprintf("%s %s %s  %s", prefix, sym, it.ID, tui.FormatSize(it.SizeGB))

		if i == m.cursor {
			b.WriteString(m.theme.Selected.Render(line))
		} else {
			b.WriteString(m.theme.Base.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(m.items) {
		b.WriteString(m.theme.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}

	// Detail area for selected item
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  ─────────────────────────────────"))
	b.WriteString("\n")
	if m.cursor >= 0 && m.cursor < len(m.items) {
		it := m.items[m.cursor]
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s · %s · %s · %s", it.ID, it.Type, tui.FormatSize(it.SizeGB), it.Action)))
		if it.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render("  " + it.Description))
		}
	}

	// Summary
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf(
		"  Download: %s  |  Remove: %s",
		tui.FormatSize(m.downloadGB), tui.FormatSize(m.removeGB),
	)))

	// Footer
	enterLabel := tui.KeyBinding{Key: "enter", Label: "Enter: apply"}
	if m.runApply == nil {
		enterLabel.Label = ""
	}
	footer := tui.FooterHints(
		m.keys.MoveUp,
		enterLabel,
		tui.KeyBinding{Key: "b", Label: "b: browse"},
		m.keys.Back,
	)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "plan",
		Right:   b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}


func (m Model) viewApply() string {
	pv := m.applyProgressView()

	var b strings.Builder
	b.WriteString(m.theme.Section.Render("Applying changes"))
	b.WriteString("\n\n")
	b.WriteString(pv.Render())
	b.WriteString("\n")
	b.WriteString(pv.RenderSummary())

	var footer string
	if m.applyDone {
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: back"},
			m.keys.Back,
		)
	} else {
		done, active := 0, 0
		for _, s := range pv.Steps {
			switch s.Status {
			case tui.StatusDone:
				done++
			case tui.StatusActive:
				active++
			}
		}
		footer = fmt.Sprintf("downloading %d/%d ...", done+active, len(pv.Steps))
	}

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "apply",
		Right:   b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}

func (m Model) applyProgressView() tui.ProgressView {
	steps := make([]tui.ProgressStep, len(m.applyItems))
	for i, s := range m.applyItems {
		steps[i] = tui.ProgressStep{
			ID:         s.id,
			Status:     s.status,
			Step:       s.step,
			Downloaded: s.downloaded,
			Total:      s.total,
			Error:      s.err,
		}
	}
	maxVis := m.height - 12
	if maxVis < 4 {
		maxVis = 4
	}
	return tui.ProgressView{
		Theme:      m.theme,
		Steps:      steps,
		MaxVisible: maxVis,
	}
}


func actionSymbol(action string) string {
	if action == "remove" {
		return "-"
	}
	return "+"
}
