# Shared TUI Design Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a shared terminal UX language (shell, styles, screens, interaction grammar) across the host `svalbard` CLI and drive `svalbard-drive` runtime, then redesign both home screens into purpose-built dashboards.

**Architecture:** A new shared Go module (`tui/`) provides the design system: shell layout, pane rendering, style roles, screen types, keyboard handling, and the command palette. The drive-runtime imports this module to replace its flat menu with a two-pane dashboard. A new host-side Go binary (`cmd/svalbard-tui/`) uses the same module for vault control. Both apps compose from the same building blocks but have distinct home screen content.

**Adaptive layout:** `ShellLayout` renders two-pane (side-by-side) when terminal width >= 80 columns, and stacked (nav above detail) on narrower screens. Callers always build both panes the same way — one code path for content, one branch point for geometry. The narrow path naturally reproduces the current drive-runtime's single-column menu + selected description footer.

**Tech Stack:** Go 1.25+, Bubble Tea v1.3+, Lip Gloss v1.1+, existing drive-runtime patterns (token-based async, action resolution, search sessions).

**Phases:**
- Phase 1: Shared TUI foundation module (`tui/`)
- Phase 2: Drive dashboard redesign (upgrade existing `drive-runtime/internal/menu/`)
- Phase 3: Host TUI shell (`cmd/svalbard-tui/` + host screens)
- Phase 4: Command palette (shared, integrated into both apps)

**Spec reference:** `docs/superpowers/specs/2026-04-16-svalbard-shared-tui-design.md`

---

## Phase 1: Shared TUI Foundation Module

This phase extracts the shared design system into `tui/`, a new Go module that both drive-runtime and the future host binary will import. No existing behavior changes yet.

### Task 1.1: Create `tui/` Go Module With Style Roles

**Files:**
- Create: `tui/go.mod`
- Create: `tui/go.sum`
- Create: `tui/style.go`
- Test: `tui/style_test.go`

The spec defines a visual language: base, focus, success, warning, danger, muted. The current drive-runtime uses ad-hoc `lipgloss.Color("124")` etc. This task creates the semantic style system.

**Step 1: Write the failing test**

```go
// tui/style_test.go
package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemeHasAllRoles(t *testing.T) {
	theme := DefaultTheme()

	// Every role must produce a non-empty style
	roles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Base", theme.Base},
		{"Focus", theme.Focus},
		{"Success", theme.Success},
		{"Warning", theme.Warning},
		{"Danger", theme.Danger},
		{"Muted", theme.Muted},
		{"Title", theme.Title},
		{"Section", theme.Section},
		{"Selected", theme.Selected},
		{"SelectedRow", theme.SelectedRow},
		{"SelectedMuted", theme.SelectedMuted},
		{"Help", theme.Help},
		{"Error", theme.Error},
		{"Status", theme.Status},
	}

	for _, r := range roles {
		t.Run(r.name, func(t *testing.T) {
			rendered := r.style.Render("test")
			if rendered == "" {
				t.Errorf("%s role rendered empty string", r.name)
			}
		})
	}
}

func TestDefaultThemeMatchesDriveRuntimeColors(t *testing.T) {
	theme := DefaultTheme()

	// Title should use the same dark-red (124) as the current drive-runtime
	got := theme.Title.GetForeground()
	want := lipgloss.Color("124")
	if got != want {
		t.Errorf("Title foreground = %v, want %v", got, want)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestTheme`
Expected: FAIL — `package tui` does not exist

**Step 3: Initialize the module and write minimal implementation**

```
# tui/go.mod
module github.com/pkronstrom/svalbard/tui

go 1.25.6

require github.com/charmbracelet/lipgloss v1.1.0
```

```go
// tui/style.go
package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the semantic visual roles shared across all Svalbard TUI screens.
// Spec reference: Design Decisions §9 "Visual Language".
type Theme struct {
	// Semantic roles
	Base    lipgloss.Style // slate/graphite background tone
	Focus   lipgloss.Style // signature accent for active element
	Success lipgloss.Style // completed/healthy state
	Warning lipgloss.Style // pending changes, drift, review-required
	Danger  lipgloss.Style // failures and destructive review
	Muted   lipgloss.Style // metadata, paths, hints

	// Composite styles (built from roles)
	Title        lipgloss.Style
	Section      lipgloss.Style
	Selected     lipgloss.Style
	SelectedRow  lipgloss.Style
	SelectedMuted lipgloss.Style
	Help         lipgloss.Style
	Error        lipgloss.Style
	Status       lipgloss.Style
}

// DefaultTheme returns the Svalbard default: calm field console with dark-first,
// low-chroma base colors and one primary accent. Preserves the existing
// drive-runtime color values for continuity.
func DefaultTheme() Theme {
	return Theme{
		Base:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Focus:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110")), // cold blue accent
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("108")),            // muted green
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("179")),            // orange-yellow
		Danger:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("131")), // bright red
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),

		Title:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("124")),
		Section:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("180")),
		Selected:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")),
		SelectedRow:   lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("255")),
		SelectedMuted: lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("252")),
		Help:          lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Error:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("131")),
		Status:        lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go mod tidy && go test ./... -v -run TestTheme`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/
git commit -m "feat(tui): add shared style module with semantic theme roles"
```

---

### Task 1.2: Shell Layout Component (Two-Pane + Top Bar + Footer)

**Files:**
- Create: `tui/shell.go`
- Test: `tui/shell_test.go`

The spec mandates a restrained two-pane operator console: left pane (sections/actions), right pane (contextual summary), top bar (identity + status), footer (key hints). This is the core layout primitive.

**Step 1: Write the failing test**

```go
// tui/shell_test.go
package tui

import (
	"strings"
	"testing"
)

func TestShellLayoutContainsAllRegions(t *testing.T) {
	theme := DefaultTheme()
	shell := ShellLayout{
		Theme:    theme,
		AppName:  "Svalbard",
		Identity: "test-vault",
		Status:   "ready",
		Left:     "Overview\nAdd Content\nPlan",
		Right:    "Vault: test-vault\nItems: 42",
		Footer:   "j/k: move | Enter: open | Esc: back",
		Width:    80,
		Height:   24,
	}

	output := shell.Render()

	if !strings.Contains(output, "Svalbard") {
		t.Error("output missing app name")
	}
	if !strings.Contains(output, "test-vault") {
		t.Error("output missing identity")
	}
	if !strings.Contains(output, "Overview") {
		t.Error("output missing left pane content")
	}
	if !strings.Contains(output, "Items: 42") {
		t.Error("output missing right pane content")
	}
	if !strings.Contains(output, "j/k: move") {
		t.Error("output missing footer")
	}
}

