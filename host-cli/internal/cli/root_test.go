package cli

import "testing"

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
