package menu

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/folder"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/tui"
)

// workspaceAction is the built-in that opens the drive's workspace folder.
const workspaceAction = "workspace"
const searchPageSize = 20

type actionFinishedMsg struct {
	err error
}

type actionOutputMsg struct {
	output string
	err    error
}

type searchSession interface {
	Info() search.SessionInfo
	Search(ctx context.Context, mode search.Mode, query string, limit int, excluded ...string) (search.SearchResponse, error)
	OpenResult(result search.Result) error
	Close() error
}

type searchResultMsg struct {
	token    int
	query    string
	limit    int
	more     bool
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

	groupSelected    int
	itemSelected     int
	itemScrollOffset int
	activeGroup      string
	inGroup          bool

	width         int
	height        int
	status        string
	lastErr       error
	showingOutput bool
	output        string

	paletteActive bool
	paletteModel  tui.PaletteModel

	settingsActive bool
	settings       settingsScreen

	searchActive       bool
	searchToken        int
	searchSession      searchSession
	searchInfo         search.SessionInfo
	searchMode         search.Mode
	searchQuery        textinput.Model
	searchResults      []search.Result
	searchPageQuery    string
	searchPageLimit    int
	searchHasMore      bool
	searchSelected     int
	searchScrollOffset int
	searchResultsFocus bool
	searchLoading      bool
	searchStatus       string
	searchErr          error
	archiveFilterOpen  bool
	searchArchives     []search.Archive
	excludedArchives   []string
	originalExclusions []string
	archiveCursor      int
}

