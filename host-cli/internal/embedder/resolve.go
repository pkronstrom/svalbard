package embedder

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/pkronstrom/svalbard/bintree"
)

// resolveBinary locates a named binary on the drive the way the drive
// runtime does (bintree.Find: <archive>.unpacked first, an archive without a
// tree unpacked into one), then falls back to the system PATH.
func resolveBinary(name, driveRoot string) (string, error) {
	if driveRoot != "" {
		if platform, err := detectPlatform(); err == nil {
			if path, err := bintree.Find(name, driveRoot, platform); err == nil {
				return path, nil
			}
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("embedder: %s not found in drive or PATH", name)
}

func detectPlatform() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "macos-arm64", nil
	case "darwin/amd64":
		return "macos-x86_64", nil
	case "linux/arm64":
		return "linux-arm64", nil
	case "linux/amd64":
		return "linux-x86_64", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}
