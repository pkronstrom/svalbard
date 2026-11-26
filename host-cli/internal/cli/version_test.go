package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsTheStampedVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "0.6.0"
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "svalbard 0.6.0" {
		t.Fatalf("--version printed %q, want %q", got, "svalbard 0.6.0")
	}
}