func NewModel(cfg config.RuntimeConfig, driveRoot string, workDir ...string) Model {
	runner := actions.NewRunner(driveRoot, workDir...)
	theme := tui.DefaultTheme()
	ti := textinput.New()
	ti.Prompt = ""
	ti.TextStyle = theme.Focus
	ti.Cursor.Style = theme.Focus
	ti.Cursor.SetMode(cursor.CursorStatic)
	return Model{
		cfg:         cfg,
		driveRoot:   driveRoot,
		runner:      runner,
		theme:       theme,
		searchQuery: ti,
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
		if m.settingsActive {
			m.scrollSettings()
		} else if m.searchActive {
			m.ensureSearchVisible()
		} else if m.inGroup {
			m.scrollGroup()
		}
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
				m.itemScrollOffset = 0
				return m, nil
			}
			for _, item := range group.Items {
				if item.ID == msg.Entry.ID {
					m.groupSelected = i
					m.inGroup = true
					m.activeGroup = group.ID
					m.itemSelected = 0
					m.itemScrollOffset = 0
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
			if msg.more {
				m.searchStatus = "Could not load more results"
			} else {
				m.searchStatus = "Search failed"
			}
			return m, nil
		}
		m.searchMode = msg.response.EffectiveMode
		m.searchPageQuery = msg.query
		m.searchPageLimit = msg.limit
		m.searchHasMore = len(msg.response.Results) == msg.limit
		if msg.more {
			firstNew := len(m.searchResults)
			m.searchResults = search.AppendUnseenResults(slices.Clone(m.searchResults), msg.response.Results)
			if len(m.searchResults) == firstNew {
				m.searchHasMore = false
				m.searchStatus = "No more results"
				return m, nil
			}
			m.searchSelected = firstNew
			m.ensureSearchVisible()
			m.searchStatus = msg.response.Status
			return m, nil
		}
		m.searchResults = msg.response.Results
		m.searchSelected = 0
		m.searchScrollOffset = 0
		m.searchResultsFocus = len(m.searchResults) > 0 && strings.TrimSpace(m.searchQuery.Value()) == msg.query
		if m.searchResultsFocus {
			m.searchQuery.Blur()
		} else {
			m.searchQuery.Focus()
		}
		switch {
		case msg.response.Status != "":
			m.searchStatus = msg.response.Status
		case len(m.searchResults) == 0:
			m.searchStatus = fmt.Sprintf("No results for %q", msg.query)
		default:
			m.searchStatus = ""
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
		if m.settingsActive {
			return m.updateSettings(msg)
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
				m.itemScrollOffset = 0
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
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.archiveFilterOpen {
		return m.updateArchiveFilter(msg)
	}
	switch msg.String() {
	case "ctrl+f":
		if !m.searchLoading {
			m.openArchiveFilter()
		}
		return m, nil
	case "tab":
		if m.searchInfo.HybridEnabled && !m.searchLoading {
			if m.searchMode == search.ModeHybrid {
				m.searchMode = search.ModeKeyword
			} else {
				m.searchMode = search.ModeHybrid
			}
			m.searchStatus = fmt.Sprintf("Mode: %s", m.searchMode)
			m.searchErr = nil
			return m.refreshSearch()
		}
		return m, nil
	case "m", "M":
		if m.searchResultsFocus {
			if m.searchHasMore && !m.searchLoading {
				return m.startSearch(m.searchPageQuery, m.searchPageLimit+searchPageSize, true)
			}
			return m, nil
		}
	case "up", "k":
		if msg.String() == "k" && !m.searchResultsFocus {
			break
		}
		if m.searchResultsFocus {
			if m.searchSelected > 0 {
				m.searchSelected--
				m.ensureSearchVisible()
			} else {
				m.searchResultsFocus = false
				m.searchQuery.Focus()
			}
		}
		return m, nil
	case "pgup":
		if m.searchResultsFocus {
			m.pageSearchResults(-1)
		}
		return m, nil
	case "down", "j", "pgdown":
		key := msg.String()
		if key == "j" && !m.searchResultsFocus {
			break
		}
		if len(m.searchResults) == 0 {
			return m, nil
		}
		if !m.searchResultsFocus {
			m.searchResultsFocus = true
			m.searchQuery.Blur()
			m.ensureSearchVisible()
			return m, nil
		}
		if m.searchSelected < len(m.searchResults)-1 {
			if key == "pgdown" {
				m.pageSearchResults(1)
			} else {
				m.searchSelected++
				m.ensureSearchVisible()
			}
		} else if m.searchHasMore && !m.searchLoading {
			return m.startSearch(m.searchPageQuery, m.searchPageLimit+searchPageSize, true)
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
		query := strings.TrimSpace(m.searchQuery.Value())
		if query == "" || m.searchLoading || m.searchSession == nil {
			return m, nil
		}
		return m.startSearch(query, searchPageSize, false)
	case "esc":
		if m.searchResultsFocus {
			m.searchResultsFocus = false
			m.searchQuery.Focus()
			return m, nil
		}
		if m.searchQuery.Value() != "" {
			m.searchQuery.SetValue("")
			m.searchResults = nil
			m.searchPageQuery = ""
			m.searchHasMore = false
			m.searchLoading = false
			m.searchToken++
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
			m.searchQuery.Focus()
			return m, nil
		}
	}

	// Delegate remaining keys (typed chars, space, backspace, left/right arrows,
	// ctrl+w, ctrl+u, home/end, delete, etc.) to the textinput when focused on
	// the query field.
	if !m.searchResultsFocus {
		m.searchQuery.Focus()
		var cmd tea.Cmd
		m.searchQuery, cmd = m.searchQuery.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) refreshSearch() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.searchQuery.Value())
	if query == "" || m.searchSession == nil {
		m.searchResults = nil
		m.searchHasMore = false
		m.searchPageQuery = ""
		return m, nil
	}
	return m.startSearch(query, searchPageSize, false)
}

func (m Model) startSearch(query string, limit int, more bool) (tea.Model, tea.Cmd) {
	m.searchToken++
	token := m.searchToken
	mode, session, excluded := m.searchMode, m.searchSession, m.excludedArchives
	m.searchLoading = true
	m.searchErr = nil
	if !more {
		m.searchResults = nil
		m.searchHasMore = false
		m.searchResultsFocus = false
		m.searchSelected, m.searchScrollOffset = 0, 0
		m.searchQuery.Focus()
	}
	switch {
	case more:
		m.searchStatus = "Loading more results..."
	case mode == search.ModeHybrid:
		m.searchStatus = "Starting semantic backend..."
	default:
		m.searchStatus = "Searching..."
	}
	return m, func() tea.Msg {
		response, err := session.Search(context.Background(), mode, query, limit, excluded...)
		return searchResultMsg{token: token, query: query, limit: limit, more: more, response: response, err: err}
	}
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
	m.searchQuery.SetValue("")
	m.searchQuery.Focus()
	m.searchResults = nil
	m.searchHasMore = false
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
	m.archiveFilterOpen = false
	m.searchArchives = nil
	m.excludedArchives = nil
	m.searchToken++
	m.searchQuery.SetValue("")
	m.searchQuery.Blur()
	m.searchResults = nil
	m.searchPageQuery = ""
	m.searchSelected = 0
	m.searchScrollOffset = 0
	m.searchResultsFocus = false
	m.searchLoading = false
	m.searchStatus = ""
	m.searchErr = nil
	m.inGroup = false
	m.activeGroup = ""
}

// searchMaxVisible reserves space for the detail pane, including a three-line
// snippet, so moving the selection keeps the result viewport stable.
func (m Model) searchMaxVisible() int {
	if m.height <= 0 {
		return 3
	}
	reserved := 12 + searchSnippetMaxLines
	return tui.MaxVisibleRows(m.height, reserved, 1)
}

func (m *Model) pageSearchResults(direction int) {
	rows := m.searchMaxVisible()
	step := direction * rows
	m.searchSelected = min(len(m.searchResults)-1, max(0, m.searchSelected+step))
	m.searchScrollOffset = min(max(0, len(m.searchResults)-rows), max(0, m.searchScrollOffset+step))
	m.ensureSearchVisible()
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
	m.searchScrollOffset = min(m.searchScrollOffset, max(0, len(m.searchResults)-maxVis))
}

func (m Model) activateItem(item config.MenuItem) (tea.Model, tea.Cmd) {
	if item.Action.Type == "builtin" {
		if builtin, err := item.Action.DecodeBuiltin(); err == nil && builtin.Name == "search" {
			if err := m.openSearchSession(); err != nil {
				m.lastErr = err
				m.status = "Action failed"
			}
			return m, nil
		} else if err == nil && builtin.Name == settingsAction {
			m.openSettings()
			return m, nil
		} else if err == nil && builtin.Name == workspaceAction {
			// The action would say this and end at once, and the menu would
			// cover the line as it came back.
			if line, missing := folder.WorkspaceMissing(m.driveRoot); missing {
				m.lastErr = nil
				m.status = line
				return m, nil
			}
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

func (m *Model) SetSelected(index int) {
	if m.inGroup {
		m.itemSelected = index
	} else {
		m.groupSelected = index
	}
	m.clampSelection()
	if m.inGroup {
		m.scrollGroup()
	}
}

func (m Model) SelectedIndex() int {
	if m.inGroup {
		return m.itemSelected
	}
	return m.groupSelected
}

func (m *Model) MoveDown() {
	visibleCount := m.visibleCount()
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
	visibleCount := m.visibleCount()
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

func (m Model) visibleCount() int {
	if m.inGroup {
		group, ok := m.CurrentGroup()
		if !ok {
			return 0
		}
		return len(group.Items)
	}

	count := 0
	for _, group := range m.cfg.Groups {
		if len(group.Items) > 0 {
			count++
		}
	}
	return count
}
