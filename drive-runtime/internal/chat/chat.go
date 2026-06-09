package chat

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/contextpicker"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func ResolveModel(driveRoot, selected string) (string, error) {
	if selected != "" {
		// Try as-is first (absolute path), then resolve relative to models dir.
		if info, err := os.Stat(selected); err == nil && !info.IsDir() {
			return selected, nil
		}
		path := filepath.Join(driveRoot, "models", selected)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("model not found: %s", selected)
	}

	pattern := filepath.Join(driveRoot, "models", "*.gguf")
	models, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	filtered := models[:0]
	for _, model := range models {
		base := filepath.Base(model)
		if strings.HasPrefix(base, "._") {
			continue
		}
		filtered = append(filtered, model)
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no GGUF model found in models/")
	}
	return filtered[0], nil
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, selected string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}
	model, err := ResolveModel(driveRoot, selected)
	if err != nil {
		return err
	}
	llamaBin, err := binary.Resolve("llama-server", driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("llama-server not found")
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8082)
	if err != nil {
		return err
	}

	ctxSize := contextpicker.Pick(llamaserve.ContextForHost())
	fmt.Fprintf(stdout, "Starting llama-server on port %d with %s (context %d)...\n", port, filepath.Base(model), ctxSize)
	args := []string{"-m", model, "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1"}
	args = append(args, llamaserve.ExtraFlags(model, ctxSize)...)
	cmd := exec.CommandContext(ctx, llamaBin, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Start(); err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	if err := opener(url); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", url)
	}
	fmt.Fprintf(stdout, "LLM: %s\n", url)
	return cmd.Wait()
}
