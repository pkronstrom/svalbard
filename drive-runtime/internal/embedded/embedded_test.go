package embedded_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/embedded"
)

func TestPackageNameStripsPlatformAndVersionSuffixes(t *testing.T) {
	cases := map[string]string{
		"toolchain-xtensa-esp-elf-linux_x86_64-14.2.0+20251107.tar.gz": "toolchain-xtensa-esp-elf",
		"framework-espidf-3.50503.0.tar.gz":                            "framework-espidf",
		"tool-openocd-esp32-darwin_arm64-2.1200.20230419.tar.gz":       "tool-openocd-esp32",
	}
	for input, want := range cases {
		if got := embedded.PackageName(input); got != want {
			t.Fatalf("PackageName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEnvironmentSetsOfflinePlatformIOSettings(t *testing.T) {
	env := embedded.Environment("/tmp/cache")
	if got, want := env["PLATFORMIO_CORE_DIR"], "/tmp/cache"; got != want {
		t.Fatalf("PLATFORMIO_CORE_DIR = %q, want %q", got, want)
	}
	if _, ok := env["PLATFORMIO_BUILD_DIR"]; ok {
		t.Fatal("Environment must not force a global PLATFORMIO_BUILD_DIR")
	}
	for key, want := range map[string]string{
		"PIP_NO_INDEX":                                    "1",
		"PLATFORMIO_SETTING_ENABLE_TELEMETRY":             "no",
		"PLATFORMIO_DISABLE_UPGRADE_CHECK":                "true",
		"PLATFORMIO_SETTING_CHECK_PRUNE_SYSTEM_THRESHOLD": "0",
	} {
		if got := env[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestPrepareRejectsMissingInventoryWithoutReadyMarker(t *testing.T) {
	root := t.TempDir()
	if _, err := embedded.Prepare(context.Background(), os.Stdout, root); err == nil || !strings.Contains(err.Error(), "platformio inventory") {
		t.Fatalf("Prepare() error = %v, want missing inventory", err)
	}
}

func TestRunRejectsUnsupportedShellBeforePreparation(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	// The empty root has no inventory, so reaching Prepare would report that instead.
	err := embedded.Run(context.Background(), os.Stdout, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "requires bash, zsh, or fish") {
		t.Fatalf("Run() error = %v, want unsupported shell before preparation", err)
	}
}

func TestPrepareRejectsChecksumMismatchBeforeBootstrap(t *testing.T) {
	root := fixtureVault(t, "not the recorded bytes")
	if _, err := embedded.Prepare(context.Background(), os.Stdout, root); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Prepare() error = %v, want checksum failure", err)
	}
	assertNoReadyMarker(t, root)
}

func fixtureVault(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "tools", "platformio", "packages", "tool.tar.gz")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("recorded bytes"))
	inventory := map[string]any{"version": 1, "packages": []map[string]string{{
		"path": "tools/platformio/packages/tool.tar.gz", "sha256": hex.EncodeToString(sum[:]),
		"owner": "platformio", "name": "tool-esptoolpy", "kind": "tool", "platform": "",
	}}}
	data, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "platformio.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertNoReadyMarker(t *testing.T, root string) {
	t.Helper()
	cacheRoot := os.Getenv("SVALBARD_PIO_CACHE")
	if cacheRoot == "" {
		return // The default user cache is intentionally not inspected by a unit test.
	}
	entries, _ := filepath.Glob(filepath.Join(cacheRoot, "**", ".ready"))
	if len(entries) != 0 {
		t.Fatalf("unexpected ready markers: %v", entries)
	}
}
