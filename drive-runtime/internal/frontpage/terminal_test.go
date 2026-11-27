package frontpage

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"golang.org/x/net/websocket"
)

func TestTerminalOpensActivatedSBShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	root := filepath.Join(t.TempDir(), "Drive $TERM's files")
	writeFile(t, root, "run", "#!/bin/sh\nprintf 'SB_RESULT: %s %s\\n' \"$DRIVE_ROOT\" \"$1\"\n")
	if err := os.Chmod(filepath.Join(root, "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	client, address, _ := startDashboardLink(t, root, config.RuntimeConfig{
		Groups: []config.MenuGroup{{Label: "Tools", Items: []config.MenuItem{{
			ID: "activate-shell", Label: "Activate sb Shell", Action: config.BuiltinAction("activate-shell", nil),
		}}}},
	})
	origin, key, _ := strings.Cut(address, "/#terminal-key=")
	body := requestBody(t, client, origin+"/api/inventory", http.StatusOK)
	if !strings.Contains(body, `"terminalMode":"sb"`) || strings.Contains(body, `"command":"./run"`) {
		t.Fatalf("expected an activated shell, got %s", body)
	}
	cfg, err := websocket.NewConfig("ws"+strings.TrimPrefix(origin, "http")+"/api/terminal?mode=sb", origin)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Protocol = []string{terminalProtocol, key}
	socket, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	_ = socket.SetDeadline(time.Now().Add(5 * time.Second))
	if err := websocket.Message.Send(socket, []byte("sb settings\r")); err != nil {
		t.Fatal(err)
	}
	want := "SB_RESULT: " + root + " settings"
	var output bytes.Buffer
	for !strings.Contains(output.String(), want) {
		var frame []byte
		if err := websocket.Message.Receive(socket, &frame); err != nil {
			t.Fatalf("waiting for the sb wrapper: %v. Output was %q", err, &output)
		}
		output.Write(frame)
		if err := websocket.Message.Send(socket, `{"type":"ack"}`); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTerminalRequiresPrivateLaunchLink(t *testing.T) {
	root := t.TempDir()
	client, address, _ := startDashboardLink(t, root, config.RuntimeConfig{})
	origin, key, found := strings.Cut(address, "/#terminal-key=")
	if !found || len(key) < 25 {
		t.Fatal("missing terminal launch key")
	}
	for _, tc := range []struct {
		origin, key, host string
	}{
		{origin, "", ""},
		{origin, "wrong-key", ""},
		{"https://other.invalid", key, ""},
		{origin, key, "other.invalid"},
	} {
		req, err := http.NewRequest(http.MethodGet, origin+"/api/terminal", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Sec-WebSocket-Protocol", terminalProtocol+", "+tc.key)
		reply, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		responseBody(t, reply, http.StatusForbidden)
	}
}

func TestTerminalRunsInteractiveShellInVault(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("PS1", "SVALBARD> ")
	root := t.TempDir()
	writeFile(t, root, "zim/Wikiciv.zim", "")
	client, address, stop := startDashboardLink(t, root, config.RuntimeConfig{
		Groups: []config.MenuGroup{{Label: "Settings", Items: []config.MenuItem{{
			ID: "settings", Label: "Settings", Action: config.BuiltinAction("settings", nil),
		}}}},
	})
	origin, key, _ := strings.Cut(address, "/#terminal-key=")
	body := requestBody(t, client, origin+"/api/inventory", http.StatusOK)
	if !strings.Contains(body, `"terminal":true`) || !strings.Contains(body, `"command":"./run settings"`) {
		t.Fatalf("Settings has no prefilled terminal: %s", body)
	}
	cfg, err := websocket.NewConfig("ws"+strings.TrimPrefix(origin, "http")+"/api/terminal", origin)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Protocol = []string{terminalProtocol, key}
	socket, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	_ = socket.SetDeadline(time.Now().Add(5 * time.Second))

	var output bytes.Buffer
	until := func(needle string) {
		t.Helper()
		for !strings.Contains(output.String(), needle) {
			var frame []byte
			if err := websocket.Message.Receive(socket, &frame); err != nil {
				t.Fatalf("waiting for %q: %v. Output was %q", needle, err, &output)
			}
			output.Write(frame)
			if err := websocket.Message.Send(socket, `{"type":"ack"}`); err != nil {
				t.Fatal(err)
			}
		}
	}
	until("SVALBARD> ")
	if err := websocket.Message.Send(socket, `{"type":"resize","rows":31,"cols":93}`); err != nil {
		t.Fatal(err)
	}
	if err := websocket.Message.Send(socket, []byte("ls zim; stty size; printf '\\033[32m%s\\033[0m\\n' READY\r")); err != nil {
		t.Fatal(err)
	}
	until("\x1b[32mREADY\x1b[0m")
	if text := output.String(); !strings.Contains(text, "Wikiciv.zim") || !strings.Contains(text, "31 93") {
		t.Fatalf("shell neither listed the vault nor resized: %q", text)
	}
	if err := websocket.Message.Send(socket, []byte("printf 'FOREGROUND_%s\\n' ACTIVE; sleep 12\r")); err != nil {
		t.Fatal(err)
	}
	until("FOREGROUND_ACTIVE")
	stop()
	for {
		var frame []byte
		err := websocket.Message.Receive(socket, &frame)
		if err == nil {
			continue // Final prompt output can already be buffered on the socket.
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("shell socket still open after the dashboard stopped")
		}
		break
	}
}
