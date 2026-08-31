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
	cmd := exec.CommandContext(ctx, llamaBin, "--model", model, "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1", "--embedding")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting llama-server: %w", err)
	}
	if err := waitForHealthy(cmd, fmt.Sprintf("http://127.0.0.1:%d/health", port), embeddingStartupTimeout); err != nil {
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
	if ctx == nil {
		ctx = context.Background()
	}
	args := []string{"--port", fmt.Sprintf("%d", port), "--address", "127.0.0.1"}
	args = append(args, zims...)
	cmd := exec.CommandContext(ctx, kiwixBin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting kiwix-serve: %w", err)
	}
	if err := waitForHealthy(cmd, fmt.Sprintf("http://127.0.0.1:%d/", port), kiwixStartupTimeout); err != nil {
		return nil, fmt.Errorf("kiwix-serve failed to become healthy: %w", err)
	}
	return cmd, nil
}

func waitForHealthy(cmd *exec.Cmd, healthURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(healthPollInterval)
	}
	stop(cmd)
	return fmt.Errorf("health check timed out")
}

func stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
