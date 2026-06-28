package platform

import (
	"fmt"
	"runtime"
	"sort"
)

// EnvList turns a key→value map into a sorted "KEY=value" slice suitable for
// exec.Cmd.Env. Sorted so the result is deterministic.
func EnvList(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	sort.Strings(out)
	return out
}

func Detect() (string, error) {
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
