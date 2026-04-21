package cli

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
)

func TestNewRootCommandHasHardResetCommands(t *testing.T) {
	cmd := NewRootCommand()
	got := map[string]bool{}
	for _, child := range cmd.Commands() {
		got[child.Name()] = true
	}
	for _, name := range []string{"init", "add", "remove", "plan", "apply", "import", "preset", "status", "index"} {
		if !got[name] {
			t.Fatalf("missing command %q", name)
		}
	}
}

func TestMapSemanticIndexEventPreservesGlobalEvents(t *testing.T) {
	ev := mapSemanticIndexEvent(commands.SemanticProgress{
		Status: "starting",
		Detail: "Starting embedding server...",
	}, true)

	if ev.File != "" {
		t.Fatalf("global semantic event file = %q, want empty", ev.File)
	}
	if ev.Detail != "Starting embedding server..." {
		t.Fatalf("global semantic event detail = %q", ev.Detail)
	}
}
