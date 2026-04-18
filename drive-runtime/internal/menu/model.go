package menu

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/tui"
)

type actionFinishedMsg struct {
	err error
}

type actionOutputMsg struct {
	output string
	err    error
}

type searchSession interface {
	Info() search.SessionInfo
	Search(ctx context.Context, mode search.Mode, query string, limit int) (search.SearchResponse, error)
	OpenResult(result search.Result) error
	Close() error
}

type searchResultMsg struct {
	token    int
	query    string
	response search.SearchResponse
	err      error
}

type searchOpenMsg struct {
	token int
	err   error
}

type Model struct {
	cfg           config.RuntimeConfig
	driveRoot     string
	runner        actions.Runner
	theme         tui.Theme
	searchFactory func(string) (searchSession, error)

	groupSelected int
	itemSelected  int
	activeGroup   string
	inGroup       bool

	width         int
	height        int
	status        string
	lastErr       error
	showingOutput bool
	output        string

	paletteActive bool
	paletteModel  tui.PaletteModel

	searchActive       bool
	searchToken        int
	searchSession      searchSession
	searchInfo         search.SessionInfo
	searchMode         search.Mode
	searchQuery        string
	searchResults      []search.Result
	searchSelected     int
	searchScrollOffset int
	searchResultsFocus bool
	searchLoading      bool
	searchStatus       string
	searchErr          error
}

func NewModel(cfg config.RuntimeConfig, driveRoot string, workDir ...string) Model {
	runner := actions.NewRunner(driveRoot)
	if len(workDir) > 0 && workDir[0] != "" {
		runner = actions.NewRunnerWithWorkDir(driveRoot, workDir[0])
	}
	return Model{
		cfg:       cfg,
		driveRoot: driveRoot,
		runner:    runner,
		theme:     tui.DefaultTheme(),
		searchFactory: func(root string) (searchSession, error) {
			return search.NewSession(root, nil)
		},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tui.PaletteCloseMsg:
		m.paletteActive = false
		return m, nil
	case tui.PaletteSelectMsg:
		m.paletteActive = false
		for i, group := range m.cfg.Groups {
			if group.ID == msg.Entry.ID {
				m.groupSelected = i
				m.inGroup = true
				m.activeGroup = group.ID
				m.itemSelected = 0
				return m, nil
			}
			for _, item := range group.Items {
				if item.ID == msg.Entry.ID {
					m.groupSelected = i
					m.inGroup = true
					m.activeGroup = group.ID
					m.itemSelected = 0
					return m, nil
				}
			}
		}
		return m, nil
	case actionFinishedMsg:
		m.lastErr = msg.err
		if msg.err != nil {
			m.status = "Action failed"
		} else {
			m.status = "Action finished"
		}
		return m, nil
	case actionOutputMsg:
		m.lastErr = msg.err
		if msg.err != nil {
			m.status = "Action failed"
		} else {
			m.status = ""
		}
		m.output = msg.output
		m.showingOutput = true
		return m, nil
	case searchResultMsg:
		if !m.searchActive || msg.token != m.searchToken {
			return m, nil
		}
		m.searchLoading = false
		m.searchErr = msg.err
		if msg.err != nil {
			m.searchStatus = "Search failed"
			return m, nil
		}
		m.searchMode = msg.response.EffectiveMode
		m.searchResults = limitSearchResults(msg.response.Results)
		m.searchSelected = 0
		m.searchScrollOffset = 0
		m.searchResultsFocus = len(m.searchResults) > 0
		switch {
		case msg.response.Status != "":
			m.searchStatus = msg.response.Status
		case len(m.searchResults) == 0:
			m.searchStatus = fmt.Sprintf("No results for %q", msg.query)
		default:
			m.searchStatus = fmt.Sprintf("%d results", len(m.searchResults))
		}
		return m, nil
	case searchOpenMsg:
		if !m.searchActive || msg.token != m.searchToken {
			return m, nil
		}
		m.searchErr = msg.err
		if msg.err != nil {
			m.searchStatus = "Open failed"
			return m, nil
		}
		m.searchStatus = "Opened in browser"
		return m, nil
	case tea.KeyMsg:
		if m.paletteActive {
			updated, cmd := m.paletteModel.Update(msg)
			m.paletteModel = updated.(tui.PaletteModel)
			return m, cmd
		}
		if m.searchActive {
			return m.updateSearch(msg)
		}
		if m.showingOutput {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter", "esc":
				m.showingOutput = false
				m.output = ""
				m.status = ""
				m.lastErr = nil
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+k":
			entries := m.buildPaletteEntries()
			m.paletteModel = tui.NewPaletteModel(entries, m.theme)
			m.paletteActive = true
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.inGroup {
				m.inGroup = false
				m.activeGroup = ""
				m.itemSelected = 0
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			m.MoveUp()
			return m, nil
		case "down", "j":
			m.MoveDown()
			return m, nil
		case "esc":
			if m.inGroup {
				m.inGroup = false
				m.activeGroup = ""
				m.itemSelected = 0
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if !m.inGroup {
				group, ok := m.SelectedGroup()
				if !ok {
					return m, nil
				}
				if group.AutoActivate && len(group.Items) > 0 {
					return m.activateItem(group.Items[0])
				}
				m.inGroup = true
				m.activeGroup = group.ID
				m.itemSelected = 0
				return m, nil
			}

			item, ok := m.SelectedItem()
			if !ok {
				return m, nil
			}
			return m.activateItem(item)
		}
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.searchInfo.HybridEnabled {
			if m.searchMode == search.ModeHybrid {
				m.searchMode = search.ModeKeyword
			} else {
				m.searchMode = search.ModeHybrid
			}
			m.searchStatus = fmt.Sprintf("Mode: %s", m.searchMode)
			m.searchErr = nil
		}
		return m, nil
	case "up", "k":
		if m.searchResultsFocus {
			if m.searchSelected > 0 {
				m.searchSelected--
				m.ensureSearchVisible()
			} else {
				m.searchResultsFocus = false
			}
		}
		return m, nil
	case "down", "j":
		if len(m.searchResults) == 0 {
			return m, nil
		}
		if !m.searchResultsFocus {
			m.searchResultsFocus = true
			m.ensureSearchVisible()
			return m, nil
		}
		if m.searchSelected < len(m.searchResults)-1 {
			m.searchSelected++
			m.ensureSearchVisible()
		}
		return m, nil
	case "enter":
		if m.searchResultsFocus {
			if len(m.searchResults) == 0 {
				return m, nil
			}
			result := m.searchResults[m.searchSelected]
			token := m.searchToken
			m.searchStatus = "Opening result..."
			m.searchErr = nil
			session := m.searchSession
			return m, func() tea.Msg {
				err := session.OpenResult(result)
				return searchOpenMsg{token: token, err: err}
			}
		}
		query := strings.TrimSpace(m.searchQuery)
		if query == "" || m.searchLoading || m.searchSession == nil {
			return m, nil
		}
		token := m.searchToken
		mode := m.searchMode
		session := m.searchSession
		m.searchLoading = true
		m.searchErr = nil
		if mode == search.ModeHybrid {
			m.searchStatus = "Starting semantic backend..."
		} else {
			m.searchStatus = "Searching..."
		}
		return m, func() tea.Msg {
			response, err := session.Search(context.Background(), mode, query, 20)
			return searchResultMsg{token: token, query: query, response: response, err: err}
		}
	case "esc":
		if m.searchResultsFocus {
			m.searchResultsFocus = false
			return m, nil
		}
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchResults = nil
			m.searchSelected = 0
			m.searchScrollOffset = 0
			m.searchStatus = ""
			m.searchErr = nil
			return m, nil
		}
		m.closeSearchSession()
		return m, nil
	case "q":
		if m.searchResultsFocus {
			m.searchResultsFocus = false
			return m, nil
		}
	case "backspace":
		if !m.searchResultsFocus && len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
		}
		return m, nil
	}

	if !m.searchResultsFocus {
		if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
		} else if msg.Type == tea.KeySpace {
			m.searchQuery += " "
		}
	}
	return m, nil
}

