package weblauncher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOwnedLauncherSurvivesRetryAndLeavesNeighbors(t *testing.T) {
	root := t.TempDir()
	entries, err := Install(root, "web-launcher", "web-launcher", []string{"macos-arm64", "linux-x86_64"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 7 {
		t.Fatalf("unexpected file set: %v", entries)
	}
	// Simulate a lost checkpoint before the last write.
	if err := os.Remove(filepath.Join(root, entries[len(entries)-1].RelativePath)); err != nil {
		t.Fatal(err)
	}
	retry, err := Install(root, "web-launcher", "web-launcher", []string{"macos-arm64", "linux-x86_64"})
	if err != nil || len(retry) != len(entries) {
		t.Fatalf("retry: %v", err)
	}
	for _, name := range []string{"index.html", "Svalbard Files.html", "Svalbard.app/Contents/Resources/notes.html"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("owner's page"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckUpdate(root, entries); err == nil {
		t.Fatal("update should preserve modified HTML")
	}
	for _, entry := range entries {
		if err := Remove(root, entry); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"index.html", "Svalbard Files.html", "Svalbard.app/Contents/Resources/notes.html"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != "owner's page" {
			t.Fatalf("owner lost %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, atlasPath)); !os.IsNotExist(err) {
		t.Fatalf("atlas remains: %v", err)
	}
}
func TestCollisionAndSymlinkAreRefusedBeforeInstalling(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		root := t.TempDir()
		if symlink {
			if err := os.Symlink(t.TempDir(), filepath.Join(root, "Svalbard.app")); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(root, "Svalbard-Linux"), []byte("local launcher"), 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Install(root, "web-launcher", "web-launcher", []string{"macos-arm64", "linux-x86_64"}); err == nil {
			t.Fatal("accepted colliding path")
		}
		if _, err := os.Stat(filepath.Join(root, atlasPath)); !os.IsNotExist(err) {
			t.Fatalf("partial install: %v", err)
		}
	}
}

func TestLaunchersMatchTheVaultOSSelection(t *testing.T) {
	for _, platform := range []string{"macos-arm64", "linux-x86_64"} {
		root := t.TempDir()
		entries, err := Install(root, "web-launcher", "web-launcher", []string{platform})
		if err != nil {
			t.Fatal(err)
		}
		if !MatchesPlatforms(entries, []string{platform}) || MatchesPlatforms(entries, []string{"linux-arm64", "macos-arm64"}) {
			t.Fatal("wrong launcher platform set")
		}
	}
}

func TestMacBundleSelectsInstalledARMRuntimeUnderRosetta(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("requires Apple silicon")
	}
	if err := exec.Command("/usr/bin/arch", "-x86_64", "/usr/bin/true").Run(); err != nil {
		t.Skip("Rosetta is unavailable")
	}
	root := t.TempDir()
	if _, err := Install(root, "web-launcher", "web-launcher", []string{"macos-arm64"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".svalbard", "runtime", "macos-arm64")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "svalbard-drive")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$DRIVE_ROOT/selected\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "Svalbard.app", "Contents", "MacOS", "svalbard-launcher")
	cmd := exec.Command("/usr/bin/arch", "-x86_64", "/bin/sh", wrapper)
	cmd.Dir = "/"
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "selected"))
	if err != nil || string(got) != "launch" {
		t.Fatalf("Rosetta launched %q, %v", got, err)
	}
}
