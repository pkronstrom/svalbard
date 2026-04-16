package wizard

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stripAnsi removes ANSI escape sequences for assertions on rendered output.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestWizardShowsAllSteps(t *testing.T) {
	m := New("")
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())

	for _, step := range wizardSteps {
		if !strings.Contains(out, step.label) {
			t.Errorf("View() should contain step label %q, got:\n%s", step.label, out)
		}
	}
}

func TestWizardPrefillsPath(t *testing.T) {
	m := New("/mnt/drive")
	if m.pathValue != "/mnt/drive" {
		t.Errorf("expected pathValue %q, got %q", "/mnt/drive", m.pathValue)
	}
}

func TestWizardStartsAtPathStep(t *testing.T) {
	m := New("")
	if m.currentStep != 0 {
		t.Errorf("expected currentStep 0, got %d", m.currentStep)
	}
}

func TestWizardAdvancesOnEnter(t *testing.T) {
	m := New("")
	m.width = 80
	m.height = 24

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.currentStep != 1 {
		t.Errorf("expected currentStep 1 after Enter, got %d", um.currentStep)
	}
}

func TestWizardGoesBackOnEsc(t *testing.T) {
	m := New("")
	m.width = 80
	m.height = 24
	m.currentStep = 2

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.currentStep != 1 {
		t.Errorf("expected currentStep 1 after Esc from step 2, got %d", um.currentStep)
	}
}

func TestWizardEscAtFirstStepQuits(t *testing.T) {
	m := New("")
	m.width = 80
	m.height = 24
	m.currentStep = 0

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (tea.Quit) when pressing Esc at step 0")
	}

	// Execute the cmd to get the message and check it's a QuitMsg
	quitMsg := cmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", quitMsg)
	}
}
