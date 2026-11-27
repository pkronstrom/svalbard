package main

import "testing"

func TestWebSubcommandUsageOnAnExistingVault(t *testing.T) {
	t.Setenv("DRIVE_ROOT", newNativeSearchDrive(t))
	output, err := runSubcommand(t, "web", "--help")
	if err != nil || output != "Usage: ./run web [--no-open]\n" {
		t.Fatalf("web help = %q, %v", output, err)
	}
}
