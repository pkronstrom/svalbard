// Private Unix socket for a single vault's browser launch, status and stop.
// Only its owning user can reach it. Clients never identify servers by TCP
// port or PID, and never send signals to a PID read from disk.
package frontpage

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"golang.org/x/sys/unix"
)

type desktopPaths struct{ dir, socket, lock, log string }

func pathsFor(root string) (desktopPaths, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return desktopPaths{}, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return desktopPaths{}, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	// Finder and Terminal can name the same directory with different Unicode
	// normalization or letter case. Device and inode identify the actual vault.
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d/%d", stat.Dev, stat.Ino)))
	// Keep sockaddr_un below Darwin's 104-byte limit even for long vault names.
	dir := filepath.Join("/tmp", "svalbard-desktop-"+strconv.Itoa(os.Getuid()))
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return desktopPaths{}, err
	}
	info, err = os.Lstat(dir)
	if err != nil {
		return desktopPaths{}, err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 || int(info.Sys().(*syscall.Stat_t).Uid) != os.Getuid() {
		return desktopPaths{}, fmt.Errorf("private launch directory %s must belong to this user with mode 0700", dir)
	}
	base := filepath.Join(dir, fmt.Sprintf("%x", digest[:16]))
	return desktopPaths{dir, base + ".sock", base + ".lock", base + ".log"}, nil
}

type launchReply struct{ URL, File, Error string }

func (p desktopPaths) request(command string) (launchReply, error) {
	conn, err := net.DialTimeout("unix", p.socket, 250*time.Millisecond)
	if err != nil {
		return launchReply{}, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(conn, command); err != nil {
		return launchReply{}, err
	}
	var r launchReply
	err = json.NewDecoder(io.LimitReader(conn, 8192)).Decode(&r)
	if err == nil && r.Error != "" {
		err = errors.New(r.Error)
	}
	return r, err
}

func tryLock(p desktopPaths) (*os.File, bool, error) {
	f, err := os.OpenFile(p.lock, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return f, true, nil
}

// Launch uses the installed executable, even when the caller is in another
// vault. The child inherits the locked file description across exec/setsid.
func Launch(root string, args []string, out io.Writer, open func(string) error) error {
	p, err := pathsFor(root)
	if err != nil {
		return err
	}
	command := "open"
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--no-open":
		command = "status"
	case len(args) == 1 && (args[0] == "stop" || args[0] == "status"):
		reply, err := p.request(args[0])
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
				fmt.Fprintln(out, "No managed dashboard is running for this vault.")
				return nil
			}
			return err
		}
		if args[0] == "stop" {
			// A server holds the lock until its workers, databases and listeners
			// close. Fixed sleeps and stale PID files cannot prove that.
			deadline := time.Now().Add(15 * time.Second)
			for {
				f, free, err := tryLock(p)
				if err != nil {
					return err
				}
				if free {
					f.Close()
					break
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("shutdown is still pending. Check %s before ejecting", p.log)
				}
				time.Sleep(50 * time.Millisecond)
			}
			fmt.Fprintln(out, "Managed dashboard stopped.")
		} else {
			fmt.Fprintln(out, reply.URL)
		}
		return nil
	default:
		return fmt.Errorf("usage: ./run launch [--no-open | status | stop]")
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", "web-launcher.zip")); err != nil {
		return fmt.Errorf("dashboard assets unavailable. Add web-launcher and apply it to this vault: %w", err)
	}
	lock, free, err := tryLock(p)
	if err != nil {
		return err
	}
	var childDone <-chan error
	var child *os.Process
	if free {
		child, childDone, err = startDetached(root, p, lock)
		lock.Close()
		if err != nil {
			return err
		}
	}
	timeout := time.NewTimer(12 * time.Second)
	defer timeout.Stop()
	for {
		reply, err := p.request(command)
		if err == nil {
			if command == "open" {
				if err = open(reply.File); err != nil {
					p.record("Browser opener failed: " + err.Error())
					return fmt.Errorf("open %s in your browser or retry the launcher. Browser error: %w. Log: %s", reply.URL, err, p.log)
				}
			}
			fmt.Fprintln(out, reply.URL)
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ECONNREFUSED) {
			return fmt.Errorf("dashboard control: %w. Log: %s", err, p.log)
		}
		select {
		case err := <-childDone:
			return fmt.Errorf("dashboard exited before becoming ready (%v). Check %s", err, p.log)
		case <-timeout.C:
			if child != nil {
				// A failed first launch must never leave an unready detached
				// child holding the drive. A caller joining a peer owns none.
				_ = child.Signal(syscall.SIGTERM)
				select {
				case <-childDone:
				case <-time.After(time.Second):
					_ = child.Kill()
					<-childDone
				}
			}
			return fmt.Errorf("dashboard did not become ready in 12 seconds. Check %s", p.log)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func startDetached(root string, p desktopPaths, lock *os.File) (*os.Process, <-chan error, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	log, err := os.OpenFile(p.log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, nil, err
	}
	defer log.Close()
	// Start in a host directory so cwd itself doesn't tie the wrapper to USB.
	cmd := exec.Command(exe, "launch", "--serve")
	cmd.Dir = "/"
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+root)
	cmd.ExtraFiles = []*os.File{lock}
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start dashboard: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return cmd.Process, done, nil
}

func ServeDesktop(ctx context.Context, root string, cfg config.RuntimeConfig) error {
	p, err := pathsFor(root)
	if err != nil {
		return err
	}
	inherited := os.NewFile(3, "desktop-lock")
	if inherited == nil {
		return fmt.Errorf("use ./run launch")
	}
	defer inherited.Close()
	supplied, err := inherited.Stat()
	if err != nil {
		return fmt.Errorf("use ./run launch")
	}
	expected, err := os.Stat(p.lock)
	if err != nil || !os.SameFile(supplied, expected) || unix.Flock(3, unix.LOCK_EX|unix.LOCK_NB) != nil {
		return fmt.Errorf("invalid desktop lock; use ./run launch")
	}
	return runDashboard(ctx, io.Discard, root, cfg, nil, &desktopSession{paths: p})
}

type desktopSession struct {
	paths      desktopPaths
	foreground bool
	stop       context.CancelFunc
	listener   net.Listener
	mu         sync.Mutex
	url, key   string
	// Browser handoffs live only until first use, expiry or server shutdown.
	grants map[string]string
	closed bool
}

func (d *desktopSession) ready(ctx context.Context, url, key string, cancel context.CancelFunc) error {
	client := http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("dashboard readiness returned %s", response.Status)
	}
	if err := os.Remove(d.paths.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	listener, err := net.Listen("unix", d.paths.socket)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.listener = listener
	d.url = url
	d.key = key
	d.stop = cancel
	d.grants = map[string]string{}
	d.mu.Unlock()
	d.paths.record("Dashboard ready at " + url)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			line, _ := bufio.NewReader(io.LimitReader(conn, 48)).ReadString('\n')
			reply := launchReply{URL: url}
			op := strings.TrimSpace(line)
			switch op {
			case "open":
				file, err := d.handoff()
				if err != nil {
					reply.Error = err.Error()
				} else {
					reply.File = file
				}
			case "status", "stop":
			default:
				reply.Error = "unknown launch operation"
			}
			_ = json.NewEncoder(conn).Encode(reply)
			conn.Close()
			if op == "stop" {
				cancel()
				return
			}
		}
	}()
	return nil
}