func (m *Model) openSearchSession() error {
	m.closeSearchSession()
	session, err := m.searchFactory(m.driveRoot)
	if err != nil {
		return err
	}
	m.searchSession = session
	m.searchInfo = session.Info()
	m.searchMode = m.searchInfo.BestMode
	if m.searchMode == "" {
		m.searchMode = search.ModeKeyword
	}
	m.searchQuery = ""
	m.searchResults = nil
	m.searchSelected = 0
	m.searchScrollOffset = 0
	m.searchResultsFocus = false
	m.searchLoading = false
	m.searchStatus = ""
	m.searchErr = nil
	m.searchActive = true
	m.searchToken++
	m.inGroup = false
	m.activeGroup = ""
	m.itemSelected = 0
	return nil
}

func (m *Model) closeSearchSession() {
	if m.searchSession != nil {
		_ = m.searchSession.Close()
	}
	m.searchSession = nil
	m.searchActive = false
	m.searchToken++
	m.searchQuery = ""
	m.searchResults = nil
	m.searchSelected = 0
	m.searchScrollOffset = 0
	m.searchResultsFocus = false
	m.searchLoading = false
	m.searchStatus = ""
	m.searchErr = nil
	m.inGroup = false
	m.activeGroup = ""
}

// searchMaxVisible returns how many result rows fit on screen.
// The search view uses ~7 lines for header/input/footer chrome plus ~5 for the
// selected-result detail block at the bottom.
func (m Model) searchMaxVisible() int {
	const chrome = 12 // header, status, input, blank, section header, detail block, footer
	avail := m.height - chrome
	if avail < 3 {
		return 3
	}
	return avail
}

// ensureSearchVisible adjusts searchScrollOffset so that searchSelected is in
// the visible window.
func (m *Model) ensureSearchVisible() {
	maxVis := m.searchMaxVisible()
	if m.searchSelected < m.searchScrollOffset {
		m.searchScrollOffset = m.searchSelected
	}
	if m.searchSelected >= m.searchScrollOffset+maxVis {
		m.searchScrollOffset = m.searchSelected - maxVis + 1
	}
}

