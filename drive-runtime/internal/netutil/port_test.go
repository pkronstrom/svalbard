package netutil

import (
	"fmt"
	"net"
	"testing"
)

func TestFindAvailablePort_ReturnsPreferred(t *testing.T) {
	port, err := FindAvailablePort("127.0.0.1", 19876)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port < 19876 || port > 19876+19 {
		t.Fatalf("expected port in range 19876-19895, got %d", port)
	}
}

func TestFindAvailablePort_SkipsOccupied(t *testing.T) {
	// Occupy port 19876 so the function should return 19877 or later.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 19876))
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer ln.Close()

	port, err := FindAvailablePort("127.0.0.1", 19876)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 19876 {
		t.Fatal("expected to skip occupied port 19876")
	}
	if port < 19877 || port > 19876+19 {
		t.Fatalf("expected port in range 19877-19895, got %d", port)
	}
}

func TestFindAvailablePort_FallsBackToEphemeral(t *testing.T) {
	// Occupy all 20 ports in the preferred range to force ephemeral fallback.
	var listeners []net.Listener
	for p := 19900; p < 19920; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			t.Fatalf("failed to occupy port %d: %v", p, err)
		}
		listeners = append(listeners, ln)
	}
	defer func() {
		for _, ln := range listeners {
			ln.Close()
		}
	}()

	port, err := FindAvailablePort("127.0.0.1", 19900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The ephemeral port should be outside the preferred range.
	if port >= 19900 && port <= 19919 {
		t.Fatalf("expected ephemeral port outside 19900-19919, got %d", port)
	}
	if port <= 0 {
		t.Fatalf("expected valid port, got %d", port)
	}
}
