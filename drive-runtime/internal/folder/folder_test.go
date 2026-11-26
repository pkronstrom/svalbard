package folder

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// action is one running Documents or Workspace action.
type action struct {
	opened chan string
	done   chan error
	stop   context.CancelFunc
	stdout *lockedBuffer
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

// start runs the action with a stand-in for the browser and waits until it
// opens an address.
func start(t *testing.T, run func(ctx context.Context, stdout io.Writer, opener func(string) error) error) (*action, string) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	a := &action{opened: make(chan string, 1), done: make(chan error, 1), stop: stop, stdout: &lockedBuffer{}}
	go func() {
		a.done <- run(ctx, a.stdout, func(address string) error {
			a.opened <- address
			return nil
		})
	}()
	t.Cleanup(func() {
		stop()
		select {
		case <-a.done:
		case <-time.After(5 * time.Second):
			t.Error("the action did not stop")
		}
	})
	select {
	case address := <-a.opened:
		return a, address
	case err := <-a.done:
		a.done <- err
		t.Fatalf("the action ended before opening anything: %v\n%s", err, a.stdout.String())
	case <-time.After(5 * time.Second):
		t.Fatal("the action opened nothing")
	}
	return nil, ""
}

func get(t *testing.T, address string) (int, string) {
	t.Helper()
	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

// loopbackPort fails the test unless address is on the loopback interface,
// and returns its port.
func loopbackPort(t *testing.T, address string) string {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if host := parsed.Hostname(); host != "127.0.0.1" && host != "localhost" {
		t.Fatalf("address %s is not a loopback address", address)
	}
	return parsed.Port()
}

// stopAndCheckClosed cancels the action and checks that it ended cleanly and
// gave the port back.
func stopAndCheckClosed(t *testing.T, a *action, port string) {
	t.Helper()
	a.stop()
	select {
	case err := <-a.done:
		a.done <- err
		if err != nil {
			t.Errorf("stopping the action returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the action did not stop")
	}
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second); err == nil {
		conn.Close()
		t.Errorf("port %s still answers after the action stopped", port)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tree lists every path below root, to show an action left nothing behind.
func tree(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		paths = append(paths, path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestDocumentsOpensACollectionListing(t *testing.T) {
	drive := t.TempDir()
	writeFile(t, filepath.Join(drive, "books", "car manuals", "volvo v60.pdf"), "%PDF-volvo")
	writeFile(t, filepath.Join(drive, "books", "car manuals", "honda.pdf"), "%PDF-honda")
	writeFile(t, filepath.Join(drive, "books", "other.pdf"), "%PDF-other")
	before := tree(t, drive)

	a, address := start(t, func(ctx context.Context, stdout io.Writer, opener func(string) error) error {
		return Documents(ctx, stdout, drive, "car manuals", opener)
	})
	port := loopbackPort(t, address)
	if !strings.HasPrefix(address, "http://127.0.0.1:") || !strings.HasSuffix(address, "/car%20manuals/") {
		t.Errorf("opened %s, want the collection folder on 127.0.0.1", address)
	}

	status, body := get(t, address)
	if status != http.StatusOK || !strings.Contains(body, `href="volvo%20v60.pdf"`) || !strings.Contains(body, `href="honda.pdf"`) {
		t.Errorf("listing = %d %q, want both documents", status, body)
	}
	if strings.Contains(body, "other.pdf") {
		t.Errorf("the collection's listing shows a document outside it: %q", body)
	}
	if status, body := get(t, address+"volvo%20v60.pdf"); status != http.StatusOK || body != "%PDF-volvo" {
		t.Errorf("document = %d %q", status, body)
	}

	// The address is printed, for a host where no browser opens.
	if out := a.stdout.String(); !strings.Contains(out, "Documents: "+address+"\n") || !strings.Contains(out, "Ctrl+C") {
		t.Errorf("output = %q, want the address and how to stop", out)
	}

	stopAndCheckClosed(t, a, port)
	if after := tree(t, drive); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("the action changed the drive:\nbefore %v\nafter  %v", before, after)
	}
}

func TestDocumentsOpensASingleDocument(t *testing.T) {
	drive := t.TempDir()
	writeFile(t, filepath.Join(drive, "books", "WHO_Basic_Emergency_Care.pdf"), "%PDF-who")

	a, address := start(t, func(ctx context.Context, stdout io.Writer, opener func(string) error) error {
		return Documents(ctx, stdout, drive, "WHO_Basic_Emergency_Care.pdf", opener)
	})
	if !strings.HasSuffix(address, "/WHO_Basic_Emergency_Care.pdf") {
		t.Errorf("opened %s, want the document itself", address)
	}
	if status, body := get(t, address); status != http.StatusOK || body != "%PDF-who" {
		t.Errorf("document = %d %q", status, body)
	}
	stopAndCheckClosed(t, a, loopbackPort(t, address))
}

func TestDocumentsRefusesWhatIsNotOnTheDrive(t *testing.T) {
	drive := t.TempDir()
	writeFile(t, filepath.Join(drive, "books", "a.pdf"), "%PDF")
	writeFile(t, filepath.Join(drive, "secret.txt"), "secret")

	for _, rel := range []string{"", "missing", "../secret.txt", "/etc/hosts"} {
		opened := false
		err := Documents(context.Background(), io.Discard, drive, rel, func(string) error {
			opened = true
			return nil
		})
		if err == nil || opened {
			t.Errorf("Documents(%q) = %v, opened %v; want an error and nothing opened", rel, err, opened)
		}
	}
}

func TestDocumentsPrintsTheAddressWhenNoBrowserOpens(t *testing.T) {
	drive := t.TempDir()
	writeFile(t, filepath.Join(drive, "books", "a.pdf"), "%PDF")

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	stdout := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- Documents(ctx, stdout, drive, "a.pdf", func(string) error { return os.ErrNotExist })
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(stdout.String(), "Ctrl+C") {
		if time.Now().After(deadline) {
			t.Fatalf("output = %q", stdout.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if out := stdout.String(); !strings.Contains(out, "  Open: http://127.0.0.1:") {
		t.Errorf("output = %q, want the address to open by hand", out)
	}
	stop()
	if err := <-done; err != nil {
		t.Errorf("stopped with %v", err)
	}
}

func TestWorkspaceOpensTheFolderListing(t *testing.T) {
	drive := t.TempDir()
	writeFile(t, filepath.Join(drive, "workspace", "notes", "page.html"), "<h1>saved</h1>")
	before := tree(t, drive)

	a, address := start(t, func(ctx context.Context, stdout io.Writer, opener func(string) error) error {
		return Workspace(ctx, stdout, drive, opener)
	})
	port := loopbackPort(t, address)
	if !strings.HasPrefix(address, "http://localhost:") || !strings.HasSuffix(address, "/") {
		t.Errorf("opened %s, want the root of the workspace on localhost", address)
	}
	// "localhost" may resolve to ::1 first; the server is on 127.0.0.1 only.
	root := "http://127.0.0.1:" + port
	if status, body := get(t, root+"/"); status != http.StatusOK || !strings.Contains(body, `href="notes/"`) {
		t.Errorf("listing = %d %q, want the notes folder", status, body)
	}
	if status, body := get(t, root+"/notes/page.html"); status != http.StatusOK || body != "<h1>saved</h1>" {
		t.Errorf("page = %d %q", status, body)
	}
	// Read-only, as for the file tools' links.
	response, err := http.Post(root+"/notes/page.html", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", response.StatusCode)
	}
	if out := a.stdout.String(); !strings.Contains(out, "Workspace: "+address+"\n") {
		t.Errorf("output = %q, want the address", out)
	}

	stopAndCheckClosed(t, a, port)
	if after := tree(t, drive); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("the action changed the drive:\nbefore %v\nafter  %v", before, after)
	}
}

func TestWorkspaceSaysSoWhenThereIsNoFolderYet(t *testing.T) {
	drive := t.TempDir()
	var stdout bytes.Buffer
	opened := false
	err := Workspace(context.Background(), &stdout, drive, func(string) error {
		opened = true
		return nil
	})
	if err != nil || opened {
		t.Fatalf("Workspace = %v, opened %v; want nil and nothing opened", err, opened)
	}
	if out := stdout.String(); strings.Count(out, "\n") != 1 || !strings.Contains(out, "Nothing in the workspace yet") {
		t.Errorf("output = %q, want one line saying the workspace is empty", out)
	}
	// Looking must not create the folder.
	if _, err := os.Stat(filepath.Join(drive, "workspace")); !os.IsNotExist(err) {
		t.Errorf("the workspace folder was created: %v", err)
	}
}

func TestWorkspaceMissing(t *testing.T) {
	drive := t.TempDir()
	if line, missing := WorkspaceMissing(drive); !missing || line == "" {
		t.Errorf("WorkspaceMissing on a new drive = %q, %v", line, missing)
	}
	if err := os.Mkdir(filepath.Join(drive, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if line, missing := WorkspaceMissing(drive); missing || line != "" {
		t.Errorf("WorkspaceMissing with the folder = %q, %v", line, missing)
	}
}