func limitSearchResults(results []search.Result) []search.Result {
	const maxResults = 20
	if len(results) <= maxResults {
		return append([]search.Result(nil), results...)
	}
	return append([]search.Result(nil), results[:maxResults]...)
}

func (m Model) activateItem(item config.MenuItem) (tea.Model, tea.Cmd) {
	if item.Action.Type == "builtin" {
		if builtin, err := item.Action.DecodeBuiltin(); err == nil && builtin.Name == "search" {
			if err := m.openSearchSession(); err != nil {
				m.lastErr = err
				m.status = "Action failed"
			}
			return m, nil
		}
	}
	resolved, err := m.runner.Resolve(item.Action)
	if err != nil {
		m.lastErr = err
		m.status = "Action failed"
		return m, nil
	}
	if resolved.Mode == actions.ModeCaptureOutput {
		return m, func() tea.Msg {
			err := resolved.Cmd.Run()
			output := ""
			if resolved.Cmd.Stdout != nil {
				if buf, ok := resolved.Cmd.Stdout.(interface{ String() string }); ok {
					output = buf.String()
				}
			}
			if resolved.Cmd.Stderr != nil {
				if buf, ok := resolved.Cmd.Stderr.(interface{ String() string }); ok {
					output += buf.String()
				}
			}
			return actionOutputMsg{output: output, err: err}
		}
	}
	return m, tea.ExecProcess(resolved.Cmd, func(err error) tea.Msg {
		return actionFinishedMsg{err: err}
	})
}

func (m Model) buildPaletteEntries() []tui.PaletteEntry {
	var entries []tui.PaletteEntry
	for _, group := range m.cfg.Groups {
		entries = append(entries, tui.PaletteEntry{
			ID:    group.ID,
			Label: group.Label,
		})
		for _, item := range group.Items {
			entries = append(entries, tui.PaletteEntry{
				ID:      item.ID,
				Label:   item.Label,
				Aliases: item.Aliases,
			})
		}
	}
	return entries
}

func (m Model) View() string {
	return renderView(m)
}

func (m *Model) SetFilter(value string) {
	_ = value
}

func (m *Model) SetSelected(index int) {
	if m.inGroup {
		m.itemSelected = index
	} else {
		m.groupSelected = index
	}
	m.clampSelection()
}

func (m Model) SelectedIndex() int {
	if m.inGroup {
		return m.itemSelected
	}
	return m.groupSelected
}

func (m *Model) MoveDown() {
	visibleCount := len(m.visibleEntries())
	if visibleCount == 0 {
		m.SetSelected(0)
		return
	}
	if m.SelectedIndex() < visibleCount-1 {
		m.SetSelected(m.SelectedIndex() + 1)
	}
}

func (m *Model) MoveUp() {
	if m.SelectedIndex() > 0 {
		m.SetSelected(m.SelectedIndex() - 1)
	}
}

func (m Model) SelectedGroup() (config.MenuGroup, bool) {
	visible := m.VisibleGroups()
	if len(visible) == 0 {
		return config.MenuGroup{}, false
	}
	if m.groupSelected < 0 || m.groupSelected >= len(visible) {
		return config.MenuGroup{}, false
	}
	return visible[m.groupSelected], true
}

func (m Model) SelectedItem() (config.MenuItem, bool) {
	visible := m.VisibleItems()
	if len(visible) == 0 {
		return config.MenuItem{}, false
	}
	if m.itemSelected < 0 || m.itemSelected >= len(visible) {
		return config.MenuItem{}, false
	}
	return visible[m.itemSelected], true
}

func (m *Model) clampSelection() {
	visibleCount := len(m.visibleEntries())
	if visibleCount == 0 {
		if m.inGroup {
			m.itemSelected = 0
		} else {
			m.groupSelected = 0
		}
		return
	}
	if m.inGroup {
		if m.itemSelected >= visibleCount {
			m.itemSelected = visibleCount - 1
		}
		if m.itemSelected < 0 {
			m.itemSelected = 0
		}
		return
	}
	if m.groupSelected >= visibleCount {
		m.groupSelected = visibleCount - 1
	}
	if m.groupSelected < 0 {
		m.groupSelected = 0
	}
}

func (m Model) CurrentGroup() (config.MenuGroup, bool) {
	for _, group := range m.cfg.Groups {
		if group.ID == m.activeGroup {
			return group, true
		}
	}
	return config.MenuGroup{}, false
}

func (m Model) VisibleGroups() []config.MenuGroup {
	result := make([]config.MenuGroup, 0, len(m.cfg.Groups))
	for _, g := range m.cfg.Groups {
		if len(g.Items) == 0 {
			continue
		}
		result = append(result, g)
	}
	return result
}

func (m Model) VisibleItems() []config.MenuItem {
	group, ok := m.CurrentGroup()
	if !ok {
		return []config.MenuItem{}
	}
	return append([]config.MenuItem(nil), group.Items...)
}

func (m Model) visibleEntries() []string {
	if m.inGroup {
		items := m.VisibleItems()
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, item.ID)
		}
		return result
	}

	groups := m.VisibleGroups()
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.ID)
	}
	return result
}
