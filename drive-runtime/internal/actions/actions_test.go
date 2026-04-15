package actions_test

import (
	"os"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
)

func TestRegistryRejectsUnknownAction(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	_, err := runner.Command("unknown", nil)
	if err == nil {
		t.Fatal("Command() error = nil, want unknown action error")
	}
}

func TestCommandUsesDriveRootAndExportsDriveRootEnv(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	cmd, err := runner.Command("browse", map[string]string{})
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	if got, want := cmd.Dir, "/tmp/drive"; got != want {
		t.Fatalf("cmd.Dir = %q, want %q", got, want)
	}

	found := false
	for _, env := range cmd.Env {
		if env == "DRIVE_ROOT=/tmp/drive" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cmd.Env = %v, want DRIVE_ROOT export", cmd.Env)
	}

	if cmd.Stdin != os.Stdin {
		t.Fatal("cmd.Stdin is not inherited from the current process")
	}
	if cmd.Stdout != os.Stdout {
		t.Fatal("cmd.Stdout is not inherited from the current process")
	}
	if cmd.Stderr != os.Stderr {
		t.Fatal("cmd.Stderr is not inherited from the current process")
	}
}
