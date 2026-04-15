package platform_test

import (
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestDetectReturnsKnownPlatform(t *testing.T) {
	got, err := platform.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	switch got {
	case "macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64":
	default:
		t.Fatalf("Detect() = %q, want known supported platform", got)
	}
}
