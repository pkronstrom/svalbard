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
			if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "hanging-health" {
					_ = os.WriteFile(os.Getenv("SVALBARD_TEST_HEALTH_ENTERED_FILE"), []byte("entered"), 0o644)
					<-r.Context().Done()
					return
				}
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

func TestStartEmbeddingRetainsHealthyChildAfterRequestCancellation(t *testing.T) {
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER", "healthy")
	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := StartEmbedding(ctx, os.Args[0], "model.gguf", availablePort(t))
	if err != nil {
		t.Fatalf("StartEmbedding() error = %v", err)
	}
	defer stop(cmd)
	cancel()
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatal("healthy embedding child exited after caller cancellation")
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("healthy embedding child is not retained: %v", err)
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

func TestEmbeddingHangingHealthRequestTimesOutAndReapsChild(t *testing.T) {
	oldTimeout, oldInterval := embeddingStartupTimeout, healthPollInterval
	embeddingStartupTimeout = 500 * time.Millisecond
	healthPollInterval = 10 * time.Millisecond
	t.Cleanup(func() {
		embeddingStartupTimeout = oldTimeout
		healthPollInterval = oldInterval
	})
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "server.pid")
	enteredFile := filepath.Join(dir, "health.entered")
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER", "hanging-health")
	t.Setenv("SVALBARD_TEST_SEARCH_SERVER_PID_FILE", pidFile)
	t.Setenv("SVALBARD_TEST_HEALTH_ENTERED_FILE", enteredFile)
	start := time.Now()
	_, err := StartEmbedding(context.Background(), os.Args[0], "model.gguf", availablePort(t))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hanging health error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("hanging health request took %s", elapsed)
	}
	if _, err := os.Stat(enteredFile); err != nil {
		t.Fatalf("health handler was not entered: %v", err)
	}
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatal(err)
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

func TestZIMFilesLeavesOutAppleDoubleCompanions(t *testing.T) {
	driveRoot := t.TempDir()
	zimDir := filepath.Join(driveRoot, "zim")
	if err := os.MkdirAll(zimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// What macOS leaves beside an archive on an exFAT stick has the
	// archive's suffix and none of its content.
	for _, name := range []string{"wikem.zim", "._wikem.zim", "nhs.zim", "._nhs.zim", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(zimDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zims, err := ZIMFiles(driveRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(zimDir, "nhs.zim"), filepath.Join(zimDir, "wikem.zim")}
	if len(zims) != 2 || zims[0] != want[0] || zims[1] != want[1] {
		t.Errorf("ZIMFiles() = %v, want %v", zims, want)
	}
	if zims, err := ZIMFiles(t.TempDir()); err != nil || len(zims) != 0 {
		t.Errorf("ZIMFiles() on a drive without archives = %v, %v", zims, err)
	}
}
