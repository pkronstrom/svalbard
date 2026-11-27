package plan

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Public types — mirrors of hosttui types, defined locally to avoid import cycle.
// ---------------------------------------------------------------------------

// PlanItem is a single entry in a reconciliation plan.
type PlanItem struct {
	ID              string
	Type            string
	SizeGB          float64
	Description     string
	Action          string
	Network         bool
	BuildDownloadGB float64
	BuildWorkGB     float64
	Requires        []string
}

// ApplyEvent reports progress of a single item during apply.
type ApplyEvent struct {
	ID         string
	Procedure  string
	State      string
	Status     string
	Step       string
	Message    string
	Downloaded int64
	Total      int64
	Error      string
}

// Config holds everything the plan screen needs from its parent.
type Config struct {
	Items           []PlanItem
	DownloadGB      float64
	RemoveGB        float64
	BuildDownloadGB float64
	BuildWorkGB     float64
	ToolsRequired   bool
	// FreeGB is the space free on the vault's disk now, and FreeAfterGB the
	// estimate once the plan is applied; FreeGB 0 means not measured.
	FreeGB      float64
	FreeAfterGB float64
	RunApply    func(ctx context.Context, onProgress func(ApplyEvent)) error
	// RunSelected, when set, lets the user choose which items to run: each
	// starts ticked, and Enter runs it with the ticked ids instead of
	// RunApply. replace asks for each installed copy to be deleted before its
	// new one arrives; it starts on when FreeGB cannot hold every new copy.
	RunSelected func(ctx context.Context, ids []string, replace bool, onProgress func(ApplyEvent)) error
	// Title and EmptyText replace the plan screen's wording when it lists
	// something other than pending changes, such as available updates.
	Title     string
	EmptyText string
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// BackMsg signals the parent to navigate back.
type BackMsg struct{}

// BrowseMsg signals the parent to open the browse screen.
type BrowseMsg struct{}

// ApplySucceededMsg lets the host route a durable successful apply into its
// existing indexing progress screen.
type ApplySucceededMsg struct{}

// Internal messages.
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
	items           []PlanItem
	downloadGB      float64
	removeGB        float64
	buildDownloadGB float64
	buildWorkGB     float64
	toolsRequired   bool
	freeGB          float64
	freeAfterGB     float64
	runApply        func(ctx context.Context, onProgress func(ApplyEvent)) error
	runSelected     func(ctx context.Context, ids []string, replace bool, onProgress func(ApplyEvent)) error
	ticked          map[string]bool
	replace         bool
	title           string
	emptyText       string
	cursor          int
	scrollOffset    int

	// Apply sub-state
	applying    bool
	applyItems  []applyStep
	applyCh     <-chan ApplyEvent
	applyResult <-chan applyResultMsg
	applyDone   bool
	applyErr    string
	// notices are messages from apply about the vault as a whole, whose
	// event id is no plan row: shown under the rows, each once.
	notices     []string
	applyCancel context.CancelFunc
	stopping    bool
	exitIntent  string

	width, height int
	theme         tui.Theme
	keys          tui.KeyMap
}

// New creates a Model from the given Config.
func New(cfg Config) Model {
	if cfg.Title == "" {
		cfg.Title = "Pending changes"
	}
	if cfg.EmptyText == "" {
		cfg.EmptyText = "No recipe downloads or removals pending"
	}
	m := Model{
		title:           cfg.Title,
		emptyText:       cfg.EmptyText,
		items:           cfg.Items,
		downloadGB:      cfg.DownloadGB,
		removeGB:        cfg.RemoveGB,
		buildDownloadGB: cfg.BuildDownloadGB,
		buildWorkGB:     cfg.BuildWorkGB,
		toolsRequired:   cfg.ToolsRequired,
		freeGB:          cfg.FreeGB,
		freeAfterGB:     cfg.FreeAfterGB,
		runApply:        cfg.RunApply,
		runSelected:     cfg.RunSelected,
		theme:           tui.DefaultTheme(),
		keys:            tui.DefaultKeyMap(),
	}
	if m.runSelected != nil {
		m.ticked = make(map[string]bool, len(m.items))
		for _, it := range m.items {
			m.ticked[it.ID] = true
		}
		m.replace = m.freeGB > 0 && m.tickedGB() > m.freeGB
	}
	return m
}

