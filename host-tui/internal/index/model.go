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
	EmbeddingModel   string // model ID last used to embed, empty if never
	EmbeddingDims    int    // effective dims of stored embeddings, 0 if unknown
}

// IndexEvent reports progress during index rebuild.
type IndexEvent struct {
	File   string
	Status string // tui.Status* constants
	Detail string // optional detail text
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
	detail string
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
	globalStep  string
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
		indexType := "full"
		if m.selected == 1 {
			indexType = "keyword"
		}
		m.rebuilding = true
		m.rebuildType = indexType
		m.rebuildDone = false
		m.steps = nil
		m.globalStep = ""
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

	case m.keys.Back.Matches(msg), m.keys.Quit.Matches(msg):
		if m.rebuildDone {
			m.rebuilding = false
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) updateStep(ev IndexEvent) {
	if ev.File == "" {
		if ev.detailOrStatus() != "" {
			m.globalStep = ev.detailOrStatus()
		}
		return
	}

	// Update existing step or append new one.
	for i := range m.steps {
		if m.steps[i].file == ev.File {
			m.steps[i].status = ev.Status
			m.steps[i].detail = ev.Detail
			return
		}
	}
	m.steps = append(m.steps, indexStep{
		file:   ev.File,
		status: ev.Status,
		detail: ev.Detail,
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
				ch <- IndexEvent{Status: tui.StatusFailed, File: err.Error()}
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
		{ID: "full", Label: "Full index"},
		{ID: "keyword", Label: "Keyword only"},
	}
	nav := tui.NavList{
		Items:    navItems,
		Selected: m.selected,
		Theme:    m.theme,
	}

	// Right pane: detail for selected index.
	detail := m.detailForSelected()

	// Footer.
	footer := tui.FooterHints(
		m.keys.MoveUp,
		tui.KeyBinding{Key: "enter", Label: "Enter: rebuild"},
		tui.KeyBinding{Key: "q", Label: "q/Esc: back"},
	)

	shell := tui.ShellLayout{
		Theme:        m.theme,
		AppName:      "Svalbard",
		Status:       "index",
		Left:   nav.Render(),
		Right:  detail.Render(),
		Footer: m.theme.Help.Render(footer),
		Width:        m.width,
		Height:       m.height,
	}
	return shell.Render()
}

func (m Model) detailForSelected() tui.DetailPane {
	if m.selected == 0 {
		return m.fullDetail()
	}
	return m.keywordDetail()
}

func (m Model) fullDetail() tui.DetailPane {
	lastBuilt := m.status.KeywordLastBuilt
	if lastBuilt == "" {
		lastBuilt = "never"
	}

	semanticNote := "ready"
	if !m.status.SemanticEnabled {
		semanticNote = m.status.SemanticStatus
		if semanticNote == "" {
			semanticNote = "unavailable"
		}
		semanticNote += " — will run keyword only"
	}

	modelNote := "not embedded yet"
	if m.status.EmbeddingModel != "" {
		if m.status.EmbeddingDims > 0 {
			modelNote = fmt.Sprintf("%s (%d dims)", m.status.EmbeddingModel, m.status.EmbeddingDims)
		} else {
			modelNote = m.status.EmbeddingModel
		}
	}

	return tui.DetailPane{
		Theme: m.theme,
		Title: "Full index",
		Fields: []tui.DetailField{
			{Label: "Includes", Value: "Keyword + Semantic search"},
			{Label: "Speed", Value: "Slower — semantic embedding takes minutes"},
			{Label: "Sources", Value: fmt.Sprintf("%d", m.status.KeywordSources)},
			{Label: "Articles", Value: fmt.Sprintf("%d", m.status.KeywordArticles)},
			{Label: "Semantic", Value: semanticNote},
			{Label: "Model", Value: modelNote},
			{Label: "Last built", Value: lastBuilt},
		},
		Body: "Complete search indexing. Builds keyword FTS5 index for\nexact matches, then generates vector embeddings for\nsimilarity search.",
	}
}

func (m Model) keywordDetail() tui.DetailPane {
	lastBuilt := m.status.KeywordLastBuilt
	if lastBuilt == "" {
		lastBuilt = "never"
	}

	return tui.DetailPane{
		Theme: m.theme,
		Title: "Keyword only",
		Fields: []tui.DetailField{
			{Label: "Engine", Value: "SQLite FTS5"},
			{Label: "Speed", Value: "Fast — seconds"},
			{Label: "Sources", Value: fmt.Sprintf("%d", m.status.KeywordSources)},
			{Label: "Articles", Value: fmt.Sprintf("%d", m.status.KeywordArticles)},
			{Label: "Last built", Value: lastBuilt},
		},
		Body: "Quick keyword search only. Searches article titles\nand body text using full-text indexing.",
	}
}

func (m Model) viewRebuilding() string {
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

	maxVis := m.height - 8
	if maxVis < 5 {
		maxVis = 5
	}
	pv := tui.ProgressView{
		Theme:        m.theme,
		Steps:        steps,
		MaxVisible:   maxVis,
		ScrollToTail: true,
	}

	title := "Rebuilding keyword index"
	if m.rebuildType == "full" {
		title = "Building full index"
	}

	var b strings.Builder
	b.WriteString(m.theme.Section.Render(title))
	b.WriteString("\n\n")
	if m.globalStep != "" {
		b.WriteString(m.theme.Status.Render(m.globalStep))
		b.WriteString("\n\n")
	}
	b.WriteString(pv.Render())
	b.WriteString("\n")
	b.WriteString(pv.RenderSummary())

	var footer string
	if m.rebuildDone {
		footer = tui.FooterHints(
			tui.KeyBinding{Key: "enter", Label: "Enter: done"},
			tui.KeyBinding{Key: "q", Label: "q/Esc: back"},
		)
	} else {
		footer = m.theme.Status.Render("rebuilding...")
	}

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  m.rebuildType + " index",
		Right:   b.String(),
		Footer:  m.theme.Help.Render(footer),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}

func (ev IndexEvent) detailOrStatus() string {
	if ev.Detail != "" {
		return ev.Detail
	}
	return ev.Status
}
