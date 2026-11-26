package serveall_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/serveall"
)

const fakeWikiEnv = "SVALBARD_SERVEALL_FAKE_WIKI"

func TestMain(m *testing.M) {
	if os.Getenv(fakeWikiEnv) != "" {
		runFakeSilverBullet()
		return
	}
	os.Exit(m.Run())
}

// runFakeSilverBullet answers the launcher's ping on SB_UNIX_SOCKET until it
// is asked to stop.
func runFakeSilverBullet() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	go func() {
		<-stop
		os.Exit(0)
	}()
	listener, err := net.Listen("unix", os.Getenv("SB_UNIX_SOCKET"))
	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}
	_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "OK") }))
}

func TestPlanServicesKeepsTheWikiOnThisComputer(t *testing.T) {
	driveRoot := t.TempDir()
	plan := serveall.PlanServices(driveRoot, map[string]string{"silverbullet": "/tmp/bin/silverbullet"})
	if len(plan) != 2 || plan[0].Name != "files" || plan[1].Name != "wiki" {
		t.Fatalf("plan = %#v, want files and wiki", plan)
	}
	// The wiki has no login: it does not follow the address the rest binds.
	if plan[1].Bind != "127.0.0.1" {
		t.Errorf("wiki bind = %q, want 127.0.0.1", plan[1].Bind)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunStartsTheWikiAndSaysItIsLocalOnly(t *testing.T) {
	driveRoot := t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Skip(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(driveRoot, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\n%s=1 exec '%s' \"$@\"\n", fakeWikiEnv, self)
	if err := os.WriteFile(filepath.Join(binDir, "silverbullet"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &lockedBuffer{}
	done := make(chan error, 1)
	go func() { done <- serveall.Run(ctx, stdout, driveRoot, "127.0.0.1") }()

	line := regexp.MustCompile(`(?m)^Wiki:   (http://localhost:\d+) \(local only\)$`)
	var url string
	for deadline := time.Now().Add(30 * time.Second); url == ""; time.Sleep(20 * time.Millisecond) {
		if match := line.FindStringSubmatch(stdout.String()); match != nil {
			url = match[1]
		}
		if time.Now().After(deadline) {
			t.Fatalf("no wiki line in the output:\n%s", stdout.String())
		}
	}
	response, err := http.Get(url + "/.ping")
	if err != nil {
		t.Fatalf("the wiki does not answer at %s: %v", url, err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("wiki ping = %d, want 200", response.StatusCode)
	}
	// It is the same wiki the menu entry starts: seeded on first use.
	if _, err := os.Stat(filepath.Join(driveRoot, "wiki", "index.md")); err != nil {
		t.Errorf("start page not seeded: %v", err)
	}
	if !strings.Contains(stdout.String(), "Files:  http://127.0.0.1:") {
		t.Errorf("the other services are missing:\n%s", stdout.String())
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() error = %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
	if response, err := http.Get(url + "/.ping"); err == nil {
		response.Body.Close()
		t.Error("the wiki still answers after Serve all stopped")
	}
}

func TestRunWithoutTheWikiSaysNothingAboutIt(t *testing.T) {
	driveRoot := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	stdout := &lockedBuffer{}
	done := make(chan error, 1)
	go func() { done <- serveall.Run(ctx, stdout, driveRoot, "127.0.0.1") }()
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(stdout.String(), "Files:"); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("Serve all did not start:\n%s", stdout.String())
		}
	}
	cancel()
	<-done
	if strings.Contains(stdout.String(), "Wiki") {
		t.Errorf("output mentions a wiki the drive does not have:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(driveRoot, "wiki")); err == nil {
		t.Error("Serve all created a wiki folder on a drive without the wiki")
	}
}
