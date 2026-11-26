package toolkit

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:embedded
var embeddedRuntime embed.FS

// extractEmbeddedBinaries extracts pre-built svalbard-drive binaries from the
// embedded filesystem. Returns a map of platform -> temp file path.
// If no embedded binaries are found (dev mode), returns an error so the caller
// can fall back to building from source.
func extractEmbeddedBinaries() (binaries map[string]string, err error) {
	binaries = make(map[string]string, len(supportedPlatforms))

	// One directory for all platforms; installRuntimeBinaries removes it once
	// the binaries are on the drive.
	tmpDir := ""
	defer func() {
		if err != nil && tmpDir != "" {
			discardRuntimeScratch(tmpDir)
		}
	}()

	for _, platform := range supportedPlatforms {
		embPath := filepath.Join("embedded", platform, runtimeBinaryName)
		data, err := fs.ReadFile(embeddedRuntime, embPath)
		if err != nil {
			return nil, fmt.Errorf("no embedded binary for %s: %w", platform, err)
		}
		// Skip placeholder/empty files (dev mode .gitkeep only)
		if len(data) < 1024 {
			return nil, fmt.Errorf("embedded binary for %s too small (%d bytes), likely placeholder", platform, len(data))
		}

		if tmpDir == "" {
			tmpDir, err = newRuntimeScratch("svalbard-embedded-runtime-")
			if err != nil {
				return nil, err
			}
		}
		outPath := filepath.Join(tmpDir, platform, runtimeBinaryName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(outPath, data, 0o755); err != nil {
			return nil, err
		}
		binaries[platform] = outPath
	}

	return binaries, nil
}
