package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

const (
	binaryName = "silverbullet"

	// These two are declared by the silverbullet recipe.
	settingOpenBrowser = RecipeID + ".open_browser"
	settingFSWatch     = RecipeID + ".fs_watch"

	logName = "silverbullet.log"
)

// readyTimeout is how long SilverBullet gets to answer. It indexes the pages
// before it does, which takes seconds for a personal wiki. It is a variable
// so tests need not wait it out.
var readyTimeout = 60 * time.Second

// socketParent is where the folder for SilverBullet's socket is made. A
// socket path is limited to about 104 bytes on macOS, whose own temporary
// directory is already half of that.
var socketParent = "/tmp"

// Instance is one running wiki: SilverBullet on a Unix socket and, in front
// of it, the proxy on localhost that browsers reach.
type Instance struct {
	// URL is the wiki's address, without a trailing slash.
	URL string
	// Dir is the page folder on the host.
	Dir string

	exited chan error
	stop   func()
}

// Exited delivers the reason once the wiki stops by itself.
func (i *Instance) Exited() <-chan error { return i.exited }

// Close stops SilverBullet and the proxy and deletes the run's state.
func (i *Instance) Close() { i.stop() }

// stateFiles is SilverBullet's whole configuration for one run, by file
// name. `silverbullet setup` is not used: it requires an admin password,
// turns on git revisions inside the page folder and applies a starter kit.
//
//   - users.json with no accounts makes a server with no login.
//   - spaces.json has one space at "/", writable without an account, with
//     revisions off, and the page folder as an absolute path on the current
//     mount.
//   - server.json switches off the runtime API, which would otherwise start
//     the host's Chrome when asked to, and SilverBullet's own MCP endpoint.
//
// The files are written fresh on every start, so a drive mounted at another
// path, or a release with another format, never meets stale state.
func stateFiles(pages string) map[string][]byte {
	spaces := map[string]any{
		"wiki": map[string]any{
			"name":      "Wiki",
			"folder":    pages,
			"binding":   map[string]string{"prefix": "/"},
			"access":    "write",
			"revisions": "disabled",
		},
	}
	server := map[string]any{"runtimeApi": false, "mcp": false}
	return map[string][]byte{
		"users.json":  []byte("{}\n"),
		"spaces.json": mustJSON(spaces),
		"server.json": mustJSON(server),
	}
}

func mustJSON(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

// environment is everything SilverBullet gets from its environment: the
// host's PATH, locale and time zone, and these. A host's own SB_ variables
// (a hostname to bind, a folder) must not reach it.
func environment(state, socket, watch string) []string {
	env := map[string]string{
		// Anything SilverBullet keeps under a home directory goes with the
		// run's state.
		"HOME": state,
		// No TCP listener: browsers reach it through the proxy only.
		"SB_UNIX_SOCKET": socket,
		// Without its service worker the page keeps no copy of the notes
		// that answers after the wiki has stopped, and nothing in the
		// browser takes requests the proxy never sees.
		"SB_DISABLE_SERVICE_WORKER": "1",
		"SB_UPDATE_CHECK":           "0",
		"SB_FS_WATCH":               watch,
		"SB_MCP":                    "0",
		"SB_SHELL_BACKEND":          "off",
		"NO_COLOR":                  "1",
	}
	list := platform.EnvList(env)
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			list = append(list, key+"="+value)
		}
	}
	return list
}

