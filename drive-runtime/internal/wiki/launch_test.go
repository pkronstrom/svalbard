package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

const (
	fakeRoleEnv   = "SVALBARD_WIKI_FAKE_ROLE"
	fakeRecordEnv = "SVALBARD_WIKI_FAKE_RECORD"
)

func TestMain(m *testing.M) {
	if role := os.Getenv(fakeRoleEnv); role != "" {
		runFakeSilverBullet(role)
		return
	}
	os.Exit(m.Run())
}

// fakeRecord is how the stand-in SilverBullet was started, and the state
// files it found.
type fakeRecord struct {
	Args  []string          `json:"args"`
	Env   []string          `json:"env"`
	Dir   string            `json:"dir"`
	State map[string]string `json:"state"`
	Modes map[string]string `json:"modes"`
}

// runFakeSilverBullet records how it was started and leaves a marker when it
// is asked to stop. The "serve" role answers on SB_UNIX_SOCKET as
// fakeSilverBullet does, "hang" never listens, and "crash" exits at once.
func runFakeSilverBullet(role string) {
	recordDir := os.Getenv(fakeRecordEnv)
	dir, _ := os.Getwd()
	record := fakeRecord{Args: os.Args[1:], Env: os.Environ(), Dir: dir, State: map[string]string{}, Modes: map[string]string{}}
	pages := ""
	if len(os.Args) > 1 {
		for _, name := range []string{"users.json", "spaces.json", "server.json"} {
			file := filepath.Join(os.Args[1], name)
			data, _ := os.ReadFile(file)
			record.State[name] = string(data)
			if info, err := os.Stat(file); err == nil {
				record.Modes[name] = info.Mode().Perm().String()
			}
		}
		var spaces map[string]struct {
			Folder string `json:"folder"`
		}
		_ = json.Unmarshal([]byte(record.State["spaces.json"]), &spaces)
		pages = spaces["wiki"].Folder
	}
	data, _ := json.Marshal(record)
	_ = os.WriteFile(filepath.Join(recordDir, "started.json"), data, 0o644)
	if role == "crash" {
		fmt.Println("ERROR invalid spaces.json: missing field `binding`")
		os.Exit(3)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	go func() {
		<-stop
		_ = os.WriteFile(filepath.Join(recordDir, "stopped"), nil, 0o644)
		os.Exit(0)
	}()
	if role == "hang" {
		fmt.Println("INFO indexing")
		select {}
	}
	listener, err := net.Listen("unix", os.Getenv("SB_UNIX_SOCKET"))
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(4)
	}
	_ = http.Serve(listener, fakeSilverBullet(pages, recordDir))
}

// fakeSilverBullet stands in for the routes of SilverBullet that matter to
// the launcher: the ping, a file write, its fetch proxy and the event stream.
// Everything else echoes what arrived.
func fakeSilverBullet(pages, recordDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "OK") })
	mux.HandleFunc("/.fs/", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Join(pages, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/.fs/")))
		switch r.Method {
		case http.MethodPut, http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			_ = os.WriteFile(file, body, 0o644)
			fmt.Fprint(w, "OK")
		case http.MethodDelete:
			_ = os.Remove(file)
			fmt.Fprint(w, "OK")
		default:
			http.ServeFile(w, r, file)
		}
	})
	mux.HandleFunc("/.proxy/", func(w http.ResponseWriter, r *http.Request) {
		_ = os.WriteFile(filepath.Join(recordDir, "proxied"), []byte(r.URL.Path), 0o644)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<script>remote()</script>")
	})
	mux.HandleFunc("/.events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": open\n\n")
		w.(http.Flusher).Flush()
		fmt.Fprint(w, "data: changed\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s host=%s forwarded=%s%s", r.URL.Path, r.Host, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Proto"))
	})
	return mux
}

