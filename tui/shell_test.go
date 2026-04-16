package tui_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/tui"
)

// stripAnsi removes ANSI escape sequences for width calculations in tests.
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func newTestShell(width int) tui.ShellLayout {
	return tui.ShellLayout{
		Theme:    tui.DefaultTheme(),
		AppName:  "TestApp",
		Identity: "my-vault",
		Status:   "online",
		Left:     "left-content",
		Right:    "right-content",
		Footer:   "q: quit | tab: switch",
		Width:    width,
		Height:   24,
	}
}

func TestShellLayoutWideContainsAllRegions(t *testing.T) {
	s := newTestShell(100)
	out := s.Render()
	plain := stripAnsi(out)

	for _, want := range []string{"TestApp", "my-vault", "left-content", "right-content", "q: quit | tab: switch"} {
		if !strings.Contains(plain, want) {
			t.Errorf("wide layout should contain %q, got:\n%s", want, plain)
		}
	}
}

func TestShellLayoutNarrowContainsAllRegions(t *testing.T) {
	s := newTestShell(60)
	out := s.Render()
	plain := stripAnsi(out)

	for _, want := range []string{"TestApp", "my-vault", "left-content", "right-content", "q: quit | tab: switch"} {
		if !strings.Contains(plain, want) {
			t.Errorf("narrow layout should contain %q, got:\n%s", want, plain)
		}
	}
}

func TestShellLayoutNarrowStacksVertically(t *testing.T) {
	s := newTestShell(60)
	out := s.Render()
	plain := stripAnsi(out)

	leftIdx := strings.Index(plain, "left-content")
	rightIdx := strings.Index(plain, "right-content")

	if leftIdx == -1 || rightIdx == -1 {
		t.Fatalf("expected both left-content and right-content in output:\n%s", plain)
	}

	if leftIdx >= rightIdx {
		t.Errorf("in narrow mode, left-content (idx %d) should appear before right-content (idx %d)", leftIdx, rightIdx)
	}
}

func TestShellLayoutRespectsWidth(t *testing.T) {
	s := newTestShell(80)
	out := s.Render()

	lines := strings.Split(out, "\n")
	for i, line := range lines {
		stripped := stripAnsi(line)
		if len(stripped) > 80 {
			t.Errorf("line %d exceeds width 80 (%d chars): %q", i, len(stripped), stripped)
		}
	}
}

func TestShellLayoutSwitchesAtBreakpoint(t *testing.T) {
	// Width=80 should be wide mode (side-by-side)
	wide := newTestShell(80)
	wideOut := wide.Render()
	widePlain := stripAnsi(wideOut)

	// Width=79 should be narrow mode (stacked)
	narrow := newTestShell(79)
	narrowOut := narrow.Render()
	narrowPlain := stripAnsi(narrowOut)

	// In wide mode, left and right content should appear on the same line
	wideHasSameLine := false
	for _, line := range strings.Split(widePlain, "\n") {
		if strings.Contains(line, "left-content") && strings.Contains(line, "right-content") {
			wideHasSameLine = true
			break
		}
	}
	if !wideHasSameLine {
		t.Errorf("at width=80 (wide mode), left and right content should be on the same line:\n%s", widePlain)
	}

	// In narrow mode, left and right content should NOT appear on the same line
	for _, line := range strings.Split(narrowPlain, "\n") {
		if strings.Contains(line, "left-content") && strings.Contains(line, "right-content") {
			t.Errorf("at width=79 (narrow mode), left and right content should NOT be on the same line:\n%s", narrowPlain)
			break
		}
	}
}