// Start seeds the page folder if it is empty, starts SilverBullet for it and
// serves the proxy on 127.0.0.1, always, whatever else the caller binds: the
// server has no login. It returns once the wiki answers. The caller closes
// the instance.
func Start(ctx context.Context, driveRoot string, prefs *settings.Store) (*Instance, error) {
	server, err := binary.ResolveOnDrive(binaryName, driveRoot)
	if err != nil {
		platformName, _ := platform.Detect()
		return nil, fmt.Errorf("the wiki (SilverBullet) is not installed for %s", platformName)
	}
	if err := os.MkdirAll(Dir(driveRoot), 0o755); err != nil {
		return nil, fmt.Errorf("the wiki folder cannot be created: %w", err)
	}
	pages := resolvedDir(driveRoot)
	if err := Seed(pages); err != nil {
		return nil, fmt.Errorf("the wiki's start page cannot be written: %w", err)
	}
	// Not worth failing for: the tools answer for the page when it is missing.
	_ = SeedHelp(pages)

	// The state is on the computer, not the drive: the search index uses
	// memory-mapped files and locks, which exFAT serves badly, and it
	// rebuilds from the pages in seconds.
	state, err := os.MkdirTemp("", "svalbard-wiki-")
	if err != nil {
		return nil, err
	}
	socketDir, err := os.MkdirTemp(socketParent, "sbw-")
	if err != nil {
		os.RemoveAll(state)
		return nil, err
	}
	removeState := func() {
		os.RemoveAll(state)
		os.RemoveAll(socketDir)
	}
	for name, content := range stateFiles(pages) {
		if err := os.WriteFile(filepath.Join(state, name), content, 0o600); err != nil {
			removeState()
			return nil, err
		}
	}
	socket := filepath.Join(socketDir, "s")

	// Held from here on, so the address printed is the address served.
	listener, port, err := netutil.Listen("127.0.0.1", PreferredPort)
	if err != nil {
		removeState()
		return nil, err
	}

	logPath := filepath.Join(state, logName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		listener.Close()
		removeState()
		return nil, err
	}
	childCtx, cancelChild := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(childCtx, server, state)
	cmd.Dir = state
	cmd.Env = environment(state, socket, fsWatch(prefs))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// SIGTERM lets SilverBullet close its index; it is killed if it lingers.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		logFile.Close()
		listener.Close()
		cancelChild()
		removeState()
		return nil, fmt.Errorf("starting SilverBullet: %w", err)
	}
	childExited := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		logFile.Close()
		childExited <- err
	}()
	stopChild := func() {
		cancelChild()
		<-childExited
	}

	if err := waitReady(ctx, socket, childExited); err != nil {
		tail := logTail(logPath)
		if !errors.Is(err, errExitedEarly) {
			stopChild()
		}
		cancelChild()
		listener.Close()
		removeState()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w%s", err, tail)
	}

	proxy := &http.Server{Handler: newProxy(socket, port, pages), ReadHeaderTimeout: 10 * time.Second}
	exited := make(chan error, 2)
	go func() {
		if err := proxy.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			exited <- fmt.Errorf("the wiki's proxy stopped: %w", err)
		}
	}()
	stopped := make(chan struct{})
	go func() {
		select {
		case err := <-childExited:
			childExited <- err
			exited <- fmt.Errorf("SilverBullet stopped (%v)%s", err, logTail(logPath))
		case <-stopped:
		}
	}()

	instance := &Instance{URL: fmt.Sprintf("http://localhost:%d", port), Dir: Dir(driveRoot), exited: exited}
	var once sync.Once
	instance.stop = func() {
		once.Do(func() {
			close(stopped)
			// Close, not Shutdown: the page's event stream never ends by itself.
			proxy.Close()
			stopChild()
			removeState()
		})
	}
	return instance, nil
}

var errExitedEarly = errors.New("SilverBullet stopped before it was ready")

// waitReady asks SilverBullet over its socket until it answers, it exits,
// readyTimeout passes or ctx ends.
func waitReady(ctx context.Context, socket string, exited chan error) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
		},
	}
	deadline := time.After(readyTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if response, err := client.Get("http://silverbullet" + pingPath); err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case err := <-exited:
			exited <- err
			return fmt.Errorf("%w (%v)", errExitedEarly, err)
		case <-deadline:
			return fmt.Errorf("SilverBullet did not answer within %s", readyTimeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// logTail returns the last lines SilverBullet wrote, as the end of an error
// message. The log goes away with the run's state, so this is its one reader.
func logTail(logPath string) string {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	return "; its last output:\n  " + strings.Join(lines, "\n  ")
}

// Run starts the wiki, opens the browser unless the setting says not to, and
// stops everything when ctx ends or SilverBullet exits.
func Run(ctx context.Context, stdout io.Writer, driveRoot string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}
	prefs := settings.OpenForAction(driveRoot, stdout)
	fmt.Fprintln(stdout, "Starting the wiki (Ctrl+C to stop)...")
	instance, err := Start(ctx, driveRoot, prefs)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer instance.Close()

	if opensBrowser(prefs) {
		if err := opener(instance.URL); err != nil {
			fmt.Fprintf(stdout, "  Open: %s\n", instance.URL)
		}
	}
	fmt.Fprintf(stdout, "Wiki:  %s\n", instance.URL)
	fmt.Fprintf(stdout, "Pages: %s (plain Markdown files)\n", instance.Dir)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop.")

	select {
	case err := <-instance.Exited():
		// Ctrl+C reaches SilverBullet and this process together, so an exit
		// just ahead of the cancellation is the user stopping, not a failure.
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(500 * time.Millisecond):
			return err
		}
	case <-ctx.Done():
		return nil
	}
}

// opensBrowser reads the recipe's open_browser setting. A drive whose schema
// file does not declare it opens the browser, as the default does.
func opensBrowser(prefs *settings.Store) bool {
	_, declared := prefs.Lookup(settingOpenBrowser)
	return !declared || prefs.Bool(settingOpenBrowser)
}

// fsWatch reads the recipe's fs_watch setting: "auto" lets the operating
// system report changed pages, "poll" looks for them on a timer, for a stick
// where reports go missing.
func fsWatch(prefs *settings.Store) string {
	if _, declared := prefs.Lookup(settingFSWatch); declared && prefs.String(settingFSWatch) == "poll" {
		return "poll"
	}
	return "auto"
}