// newWikiDrive lays out a drive whose SilverBullet is this test binary in
// the given role, and returns the drive and where the stand-in records.
func newWikiDrive(t *testing.T, role string) (driveRoot, recordDir string) {
	t.Helper()
	driveRoot, recordDir = t.TempDir(), t.TempDir()
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
	script := fmt.Sprintf("#!/bin/sh\n%s=%s %s='%s' exec '%s' \"$@\"\n", fakeRoleEnv, role, fakeRecordEnv, recordDir, self)
	if err := os.WriteFile(filepath.Join(binDir, binaryName), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return driveRoot, recordDir
}

func readStarted(t *testing.T, recordDir string) fakeRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(recordDir, "started.json"))
	if err != nil {
		t.Fatalf("SilverBullet was not started: %v", err)
	}
	var record fakeRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func envValue(env []string, key string) (string, bool) {
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, key+"="); ok {
			return value, true
		}
	}
	return "", false
}

// writeSettings gives the drive the schema apply generates from the
// silverbullet recipe, and the user's values.
func writeSettings(t *testing.T, driveRoot, values string) {
	t.Helper()
	schema := `{"version": 1, "groups": [{"owner": "silverbullet", "label": "Wiki", "settings": [
  {"name": "open_browser", "type": "bool", "default": true, "label": "Open browser on start", "description": "Open the browser when the wiki is ready."},
  {"name": "fs_watch", "type": "choice", "default": "auto", "choices": ["auto", "poll"], "label": "Watch for changed pages", "description": "How the wiki notices pages changed on the drive."}
]}]}`
	for name, content := range map[string]string{"settings.schema.json": schema, "settings.json": values} {
		if content == "" {
			continue
		}
		file := filepath.Join(driveRoot, ".svalbard", name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func settingsFor(t *testing.T, driveRoot string) *settings.Store {
	t.Helper()
	return settings.OpenForAction(driveRoot, io.Discard)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startRun runs the action until it opens the browser, and returns the
// address it opened, what it printed so far and how to stop it.
func startRun(t *testing.T, driveRoot string) (url string, stdout *syncBuffer, stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stdout = &syncBuffer{}
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, stdout, driveRoot, func(url string) error { opened <- url; return nil })
	}()
	select {
	case url = <-opened:
	case err := <-done:
		t.Fatalf("Run() returned before opening the browser: %v\n%s", err, stdout.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("browser was not opened\n%s", stdout.String())
	}
	return url, stdout, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(15 * time.Second):
			t.Fatal("Run() did not return after cancellation")
			return nil
		}
	}
}

func TestStateFilesMakeAWritableSpaceWithoutAccountsOrRevisions(t *testing.T) {
	files := stateFiles("/Volumes/STICK/wiki")
	if len(files) != 3 {
		t.Fatalf("state files = %v, want users.json, spaces.json and server.json", slices.Sorted(maps.Keys(files)))
	}

	// No accounts: a server with no login.
	var users map[string]any
	if err := json.Unmarshal(files["users.json"], &users); err != nil || len(users) != 0 {
		t.Errorf("users.json = %s (%v), want an empty object", files["users.json"], err)
	}

	var spaces map[string]struct {
		Name    string `json:"name"`
		Folder  string `json:"folder"`
		Binding struct {
			Prefix string `json:"prefix"`
			Host   string `json:"host"`
		} `json:"binding"`
		Access    string         `json:"access"`
		Revisions string         `json:"revisions"`
		Members   map[string]any `json:"members"`
		ReadOnly  bool           `json:"readOnly"`
		Shell     map[string]any `json:"shell"`
		GitSync   map[string]any `json:"gitSync"`
	}
	if err := json.Unmarshal(files["spaces.json"], &spaces); err != nil || len(spaces) != 1 {
		t.Fatalf("spaces.json = %s (%v), want one space", files["spaces.json"], err)
	}
	for _, space := range spaces {
		// The page folder is absolute: a relative one would be looked for
		// inside the state folder.
		if space.Folder != "/Volumes/STICK/wiki" || !filepath.IsAbs(space.Folder) {
			t.Errorf("folder = %q, want the absolute page folder", space.Folder)
		}
		if space.Binding.Prefix != "/" || space.Binding.Host != "" {
			t.Errorf("binding = %+v, want the whole server at /", space.Binding)
		}
		if space.Access != "write" || space.ReadOnly || len(space.Members) != 0 {
			t.Errorf("access = %q, readOnly = %v, members = %v; want anonymous write", space.Access, space.ReadOnly, space.Members)
		}
		// Revisions would run git inside the page folder.
		if space.Revisions != "disabled" || space.GitSync != nil {
			t.Errorf("revisions = %q, gitSync = %v; want revisions disabled", space.Revisions, space.GitSync)
		}
		// Shell commands stay at SilverBullet's default, off.
		if space.Shell != nil {
			t.Errorf("shell = %v, want it left unset", space.Shell)
		}
	}

	var server map[string]any
	if err := json.Unmarshal(files["server.json"], &server); err != nil {
		t.Fatalf("server.json = %s (%v)", files["server.json"], err)
	}
	// The runtime API starts the host's Chrome when it is called.
	if server["runtimeApi"] != false || server["mcp"] != false {
		t.Errorf("server.json = %v, want the runtime API and MCP off", server)
	}
}

func TestRunStartsTheWikiAndLeavesNothingBehind(t *testing.T) {
	driveRoot, recordDir := newWikiDrive(t, "serve")
	// A host that runs its own SilverBullet must not configure this one.
	t.Setenv("SB_HOSTNAME", "0.0.0.0")
	t.Setenv("SB_FOLDER", "/somewhere/else")

	url, stdout, stop := startRun(t, driveRoot)
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Fatalf("opened %q, want a localhost address", url)
	}
	pages, err := filepath.EvalSymlinks(Dir(driveRoot))
	if err != nil {
		t.Fatalf("the page folder was not created: %v", err)
	}
	if index, err := os.ReadFile(filepath.Join(pages, "index.md")); err != nil || !strings.Contains(string(index), "# Wiki") {
		t.Errorf("start page = %q (%v), want the seed", index, err)
	}

	record := readStarted(t, recordDir)
	if len(record.Args) != 1 {
		t.Fatalf("args = %v, want only the state folder", record.Args)
	}
	state := record.Args[0]
	// The state is on the computer: on the drive it would put a
	// memory-mapped index on exFAT and go stale when the mount path changes.
	if rel, err := filepath.Rel(os.TempDir(), state); err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("state folder = %q, want it in the host's temporary directory %q", state, os.TempDir())
	}
	if strings.HasPrefix(state, driveRoot) {
		t.Errorf("state folder %q is on the drive", state)
	}
	for name, want := range stateFiles(pages) {
		if record.State[name] != string(want) {
			t.Errorf("%s = %q, want %q", name, record.State[name], want)
		}
		if record.Modes[name] != "-rw-------" {
			t.Errorf("%s mode = %s, want it readable by the user only", name, record.Modes[name])
		}
	}

	socket, _ := envValue(record.Env, "SB_UNIX_SOCKET")
	socketDir := filepath.Dir(socket)
	// macOS limits a socket path to about 104 bytes.
	if !strings.HasPrefix(socket, "/tmp/") || len(socket) > 40 {
		t.Errorf("SB_UNIX_SOCKET = %q, want a short path under /tmp", socket)
	}
	if info, err := os.Stat(socketDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("socket folder = %v (%v), want it open to the user only", info, err)
	}
	for key, want := range map[string]string{
		"SB_UPDATE_CHECK": "0", "SB_FS_WATCH": "auto", "SB_MCP": "0", "SB_SHELL_BACKEND": "off", "HOME": state,
	} {
		if got, _ := envValue(record.Env, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	for _, key := range []string{"SB_HOSTNAME", "SB_FOLDER", "SB_PORT"} {
		if got, set := envValue(record.Env, key); set {
			t.Errorf("%s = %q reached SilverBullet from the host", key, got)
		}
	}

	// The proxy answers on the address that was opened and names its folder,
	// which is how the wiki tools find a running wiki.
	response, err := http.Get(url + pingPath)
	if err != nil {
		t.Fatalf("the wiki does not answer: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get(folderHeader) != pages {
		t.Errorf("ping = %d with folder %q, want 200 and %q", response.StatusCode, response.Header.Get(folderHeader), pages)
	}
	var port int
	fmt.Sscanf(url, "http://localhost:%d", &port)
	if got := runningURL(driveRoot, port, 1); got != url {
		t.Errorf("runningURL() = %q, want %q", got, url)
	}
	out := stdout.String()
	for _, want := range []string{"Wiki:  " + url + "\n", "Pages: " + Dir(driveRoot)} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}

	if err := stop(); err != nil {
		t.Fatalf("Run() error = %v after cancellation, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(recordDir, "stopped")); err != nil {
		t.Errorf("SilverBullet was not asked to stop: %v", err)
	}
	for _, dir := range []string{state, socketDir} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s was left behind", dir)
		}
	}
	if response, err := http.Get(url + pingPath); err == nil {
		response.Body.Close()
		t.Error("the wiki still answers after Run() returned")
	}
	// The pages stay.
	if _, err := os.Stat(filepath.Join(pages, "index.md")); err != nil {
		t.Errorf("the start page is gone: %v", err)
	}
	// Nothing but the pages is on the drive.
	if entries, _ := os.ReadDir(driveRoot); len(entries) != 2 {
		t.Errorf("drive = %v, want only bin and wiki", entries)
	}
}

