package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkronstrom/svalbard/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ID     string
	Status string // "queued", "active", "done", "failed"
	Error  string
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
	id     string
	status string // "", "active", "done", "failed"
	err    string
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
	applying   bool
	applyItems []applyStep
	applyCh    <-chan ApplyEvent
	applyDone  bool
	applyErr   string

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
		m.updateApplyStep(msg.event)
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
		return m, startApply(m.runApply, m.items)

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
		return m, nil
	}

	return m, nil
}

func (m *Model) updateApplyStep(ev ApplyEvent) {
	for i := range m.applyItems {
		if m.applyItems[i].id == ev.ID {
			m.applyItems[i].status = ev.Status
			m.applyItems[i].err = ev.Error
			if ev.Status == "failed" && ev.Error != "" {
				m.applyErr = ev.Error
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Async helpers
// ---------------------------------------------------------------------------

func startApply(runApply func(ctx context.Context, onProgress func(ApplyEvent)) error, items []PlanItem) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan ApplyEvent, 16)
		go func() {
			defer close(ch)
			if err := runApply(context.Background(), func(ev ApplyEvent) {
				ch <- ev
			}); err != nil {
				ch <- ApplyEvent{Status: "failed", Error: err.Error()}
			}
		}()
		return applyStartedMsg{ch: ch}
	}
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
		Left:    body,
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
		line := fmt.Sprintf("%s %s %s  %s", prefix, sym, it.ID, formatSize(it.SizeGB))

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
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  %s · %s · %s · %s", it.ID, it.Type, formatSize(it.SizeGB), it.Action)))
		if it.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render("  " + it.Description))
		}
	}

	// Summary
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf(
		"  Download: %s  |  Remove: %s",
		formatSize(m.downloadGB), formatSize(m.removeGB),
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
		Left:    b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}

func formatSize(gb float64) string {
	if gb < 0.01 {
		return fmt.Sprintf("%.0f MB", gb*1024)
	}
	if gb < 1 {
		return fmt.Sprintf("%.0f MB", gb*1024)
	}
	return fmt.Sprintf("%.1f GB", gb)
}

func (m Model) viewApply() string {
	var b strings.Builder

	b.WriteString(m.theme.Section.Render("Applying changes"))
	b.WriteString("\n\n")

	for _, step := range m.applyItems {
		var symbol string
		var style lipgloss.Style
		switch step.status {
		case "done":
			symbol = m.theme.Success.Render("✓")
			style = m.theme.Base
		case "active":
			symbol = m.theme.Warning.Render("·")
			style = m.theme.Base
		case "failed":
			symbol = m.theme.Danger.Render("✗")
			style = m.theme.Danger
		default:
			symbol = m.theme.Muted.Render(" ")
			style = m.theme.Muted
		}

		// Find description from original items.
		desc := step.id
		for _, it := range m.items {
			if it.ID == step.id {
				desc = it.Description
				break
			}
		}

		b.WriteString(fmt.Sprintf("  %s  %s", symbol, style.Render(desc)))
		if step.err != "" {
			b.WriteString("  " + m.theme.Danger.Render(step.err))
		}
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("\n")
	doneCount := 0
	failedCount := 0
	for _, s := range m.applyItems {
		switch s.status {
		case "done":
			doneCount++
		case "failed":
			failedCount++
		}
	}
	total := len(m.applyItems)
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf(
		"%d/%d complete", doneCount, total,
	)))
	if failedCount > 0 {
		b.WriteString("  " + m.theme.Danger.Render(fmt.Sprintf("%d failed", failedCount)))
	}

	// Footer
	var footer string
	if m.applyDone {
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: back"},
			m.keys.Back,
		)
	} else {
		footer = m.theme.Status.Render("applying...")
	}

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "apply",
		Left:    b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}

func actionSymbol(action string) string {
	if action == "remove" {
		return "-"
	}
	return "+"
}