// tickedItems returns the ticked items in list order.
func (m Model) tickedItems() []PlanItem {
	var items []PlanItem
	for _, it := range m.items {
		if m.ticked[it.ID] {
			items = append(items, it)
		}
	}
	return items
}

func (m Model) tickedGB() float64 {
	var gb float64
	for _, it := range m.tickedItems() {
		gb += it.SizeGB
	}
	return gb
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
		m.applyResult = msg.result
		return m, waitForApplyEvent(m.applyCh)

	case applyTickMsg:
		if msg.done {
			return m, waitForApplyResult(m.applyResult)
		}
		// Global error (empty ID) — mark remaining queued items as failed.
		if msg.event.ID == "" && msg.event.Status == tui.StatusFailed {
			m.applyErr = msg.event.Error
		} else {
			m.updateApplyStep(msg.event)
		}
		return m, waitForApplyEvent(m.applyCh)

	case applyResultMsg:
		m.applyDone = true
		for _, event := range msg.outcomes {
			m.updateApplyStep(event)
		}
		if msg.err != nil {
			m.applyErr = msg.err.Error()
		}
		if m.exitIntent == "quit" {
			return m, tea.Quit
		}
		if m.exitIntent == "back" {
			return m, func() tea.Msg { return BackMsg{} }
		}
		if msg.err == nil {
			return m, func() tea.Msg { return ApplySucceededMsg{} }
		}
		return m, nil

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

	case m.runSelected != nil && m.keys.Toggle.Matches(msg):
		if m.cursor < len(m.items) {
			id := m.items[m.cursor].ID
			m.ticked[id] = !m.ticked[id]
		}
		return m, nil

	case m.runSelected != nil && msg.String() == "a":
		all := len(m.tickedItems()) < len(m.items)
		for _, it := range m.items {
			m.ticked[it.ID] = all
		}
		return m, nil

	case m.runSelected != nil && msg.String() == "r":
		m.replace = !m.replace
		return m, nil

	case m.runSelected != nil && m.keys.Enter.Matches(msg):
		items := m.tickedItems()
		if len(items) == 0 {
			return m, nil
		}
		ids := make([]string, len(items))
		for i, it := range items {
			ids[i] = it.ID
		}
		replace, run := m.replace, m.runSelected
		return m.startApplying(items, func(ctx context.Context, onProgress func(ApplyEvent)) error {
			return run(ctx, ids, replace, onProgress)
		})

	case m.keys.Enter.Matches(msg):
		if len(m.items) == 0 && m.runApply == nil {
			return m, func() tea.Msg { return BrowseMsg{} }
		}
		if m.runApply == nil {
			return m, nil
		}
		return m.startApplying(m.items, m.runApply)

	case m.keys.Back.Matches(msg), m.keys.Quit.Matches(msg):
		return m, func() tea.Msg { return BackMsg{} }
	}

	// "b" for browse
	if msg.String() == "b" {
		return m, func() tea.Msg { return BrowseMsg{} }
	}

	return m, nil
}

func (m Model) startApplying(items []PlanItem, run func(ctx context.Context, onProgress func(ApplyEvent)) error) (tea.Model, tea.Cmd) {
	m.applying = true
	m.applyDone = false
	m.applyErr = ""
	m.notices = nil
	m.stopping = false
	m.exitIntent = ""
	m.applyItems = make([]applyStep, len(items))
	for i, it := range items {
		m.applyItems[i] = applyStep{id: it.ID}
	}
	cmd, cancel := startApply(run, items)
	m.applyCancel = cancel
	return m, cmd
}

func (m Model) updateApplying(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		if m.applyDone {
			return m, tea.Quit
		}
		return m.stopApply("quit")

	case m.keys.Enter.Matches(msg):
		if m.applyDone {
			return m, func() tea.Msg { return BackMsg{} }
		}
		return m, nil

	case m.keys.Back.Matches(msg):
		if m.applyDone {
			return m, func() tea.Msg { return BackMsg{} }
		}
		return m.stopApply("back")
	}

	return m, nil
}