func TestRunFollowsTheDriveSettings(t *testing.T) {
	driveRoot, recordDir := newWikiDrive(t, "serve")
	writeSettings(t, driveRoot, `{"silverbullet": {"open_browser": false, "fs_watch": "poll"}}`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &syncBuffer{}
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, stdout, driveRoot, func(url string) error { opened <- url; return nil })
	}()
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(stdout.String(), "Press Ctrl+C to stop.") {
		if time.Now().After(deadline) {
			t.Fatalf("the wiki did not start:\n%s", stdout.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {
	case url := <-opened:
		t.Errorf("the browser was opened at %s with open_browser off", url)
	default:
	}
	// The address is still printed.
	if !strings.Contains(stdout.String(), "Wiki:  http://localhost:") {
		t.Errorf("output has no address:\n%s", stdout.String())
	}
	if got, _ := envValue(readStarted(t, recordDir).Env, "SB_FS_WATCH"); got != "poll" {
		t.Errorf("SB_FS_WATCH = %q, want poll", got)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestSettingsDefaultWhenTheDriveDoesNotDeclareThem(t *testing.T) {
	for name, tc := range map[string]struct {
		schema  bool
		values  string
		browser bool
		watch   string
	}{
		"no schema":     {false, "", true, "auto"},
		"defaults":      {true, "", true, "auto"},
		"both changed":  {true, `{"silverbullet": {"open_browser": false, "fs_watch": "poll"}}`, false, "poll"},
		"invalid value": {true, `{"silverbullet": {"open_browser": "maybe", "fs_watch": "inotify"}}`, true, "auto"},
		"other owner":   {true, `{"open-webui": {"open_browser": false}}`, true, "auto"},
	} {
		driveRoot := t.TempDir()
		if tc.schema {
			writeSettings(t, driveRoot, tc.values)
		}
		prefs := settingsFor(t, driveRoot)
		if got := opensBrowser(prefs); got != tc.browser {
			t.Errorf("%s: opensBrowser = %v, want %v", name, got, tc.browser)
		}
		if got := fsWatch(prefs); got != tc.watch {
			t.Errorf("%s: fsWatch = %q, want %q", name, got, tc.watch)
		}
	}
}

func TestStartFailsCleanlyWithoutSilverBullet(t *testing.T) {
	driveRoot := t.TempDir()
	if Installed(driveRoot) {
		t.Fatal("Installed() is true on an empty drive")
	}
	err := Run(context.Background(), io.Discard, driveRoot, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "the wiki (SilverBullet) is not installed for ") {
		t.Fatalf("Run() error = %v, want a not-installed error", err)
	}
	// A drive without the wiki does not get a wiki folder from trying.
	if entries, _ := os.ReadDir(driveRoot); len(entries) != 0 {
		t.Errorf("drive = %v, want it untouched", entries)
	}
}

func TestStartNeverUsesASilverBulletFromTheHost(t *testing.T) {
	// A host install would be another release, started on this state.
	hostBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostBin, binaryName), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", hostBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	driveRoot := t.TempDir()
	if Installed(driveRoot) {
		t.Error("Installed() found SilverBullet on the host's PATH")
	}
	if _, err := Start(context.Background(), driveRoot, settingsFor(t, driveRoot)); err == nil {
		t.Error("Start() used SilverBullet from the host's PATH")
	}
}

func TestStartReportsASilverBulletThatExitsOrNeverAnswers(t *testing.T) {
	previous := readyTimeout
	t.Cleanup(func() { readyTimeout = previous })

	for role, wants := range map[string][]string{
		// Its own words are the only clue, and the log goes with the state.
		"crash": {"SilverBullet stopped before it was ready", "exit status 3", "invalid spaces.json: missing field `binding`"},
		"hang":  {"SilverBullet did not answer within 3s", "INFO indexing"},
	} {
		// Long enough for the stand-in to start on a busy machine; the one
		// that exits is not waited out at all.
		readyTimeout = previous
		if role == "hang" {
			readyTimeout = 3 * time.Second
		}
		driveRoot, recordDir := newWikiDrive(t, role)
		instance, err := Start(context.Background(), driveRoot, settingsFor(t, driveRoot))
		if err == nil {
			instance.Close()
			t.Errorf("%s: Start() succeeded", role)
			continue
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: Start() error = %q, want it to mention %q", role, err, want)
			}
		}
		record := readStarted(t, recordDir)
		socket, _ := envValue(record.Env, "SB_UNIX_SOCKET")
		for _, dir := range []string{record.Args[0], filepath.Dir(socket)} {
			if _, err := os.Stat(dir); err == nil {
				t.Errorf("%s: %s was left behind", role, dir)
			}
		}
		// No child is left running.
		if role == "hang" {
			if _, err := os.Stat(filepath.Join(recordDir, "stopped")); err != nil {
				t.Errorf("hang: SilverBullet was not stopped: %v", err)
			}
		}
	}
}

func TestStartStopsWhenTheUserGivesUpWaiting(t *testing.T) {
	driveRoot, recordDir := newWikiDrive(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, io.Discard, driveRoot, func(string) error { return nil }) }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(recordDir, "started.json")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SilverBullet was not started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		// Ctrl+C while waiting is the user stopping, not a failure.
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
	if _, err := os.Stat(filepath.Join(recordDir, "stopped")); err != nil {
		t.Errorf("SilverBullet was not stopped: %v", err)
	}
	if _, err := os.Stat(readStarted(t, recordDir).Args[0]); err == nil {
		t.Error("the state folder was left behind")
	}
}

func TestTheWikiFollowsTheDriveToAnotherMountPath(t *testing.T) {
	driveRoot, recordDir := newWikiDrive(t, "serve")
	_, _, stop := startRun(t, driveRoot)
	first := readStarted(t, recordDir)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(driveRoot), "Garden.md"), []byte("beans\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved := filepath.Join(t.TempDir(), "second mount")
	if err := os.Rename(driveRoot, moved); err != nil {
		t.Fatal(err)
	}
	url, _, stop := startRun(t, moved)
	second := readStarted(t, recordDir)
	pages, _ := filepath.EvalSymlinks(Dir(moved))
	if !strings.Contains(second.State["spaces.json"], pages) || strings.Contains(second.State["spaces.json"], driveRoot) {
		t.Errorf("spaces.json after the move = %s, want the folder %q", second.State["spaces.json"], pages)
	}
	// Each run has its own state: nothing of the first is read again.
	if second.Args[0] == first.Args[0] {
		t.Errorf("both runs used the state folder %q", first.Args[0])
	}
	// The pages came along, and the start page was not written over them.
	response, err := http.Get(url + "/.fs/Garden.md")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "beans\n" {
		t.Errorf("page after the move = %q, want beans", body)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestStartHoldsItsPortFromTheStart(t *testing.T) {
	// Two wikis at once (two drives) each get their own address.
	first, _ := newWikiDrive(t, "serve")
	second, _ := newWikiDrive(t, "serve")
	a, err := Start(context.Background(), first, settingsFor(t, first))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Start(context.Background(), second, settingsFor(t, second))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.URL == b.URL {
		t.Fatalf("both wikis are at %s", a.URL)
	}
	for instance, driveRoot := range map[*Instance]string{a: first, b: second} {
		pages, _ := filepath.EvalSymlinks(Dir(driveRoot))
		response, err := http.Get(instance.URL + pingPath)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if got := response.Header.Get(folderHeader); got != pages {
			t.Errorf("%s serves %q, want %q", instance.URL, got, pages)
		}
	}
	// Closing twice is harmless.
	a.Close()
	a.Close()
}
