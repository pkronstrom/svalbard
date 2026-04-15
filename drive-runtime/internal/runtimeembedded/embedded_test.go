package runtimeembedded_test

import (
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeembedded"
)

func TestPackageNameStripsPlatformAndVersionSuffixes(t *testing.T) {
	cases := map[string]string{
		"toolchain-xtensa-esp-elf-linux_x86_64-14.2.0+20251107.tar.gz": "toolchain-xtensa-esp-elf",
		"framework-espidf-3.50503.0.tar.gz":                            "framework-espidf",
		"tool-openocd-esp32-darwin_arm64-2.1200.20230419.tar.gz":       "tool-openocd-esp32",
	}
	for input, want := range cases {
		if got := runtimeembedded.PackageName(input); got != want {
			t.Fatalf("PackageName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEnvironmentSetsPlatformIODirs(t *testing.T) {
	env := runtimeembedded.Environment("/tmp/cache")
	if got, want := env["PLATFORMIO_CORE_DIR"], "/tmp/cache"; got != want {
		t.Fatalf("PLATFORMIO_CORE_DIR = %q, want %q", got, want)
	}
	if got, want := env["PLATFORMIO_BUILD_DIR"], "/tmp/svalbard-pio-build"; got != want {
		t.Fatalf("PLATFORMIO_BUILD_DIR = %q, want %q", got, want)
	}
}
