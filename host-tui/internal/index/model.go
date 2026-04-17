package index

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

// IndexStatus describes the state of search indexes.
type IndexStatus struct {
	KeywordEnabled   bool
	KeywordSources   int64
	KeywordArticles  int64
	KeywordLastBuilt string
	SemanticEnabled  bool
	SemanticStatus   string
}

// IndexEvent reports progress during index rebuild.
type IndexEvent struct {
	File   string
	Status string // "indexing", "skip", "done", "failed"
}

// Config holds everything the index screen needs from its parent.
type Config struct {
	Status   IndexStatus
	RunIndex func(ctx context.Context, indexType string, onProgress func(IndexEvent)) error
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// BackMsg signals the parent to navigate back.
type BackMsg struct{}

// Internal messages.
type rebuildStartedMsg struct{ ch <-chan IndexEvent }
type rebuildTickMsg struct {
	event IndexEvent
	done  bool
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type indexStep struct {
	file   string
	status string
}

// Model is the bubbletea model for the Index management screen.
type Model struct {
	status   IndexStatus
	runIndex func(ctx context.Context, indexType string, onProgress func(IndexEvent)) error
	selected int // 0=keyword, 1=semantic

	// Rebuild sub-state
	rebuilding  bool
	rebuildType string
	steps       []indexStep
	rebuildCh   <-chan IndexEvent
	rebuildDone bool

	width, height int
	theme         tui.Theme
	keys          tui.KeyMap
}

// New creates a Model from the given Config.
func New(cfg Config) Model {
	return Model{
		status:   cfg.Status,
		runIndex: cfg.RunIndex,
		theme:    tui.DefaultTheme(),
		keys:     tui.DefaultKeyMap(),
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

	case rebuildStartedMsg:
		m.rebuildCh = msg.ch
		return m, waitForRebuildEvent(m.rebuildCh)

	case rebuildTickMsg:
		if msg.done {
			m.rebuildDone = true
			return m, nil
		}
		m.updateStep(msg.event)
		return m, waitForRebuildEvent(m.rebuildCh)

	case tea.KeyMsg:
		if m.rebuilding {
			return m.updateRebuilding(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.MoveDown.Matches(msg):
		if m.selected < 1 {
			m.selected = 1
		}
		return m, nil

	case m.keys.MoveUp.Matches(msg):
		if m.selected > 0 {
			m.selected = 0
		}
		return m, nil

	case m.keys.Enter.Matches(msg):
		if m.runIndex == nil {
			return m, nil
		}
		indexType := "keyword"
		if m.selected == 1 {
			indexType = "semantic"
		}
		m.rebuilding = true
		m.rebuildType = indexType
		m.rebuildDone = false
		m.steps = nil
		return m, startRebuild(m.runIndex, indexType)

	case m.keys.Back.Matches(msg), m.keys.Quit.Matches(msg):
		return m, func() tea.Msg { return BackMsg{} }
	}

	return m, nil
}

func (m Model) updateRebuilding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.ForceQuit.Matches(msg):
		return m, tea.Quit

	case m.keys.Enter.Matches(msg):
		if m.rebuildDone {
			m.rebuilding = false
			return m, nil
		}
		return m, nil

	case m.keys.Back.Matches(msg):
		if m.rebuildDone {
			m.rebuilding = false
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) updateStep(ev IndexEvent) {
	// Update existing step or append new one.
	for i := range m.steps {
		if m.steps[i].file == ev.File {
			m.steps[i].status = ev.Status
			return
		}
	}
	m.steps = append(m.steps, indexStep{
		file:   ev.File,
		status: ev.Status,
	})
}

// ---------------------------------------------------------------------------
// Async helpers
// ---------------------------------------------------------------------------

func startRebuild(runIndex func(ctx context.Context, indexType string, onProgress func(IndexEvent)) error, indexType string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan IndexEvent, 16)
		go func() {
			defer close(ch)
			if err := runIndex(context.Background(), indexType, func(ev IndexEvent) {
				ch <- ev
			}); err != nil {
				ch <- IndexEvent{Status: "failed", File: err.Error()}
			}
		}()
		return rebuildStartedMsg{ch: ch}
	}
}

func waitForRebuildEvent(ch <-chan IndexEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return rebuildTickMsg{event: ev, done: !ok}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the index management screen.
func (m Model) View() string {
	if m.rebuilding {
		return m.viewRebuilding()
	}
	return m.viewNormal()
}

func (m Model) viewNormal() string {
	// Left pane: navigation list.
	navItems := []tui.NavItem{
		{ID: "keyword", Label: "Keyword search", Description: "SQLite FTS5 full-text"},
		{ID: "semantic", Label: "Semantic search", Description: "embedding-based similarity"},
	}
	nav := tui.NavList{
		Items:    navItems,
		Selected: m.selected,
		Theme:    m.theme,
	}

	// Right pane: detail for selected index.
	detail := m.detailForSelected()

	// Footer.
	enter := m.keys.Enter
	enter.Label = "Enter: rebuild"
	footer := tui.FooterHints(
		m.keys.MoveUp,
		enter,
		m.keys.Back,
	)

	shell := tui.ShellLayout{
		Theme:        m.theme,
		AppName:      "Svalbard",
		Status:       "index",
		Left:         nav.Render(),
		Right:        detail.Render(),
		CompactRight: detail.Title,
		Footer:       m.theme.Help.Render(footer),
		Width:        m.width,
		Height:       m.height,
	}
	return shell.Render()
}

func (m Model) detailForSelected() tui.DetailPane {
	if m.selected == 0 {
		return m.keywordDetail()
	}
	return m.semanticDetail()
}

func (m Model) keywordDetail() tui.DetailPane {
	enabledStr := "yes"
	if !m.status.KeywordEnabled {
		enabledStr = "no"
	}

	lastBuilt := m.status.KeywordLastBuilt
	if lastBuilt == "" {
		lastBuilt = "never"
	}

	return tui.DetailPane{
		Theme: m.theme,
		Title: "Keyword search",
		Fields: []tui.DetailField{
			{Label: "Engine", Value: "SQLite FTS5"},
			{Label: "Enabled", Value: enabledStr},
			{Label: "Sources", Value: fmt.Sprintf("%d", m.status.KeywordSources)},
			{Label: "Articles", Value: fmt.Sprintf("%d", m.status.KeywordArticles)},
			{Label: "Last built", Value: lastBuilt},
		},
		Body: "Fast exact matching. Searches article titles and\nbody text using full-text indexing.",
	}
}

func (m Model) semanticDetail() tui.DetailPane {
	enabledStr := "yes"
	if !m.status.SemanticEnabled {
		enabledStr = "no"
	}

	statusStr := m.status.SemanticStatus
	if statusStr == "" {
		statusStr = "ready"
	}

	return tui.DetailPane{
		Theme: m.theme,
		Title: "Semantic search",
		Fields: []tui.DetailField{
			{Label: "Engine", Value: "nomic-embed-text-v1.5"},
			{Label: "Enabled", Value: enabledStr},
			{Label: "Status", Value: statusStr},
		},
		Body: "Embedding-based similarity search. Finds conceptually\nrelated content even without exact keyword matches.",
	}
}

func (m Model) viewRebuilding() string {
	var b strings.Builder

	title := "Rebuilding keyword index"
	if m.rebuildType == "semantic" {
		title = "Rebuilding semantic index"
	}
	b.WriteString(m.theme.Section.Render(title))
	b.WriteString("\n\n")

	// Show recent steps (keep list manageable).
	maxVisible := m.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}
	start := 0
	if len(m.steps) > maxVisible {
		start = len(m.steps) - maxVisible
	}

	for i := start; i < len(m.steps); i++ {
		step := m.steps[i]
		var symbol, label string
		switch step.status {
		case "done":
			symbol = m.theme.Success.Render("✓")
			label = m.theme.Base.Render(step.file)
		case "indexing":
			symbol = m.theme.Warning.Render("·")
			label = m.theme.Base.Render(step.file)
		case "skip":
			symbol = m.theme.Muted.Render("–")
			label = m.theme.Muted.Render(step.file)
		case "failed":
			symbol = m.theme.Danger.Render("✗")
			label = m.theme.Danger.Render(step.file)
		default:
			symbol = m.theme.Muted.Render(" ")
			label = m.theme.Muted.Render(step.file)
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", symbol, label))
	}

	// Summary line.
	b.WriteString("\n")
	doneCount := 0
	failedCount := 0
	for _, s := range m.steps {
		switch s.status {
		case "done":
			doneCount++
		case "failed":
			failedCount++
		}
	}
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("%d files indexed", doneCount)))
	if failedCount > 0 {
		b.WriteString("  " + m.theme.Danger.Render(fmt.Sprintf("%d failed", failedCount)))
	}

	// Footer.
	var footer string
	if m.rebuildDone {
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: done"},
			m.keys.Back,
		)
	} else {
		footer = m.theme.Status.Render("rebuilding...")
	}

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  m.rebuildType + " index",
		Left:    b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}
