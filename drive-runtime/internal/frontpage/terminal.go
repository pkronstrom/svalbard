package frontpage

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/shell"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/unix"
)

const terminalProtocol = "svalbard-terminal"

func (d *dashboard) serveTerminal(w http.ResponseWriter, r *http.Request) {
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	protocol, token, _ := strings.Cut(r.Header.Get("Sec-WebSocket-Protocol"), ",")
	if !net.ParseIP(remote).IsLoopback() || r.Header.Get("Origin") != "http://"+r.Host ||
		protocol != terminalProtocol || d.terminalKey == "" ||
		subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(d.terminalKey)) != 1 {
		http.Error(w, "Open the dashboard from its private launch link to start a terminal.", http.StatusForbidden)
		return
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "A terminal requires a WebSocket connection.", http.StatusBadRequest)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode != "" && mode != "sb" {
		http.Error(w, "Unknown terminal mode.", http.StatusBadRequest)
		return
	}
	d.mu.Lock()
	if d.ctx.Err() != nil || d.draining {
		d.mu.Unlock()
		http.Error(w, "The dashboard is stopping.", http.StatusServiceUnavailable)
		return
	}
	d.workers.Add(1)
	d.terminals++
	d.mu.Unlock()
	defer func() { d.mu.Lock(); d.terminals--; d.mu.Unlock(); d.workers.Done() }()
	server := websocket.Server{
		Handshake: func(cfg *websocket.Config, _ *http.Request) error {
			cfg.Protocol = []string{terminalProtocol}
			return nil
		},
		Handler: func(socket *websocket.Conn) { d.runTerminal(socket, mode == "sb") },
	}
	server.ServeHTTP(w, r)
}

type terminalFrame struct {
	data   []byte
	binary bool
}

var terminalFrames = websocket.Codec{
	Unmarshal: func(data []byte, payload byte, dst any) error {
		*dst.(*terminalFrame) = terminalFrame{data: data, binary: payload == websocket.BinaryFrame}
		return nil
	},
}

func (d *dashboard) runTerminal(socket *websocket.Conn, activate bool) {
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()
	var cmd *exec.Cmd
	if activate {
		activated, cleanup, err := shell.Prepare(ctx, d.root)
		if err != nil {
			reportTerminalError(socket, err)
			return
		}
		defer cleanup()
		cmd = activated
	} else {
		shellPath := os.Getenv("SHELL")
		if shellPath == "" {
			shellPath = "/bin/sh"
		}
		cmd = exec.CommandContext(ctx, shellPath, "-i")
	}
	cmd.Dir = d.root
	cmd.Env = append(cmd.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	screen, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err == nil {
		screen, err = pollableTerminal(screen)
		if err != nil {
			cancel()
			_ = cmd.Wait()
		}
	}
	if err != nil {
		reportTerminalError(socket, err)
		return
	}
	socket.MaxPayloadBytes = 64 << 10
	ack := make(chan struct{}, 1)
	inputDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = screen.Close()
		_ = socket.SetDeadline(time.Now())
	})
	go func() {
		defer close(inputDone)
		defer cancel()
		readTerminalInput(socket, screen, ack)
	}()
	defer func() {
		cancel()
		_ = screen.Close()
		_ = socket.SetDeadline(time.Now())
		<-inputDone
		_ = cmd.Wait()
		stop()
	}()
	buf := make([]byte, 16<<10)
	for {
		n, err := screen.Read(buf)
		if n > 0 {
			_ = socket.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if websocket.Message.Send(socket, buf[:n]) != nil {
				return
			}
			// Keep one frame in flight so fast output cannot build an
			// unbounded queue inside the browser renderer.
			select {
			case <-ack:
			case <-ctx.Done():
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func reportTerminalError(socket *websocket.Conn, err error) {
	_ = socket.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_ = websocket.Message.Send(socket, []byte(fmt.Sprintf("Could not start the local shell: %v\r\n", err)))
}

func pollableTerminal(screen *os.File) (*os.File, error) {
	// Register a nonblocking descriptor with Go before any reads. Closing it
	// can then wake the blocked relay even when a foreground job is idle.
	defer screen.Close()
	fd, err := unix.FcntlInt(screen.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), screen.Name()), nil
}

func readTerminalInput(socket *websocket.Conn, screen *os.File, ack chan<- struct{}) {
	for {
		var frame terminalFrame
		if terminalFrames.Receive(socket, &frame) != nil {
			return
		}
		if frame.binary {
			if _, err := screen.Write(frame.data); err != nil {
				return
			}
			continue
		}
		var control struct {
			Type       string
			Rows, Cols uint16
		}
		if json.Unmarshal(frame.data, &control) != nil {
			return
		}
		switch control.Type {
		case "ack":
			select {
			case ack <- struct{}{}:
			default:
			}
		case "resize":
			if control.Rows > 0 && control.Rows <= 500 && control.Cols > 0 && control.Cols <= 500 {
				_ = pty.Setsize(screen, &pty.Winsize{Rows: control.Rows, Cols: control.Cols})
			}
		}
	}
}
