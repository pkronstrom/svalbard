// Package server starts concrete local services used by offline search.
package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	embeddingStartupTimeout = 30 * time.Second
	kiwixStartupTimeout     = 10 * time.Second
	healthPollInterval      = 500 * time.Millisecond
)

// ZIMFiles lists the archives in the drive's zim folder, in name order. On
// an exFAT stick macOS keeps a "._name.zim" file beside each archive, and
// kiwix-serve does not start when it is handed one.
func ZIMFiles(driveRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(driveRoot, "zim", "*.zim"))
	if err != nil {
		return nil, err
	}
	zims := matches[:0]
	for _, zim := range matches {
		if !strings.HasPrefix(filepath.Base(zim), "._") {
			zims = append(zims, zim)
		}
	}
	return zims, nil
}

// FindEmbeddingModel returns the first packaged embedding model.
func FindEmbeddingModel(driveRoot string) string {
	matches, _ := filepath.Glob(filepath.Join(driveRoot, "models", "embed", "*.gguf"))
	for _, model := range matches {
		if !strings.HasPrefix(filepath.Base(model), "._") {
			return model
		}
	}
	return ""
}

// StartEmbedding starts llama-server and returns only after its health endpoint
// responds successfully.
func StartEmbedding(ctx context.Context, llamaBin, model string, port int) (*exec.Cmd, error) {
	if model == "" {
		return nil, fmt.Errorf("embedding model required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// The backend owns the child after a successful start.  In particular, a
	// cancelled request which happened to start it must not kill a healthy
	// shared server.
	cmd := exec.Command(llamaBin, "--model", model, "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1", "--embedding")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting llama-server: %w", err)
	}
	if err := waitForHealthy(ctx, cmd, fmt.Sprintf("http://127.0.0.1:%d/health", port), embeddingStartupTimeout); err != nil {
		return nil, fmt.Errorf("llama-server failed to become healthy: %w", err)
	}
	return cmd, nil
}

// StartKiwix starts kiwix-serve and returns only after its root endpoint
// responds successfully.
func StartKiwix(ctx context.Context, kiwixBin string, zims []string, port int) (*exec.Cmd, error) {
	if len(zims) == 0 {
		return nil, fmt.Errorf("no ZIM files found")
	}
	return startKiwix(ctx, kiwixBin, zims, port)
}

func startKiwix(ctx context.Context, kiwixBin string, inputs []string, port int) (*exec.Cmd, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	args := []string{"--port", fmt.Sprintf("%d", port), "--address", "127.0.0.1"}
	args = append(args, inputs...)
	cmd := exec.Command(kiwixBin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting kiwix-serve: %w", err)
	}
	if err := waitForHealthy(ctx, cmd, fmt.Sprintf("http://127.0.0.1:%d/", port), kiwixStartupTimeout); err != nil {
		return nil, fmt.Errorf("kiwix-serve failed to become healthy: %w", err)
	}
	return cmd, nil
}

func waitForHealthy(ctx context.Context, cmd *exec.Cmd, healthURL string, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		req, err := http.NewRequestWithContext(healthCtx, http.MethodGet, healthURL, nil)
		if err != nil {
			stop(cmd)
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-healthCtx.Done():
			stop(cmd)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("health check timed out: %w", healthCtx.Err())
		case <-time.After(healthPollInterval):
		}
	}
}

func stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
