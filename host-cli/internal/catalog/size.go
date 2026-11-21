package catalog

import "strings"

// PerPlatform reports whether the recipe's payload is downloaded or built
// once per target platform: a recipe with a platforms map, and the Python
// runtime and packages, which the python-venv build installs per platform.
func (it Item) PerPlatform() bool {
	return len(it.Platforms) > 0 || it.Type == "python-package" || it.Type == "python-venv"
}

// SupportsPlatform reports whether a per-platform recipe has a payload for
// platform. Python recipes are built for every target.
func (it Item) SupportsPlatform(platform string) bool {
	if len(it.Platforms) == 0 {
		return it.PerPlatform()
	}
	for _, key := range PlatformCandidates(platform) {
		if _, ok := it.Platforms[key]; ok {
			return true
		}
	}
	return false
}

// SizeGBFor estimates the recipe's size on a vault that targets platforms.
// size_gb is one platform's download for a per-platform recipe, so it counts
// once for each targeted platform the recipe supports; a single-asset recipe
// counts once. With no platforms, one platform (the host) is assumed.
func (it Item) SizeGBFor(platforms []string) float64 {
	if !it.PerPlatform() || len(platforms) == 0 {
		return it.SizeGB
	}
	n := 0
	for _, p := range platforms {
		if it.SupportsPlatform(p) {
			n++
		}
	}
	return it.SizeGB * float64(n)
}

// TotalSizeGB is the estimated size of items on a vault that targets
// platforms. Every size estimate in the CLI and TUI goes through it or
// SizeGBFor.
func TotalSizeGB(items []Item, platforms []string) float64 {
	var total float64
	for _, item := range items {
		total += item.SizeGBFor(platforms)
	}
	return total
}

// PlatformCandidates returns the platforms map keys that can serve platform,
// most specific first: "macos-arm64" also matches "darwin-arm64" and
// "macos-aarch64".
func PlatformCandidates(platform string) []string {
	candidates := []string{platform}
	osName, archName, ok := strings.Cut(platform, "-")
	if !ok {
		return candidates
	}
	switch osName {
	case "macos":
		candidates = append(candidates, "darwin-"+archName)
	case "darwin":
		candidates = append(candidates, "macos-"+archName)
	}
	if archName == "arm64" {
		candidates = append(candidates, osName+"-aarch64")
	}
	if archName == "x86_64" {
		candidates = append(candidates, osName+"-amd64")
	}
	return candidates
}
