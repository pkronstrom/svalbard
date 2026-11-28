package menu

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/tui"
)

func (m *Model) openArchiveFilter() {
	archives, err := search.ListArchives(m.driveRoot)
	if err != nil {
		m.searchStatus, m.searchErr = "Could not list archives", err
		return
	}
	m.searchArchives = archives
	m.archiveCursor = 0
	m.archiveFilterOpen = true
}

func (m Model) updateArchiveFilter(msg tea.KeyMsg) Model {
	rows := len(m.searchArchives)
	switch msg.String() {
	case "esc", "enter", "ctrl+f":
		m.archiveFilterOpen = false
	case "up", "k":
		m.archiveCursor = max(0, m.archiveCursor-1)
	case "down", "j":
		m.archiveCursor = max(0, min(rows-1, m.archiveCursor+1))
	case "pgup":
		m.archiveCursor = max(0, m.archiveCursor-m.archiveVisibleRows())
	case "pgdown":
		m.archiveCursor = max(0, min(rows-1, m.archiveCursor+m.archiveVisibleRows()))
	case " ", "space":
		if rows > 0 {
			filename := m.searchArchives[m.archiveCursor].Filename
			excluded := slices.Clone(m.excludedArchives)
			if index := slices.Index(excluded, filename); index >= 0 {
				excluded = slices.Delete(excluded, index, index+1)
			} else {
				excluded = append(excluded, filename)
			}
			m.applyArchiveFilter(excluded)
		}
	case "w":
		m.applyArchiveFilter(search.ToggleWikiArchives(m.excludedArchives, m.searchArchives))
	}
	return m
}

func (m *Model) applyArchiveFilter(excluded []string) {
	if slices.Equal(m.excludedArchives, excluded) {
		return
	}
	m.excludedArchives = excluded
	m.searchResults = nil
	m.searchHasMore = false
	m.searchResultsFocus = false
	m.searchSelected, m.searchScrollOffset = 0, 0
	m.searchStatus, m.searchErr = "", nil
	m.searchQuery.Focus()
}

func (m Model) archiveVisibleRows() int {
	return tui.MaxVisibleRows(m.height, 5, 1)
}

func (m Model) renderArchiveFilter() string {
	var b strings.Builder
	total := len(m.searchArchives)
	fmt.Fprintln(&b, m.theme.Title.Render("Search archives"))
	fmt.Fprintln(&b, m.theme.Muted.Render(fmt.Sprintf("%d of %d included • Standalone documents stay searchable", total-len(m.excludedArchives), total)))
	visible := m.archiveVisibleRows()
	start := max(0, min(m.archiveCursor-visible/2, total-visible))
	end := min(start+visible, total)
	for i := start; i < end; i++ {
		archive := m.searchArchives[i]
		check := "[x]"
		if slices.Contains(m.excludedArchives, archive.Filename) {
			check = "[ ]"
		}
		size := tui.FormatBytes(archive.Size)
		name := strings.TrimSuffix(archive.Filename, ".zim")
		if width := m.width - len(size) - 12; width > 1 {
			if runes := []rune(name); len(runes) > width {
				name = string(runes[:width-1]) + "…"
			}
		}
		line := fmt.Sprintf("%s %s · %s", check, name, size)
		if i == m.archiveCursor {
			fmt.Fprintln(&b, "> "+m.theme.Selected.Render(line))
		} else {
			fmt.Fprintln(&b, "  "+line)
		}
	}
	if total > 0 {
		fmt.Fprintln(&b, m.theme.Muted.Render(fmt.Sprintf("Showing %d–%d of %d", start+1, end, total)))
	}
	fmt.Fprintln(&b, m.theme.Help.Render("Space toggle • W toggle Wikipedia and Wiktionary • Enter back"))
	return b.String()
}