func TestShellLayoutRespectsWidth(t *testing.T) {
	theme := DefaultTheme()
	shell := ShellLayout{
		Theme:  theme,
		Left:   "left",
		Right:  "right",
		Footer: "help",
		Width:  60,
		Height: 20,
	}

	output := shell.Render()
	for _, line := range strings.Split(output, "\n") {
		// Allow ANSI escape sequences to exceed raw width
		stripped := stripAnsi(line)
		if len(stripped) > 60 {
			t.Errorf("line exceeds width 60: %d chars: %q", len(stripped), stripped)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestShellLayout`
Expected: FAIL — `ShellLayout` undefined

**Step 3: Write minimal implementation**

```go
// tui/shell.go
package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ShellLayout renders the shared two-pane operator console.
// Spec reference: Design Decisions §3 "Home Screen Model".
type ShellLayout struct {
	Theme    Theme
	AppName  string
	Identity string // vault name, drive identity
	Status   string // status badge text
	Left     string // pre-rendered left pane content
	Right    string // pre-rendered right pane content
	Footer   string // key hint line
	Width    int
	Height   int
}

// LeftFraction is the proportion of width allocated to the left pane.
const LeftFraction = 0.40

func (s ShellLayout) Render() string {
	w := s.Width
	if w < 20 {
		w = 20
	}

	// Top bar
	topBar := s.renderTopBar(w)

	// Pane widths: left gets 40%, right gets the rest, minus a 2-char gutter
	gutter := 2
	leftW := int(float64(w) * LeftFraction)
	rightW := w - leftW - gutter
	if rightW < 10 {
		rightW = 10
	}

	leftPane := lipgloss.NewStyle().Width(leftW).Render(s.Left)
	rightPane := lipgloss.NewStyle().
		Width(rightW).
		Foreground(s.Theme.Muted.GetForeground()).
		Render(s.Right)

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPane,
		strings.Repeat(" ", gutter),
		rightPane,
	)

	footer := s.Theme.Help.Width(w).Render(s.Footer)

	return lipgloss.JoinVertical(lipgloss.Left, topBar, "", body, "", footer)
}

func (s ShellLayout) renderTopBar(w int) string {
	parts := []string{s.Theme.Title.Render(s.AppName)}
	if s.Identity != "" {
		parts = append(parts, s.Theme.Muted.Render(s.Identity))
	}
	if s.Status != "" {
		parts = append(parts, s.Theme.Status.Render(s.Status))
	}
	return lipgloss.NewStyle().Width(w).Render(strings.Join(parts, "  "))
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go test ./... -v -run TestShellLayout`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/shell.go tui/shell_test.go
git commit -m "feat(tui): add two-pane shell layout component"
```

---

### Task 1.3: Navigation List Component

**Files:**
- Create: `tui/nav.go`
- Test: `tui/nav_test.go`

A reusable list with caret selection, vi keys, optional descriptions, and subheader grouping. Replaces the raw rendering in current `view.go`.

**Step 1: Write the failing test**

```go
// tui/nav_test.go
package tui

import (
	"strings"
	"testing"
)

func TestNavListRendersCaretOnSelected(t *testing.T) {
	items := []NavItem{
		{Label: "Overview"},
		{Label: "Add Content"},
		{Label: "Plan"},
	}
	list := NavList{Items: items, Selected: 1, Theme: DefaultTheme()}
	output := list.Render()

	if !strings.Contains(output, "> Add Content") {
		t.Errorf("missing caret on selected item: %q", output)
	}
	if strings.Contains(output, "> Overview") {
		t.Errorf("unexpected caret on non-selected item: %q", output)
	}
}

func TestNavListSubheaderGrouping(t *testing.T) {
	items := []NavItem{
		{Label: "Wikipedia", Subheader: "Archives"},
		{Label: "Wiktionary", Subheader: "Archives"},
		{Label: "Inspect", Subheader: "Tools"},
	}
	list := NavList{Items: items, Selected: 0, Theme: DefaultTheme()}
	output := list.Render()

	if !strings.Contains(output, "Archives") {
		t.Error("missing subheader 'Archives'")
	}
	if !strings.Contains(output, "Tools") {
		t.Error("missing subheader 'Tools'")
	}
	// Subheader should appear only once
	if strings.Count(output, "Archives") != 1 {
		t.Errorf("subheader 'Archives' appears %d times, want 1", strings.Count(output, "Archives"))
	}
}

func TestNavListMoveUpDown(t *testing.T) {
	items := []NavItem{{Label: "A"}, {Label: "B"}, {Label: "C"}}
	list := NavList{Items: items, Selected: 0}

	list.MoveDown()
	if list.Selected != 1 {
		t.Errorf("after MoveDown: Selected = %d, want 1", list.Selected)
	}
	list.MoveDown()
	list.MoveDown() // should clamp
	if list.Selected != 2 {
		t.Errorf("after 3x MoveDown: Selected = %d, want 2", list.Selected)
	}
	list.MoveUp()
	if list.Selected != 1 {
		t.Errorf("after MoveUp: Selected = %d, want 1", list.Selected)
	}
}

func TestNavListDisabledItemRendering(t *testing.T) {
	items := []NavItem{
		{Label: "Browse"},
		{Label: "Maps", Disabled: true},
	}
	list := NavList{Items: items, Selected: 0, Theme: DefaultTheme()}
	output := list.Render()

	if !strings.Contains(output, "Maps") {
		t.Error("disabled item should still render")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestNavList`
Expected: FAIL — `NavItem`, `NavList` undefined

**Step 3: Write minimal implementation**

```go
// tui/nav.go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NavItem represents one row in a navigation list.
type NavItem struct {
	ID        string
	Label     string
	Subheader string // optional — groups items under a section header
	Disabled  bool   // visible but not activatable
}

// NavList renders a selectable list with caret indicator, subheader grouping,
// and disabled-item styling. Spec reference: §6 "Picker / editor" and §8 "Interaction Grammar".
type NavList struct {
	Items    []NavItem
	Selected int
	Theme    Theme
}

const (
	caretSpace = "  "
	caret      = "> "
	subIndent  = "  "
)

func (n *NavList) MoveDown() {
	if n.Selected < len(n.Items)-1 {
		n.Selected++
	}
	n.skipDisabledDown()
}

func (n *NavList) MoveUp() {
	if n.Selected > 0 {
		n.Selected--
	}
	n.skipDisabledUp()
}

func (n *NavList) skipDisabledDown() {
	for n.Selected < len(n.Items)-1 && n.Items[n.Selected].Disabled {
		n.Selected++
	}
}

func (n *NavList) skipDisabledUp() {
	for n.Selected > 0 && n.Items[n.Selected].Disabled {
		n.Selected--
	}
}

func (n *NavList) Clamp() {
	if len(n.Items) == 0 {
		n.Selected = 0
		return
	}
	if n.Selected >= len(n.Items) {
		n.Selected = len(n.Items) - 1
	}
	if n.Selected < 0 {
		n.Selected = 0
	}
}

func (n NavList) SelectedItem() (NavItem, bool) {
	if n.Selected < 0 || n.Selected >= len(n.Items) {
		return NavItem{}, false
	}
	return n.Items[n.Selected], true
}

func (n NavList) Render() string {
	var b strings.Builder
	currentSubheader := ""
	hasSubheaders := false
	for _, item := range n.Items {
		if item.Subheader != "" {
			hasSubheaders = true
			break
		}
	}

	for idx, item := range n.Items {
		// Render subheader if new
		if item.Subheader != "" && item.Subheader != currentSubheader {
			currentSubheader = item.Subheader
			if idx > 0 {
				b.WriteString("\n")
			}
			b.WriteString(n.Theme.Section.Render(item.Subheader))
			b.WriteString("\n")
		}

		selected := idx == n.Selected
		label := n.renderRow(item, selected, hasSubheaders && item.Subheader != "")
		b.WriteString(label)
		b.WriteString("\n")
	}

	return b.String()
}

func (n NavList) renderRow(item NavItem, selected bool, indented bool) string {
	prefix := caretSpace
	if selected {
		prefix = caret
	}

	indent := ""
	if indented {
		indent = subIndent
	}

	text := indent + prefix + item.Label

	switch {
	case item.Disabled:
		return n.Theme.Muted.Render(text)
	case selected:
		return n.Theme.Selected.Render(text)
	default:
		return lipgloss.NewStyle().Render(text)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go test ./... -v -run TestNavList`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/nav.go tui/nav_test.go
git commit -m "feat(tui): add navigation list component with subheaders and disabled items"
```

---

### Task 1.4: Key Map and Interaction Grammar

**Files:**
- Create: `tui/keys.go`
- Test: `tui/keys_test.go`

Centralizes the keyboard bindings from spec §8. Both apps import this instead of scattering key-matching strings.

**Step 1: Write the failing test**

```go
// tui/keys_test.go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyMatchMoveDown(t *testing.T) {
	keys := DefaultKeyMap()
	for _, k := range []string{"j", "down"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "down" {
			msg = tea.KeyMsg{Type: tea.KeyDown}
		}
		if !keys.MoveDown.Matches(msg) {
			t.Errorf("MoveDown should match %q", k)
		}
	}
}

func TestKeyMatchPalette(t *testing.T) {
	keys := DefaultKeyMap()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x0b}} // Ctrl+K
	// Ctrl+K is sent as a special key
	ctrlK := tea.KeyMsg{Type: 0, Runes: nil}
	_ = ctrlK
	// Just verify the binding exists
	if keys.Palette.Key == "" {
		t.Error("Palette key binding is empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestKeyMatch`
Expected: FAIL — `DefaultKeyMap` undefined

**Step 3: Write minimal implementation**

```go
// tui/keys.go
package tui

import tea "github.com/charmbracelet/bubbletea"

// KeyBinding represents a single key binding with a display label.
type KeyBinding struct {
	Key   string // primary key string
	Alt   string // alternative key string (e.g., arrow key)
	Label string // display label for help text
}

// Matches returns true if the given key message matches this binding.
func (kb KeyBinding) Matches(msg tea.KeyMsg) bool {
	s := msg.String()
	return s == kb.Key || (kb.Alt != "" && s == kb.Alt)
}

// KeyMap defines the complete interaction grammar shared across both apps.
// Spec reference: Design Decisions §8 "Interaction Grammar".
type KeyMap struct {
	MoveUp   KeyBinding
	MoveDown KeyBinding
	Enter    KeyBinding
	Back     KeyBinding
	Filter   KeyBinding
	Palette  KeyBinding
	Toggle   KeyBinding
	SwitchPane KeyBinding
	Quit     KeyBinding
	ForceQuit KeyBinding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		MoveUp:     KeyBinding{Key: "k", Alt: "up", Label: "j/k: move"},
		MoveDown:   KeyBinding{Key: "j", Alt: "down", Label: ""},
		Enter:      KeyBinding{Key: "enter", Label: "Enter: open"},
		Back:       KeyBinding{Key: "esc", Label: "Esc: back"},
		Filter:     KeyBinding{Key: "/", Label: "/: filter"},
		Palette:    KeyBinding{Key: "ctrl+k", Label: "Ctrl+K: palette"},
		Toggle:     KeyBinding{Key: " ", Label: "Space: toggle"},
		SwitchPane: KeyBinding{Key: "tab", Label: "Tab: switch pane"},
		Quit:       KeyBinding{Key: "q", Label: "q: quit"},
		ForceQuit:  KeyBinding{Key: "ctrl+c", Label: ""},
	}
}

// FooterHints returns a formatted string of active key hints for the footer.
func FooterHints(bindings ...KeyBinding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.Label != "" {
			parts = append(parts, b.Label)
		}
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " | "
		}
		result += p
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go test ./... -v -run TestKeyMatch`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/keys.go tui/keys_test.go
git commit -m "feat(tui): add shared key map and interaction grammar"
```

---

### Task 1.5: Detail Pane Renderer

**Files:**
- Create: `tui/detail.go`
- Test: `tui/detail_test.go`

The right pane of the dashboard shows contextual summary for the focused item. This is a simple key-value + paragraph renderer used by both apps.

**Step 1: Write the failing test**

```go
// tui/detail_test.go
package tui

import (
	"strings"
	"testing"
)

func TestDetailPaneRendersFields(t *testing.T) {
	theme := DefaultTheme()
	detail := DetailPane{
		Theme: theme,
		Title: "Overview",
		Fields: []DetailField{
			{Label: "Vault", Value: "my-vault"},
			{Label: "Items", Value: "42"},
			{Label: "Path", Value: "/mnt/drive"},
		},
		Body: "No pending changes.",
	}

	output := detail.Render()

	if !strings.Contains(output, "Overview") {
		t.Error("missing title")
	}
	if !strings.Contains(output, "Vault") || !strings.Contains(output, "my-vault") {
		t.Error("missing field")
	}
	if !strings.Contains(output, "No pending changes.") {
		t.Error("missing body")
	}
}

func TestDetailPaneEmptyGraceful(t *testing.T) {
	detail := DetailPane{Theme: DefaultTheme()}
	output := detail.Render()
	if output == "" {
		t.Error("empty detail should still render something")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestDetailPane`
Expected: FAIL — `DetailPane` undefined

**Step 3: Write minimal implementation**

```go
// tui/detail.go
package tui

import "strings"

// DetailField is a label-value pair shown in the right pane.
type DetailField struct {
	Label string
	Value string
}

// DetailPane renders contextual summary for the focused navigation item.
// Used as the right pane content in dashboard and picker screens.
type DetailPane struct {
	Theme  Theme
	Title  string
	Fields []DetailField
	Body   string // optional paragraph below fields
}

func (d DetailPane) Render() string {
	var b strings.Builder

	if d.Title != "" {
		b.WriteString(d.Theme.Section.Render(d.Title))
		b.WriteString("\n")
	}

	if len(d.Fields) > 0 {
		// Find max label width for alignment
		maxLabel := 0
		for _, f := range d.Fields {
			if len(f.Label) > maxLabel {
				maxLabel = len(f.Label)
			}
		}
		for _, f := range d.Fields {
			label := d.Theme.Muted.Render(padRight(f.Label, maxLabel) + "  ")
			value := d.Theme.Base.Render(f.Value)
			b.WriteString(label + value + "\n")
		}
	}

	if d.Body != "" {
		if len(d.Fields) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(d.Theme.Muted.Render(d.Body))
		b.WriteString("\n")
	}

	if b.Len() == 0 {
		b.WriteString(d.Theme.Muted.Render("No details available."))
		b.WriteString("\n")
	}

	return b.String()
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go test ./... -v -run TestDetailPane`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/detail.go tui/detail_test.go
git commit -m "feat(tui): add detail pane renderer for right-side context"
```

---

### Task 1.6: Progress / Run Screen Component

**Files:**
- Create: `tui/progress.go`
- Test: `tui/progress_test.go`

Spec §6: progress screen shows current phase, completed steps, active step, errors, optional logs. Used for host `apply`, imports, indexing, and drive runtime actions.

**Step 1: Write the failing test**

```go
// tui/progress_test.go
package tui

import (
	"strings"
	"testing"
)

func TestProgressViewShowsPhases(t *testing.T) {
	theme := DefaultTheme()
	p := ProgressView{
		Theme: theme,
		Title: "Applying changes",
		Steps: []ProgressStep{
			{Label: "Download archives", Status: StepDone},
			{Label: "Build search index", Status: StepActive},
			{Label: "Generate viewers", Status: StepPending},
		},
	}

	output := p.Render()

	if !strings.Contains(output, "Download archives") {
		t.Error("missing completed step")
	}
	if !strings.Contains(output, "Build search index") {
		t.Error("missing active step")
	}
	if !strings.Contains(output, "Generate viewers") {
		t.Error("missing pending step")
	}
}

func TestProgressViewShowsError(t *testing.T) {
	theme := DefaultTheme()
	p := ProgressView{
		Theme: theme,
		Title: "Apply",
		Steps: []ProgressStep{
			{Label: "Download", Status: StepFailed, Error: "connection timeout"},
		},
	}

	output := p.Render()
	if !strings.Contains(output, "connection timeout") {
		t.Error("missing error message")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestProgressView`
Expected: FAIL — `ProgressView` undefined

**Step 3: Write minimal implementation**

```go
// tui/progress.go
package tui

import "strings"

// StepStatus represents the state of a progress step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepActive
	StepDone
	StepFailed
)

// ProgressStep is one phase in a progress view.
type ProgressStep struct {
	Label  string
	Status StepStatus
	Error  string // only relevant when Status == StepFailed
}

// ProgressView renders a multi-step progress screen.
// Spec reference: §6 "Progress / run".
type ProgressView struct {
	Theme  Theme
	Title  string
	Steps  []ProgressStep
	Log    string // optional expandable log content
}

func (p ProgressView) Render() string {
	var b strings.Builder

	if p.Title != "" {
		b.WriteString(p.Theme.Title.Render(p.Title))
		b.WriteString("\n\n")
	}

	for _, step := range p.Steps {
		icon, style := p.stepPresentation(step)
		line := icon + " " + step.Label
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		if step.Status == StepFailed && step.Error != "" {
			b.WriteString(p.Theme.Error.Render("  " + step.Error))
			b.WriteString("\n")
		}
	}

	if p.Log != "" {
		b.WriteString("\n")
		b.WriteString(p.Theme.Muted.Render(p.Log))
		b.WriteString("\n")
	}

	return b.String()
}

func (p ProgressView) stepPresentation(step ProgressStep) (string, Theme) {
	// Returns icon and a theme (we use theme just for style access)
	_ = p.Theme
	switch step.Status {
	case StepDone:
		return "[done]", p.Theme
	case StepActive:
		return "[....]", p.Theme
	case StepFailed:
		return "[FAIL]", p.Theme
	default:
		return "[    ]", p.Theme
	}
}

// Note: stepPresentation returns Theme which is wrong — fix to return style
```

Wait — `stepPresentation` has the wrong return type. Let me fix that:

```go
// tui/progress.go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StepStatus int

const (
	StepPending StepStatus = iota
	StepActive
	StepDone
	StepFailed
)

type ProgressStep struct {
	Label  string
	Status StepStatus
	Error  string
}

type ProgressView struct {
	Theme Theme
	Title string
	Steps []ProgressStep
	Log   string
}

func (p ProgressView) Render() string {
	var b strings.Builder

	if p.Title != "" {
		b.WriteString(p.Theme.Title.Render(p.Title))
		b.WriteString("\n\n")
	}

	for _, step := range p.Steps {
		icon, style := p.stepPresentation(step)
		line := icon + " " + step.Label
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		if step.Status == StepFailed && step.Error != "" {
			b.WriteString(p.Theme.Error.Render("  " + step.Error))
			b.WriteString("\n")
		}
	}

	if p.Log != "" {
		b.WriteString("\n")
		b.WriteString(p.Theme.Muted.Render(p.Log))
		b.WriteString("\n")
	}

	return b.String()
}

func (p ProgressView) stepPresentation(step ProgressStep) (string, lipgloss.Style) {
	switch step.Status {
	case StepDone:
		return "[done]", p.Theme.Success
	case StepActive:
		return "[....]", p.Theme.Focus
	case StepFailed:
		return "[FAIL]", p.Theme.Danger
	default:
		return "[    ]", p.Theme.Muted
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd tui && go test ./... -v -run TestProgressView`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/progress.go tui/progress_test.go
git commit -m "feat(tui): add progress/run screen component"
```

---

## Phase 2: Drive Dashboard Redesign

Upgrade the existing `drive-runtime/internal/menu/` from a flat two-level menu to the spec's two-pane dashboard model. Import the shared `tui/` module. The existing search flow and action resolution stay intact.

### Task 2.1: Add `tui/` Dependency to `drive-runtime`

**Files:**
- Modify: `drive-runtime/go.mod` (add `require github.com/pkronstrom/svalbard/tui`)

**Step 1: Add the local module dependency**

```bash
cd drive-runtime
# Use replace directive for local development
go mod edit -require github.com/pkronstrom/svalbard/tui@v0.0.0
go mod edit -replace github.com/pkronstrom/svalbard/tui=../tui
go mod tidy
```

**Step 2: Verify the module resolves**

Run: `cd drive-runtime && go build ./...`
Expected: builds without errors

**Step 3: Commit**

```bash
git add drive-runtime/go.mod drive-runtime/go.sum
git commit -m "chore(drive): add tui module dependency"
```

---

### Task 2.2: Replace Hardcoded Styles With Theme

**Files:**
- Modify: `drive-runtime/internal/menu/view.go` (replace `var (...)` block with theme usage)
- Modify: `drive-runtime/internal/menu/model.go` (add `theme tui.Theme` field)
- Modify: `drive-runtime/internal/menu/model_test.go` (update constructor calls if needed)

**Step 1: Write the failing test**

Add to `model_test.go`:

```go
func TestModelUsesSharedTheme(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	view := m.View()
	// The view should still render correctly with the shared theme
	if !strings.Contains(view, "Svalbard") {
		t.Fatal("View() missing title after theme migration")
	}
}
```

**Step 2: Run test to verify it passes (baseline)**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestModelUsesSharedTheme`
Expected: PASS (existing code still works)

**Step 3: Migrate styles**

Replace the `var (...)` block in `view.go` with theme field access. Change `titleStyle.Render(...)` to `m.theme.Title.Render(...)` etc. throughout the file. Add `theme tui.Theme` to the Model struct and initialize it in `NewModel`.

Key mapping of old style vars to theme roles:

| Old var | Theme role |
|---------|-----------|
| `titleStyle` | `theme.Title` |
| `sectionStyle` | `theme.Section` |
| `selectedStyle` | `theme.Selected` |
| `descriptionStyle` | `theme.Muted` (for descriptions) or `theme.Base` |
| `statusStyle` | `theme.Status` |
| `errorStyle` | `theme.Error` |
| `helpStyle` | `theme.Help` |
| `selectedRowStyle` | `theme.SelectedRow` |
| `selectedMutedStyle` | `theme.SelectedMuted` |
| `numberStyle` | `theme.Muted` |
| `selectedNumberStyle` | `theme.Selected` |

This is a mechanical replacement. All rendering functions gain `(m Model)` or accept theme as param.

**Step 4: Run all existing menu tests**

Run: `cd drive-runtime && go test ./internal/menu/ -v`
Expected: all 16 tests PASS (no behavior change)

**Step 5: Commit**

```bash
git add drive-runtime/internal/menu/
git commit -m "refactor(drive): migrate hardcoded styles to shared tui.Theme"
```

---

### Task 2.3: Refactor View to Two-Pane Shell Layout

**Files:**
- Modify: `drive-runtime/internal/menu/view.go` (use `tui.ShellLayout` for top-level rendering)
- Modify: `drive-runtime/internal/menu/model.go` (track pane content per selected destination)

**Step 1: Write the failing test**

```go
func TestDashboardRendersTwoPanes(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.width = 80
	m.height = 24

	view := m.View()

	// Should have the identity/status in top bar
	if !strings.Contains(view, "Svalbard") {
		t.Fatal("missing app name in top bar")
	}
	// Right pane should show context for the selected destination
	if !strings.Contains(view, "Search across") {
		t.Fatal("missing right pane context for selected item")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestDashboardRendersTwoPanes`
Expected: FAIL (current view doesn't use two-pane layout at this width)

**Step 3: Implement two-pane layout**

Replace `renderTopLevelView` to compose a `tui.ShellLayout`:
- Left pane: `tui.NavList` rendered from visible groups (drive home destinations)
- Right pane: `tui.DetailPane` with contextual fields based on selected destination
- Top bar: "Svalbard" + drive identity (preset name) + status
- Footer: key hints from `tui.FooterHints`

The existing group submenu rendering (`renderGroupView`) and search/output overlays remain as-is for now — they layer on top of the shell.

**Step 4: Run all existing tests**

Run: `cd drive-runtime && go test ./internal/menu/ -v`
Expected: most PASS, some view-assertion tests may need updating for new layout. Update assertions to match two-pane structure while preserving same behavioral coverage.

**Step 5: Commit**

```bash
git add drive-runtime/internal/menu/
git commit -m "feat(drive): render home as two-pane dashboard shell"
```

---

### Task 2.4: Drive Section Visibility Rules

**Files:**
- Modify: `drive-runtime/internal/menu/model.go` (add visibility logic)
- Test: add to `drive-runtime/internal/menu/model_test.go`

Spec §5: core sections (Browse, Search, Verify) always visible; capability sections (Maps, Chat, Apps, Share, Embedded) hidden when capability absent.

**Step 1: Write the failing test**

```go
func TestDriveSectionsHideWhenCapabilityAbsent(t *testing.T) {
	cfg := config.RuntimeConfig{
		Version: 2,
		Groups: []config.MenuGroup{
			{ID: "search", Label: "Search", Items: []config.MenuItem{}},
			{ID: "browse", Label: "Browse", Items: []config.MenuItem{}},
			{ID: "maps", Label: "Maps", Items: []config.MenuItem{}},
		},
	}
	m := NewModel(cfg, "/tmp/drive")

	visible := m.VisibleGroups()
	for _, g := range visible {
		if g.ID == "maps" {
			t.Error("maps should be hidden when it has no items")
		}
	}
}

func TestDriveCoreSectionsAlwaysVisible(t *testing.T) {
	cfg := config.RuntimeConfig{
		Version: 2,
		Groups: []config.MenuGroup{
			{ID: "search", Label: "Search", Items: []config.MenuItem{}},
			{ID: "browse", Label: "Browse", Items: []config.MenuItem{}},
		},
	}
	m := NewModel(cfg, "/tmp/drive")

	visible := m.VisibleGroups()
	ids := make(map[string]bool)
	for _, g := range visible {
		ids[g.ID] = true
	}
	if !ids["search"] {
		t.Error("search should always be visible")
	}
	if !ids["browse"] {
		t.Error("browse should always be visible")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestDriveSection`
Expected: FAIL — maps group currently shows regardless

**Step 3: Implement visibility rules**

Update `VisibleGroups()` to filter:
- Core IDs (`search`, `browse`, `verify`): always included
- Capability IDs (`maps`, `chat`, `apps`, `share`, `embedded`): hidden if `.Items` is empty

**Step 4: Run all tests**

Run: `cd drive-runtime && go test ./internal/menu/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add drive-runtime/internal/menu/
git commit -m "feat(drive): implement section visibility rules per spec §5"
```

---

### Task 2.5: Drive Right-Pane Context Content

**Files:**
- Create: `drive-runtime/internal/menu/context.go`
- Test: `drive-runtime/internal/menu/context_test.go`

Each drive home destination shows different right-pane content (spec §5). This task defines what each destination displays.

**Step 1: Write the failing test**

```go
// drive-runtime/internal/menu/context_test.go
package menu

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestContextForBrowse(t *testing.T) {
	group := config.MenuGroup{
		ID:    "browse",
		Label: "Browse",
		Items: []config.MenuItem{
			{ID: "wikipedia", Label: "Wikipedia"},
			{ID: "wiktionary", Label: "Wiktionary"},
		},
	}

	detail := contextForGroup(group)
	if !strings.Contains(detail.Title, "Browse") {
		t.Error("missing title")
	}
	// Should show archive count
	found := false
	for _, f := range detail.Fields {
		if f.Label == "Archives" && f.Value == "2" {
			found = true
		}
	}
	if !found {
		t.Error("missing archive count field")
	}
}

func TestContextForSearch(t *testing.T) {
	group := config.MenuGroup{
		ID:    "search",
		Label: "Search",
		Items: []config.MenuItem{
			{ID: "search-all", Label: "Search all content"},
		},
	}

	detail := contextForGroup(group)
	if detail.Title != "Search" {
		t.Errorf("title = %q, want Search", detail.Title)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestContextFor`
Expected: FAIL — `contextForGroup` undefined

**Step 3: Write implementation**

```go
// drive-runtime/internal/menu/context.go
package menu

import (
	"fmt"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/tui"
)

func contextForGroup(group config.MenuGroup) tui.DetailPane {
	switch group.ID {
	case "browse", "library":
		return tui.DetailPane{
			Title: group.Label,
			Fields: []tui.DetailField{
				{Label: "Archives", Value: fmt.Sprintf("%d", len(group.Items))},
			},
			Body: group.Description,
		}
	case "search":
		return tui.DetailPane{
			Title: group.Label,
			Body:  group.Description,
		}
	case "maps":
		return tui.DetailPane{
			Title: group.Label,
			Fields: []tui.DetailField{
				{Label: "Layers", Value: fmt.Sprintf("%d", len(group.Items))},
			},
			Body: group.Description,
		}
	case "chat":
		return tui.DetailPane{
			Title: group.Label,
			Body:  group.Description,
		}
	case "apps", "tools":
		return tui.DetailPane{
			Title: group.Label,
			Fields: []tui.DetailField{
				{Label: "Tools", Value: fmt.Sprintf("%d", len(group.Items))},
			},
			Body: group.Description,
		}
	case "verify":
		return tui.DetailPane{
			Title: group.Label,
			Body:  group.Description,
		}
	default:
		return tui.DetailPane{
			Title: group.Label,
			Body:  group.Description,
		}
	}
}
```

**Step 4: Run tests**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestContextFor`
Expected: PASS

**Step 5: Commit**

```bash
git add drive-runtime/internal/menu/context.go drive-runtime/internal/menu/context_test.go
git commit -m "feat(drive): add right-pane context content per destination"
```

---

## Phase 3: Host TUI Shell

Build the host-side Go TUI binary. The host currently lives in Python (`src/svalbard/cli.py`). This phase creates a Go TUI that can be invoked as `svalbard` (or initially as `svalbard-tui` during migration), providing the vault dashboard, init wizard, and plan/apply review screens.

### Task 3.1: Create Host TUI Go Module and Entry Point

**Files:**
- Create: `host-tui/go.mod`
- Create: `host-tui/cmd/svalbard-tui/main.go`
- Create: `host-tui/internal/vault/resolve.go`
- Test: `host-tui/internal/vault/resolve_test.go`

**Step 1: Write the failing test**

```go
// host-tui/internal/vault/resolve_test.go
package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsManifestInCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", dir, err)
	}
	if path != dir {
		t.Errorf("Resolve() = %q, want %q", path, dir)
	}
}

func TestResolveFindsManifestInParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "manifest.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := Resolve(child)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", child, err)
	}
	if path != parent {
		t.Errorf("Resolve() = %q, want %q", path, parent)
	}
}

func TestResolveReturnsErrorWhenNoVault(t *testing.T) {
	dir := t.TempDir()

	_, err := Resolve(dir)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for no vault")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-tui && go test ./internal/vault/ -v`
Expected: FAIL — package does not exist

**Step 3: Initialize module and write implementation**

```
# host-tui/go.mod
module github.com/pkronstrom/svalbard/host-tui

go 1.25.6

require (
    github.com/charmbracelet/bubbletea v1.3.10
    github.com/pkronstrom/svalbard/tui v0.0.0
)

replace github.com/pkronstrom/svalbard/tui => ../tui
```

```go
// host-tui/internal/vault/resolve.go
package vault

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoVault is returned when no vault can be found from the given directory.
var ErrNoVault = errors.New("no vault found (no manifest.yaml in directory tree)")

// Resolve walks up from startDir looking for a directory containing manifest.yaml.
// Spec reference: §2 "Default Entry Points" — resolve from cwd or nearest parent.
func Resolve(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "manifest.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoVault
		}
		dir = parent
	}
}
```

```go
// host-tui/cmd/svalbard-tui/main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/host-tui/internal/vault"
	"github.com/pkronstrom/svalbard/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Resolve(cwd)
	if err != nil {
		// No vault found — show welcome screen
		return runWelcome()
	}

	return runDashboard(vaultPath)
}

func runWelcome() error {
	// Placeholder — Phase 3 Task 3.4 implements the full welcome screen
	theme := tui.DefaultTheme()
	fmt.Println(theme.Title.Render("Svalbard"))
	fmt.Println(theme.Muted.Render("No vault found. Run 'svalbard init' to create one."))
	return nil
}

func runDashboard(vaultPath string) error {
	// Placeholder — Phase 3 Task 3.2 implements the full dashboard
	_ = vaultPath
	p := tea.NewProgram(
		newDashboardModel(vaultPath),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
```

**Step 4: Run tests**

Run: `cd host-tui && go mod tidy && go test ./internal/vault/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-tui/
git commit -m "feat(host-tui): add Go module, entry point, and vault resolver"
```

---

### Task 3.2: Host Dashboard Model

**Files:**
- Create: `host-tui/internal/dashboard/model.go`
- Create: `host-tui/internal/dashboard/view.go`
- Create: `host-tui/internal/dashboard/context.go`
- Test: `host-tui/internal/dashboard/model_test.go`

The host home screen (spec §4) shows: Overview, Add Content, Remove Content, Import, Plan, Apply, Presets. Left pane = destinations. Right pane = contextual summary per destination.

**Step 1: Write the failing test**

```go
// host-tui/internal/dashboard/model_test.go
package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDashboardShowsAllDestinations(t *testing.T) {
	m := New("/tmp/vault")
	m.width = 80
	m.height = 24

	view := m.View()

	destinations := []string{
		"Overview", "Add Content", "Remove Content",
		"Import", "Plan", "Apply", "Presets",
	}
	for _, dest := range destinations {
		if !strings.Contains(view, dest) {
			t.Errorf("missing destination %q in view", dest)
		}
	}
}

func TestDashboardNavigateDownUp(t *testing.T) {
	m := New("/tmp/vault")
	if m.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.selected)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := updated.(Model)
	if got.selected != 1 {
		t.Fatalf("after j: selected = %d, want 1", got.selected)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	got = updated.(Model)
	if got.selected != 0 {
		t.Fatalf("after k: selected = %d, want 0", got.selected)
	}
}

func TestDashboardRightPaneChangesWithSelection(t *testing.T) {
	m := New("/tmp/vault")
	m.width = 80
	m.height = 24

	view0 := m.View()
	m.selected = 4 // Plan
	view4 := m.View()

	if view0 == view4 {
		t.Error("right pane should differ between Overview and Plan")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-tui && go test ./internal/dashboard/ -v`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

```go
// host-tui/internal/dashboard/model.go
package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type destination struct {
	id    string
	label string
}

var hostDestinations = []destination{
	{id: "overview", label: "Overview"},
	{id: "add", label: "Add Content"},
	{id: "remove", label: "Remove Content"},
	{id: "import", label: "Import"},
	{id: "plan", label: "Plan"},
	{id: "apply", label: "Apply"},
	{id: "presets", label: "Presets"},
}

type Model struct {
	vaultPath string
	selected  int
	width     int
	height    int
	theme     tui.Theme
	keys      tui.KeyMap
}

func New(vaultPath string) Model {
	return Model{
		vaultPath: vaultPath,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
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
	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Quit.Matches(msg):
			return m, tea.Quit
		case m.keys.MoveDown.Matches(msg):
			if m.selected < len(hostDestinations)-1 {
				m.selected++
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case m.keys.Back.Matches(msg):
			return m, tea.Quit
		case m.keys.Enter.Matches(msg):
			// Phase 3 tasks 3.3+ implement destination activation
			return m, nil
		}
	}
	return m, nil
}
```

```go
// host-tui/internal/dashboard/view.go
package dashboard

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/tui"
)

func (m Model) View() string {
	items := make([]tui.NavItem, len(hostDestinations))
	for i, d := range hostDestinations {
		items[i] = tui.NavItem{ID: d.id, Label: d.label}
	}

	nav := tui.NavList{Items: items, Selected: m.selected, Theme: m.theme}
	detail := contextForDestination(hostDestinations[m.selected], m)
	detail.Theme = m.theme

	keys := m.keys
	shell := tui.ShellLayout{
		Theme:    m.theme,
		AppName:  "Svalbard",
		Identity: filepath.Base(m.vaultPath),
		Left:     nav.Render(),
		Right:    detail.Render(),
		Footer:   tui.FooterHints(keys.MoveUp, keys.Enter, keys.Back, keys.Palette),
		Width:    m.width,
		Height:   m.height,
	}

	return shell.Render()
}
```

```go
// host-tui/internal/dashboard/context.go
package dashboard

import "github.com/pkronstrom/svalbard/tui"

func contextForDestination(dest destination, m Model) tui.DetailPane {
	switch dest.id {
	case "overview":
		return tui.DetailPane{
			Title: "Overview",
			Fields: []tui.DetailField{
				{Label: "Vault", Value: m.vaultPath},
			},
			Body: "Vault summary and status. Select to view details.",
		}
	case "add":
		return tui.DetailPane{
			Title: "Add Content",
			Body:  "Choose content to add to this vault's desired state.",
		}
	case "remove":
		return tui.DetailPane{
			Title: "Remove Content",
			Body:  "Select items to remove from the vault's desired state.",
		}
	case "import":
		return tui.DetailPane{
			Title: "Import",
			Body:  "Import local files, URLs, or YouTube content into the vault.",
		}
	case "plan":
		return tui.DetailPane{
			Title: "Plan",
			Body:  "Review what changes will be made to reconcile desired and actual state.",
		}
	case "apply":
		return tui.DetailPane{
			Title: "Apply",
			Body:  "Execute the reconciliation plan. Downloads, removes, and regenerates as needed.",
		}
	case "presets":
		return tui.DetailPane{
			Title: "Presets",
			Body:  "Browse and apply preset configurations.",
		}
	default:
		return tui.DetailPane{Title: dest.label}
	}
}
```

**Step 4: Run tests**

Run: `cd host-tui && go mod tidy && go test ./internal/dashboard/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-tui/internal/dashboard/
git commit -m "feat(host-tui): add vault dashboard with two-pane layout"
```

---

### Task 3.3: Init Wizard Shell

**Files:**
- Create: `host-tui/internal/wizard/model.go`
- Create: `host-tui/internal/wizard/view.go`
- Test: `host-tui/internal/wizard/model_test.go`

Spec §7: `svalbard init` opens a guided setup flow using the same shell language. Steps: Vault Path, Choose Preset, Adjust Contents, Review Plan, Apply.

**Step 1: Write the failing test**

```go
// host-tui/internal/wizard/model_test.go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardShowsAllSteps(t *testing.T) {
	m := New("")
	m.width = 80
	m.height = 24

	view := m.View()
	steps := []string{"Vault Path", "Choose Preset", "Adjust Contents", "Review Plan", "Apply"}
	for _, s := range steps {
		if !strings.Contains(view, s) {
			t.Errorf("missing step %q", s)
		}
	}
}

func TestWizardPrefillsPath(t *testing.T) {
	m := New("/mnt/drive")
	if m.pathValue != "/mnt/drive" {
		t.Errorf("pathValue = %q, want /mnt/drive", m.pathValue)
	}
}

func TestWizardStartsAtPathStep(t *testing.T) {
	m := New("")
	if m.currentStep != 0 {
		t.Errorf("currentStep = %d, want 0", m.currentStep)
	}
}

func TestWizardAdvancesOnEnter(t *testing.T) {
	m := New("/tmp/vault")
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.currentStep != 1 {
		t.Errorf("after Enter: currentStep = %d, want 1", got.currentStep)
	}
}

func TestWizardGoesBackOnEsc(t *testing.T) {
	m := New("/tmp/vault")
	m.currentStep = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.currentStep != 1 {
		t.Errorf("after Esc: currentStep = %d, want 1", got.currentStep)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-tui && go test ./internal/wizard/ -v`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

```go
// host-tui/internal/wizard/model.go
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type step struct {
	id    string
	label string
}

var wizardSteps = []step{
	{id: "path", label: "Vault Path"},
	{id: "preset", label: "Choose Preset"},
	{id: "adjust", label: "Adjust Contents"},
	{id: "review", label: "Review Plan"},
	{id: "apply", label: "Apply"},
}

type Model struct {
	pathValue   string
	currentStep int
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap
}

func New(prefillPath string) Model {
	return Model{
		pathValue: prefillPath,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Enter.Matches(msg):
			if m.currentStep < len(wizardSteps)-1 {
				m.currentStep++
			}
			return m, nil
		case m.keys.Back.Matches(msg):
			if m.currentStep > 0 {
				m.currentStep--
			} else {
				return m, tea.Quit
			}
			return m, nil
		}
	}
	return m, nil
}
```

```go
// host-tui/internal/wizard/view.go
package wizard

import "github.com/pkronstrom/svalbard/tui"

func (m Model) View() string {
	items := make([]tui.NavItem, len(wizardSteps))
	for i, s := range wizardSteps {
		items[i] = tui.NavItem{ID: s.id, Label: s.label}
	}

	nav := tui.NavList{Items: items, Selected: m.currentStep, Theme: m.theme}
	detail := m.contextForStep()

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard Init",
		Left:    nav.Render(),
		Right:   detail.Render(),
		Footer:  tui.FooterHints(m.keys.Enter, m.keys.Back),
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}

func (m Model) contextForStep() tui.DetailPane {
	s := wizardSteps[m.currentStep]
	switch s.id {
	case "path":
		body := "Choose the directory for your vault."
		if m.pathValue != "" {
			body = "Path: " + m.pathValue + "\nPress Enter to continue."
		}
		return tui.DetailPane{
			Theme: m.theme,
			Title: "Vault Path",
			Body:  body,
		}
	case "preset":
		return tui.DetailPane{
			Theme: m.theme,
			Title: "Choose Preset",
			Body:  "Select a preset to configure the vault's initial content.",
		}
	case "adjust":
		return tui.DetailPane{
			Theme: m.theme,
			Title: "Adjust Contents",
			Body:  "Add or remove items from the preset selection.",
		}
	case "review":
		return tui.DetailPane{
			Theme: m.theme,
			Title: "Review Plan",
			Body:  "Review what will be downloaded and configured.",
		}
	case "apply":
		return tui.DetailPane{
			Theme: m.theme,
			Title: "Apply",
			Body:  "Execute the setup. Downloads content and builds the vault.",
		}
	default:
		return tui.DetailPane{Theme: m.theme, Title: s.label}
	}
}
```

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/
git commit -m "feat(host-tui): add init wizard shell with step navigation"
```

---

### Task 3.4: Welcome Screen (No Vault Found)

**Files:**
- Create: `host-tui/internal/welcome/model.go`
- Test: `host-tui/internal/welcome/model_test.go`

Spec §12: when no vault is found, show a welcome screen with "Init Vault" and "Choose Preset" as primary destinations, using the same shell language.

**Step 1: Write the failing test**

```go
// host-tui/internal/welcome/model_test.go
package welcome

import (
	"strings"
	"testing"
)

func TestWelcomeShowsInitAndPreset(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	view := m.View()

	if !strings.Contains(view, "Init Vault") {
		t.Error("missing Init Vault")
	}
	if !strings.Contains(view, "Choose Preset") {
		t.Error("missing Choose Preset")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-tui && go test ./internal/welcome/ -v`
Expected: FAIL

**Step 3: Write implementation**

```go
// host-tui/internal/welcome/model.go
package welcome

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type Model struct {
	selected int
	width    int
	height   int
	theme    tui.Theme
	keys     tui.KeyMap
}

func New() Model {
	return Model{
		theme: tui.DefaultTheme(),
		keys:  tui.DefaultKeyMap(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case m.keys.ForceQuit.Matches(msg):
			return m, tea.Quit
		case m.keys.Quit.Matches(msg):
			return m, tea.Quit
		case m.keys.MoveDown.Matches(msg):
			if m.selected < 1 {
				m.selected++
			}
			return m, nil
		case m.keys.MoveUp.Matches(msg):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	items := []tui.NavItem{
		{ID: "init", Label: "Init Vault"},
		{ID: "preset", Label: "Choose Preset"},
	}
	nav := tui.NavList{Items: items, Selected: m.selected, Theme: m.theme}

	detail := tui.DetailPane{
		Theme: m.theme,
		Title: "Welcome",
		Body:  "No vault found in the current directory.\nCreate a new vault or browse presets to get started.",
	}

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard",
		Status:  "no vault",
		Left:    nav.Render(),
		Right:   detail.Render(),
		Footer:  tui.FooterHints(m.keys.MoveUp, m.keys.Enter, m.keys.Quit),
		Width:   m.width,
		Height:  m.height,
	}
	return shell.Render()
}
```

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/welcome/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-tui/internal/welcome/
git commit -m "feat(host-tui): add welcome screen for no-vault state"
```

---

### Task 3.5: Wire Entry Point to Dashboard, Welcome, and Init

**Files:**
- Modify: `host-tui/cmd/svalbard-tui/main.go`

Wire up `run()` to use:
- `welcome.Model` when no vault found
- `dashboard.Model` when vault resolved
- Add `init` subcommand routing to `wizard.Model`

**Step 1: Update the entry point**

Replace the placeholder functions in `main.go` with real model instantiation using the packages from tasks 3.2–3.4. Add `os.Args` parsing for `init [PATH]`.

**Step 2: Build and verify**

Run: `cd host-tui && go build ./cmd/svalbard-tui/`
Expected: builds without errors

**Step 3: Manual smoke test**

Run: `cd /tmp && ./path/to/svalbard-tui` — should show welcome screen
Run: `cd /path/to/vault && ./path/to/svalbard-tui` — should show dashboard
Run: `./path/to/svalbard-tui init /tmp/new-vault` — should show wizard

**Step 4: Commit**

```bash
git add host-tui/cmd/svalbard-tui/main.go
git commit -m "feat(host-tui): wire entry point to dashboard, welcome, and init wizard"
```

---

## Phase 4: Command Palette

Add a global command palette (Ctrl+K) to both apps. The palette is a shared component in `tui/` that each app populates with its own index.

### Task 4.1: Palette Component

**Files:**
- Create: `tui/palette.go`
- Test: `tui/palette_test.go`

Spec §11: label-and-alias matching with optional verb prefixes. Milestone 1 is intentionally limited.

**Step 1: Write the failing test**

```go
// tui/palette_test.go
package tui

import (
	"testing"
)

func TestPaletteMatchesLabel(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Overview", ID: "overview"},
		{Label: "Add Content", ID: "add"},
		{Label: "Plan", ID: "plan"},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("plan")
	if len(results) != 1 || results[0].ID != "plan" {
		t.Errorf("Match(plan) = %v, want [plan]", results)
	}
}

func TestPaletteMatchesAlias(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Wikipedia", ID: "wikipedia", Aliases: []string{"wiki"}},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("wiki")
	if len(results) != 1 {
		t.Errorf("Match(wiki) = %v, want 1 result", results)
	}
}

func TestPaletteMatchesFuzzy(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Add Content", ID: "add"},
		{Label: "Remove Content", ID: "remove"},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("add")
	if len(results) != 1 || results[0].ID != "add" {
		t.Errorf("Match(add) = %v, want [add]", results)
	}
}

func TestPaletteMatchesVerbPrefix(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Wikipedia", ID: "wikipedia", Verbs: []string{"browse", "open"}},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("browse wikipedia")
	if len(results) != 1 {
		t.Errorf("Match('browse wikipedia') = %v, want 1 result", results)
	}
}

func TestPaletteImportPrefill(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Import", ID: "import", Verbs: []string{"import"}, AcceptsFreeform: true},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("import /path/to/file.pdf")
	if len(results) != 1 {
		t.Fatalf("Match('import /path/to/file.pdf') = %v, want 1 result", results)
	}
	if results[0].FreeformArg != "/path/to/file.pdf" {
		t.Errorf("FreeformArg = %q, want /path/to/file.pdf", results[0].FreeformArg)
	}
}

func TestPaletteEmptyQuery(t *testing.T) {
	entries := []PaletteEntry{
		{Label: "Overview", ID: "overview"},
		{Label: "Plan", ID: "plan"},
	}
	palette := Palette{Entries: entries}

	results := palette.Match("")
	if len(results) != 2 {
		t.Errorf("Match('') should return all entries, got %d", len(results))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestPalette`
Expected: FAIL — `Palette`, `PaletteEntry` undefined

**Step 3: Write implementation**

```go
// tui/palette.go
package tui

import "strings"

// PaletteEntry is one indexable item in the command palette.
type PaletteEntry struct {
	ID             string
	Label          string
	Aliases        []string
	Verbs          []string // optional verb prefixes (e.g., "browse", "add", "remove")
	AcceptsFreeform bool   // when true, trailing text after verb becomes FreeformArg
}

// PaletteResult is a matched entry with optional parsed freeform argument.
type PaletteResult struct {
	PaletteEntry
	FreeformArg string
}

// Palette provides label-and-alias matching with optional verb prefix support.
// Spec reference: §11 "Command Palette Role".
type Palette struct {
	Entries []PaletteEntry
}

// Match returns entries matching the query. Empty query returns all entries.
func (p Palette) Match(query string) []PaletteResult {
	query = strings.TrimSpace(query)
	if query == "" {
		results := make([]PaletteResult, len(p.Entries))
		for i, e := range p.Entries {
			results[i] = PaletteResult{PaletteEntry: e}
		}
		return results
	}

	q := strings.ToLower(query)
	var results []PaletteResult

	for _, entry := range p.Entries {
		if result, ok := matchEntry(entry, q); ok {
			results = append(results, result)
		}
	}
	return results
}

func matchEntry(entry PaletteEntry, query string) (PaletteResult, bool) {
	label := strings.ToLower(entry.Label)
	id := strings.ToLower(entry.ID)

	// Direct label or ID match
	if strings.Contains(label, query) || strings.Contains(id, query) {
		return PaletteResult{PaletteEntry: entry}, true
	}

	// Alias match
	for _, alias := range entry.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			return PaletteResult{PaletteEntry: entry}, true
		}
	}

	// Verb prefix match: "verb rest" where verb matches entry.Verbs
	parts := strings.SplitN(query, " ", 2)
	if len(parts) == 2 {
		verb := parts[0]
		rest := strings.TrimSpace(parts[1])
		for _, v := range entry.Verbs {
			if strings.ToLower(v) == verb {
				// Check if the rest matches the label/id/alias
				if rest == "" || strings.Contains(label, rest) || strings.Contains(id, rest) {
					return PaletteResult{PaletteEntry: entry}, true
				}
				for _, alias := range entry.Aliases {
					if strings.Contains(strings.ToLower(alias), rest) {
						return PaletteResult{PaletteEntry: entry}, true
					}
				}
				// Freeform argument: entry accepts it and rest is arbitrary text
				if entry.AcceptsFreeform {
					return PaletteResult{PaletteEntry: entry, FreeformArg: rest}, true
				}
			}
		}
	}

	return PaletteResult{}, false
}
```

**Step 4: Run tests**

Run: `cd tui && go test ./... -v -run TestPalette`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/palette.go tui/palette_test.go
git commit -m "feat(tui): add command palette with label, alias, and verb matching"
```

---

### Task 4.2: Palette TUI Model (Overlay)

**Files:**
- Create: `tui/palette_model.go`
- Test: `tui/palette_model_test.go`

The palette is a modal overlay triggered by Ctrl+K. It renders on top of the current screen, accepts typed input, shows filtered results, and dispatches a selection message.

**Step 1: Write the failing test**

```go
// tui/palette_model_test.go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPaletteModelShowsEntries(t *testing.T) {
	entries := []PaletteEntry{
		{ID: "plan", Label: "Plan"},
		{ID: "apply", Label: "Apply"},
	}
	m := NewPaletteModel(entries, DefaultTheme())
	view := m.View()

	if !strings.Contains(view, "Plan") || !strings.Contains(view, "Apply") {
		t.Errorf("View() missing entries: %q", view)
	}
}

func TestPaletteModelFiltersOnType(t *testing.T) {
	entries := []PaletteEntry{
		{ID: "plan", Label: "Plan"},
		{ID: "apply", Label: "Apply"},
	}
	m := NewPaletteModel(entries, DefaultTheme())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	got := updated.(PaletteModel)
	if len(got.filtered) != 1 || got.filtered[0].ID != "plan" {
		t.Errorf("after typing 'p': filtered = %v", got.filtered)
	}
}

func TestPaletteModelEscCloses(t *testing.T) {
	m := NewPaletteModel(nil, DefaultTheme())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("Esc should produce a close command")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd tui && go test ./... -v -run TestPaletteModel`
Expected: FAIL — `PaletteModel`, `NewPaletteModel` undefined

**Step 3: Write implementation**

```go
// tui/palette_model.go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PaletteCloseMsg is sent when the palette is dismissed.
type PaletteCloseMsg struct{}

// PaletteSelectMsg is sent when a palette entry is selected.
type PaletteSelectMsg struct {
	Entry       PaletteEntry
	FreeformArg string
}

// PaletteModel is a modal overlay for the command palette.
type PaletteModel struct {
	palette  Palette
	query    string
	filtered []PaletteResult
	selected int
	theme    Theme
}

func NewPaletteModel(entries []PaletteEntry, theme Theme) PaletteModel {
	p := Palette{Entries: entries}
	return PaletteModel{
		palette:  p,
		filtered: p.Match(""),
		theme:    theme,
	}
}

func (m PaletteModel) Init() tea.Cmd { return nil }

func (m PaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return PaletteCloseMsg{} }
		case "enter":
			if len(m.filtered) > 0 && m.selected < len(m.filtered) {
				result := m.filtered[m.selected]
				return m, func() tea.Msg {
					return PaletteSelectMsg{
						Entry:       result.PaletteEntry,
						FreeformArg: result.FreeformArg,
					}
				}
			}
			return m, nil
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, nil
		case "backspace":
			if len(m.query) > 0 {
				runes := []rune(m.query)
				m.query = string(runes[:len(runes)-1])
				m.filtered = m.palette.Match(m.query)
				m.selected = 0
			}
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.query += msg.String()
				m.filtered = m.palette.Match(m.query)
				m.selected = 0
			}
		}
	}
	return m, nil
}

func (m PaletteModel) View() string {
	var b strings.Builder

	b.WriteString(m.theme.Section.Render("Command Palette"))
	b.WriteString("\n")

	cursor := "█"
	b.WriteString(m.theme.Focus.Render("> " + m.query + cursor))
	b.WriteString("\n\n")

	for i, result := range m.filtered {
		label := result.Label
		if i == m.selected {
			b.WriteString(m.theme.Selected.Render("> " + label))
		} else {
			b.WriteString("  " + label)
		}
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 && m.query != "" {
		b.WriteString(m.theme.Muted.Render("No matches."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("Enter: select | Esc: close | j/k: move"))
	b.WriteString("\n")

	return b.String()
}
```

**Step 4: Run tests**

Run: `cd tui && go test ./... -v -run TestPaletteModel`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/palette_model.go tui/palette_model_test.go
git commit -m "feat(tui): add palette TUI model as modal overlay"
```

---

### Task 4.3: Integrate Palette Into Drive Dashboard

**Files:**
- Modify: `drive-runtime/internal/menu/model.go` (add palette state and Ctrl+K handling)
- Modify: `drive-runtime/internal/menu/view.go` (render palette overlay when active)
- Test: add to `drive-runtime/internal/menu/model_test.go`

**Step 1: Write the failing test**

```go
func TestCtrlKOpensPalette(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x0b}})
	got := updated.(Model)
	if !got.paletteActive {
		t.Fatal("paletteActive = false after Ctrl+K")
	}
}

func TestPaletteEscCloses(t *testing.T) {
	m := NewModel(sampleGroupedConfig(), "/tmp/drive")
	m.paletteActive = true

	// Send PaletteCloseMsg
	updated, _ := m.Update(tui.PaletteCloseMsg{})
	got := updated.(Model)
	if got.paletteActive {
		t.Fatal("paletteActive = true after PaletteCloseMsg")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd drive-runtime && go test ./internal/menu/ -v -run TestCtrlK`
Expected: FAIL — `paletteActive` undefined

**Step 3: Add palette integration**

Add to `Model`:
- `paletteActive bool`
- `paletteModel tui.PaletteModel`

In `Update`:
- On Ctrl+K: set `paletteActive = true`, initialize `paletteModel` with drive entries
- When `paletteActive`: delegate to `paletteModel.Update`
- Handle `PaletteCloseMsg` and `PaletteSelectMsg`

In `View`:
- When `paletteActive`: render palette overlay

Build the palette entry index from `cfg.Groups` — each group becomes an entry, each item becomes an entry with its group as verb context.

**Step 4: Run all tests**

Run: `cd drive-runtime && go test ./internal/menu/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add drive-runtime/internal/menu/
git commit -m "feat(drive): integrate command palette with Ctrl+K"
```

---

### Task 4.4: Integrate Palette Into Host Dashboard

**Files:**
- Modify: `host-tui/internal/dashboard/model.go`
- Modify: `host-tui/internal/dashboard/view.go`
- Test: add to `host-tui/internal/dashboard/model_test.go`

Same pattern as Task 4.3 but for host destinations. Entries: Overview, Add Content, Remove Content, Import, Plan, Apply, Presets. Import entry has `AcceptsFreeform: true`.

**Step 1: Write the failing test**

```go
func TestHostPaletteOpensOnCtrlK(t *testing.T) {
	m := New("/tmp/vault")
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0x0b}})
	got := updated.(Model)
	if !got.paletteActive {
		t.Fatal("paletteActive = false after Ctrl+K")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd host-tui && go test ./internal/dashboard/ -v -run TestHostPalette`
Expected: FAIL

**Step 3: Implement** (mirror Task 4.3 pattern for host destinations)

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/dashboard/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add host-tui/internal/dashboard/
git commit -m "feat(host-tui): integrate command palette with Ctrl+K"
```

---

### Task 4.5: Full Integration Test

**Files:**
- Create: `tui/integration_test.go`

Verify both apps' palette entries don't collide, shared theme renders consistently, and all screen types compose correctly.

**Step 1: Write the test**

```go
// tui/integration_test.go
package tui

import (
	"strings"
	"testing"
)

func TestShellLayoutWithNavAndDetail(t *testing.T) {
	theme := DefaultTheme()
	nav := NavList{
		Items: []NavItem{
			{Label: "Overview"},
			{Label: "Plan"},
		},
		Selected: 0,
		Theme:    theme,
	}
	detail := DetailPane{
		Theme: theme,
		Title: "Overview",
		Fields: []DetailField{{Label: "Vault", Value: "my-vault"}},
	}
	shell := ShellLayout{
		Theme:   theme,
		AppName: "Svalbard",
		Left:    nav.Render(),
		Right:   detail.Render(),
		Footer:  FooterHints(DefaultKeyMap().MoveUp, DefaultKeyMap().Enter),
		Width:   80,
		Height:  24,
	}

	output := shell.Render()
	if !strings.Contains(output, "Svalbard") {
		t.Error("missing app name")
	}
	if !strings.Contains(output, "> Overview") {
		t.Error("missing selected nav item")
	}
	if !strings.Contains(output, "my-vault") {
		t.Error("missing detail field")
	}
}

func TestProgressViewInShell(t *testing.T) {
	theme := DefaultTheme()
	progress := ProgressView{
		Theme: theme,
		Title: "Applying",
		Steps: []ProgressStep{
			{Label: "Download", Status: StepDone},
			{Label: "Index", Status: StepActive},
		},
	}

	output := progress.Render()
	if !strings.Contains(output, "Download") || !strings.Contains(output, "Index") {
		t.Error("missing progress steps")
	}
}
```

**Step 2: Run tests**

Run: `cd tui && go test ./... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add tui/integration_test.go
git commit -m "test(tui): add integration tests for shared component composition"
```

---

## Summary

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| **1: Foundation** | 1.1–1.6 | `tui/` module: Theme, ShellLayout, NavList, KeyMap, DetailPane, ProgressView |
| **2: Drive Redesign** | 2.1–2.5 | Drive-runtime upgraded to two-pane dashboard with section visibility |
| **3: Host TUI** | 3.1–3.5 | New `host-tui/` binary: vault resolver, dashboard, init wizard, welcome |
| **4: Palette** | 4.1–4.5 | Global command palette in both apps with label/alias/verb matching |

**Dependencies between phases:**
- Phase 2 depends on Phase 1 (imports `tui/`)
- Phase 3 depends on Phase 1 (imports `tui/`)
- Phase 4 depends on Phases 1–3 (adds palette to both apps)
- Phases 2 and 3 are independent of each other (can be parallelized)
