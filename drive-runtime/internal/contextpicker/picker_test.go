package contextpicker

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{}
}

func TestPickerDefaultsToAutoOnEnter(t *testing.T) {
	m, _ := newModel(32768).Update(key("enter"))
	if got := m.(model).selected; got != 32768 {
		t.Fatalf("enter on default selected = %d, want auto 32768", got)
	}
}

func TestPickerDownThenEnterPicksFast(t *testing.T) {
	var m tea.Model = newModel(32768)
	m, _ = m.Update(key("down")) // Auto -> Fast
	m, _ = m.Update(key("enter"))
	if got := m.(model).selected; got != llamaserve.CtxFast {
		t.Fatalf("down+enter selected = %d, want CtxFast %d", got, llamaserve.CtxFast)
	}
}

func TestPickerEscKeepsAuto(t *testing.T) {
	var m tea.Model = newModel(65536)
	m, _ = m.Update(key("down")) // move off Auto
	m, _ = m.Update(key("esc"))  // but esc keeps Auto
	if got := m.(model).selected; got != 65536 {
		t.Fatalf("esc selected = %d, want auto 65536", got)
	}
}

func TestFmtTokens(t *testing.T) {
	cases := map[int]string{8192: "8K", 32768: "32K", 65536: "64K", 131072: "128K", 1000: "1000"}
	for in, want := range cases {
		if got := fmtTokens(in); got != want {
			t.Errorf("fmtTokens(%d) = %q, want %q", in, got, want)
		}
	}
}
