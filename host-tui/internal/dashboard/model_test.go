package dashboard

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stripAnsi removes ANSI escape sequences for plain-text assertions.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestDashboardShowsAllDestinations(t *testing.T) {
	m := New("/tmp/test-vault")
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	for _, d := range hostDestinations {
		if !strings.Contains(out, d.label) {
			t.Errorf("View() should contain destination label %q, got:\n%s", d.label, out)
		}
	}
}

func TestDashboardNavigateDownUp(t *testing.T) {
	m := New("/tmp/test-vault")
	m.width = 80
	m.height = 24

	// Start at 0
	if m.selected != 0 {
		t.Fatalf("expected initial selected=0, got %d", m.selected)
	}

	// Move down
	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("after j, expected selected=1, got %d", m.selected)
	}

	// Move down again
	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.selected != 2 {
		t.Errorf("after second j, expected selected=2, got %d", m.selected)
	}

	// Move up
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 1 {
		t.Errorf("after k, expected selected=1, got %d", m.selected)
	}

	// Clamp at top: go back to 0, then try going up again
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("expected selected=0, got %d", m.selected)
	}
	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("should clamp at 0, got %d", m.selected)
	}

	// Clamp at bottom: navigate to last item, then try going down
	for i := 0; i < len(hostDestinations); i++ {
		result, _ = m.Update(downMsg)
		m = result.(Model)
	}
	if m.selected != len(hostDestinations)-1 {
		t.Errorf("should clamp at %d, got %d", len(hostDestinations)-1, m.selected)
	}
}

func TestDashboardRightPaneChangesWithSelection(t *testing.T) {
	m := New("/tmp/test-vault")
	m.width = 80
	m.height = 24

	// View at selected=0 (Status)
	view0 := m.View()

	// Move to selected=2 (Plan)
	m.selected = 2
	view2 := m.View()

	if view0 == view2 {
		t.Errorf("View() should differ between selected=0 and selected=2")
	}

	// Verify the right-pane content reflects the selection
	plain0 := stripAnsi(view0)
	plain2 := stripAnsi(view2)

	if !strings.Contains(plain0, "sync state") {
		t.Errorf("selected=0 should show Status body, got:\n%s", plain0)
	}
	if !strings.Contains(plain2, "reconcile") {
		t.Errorf("selected=2 should show Plan body, got:\n%s", plain2)
	}
}

func TestDashboardNumberKeyJumps(t *testing.T) {
	m := New("/tmp/test-vault")
	m.width = 80
	m.height = 24

	// Press '3' to jump to item index 2 (Plan)
	key3 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	result, _ := m.Update(key3)
	m = result.(Model)
	if m.selected != 2 {
		t.Errorf("after pressing '3', expected selected=2, got %d", m.selected)
	}

	// Press '1' to jump back to item index 0 (Status)
	key1 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	result, _ = m.Update(key1)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("after pressing '1', expected selected=0, got %d", m.selected)
	}

	// Press '9' — out of range, should not change
	key9 := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}}
	result, _ = m.Update(key9)
	m = result.(Model)
	if m.selected != 0 {
		t.Errorf("pressing '9' (out of range) should not change selection, got %d", m.selected)
	}
}

func TestDashboardNewVaultEmitsMsg(t *testing.T) {
	m := New("/tmp/test-vault")
	m.width = 80
	m.height = 24

	// Navigate to New Vault (last item)
	m.selected = len(hostDestinations) - 1

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)
	if cmd == nil {
		t.Fatal("pressing Enter on New Vault should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(NewVaultMsg); !ok {
		t.Errorf("expected NewVaultMsg, got %T", msg)
	}
}
