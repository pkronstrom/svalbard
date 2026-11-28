// Package folder opens a folder of the drive in the browser: a document or a
// collection under books/, and the workspace the assistant saves pages to.
// The workspace server does the serving: read-only, one folder, loopback Host
// names only, for as long as the action runs.
package folder

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

// workspacePort is where Open WebUI's file tools serve the workspace. Asking
// for the same port keeps the links the assistant wrote into a chat working
// while that front end is not running.
const workspacePort = 8086

// Documents serves the drive's documents and opens rel in the browser: a
// collection's folder or a single document, as a slash-separated path below
// books/. It returns when ctx is done.
func Documents(ctx context.Context, stdout io.Writer, driveRoot, rel string, opener func(string) error) error {
	return DocumentsForDashboard(ctx, stdout, driveRoot, rel, 0, opener)
}

// DocumentsForDashboard also allows the dashboard to display the documents in
// its reader when dashboardPort is set.
func DocumentsForDashboard(ctx context.Context, stdout io.Writer, driveRoot, rel string, dashboardPort int, opener func(string) error) error {
	dir := filepath.Join(driveRoot, engine.DocumentsDir)
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) {
		return fmt.Errorf("not a path below %s/: %q", engine.DocumentsDir, rel)
	}
	info, err := os.Stat(filepath.Join(dir, local))
	if err != nil {
		return fmt.Errorf("%s/%s is not on this drive", engine.DocumentsDir, rel)
	}
	if !info.IsDir() && !engine.IsDocumentFile(local) {
		return fmt.Errorf("%s/%s is not a document", engine.DocumentsDir, rel)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	address := workspace.URL(base, filepath.ToSlash(filepath.Clean(local)))
	if info.IsDir() && local != "." {
		address += "/"
	}
	// Only documents: a download leaves its record beside the file.
	return serve(ctx, stdout, "Documents", dir, workspace.Options{Serves: engine.IsDocumentFile, DashboardPort: dashboardPort}, listener, address, opener)
}

// Workspace serves the drive's workspace folder and opens its listing in the
// browser. It returns when ctx is done, or at once when the folder does not
// exist yet.
func Workspace(ctx context.Context, stdout io.Writer, driveRoot string, opener func(string) error) error {
	return WorkspaceForDashboard(ctx, stdout, driveRoot, 0, opener)
}

// WorkspaceForDashboard also allows the dashboard to display workspace apps
// in the reader when dashboardPort is set.
func WorkspaceForDashboard(ctx context.Context, stdout io.Writer, driveRoot string, dashboardPort int, opener func(string) error) error {
	if line, missing := WorkspaceMissing(driveRoot); missing {
		fmt.Fprintln(stdout, line)
		return nil
	}
	listener, port, err := netutil.Listen("127.0.0.1", workspacePort)
	if err != nil {
		return err
	}
	// Spelled with "localhost", as the links the file tools give out are.
	host := "localhost"
	if dashboardPort > 0 {
		host = "127.0.0.1"
	}
	address := fmt.Sprintf("http://%s:%d/", host, port)
	return serve(ctx, stdout, "Workspace", filepath.Join(driveRoot, workspace.DirName), workspace.Options{DashboardPort: dashboardPort}, listener, address, opener)
}

// WorkspaceMissing reports whether the drive has no workspace folder yet, and
// the line that says so. The folder is not created to show it empty: opening
// something must not write to the drive.
func WorkspaceMissing(driveRoot string) (string, bool) {
	if _, err := os.Stat(filepath.Join(driveRoot, workspace.DirName)); !os.IsNotExist(err) {
		return "", false
	}
	return "Nothing in the workspace yet: the assistant saves pages there from Open WebUI.", true
}

// serve answers for dir on listener until ctx is done, and opens address once
// it does.
func serve(ctx context.Context, stdout io.Writer, name, dir string, opts workspace.Options, listener net.Listener, address string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}
	folder, err := workspace.OpenWith(dir, opts)
	if err != nil {
		listener.Close()
		return err
	}
	defer folder.Close()

	served := make(chan error, 1)
	go func() { served <- folder.Serve(ctx, listener) }()

	if err := opener(address); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", address)
	}
	fmt.Fprintf(stdout, "%s: %s\n", name, address)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop.")
	return <-served
}
