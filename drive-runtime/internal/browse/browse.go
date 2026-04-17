package browse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func ResolveTargets(driveRoot, selected string) ([]string, error) {
	if selected != "" {
		path := filepath.Join(driveRoot, "zim", selected)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("selected zim not found: %s", selected)
		}
		return []string{path}, nil
	}

	pattern := filepath.Join(driveRoot, "zim", "*.zim")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	filtered := paths[:0]
	for _, path := range paths {
		if strings.HasPrefix(filepath.Base(path), "._") {
			continue
		}
		filtered = append(filtered, path)
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no ZIM files found in zim/")
	}
	return filtered, nil
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, selected string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}
	targets, err := ResolveTargets(driveRoot, selected)
	if err != nil {
		return err
	}
	kiwixBin, err := binary.Resolve("kiwix-serve", driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("kiwix-serve not found")
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8080)
	if err != nil {
		return err
	}

	args := buildKiwixArgs(port, targets)
	var stderrBuf strings.Builder
	cmd := exec.CommandContext(ctx, kiwixBin, args...)
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stdout, &stderrBuf)
	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for kiwix-serve to be ready before opening the browser.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	url := fmt.Sprintf("http://localhost:%d", port)
	if err := waitHealthy(ctx, url, waitCh); err != nil {
		// Process already exited — include its output in the error.
		detail := strings.TrimSpace(stderrBuf.String())
		if detail != "" {
			return fmt.Errorf("kiwix-serve failed: %s", detail)
		}
		return fmt.Errorf("kiwix-serve failed: %w", err)
	}

	if err := opener(url); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", url)
	}
	fmt.Fprintf(stdout, "Kiwix: %s\n", url)

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return <-waitCh
	case err := <-waitCh:
		return err
	}
}

// waitHealthy polls the kiwix-serve URL until it responds or the process exits.
func waitHealthy(ctx context.Context, url string, exitCh <-chan error) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.After(15 * time.Second)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case err := <-exitCh:
			if err != nil {
				return err
			}
			return fmt.Errorf("kiwix-serve exited immediately")
		case <-deadline:
			return fmt.Errorf("kiwix-serve not ready after 15s")
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func buildKiwixArgs(port int, targets []string) []string {
	args := []string{"--port", fmt.Sprintf("%d", port), "--address", "127.0.0.1"}
	args = append(args, targets...)
	return args
}