// A short-lived form sends the one-use grant in a POST body. Neither the HTTP
// address, browser history nor process arguments contain a terminal credential.
var handoffPage = template.Must(template.New("handoff").Parse(`<!doctype html><meta charset="utf-8"><title>Opening Svalbard</title><form method="post" action="{{.URL}}browser-launch"><input type="hidden" name="grant" value="{{.Grant}}"><button>Open Svalbard</button></form><script>document.forms[0].submit()</script>`))

func (d *desktopSession) handoff() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return "", errors.New("dashboard is stopping")
	}
	f, err := os.CreateTemp(d.paths.dir, "open-*.html")
	if err != nil {
		return "", err
	}
	grant := rand.Text()
	err = handoffPage.Execute(f, struct{ URL, Grant string }{d.url, grant})
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	d.grants[grant] = f.Name()
	time.AfterFunc(time.Minute, func() { d.mu.Lock(); defer d.mu.Unlock(); d.forget(grant) })
	return f.Name(), nil
}

func (d *desktopSession) forget(grant string) {
	if name, ok := d.grants[grant]; ok {
		_ = os.Remove(name)
		delete(d.grants, grant)
	}
}

func (d *desktopSession) redeem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		http.Error(w, "Use the vault launcher.", 403)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid handoff", 400)
		return
	}
	grant := r.PostForm.Get("grant")
	d.mu.Lock()
	_, ok := d.grants[grant]
	d.forget(grant)
	key := d.key
	d.mu.Unlock()
	if !ok {
		http.Error(w, "This launch has expired. Open the vault launcher again.", 403)
		return
	}
	d.paths.record("Browser accepted the launch handoff.")
	nonce := rand.Text()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; frame-ancestors 'none'; base-uri 'none'")
	// Both key and nonce are generated by crypto/rand.Text's base32 alphabet.
	fmt.Fprintf(w, `<!doctype html><title>Svalbard</title><script nonce="%s">try{sessionStorage.setItem("svalbard-terminal-key","%s")}catch{}location.replace("/")</script><a href="/">Open dashboard</a>`, nonce, key)
}

func (d *desktopSession) close() {
	d.paths.record("Dashboard stopped.")
	d.listener.Close()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	for grant := range d.grants {
		d.forget(grant)
	}
}

// Foreground keeps the terminal workflow and shares instance ownership with
// the desktop entry. The process that started the server retains its lifetime.
func Foreground(ctx context.Context, out io.Writer, root string, cfg config.RuntimeConfig, open func(string) error) error {
	p, err := pathsFor(root)
	if err != nil {
		return err
	}
	lock, free, err := tryLock(p)
	if err != nil {
		return err
	}
	if !free {
		args := []string{"--no-open"}
		if open != nil {
			args = nil
		}
		return Launch(root, args, out, open)
	}
	defer lock.Close()
	return runDashboard(ctx, out, root, cfg, open, &desktopSession{paths: p, foreground: true})
}

func (p desktopPaths) record(message string) {
	f, err := os.OpenFile(p.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, time.Now().UTC().Format(time.RFC3339), message)
}