func (m Model) stopApply(intent string) (tea.Model, tea.Cmd) {
	if !m.stopping {
		m.stopping = true
		m.exitIntent = intent
		if m.applyCancel != nil {
			m.applyCancel()
		}
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
	if ev.Message != "" && !slices.Contains(m.notices, ev.Message) {
		m.notices = append(m.notices, ev.Message)
	}
}

// ---------------------------------------------------------------------------
// Async helpers
// ---------------------------------------------------------------------------

func startApply(runApply func(ctx context.Context, onProgress func(ApplyEvent)) error, items []PlanItem) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := func() tea.Msg {
		ch := make(chan ApplyEvent, 16)
		result := make(chan applyResultMsg, 1)
		outcomes := make(map[string]ApplyEvent)
		var outcomesMu sync.Mutex
		go func() {
			defer close(ch)
			err := runApply(ctx, func(ev ApplyEvent) {
				if ev.ID != "" {
					outcomesMu.Lock()
					outcomes[ev.ID] = ev
					outcomesMu.Unlock()
				}
				select {
				case ch <- ev:
				case <-ctx.Done():
				}
			})
			if err != nil {
				select {
				case ch <- ApplyEvent{Status: tui.StatusFailed, Error: err.Error()}:
				case <-ctx.Done():
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
	return cmd, cancel
}

func waitForApplyResult(result <-chan applyResultMsg) tea.Cmd {
	return func() tea.Msg { return <-result }
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
	if m.applying {
		return m.viewApply()
	}
	if len(m.items) == 0 {
		return m.viewEmpty()
	}
	return m.viewPlan()
}

func (m Model) viewEmpty() string {
	body := m.theme.Muted.Render(m.emptyText)

	enter := tui.KeyBinding{Key: "enter", Label: "Enter: browse"}
	if m.runApply != nil {
		enter.Label = "Enter: apply/finalize"
	}
	footer := tui.FooterHints(
		enter,
		tui.KeyBinding{Key: "b", Label: "b: browse"},
		tui.KeyBinding{Key: "q", Label: "q/Esc: back"},
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

	b.WriteString(m.theme.Section.Render(m.title))
	if m.runSelected != nil {
		b.WriteString(fmt.Sprintf("  %d of %d ticked", len(m.tickedItems()), len(m.items)))
	}
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

		sym := "+"
		switch it.Action {
		case "remove":
			sym = "-"
		case "update":
			sym = "↻"
		}
		if m.runSelected != nil {
			check := "[ ]"
			if m.ticked[it.ID] {
				check = "[✓]"
			}
			sym = check + " " + sym
		}
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
		var buildDetails []string
		if it.Network {
			buildDetails = append(buildDetails, "network")
		}
		if it.BuildDownloadGB > 0 {
			buildDetails = append(buildDetails, fmt.Sprintf("~%.1f GB source", it.BuildDownloadGB))
		}
		if it.BuildWorkGB > 0 {
			buildDetails = append(buildDetails, fmt.Sprintf("~%.1f GB staging", it.BuildWorkGB))
		}
		if len(it.Requires) > 0 {
			buildDetails = append(buildDetails, "tools: "+strings.Join(it.Requires, ", "))
		}
		if len(buildDetails) > 0 {
			b.WriteString("\n")
			b.WriteString(m.theme.Warning.Render("  Build: " + strings.Join(buildDetails, " · ")))
		}
	}

	// Summary
	b.WriteString("\n\n")
	if m.runSelected != nil {
		b.WriteString(m.selectedSummary())
	} else {
		b.WriteString(m.planSummary())
	}
	if m.buildDownloadGB > 0 || m.buildWorkGB > 0 || m.toolsRequired {
		tools := "no tools image"
		if m.toolsRequired {
			tools = "tools image required"
		}
		b.WriteString("\n")
		b.WriteString(m.theme.Warning.Render(fmt.Sprintf(
			"  Build sources: ~%.1f GB  |  Staging: ~%.1f GB  |  %s",
			m.buildDownloadGB, m.buildWorkGB, tools,
		)))
	}

	// Footer
	enterLabel := tui.KeyBinding{Key: "enter", Label: "Enter: apply"}
	if m.runApply == nil {
		enterLabel.Label = ""
	}
	footer := tui.FooterHints(
		m.keys.MoveUp,
		enterLabel,
		tui.KeyBinding{Key: "b", Label: "b: browse"},
		tui.KeyBinding{Key: "q", Label: "q/Esc: back"},
	)
	if m.runSelected != nil {
		replaceLabel := "r: delete first"
		if m.replace {
			replaceLabel = "r: keep copies"
		}
		footer = tui.FooterHints(
			m.keys.MoveUp,
			tui.KeyBinding{Key: " ", Label: "Space: tick"},
			tui.KeyBinding{Key: "a", Label: "a: all/none"},
			tui.KeyBinding{Key: "r", Label: replaceLabel},
			tui.KeyBinding{Key: "enter", Label: "Enter: update"},
			tui.KeyBinding{Key: "q", Label: "q/Esc: back"},
		)
	}

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

// planSummary gives the plan's download and removal sizes and free space.
func (m Model) planSummary() string {
	var b strings.Builder
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf(
		"  Download: %s  |  Remove: %s",
		tui.FormatSize(m.downloadGB), tui.FormatSize(m.removeGB),
	)))
	if m.freeGB > 0 && m.freeAfterGB < 0 {
		b.WriteString("\n")
		b.WriteString(m.theme.Warning.Render(fmt.Sprintf(
			"  Not enough space: %s free, %s short", tui.FormatSize(m.freeGB), tui.FormatSize(-m.freeAfterGB),
		)))
	} else if m.freeGB > 0 {
		b.WriteString("\n")
		b.WriteString(m.theme.Muted.Render(fmt.Sprintf(
			"  Free: %s  |  After: %s", tui.FormatSize(m.freeGB), tui.FormatSize(m.freeAfterGB),
		)))
	}
	return b.String()
}

// selectedSummary describes the download of the ticked items and whether
// the vault has room to keep each installed copy until its new one arrives.
func (m Model) selectedSummary() string {
	need := m.tickedGB()
	lines := []string{m.theme.Muted.Render("  Download: " + tui.FormatSize(need))}
	switch {
	case m.replace:
		lines = append(lines, m.theme.Warning.Render(
			"  Delete first: a failed update leaves the item to download again"))
	case m.freeGB > 0 && need > m.freeGB:
		lines = append(lines, m.theme.Warning.Render(fmt.Sprintf(
			"  Not enough room to keep the installed copies: needs %s, %s free; untick some or press r",
			tui.FormatSize(need), tui.FormatSize(m.freeGB))))
	case m.freeGB > 0:
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf(
			"  Free: %s  |  Installed copies stay until each new one is in place",
			tui.FormatSize(m.freeGB))))
	default:
		lines = append(lines, m.theme.Muted.Render("  Installed copies stay until each new one is in place"))
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewApply() string {
	pv := m.applyProgressView()

	var b strings.Builder
	b.WriteString(m.theme.Section.Render("Applying changes"))
	b.WriteString("\n\n")
	b.WriteString(pv.Render())
	b.WriteString("\n")
	if m.stopping {
		b.WriteString(m.theme.Warning.Render("Stopping… saving progress"))
		b.WriteString("\n")
	} else {
		b.WriteString(pv.RenderSummary())
	}
	for _, notice := range m.notices {
		b.WriteString(m.theme.Help.Render("Note: " + notice))
		b.WriteString("\n")
	}
	if m.applyDone {
		done, failed := 0, 0
		for _, step := range m.applyItems {
			if step.status == tui.StatusDone {
				done++
			}
			if step.status == tui.StatusFailed {
				failed++
			}
		}
		pending := len(m.applyItems) - done - failed
		if m.applyErr != "" || failed > 0 || pending > 0 {
			b.WriteString(m.theme.Warning.Render(fmt.Sprintf("Completed: %d  Failed: %d  Pending: %d\n%s\nReopen this vault and apply the remaining plan to resume.", done, failed, pending, m.applyErr)))
		}
	}

	var footer string
	if m.applyDone {
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: back"},
			tui.KeyBinding{Key: "esc", Label: "Esc: back"},
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
		footer = fmt.Sprintf("downloading %d/%d ...  |  Esc: stop and keep what is downloaded", done+active, len(pv.Steps))
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
	return tui.ProgressView{
		Theme:      m.theme,
		Steps:      steps,
		MaxVisible: tui.MaxVisibleRows(m.height, 12, 4),
	}
}
