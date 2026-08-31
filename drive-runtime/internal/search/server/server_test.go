package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	mode := os.Getenv("SVALBARD_TEST_SEARCH_SERVER")
	if mode == "" {
		os.Exit(m.Run())
	}
	if pidFile := os.Getenv("SVALBARD_TEST_SEARCH_SERVER_PID_FILE"); pidFile != "" {
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
			os.Exit(1)
		}
	}
	if mode == "unhealthy" {
		select {}
	}
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})); err != nil {
				os.Exit(1)
			}
		}
	}
	os.Exit(1)
}

func TestStartEmbeddingAndKiwixWaitForReadiness(t *testing.T) {
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER", "healthy")

	for _, test := range []struct {
		name  string
		start func(port int) error
	}{
		{
			name: "embedding",
			start: func(port int) error {
				cmd, err := StartEmbedding(context.Background(), os.Args[0], "model.gguf", port)
				if cmd != nil {
					t.Cleanup(func() { stop(cmd) })
				}
				return err
			},
		},
		{
			name: "kiwix",
			start: func(port int) error {
				cmd, err := StartKiwix(context.Background(), os.Args[0], []string{"wiki.zim"}, port)
				if cmd != nil {
					t.Cleanup(func() { stop(cmd) })
				}
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.start(availablePort(t)); err != nil {
				t.Fatalf("start() error = %v", err)
			}
		})
	}
}

func TestEmbeddingTimeoutReapsChild(t *testing.T) {
	oldTimeout, oldInterval := embeddingStartupTimeout, healthPollInterval
	embeddingStartupTimeout = 20 * time.Millisecond
	healthPollInterval = time.Millisecond
	t.Cleanup(func() {
		embeddingStartupTimeout = oldTimeout
		healthPollInterval = oldInterval
	})
	pidFile := filepath.Join(t.TempDir(), "server.pid")
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER", "unhealthy")
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER_PID_FILE", pidFile)

	if _, err := StartEmbedding(context.Background(), os.Args[0], "model.gguf", availablePort(t)); err == nil {
		t.Fatal("StartEmbedding() error = nil after readiness timeout")
	}

	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read child PID: %v", err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatalf("parse child PID: %v", err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("timed-out child %d still exists: %v", pid, err)
	}
}
func TestLaunchersReportStartFailure(t *testing.T) {
	if _, err := StartEmbedding(context.Background(), "/does/not/exist", "model.gguf", availablePort(t)); err == nil {
		t.Fatal("StartEmbedding() error = nil for missing binary")
	}
	if _, err := StartKiwix(context.Background(), "/does/not/exist", []string{"wiki.zim"}, availablePort(t)); err == nil {
		t.Fatal("StartKiwix() error = nil for missing binary")
	}
}

func TestFindEmbeddingModelSkipsAppleMetadata(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "models", "embed")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "._ignored.gguf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(modelDir, "model.gguf")
	if err := os.WriteFile(want, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindEmbeddingModel(root); got != want {
		t.Fatalf("FindEmbeddingModel() = %q, want %q", got, want)
	}
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
