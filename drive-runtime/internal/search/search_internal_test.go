package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSQLitePlacesDatabaseBeforeSQL(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "args.log")
	binPath := filepath.Join(tmp, "sqlite3")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" >\"" + logPath + "\"\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := runSQLite(binPath, "/tmp/test.db", "SELECT 1;"); err != nil {
		t.Fatalf("runSQLite() error = %v", err)
	}

	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(args)), "\n")
	want := []string{"/tmp/test.db", "SELECT 1;"}
	if len(got) != len(want) {
		t.Fatalf("len(args) = %d, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunSQLitePlacesFlagsBeforeDatabase(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "args.log")
	binPath := filepath.Join(tmp, "sqlite3")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" >\"" + logPath + "\"\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := runSQLite(binPath, "/tmp/test.db", "-separator", "\t", "SELECT 1;"); err != nil {
		t.Fatalf("runSQLite() error = %v", err)
	}

	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(args)), "\n")
	want := []string{"-separator", "\t", "/tmp/test.db", "SELECT 1;"}
	if len(got) != len(want) {
		t.Fatalf("len(args) = %d, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
