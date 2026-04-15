package actions_test

import (
	"os"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
)

func TestRegistryRejectsUnknownAction(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	_, err := runner.Resolve("unknown", nil)
	if err == nil {
		t.Fatal("Resolve() error = nil, want unknown action error")
	}
}

func TestCommandUsesDriveRootAndExportsDriveRootEnv(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve("browse", map[string]string{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	cmd := resolved.Cmd

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

func TestInspectUsesCapturedOutputMode(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve("inspect", map[string]string{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Mode, actions.ModeCaptureOutput; got != want {
		t.Fatalf("resolved.Mode = %v, want %v", got, want)
	}
}
