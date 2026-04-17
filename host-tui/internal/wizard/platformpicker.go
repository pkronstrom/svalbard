package wizard

import (
	"fmt"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// allPlatforms is the ordered list of supported platforms.
var allPlatforms = []string{
	"macos-arm64",
	"macos-x86_64",
	"linux-arm64",
	"linux-x86_64",
}

// platformDoneMsg is sent when the user confirms platform selection.
type platformDoneMsg struct {
	platforms []string
}

// platformPickerModel lets the user select which platforms to include binaries for.
type platformPickerModel struct {
	checked      map[string]bool
	hostPlatform string // detected host, e.g. "macos-arm64"
	cursor       int
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

func newPlatformPicker() platformPickerModel {
	host := detectHostPlatform()
	checked := map[string]bool{host: true}
	return platformPickerModel{
		checked:      checked,
		hostPlatform: host,
		theme:        tui.DefaultTheme(),
		keys:         tui.DefaultKeyMap(),
	}
}

func (m platformPickerModel) Init() tea.Cmd { return nil }

func (m platformPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < len(allPlatforms)-1 {
				m.cursor++
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case m.keys.Toggle.Matches(msg):
			p := allPlatforms[m.cursor]
			m.checked[p] = !m.checked[p]
			return m, nil
		case m.keys.Enter.Matches(msg):
			selected := m.selectedPlatforms()
			if len(selected) == 0 {
				// Must select at least one
				return m, nil
			}
			return m, func() tea.Msg { return platformDoneMsg{platforms: selected} }
		default:
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'h':
					// This host only
					m.checked = map[string]bool{m.hostPlatform: true}
					return m, nil
				case 'a':
					// All platforms
					for _, p := range allPlatforms {
						m.checked[p] = true
					}
					return m, nil
				}
			}
		}
	}
	return m, nil
}

func (m platformPickerModel) View() string {
	var b strings.Builder

	b.WriteString(m.theme.Section.Render("Target Platforms"))
	b.WriteString(fmt.Sprintf("  %d selected\n\n", m.selectedCount()))

	// Shortcut hints
	b.WriteString(m.theme.Muted.Render("  h  This host only"))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  a  All platforms"))
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("  ─────────────────────────────"))
	b.WriteString("\n")

	// Platform checkboxes
	for i, p := range allPlatforms {
		check := "[ ]"
		if m.checked[p] {
			check = "[✓]"
		}

		label := p
		if p == m.hostPlatform {
			label += "  (this host)"
		}

		var line string
		if i == m.cursor {
			line = m.theme.Selected.Render(fmt.Sprintf("  %s %s", check, label))
		} else if m.checked[p] {
			line = m.theme.Base.Render(fmt.Sprintf("  %s %s", check, label))
		} else {
			line = m.theme.Muted.Render(fmt.Sprintf("  %s %s", check, label))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Size estimate
	count := m.selectedCount()
	estimate := float64(count) * 0.5 // rough ~0.5 GB per platform for all tools
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("  Binaries: ~%.1f GB for %d platform(s)", estimate, count)))
	b.WriteString("\n\n")

	b.WriteString(m.theme.Help.Render("space toggle · h this host · a all · enter continue"))

	return b.String()
}

func (m platformPickerModel) selectedPlatforms() []string {
	var selected []string
	for _, p := range allPlatforms {
		if m.checked[p] {
			selected = append(selected, p)
		}
	}
	return selected
}

func (m platformPickerModel) selectedCount() int {
	count := 0
	for _, p := range allPlatforms {
		if m.checked[p] {
			count++
		}
	}
	return count
}

// detectHostPlatform returns the svalbard platform string for the current machine.
func detectHostPlatform() string {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	if osName == "darwin" {
		osName = "macos"
	}
	if archName == "amd64" {
		archName = "x86_64"
	}

	return osName + "-" + archName
}
