package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// newWorkspace opens a workspace on a fresh drive and returns it with a
// directory outside the drive that nothing may reach.
func newWorkspace(t *testing.T) (ws *Workspace, outside string) {
	t.Helper()
	ws, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	outside = t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws, outside
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCreatesTheFolderAtTheDriveRoot(t *testing.T) {
	drive := t.TempDir()
	ws, err := Open(drive)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	info, err := os.Stat(filepath.Join(drive, "workspace"))
	if err != nil || !info.IsDir() {
		t.Fatalf("workspace folder missing: %v", err)
	}
	// A second open finds the same folder with its content.
	if err := ws.WriteFile("page.html", []byte("x")); err != nil {
		t.Fatal(err)
	}
	again, err := Open(drive)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if data, err := again.ReadFile("page.html", 10); err != nil || string(data) != "x" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
}

func TestResolveCleansPathsInsideTheWorkspace(t *testing.T) {
	ws, _ := newWorkspace(t)
	for name, want := range map[string]string{
		"":                   ".",
		".":                  ".",
		"page.html":          "page.html",
		"./notes//page.html": "notes/page.html",
		"notes/":             "notes",
		"a b/ä.html":         "a b/ä.html",
	} {
		got, err := ws.Resolve(name)
		if err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestResolveRejectsPathsThatLeaveTheWorkspace(t *testing.T) {
	ws, outside := newWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws.Dir(), "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, outside, filepath.Join(ws.Dir(), "out"))
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(ws.Dir(), "secret-link.txt"))
	symlink(t, filepath.Join(outside, "not-yet.txt"), filepath.Join(ws.Dir(), "dangling.txt"))
	symlink(t, outside, filepath.Join(ws.Dir(), "notes", "deep"))

	for name, wantInError := range map[string]string{
		"/etc/passwd":                "absolute",
		ws.Dir() + "/page.html":      "absolute",
		"~/page.html":                "absolute",
		"../page.html":               `".."`,
		"notes/../../page.html":      `".."`,
		"notes/..":                   `".."`,
		`notes\page.html`:            `"/"`,
		"page\x00.html":              "NUL",
		".hidden":                    "dot",
		"notes/.page.html.tmp-00":    "dot",
		"out":                        "through a link",
		"out/secret.txt":             "through a link",
		"out/new/folder/page.html":   "through a link",
		"secret-link.txt":            "through a link",
		"dangling.txt":               "through a link",
		"notes/deep/secret.txt":      "through a link",
		"notes/deep/new-folder/page": "through a link",
	} {
		got, err := ws.Resolve(name)
		if err == nil {
			t.Errorf("Resolve(%q) = %q, want an error", name, got)
			continue
		}
		if !strings.Contains(err.Error(), wantInError) {
			t.Errorf("Resolve(%q) error = %q, want it to mention %s", name, err, wantInError)
		}
		// The error is the model's next instruction.
		if !strings.Contains(err.Error(), "notes/page.html") && !strings.Contains(err.Error(), "without the leading dot") {
			t.Errorf("Resolve(%q) error = %q, want it to say what to send instead", name, err)
		}
	}
}

func TestResolveFollowsALinkThatStaysInside(t *testing.T) {
	ws, _ := newWorkspace(t)
	if err := ws.WriteFile("notes/page.html", []byte("inside")); err != nil {
		t.Fatal(err)
	}
	symlink(t, "notes", filepath.Join(ws.Dir(), "alias"))

	rel, err := ws.Resolve("alias/page.html")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if data, err := ws.ReadFile(rel, 100); err != nil || string(data) != "inside" {
		t.Fatalf("ReadFile(%q) = %q, %v", rel, data, err)
	}
}

func TestWriteFileCreatesParentsAndReplacesWithoutLeftovers(t *testing.T) {
	ws, _ := newWorkspace(t)
	if err := ws.WriteFile("reports/2026/page.html", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteFile("reports/2026/page.html", []byte("second")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.Dir(), "reports", "2026", "page.html"))
	if err != nil || string(data) != "second" {
		t.Fatalf("file = %q, %v; want the replacement", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(ws.Dir(), "reports", "2026"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("folder holds %d entries, want only the file (a temporary file was left)", len(entries))
	}
}

func TestWriteFileRejectsWhatItMustNotWrite(t *testing.T) {
	ws, _ := newWorkspace(t)
	if err := ws.WriteFile("notes/page.html", []byte("kept")); err != nil {
		t.Fatal(err)
	}

	if err := ws.WriteFile("big.txt", bytes.Repeat([]byte("x"), MaxWriteBytes+1)); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("oversized write error = %v, want the limit", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir(), "big.txt")); err == nil {
		t.Error("oversized write left a file")
	}
	if err := ws.WriteFile("exact.txt", bytes.Repeat([]byte("x"), MaxWriteBytes)); err != nil {
		t.Errorf("write at the limit error = %v", err)
	}
	if err := ws.WriteFile("notes", []byte("x")); err == nil || !strings.Contains(err.Error(), "is a folder") {
		t.Errorf("write over a folder error = %v", err)
	}
	if err := ws.WriteFile(".", []byte("x")); err == nil {
		t.Error("write to the workspace root succeeded")
	}
	if err := ws.WriteFile("notes/page.html/inner.txt", []byte("x")); err == nil || !strings.Contains(err.Error(), "is a file, not a folder") {
		t.Errorf("write below a file error = %v", err)
	}
	if data, _ := ws.ReadFile("notes/page.html", 100); string(data) != "kept" {
		t.Errorf("existing file changed to %q", data)
	}
}

// Resolve is the check with the readable errors; the os.Root underneath must
// hold on its own if a path reaches the file operations without it.
func TestFileOperationsCannotLeaveTheWorkspaceWithoutResolve(t *testing.T) {
	ws, outside := newWorkspace(t)
	symlink(t, outside, filepath.Join(ws.Dir(), "out"))
	symlink(t, filepath.Join(outside, "new.txt"), filepath.Join(ws.Dir(), "dangling.txt"))

	for _, rel := range []string{"out/new.txt", "../new.txt", "dangling.txt", "out/secret.txt"} {
		if err := ws.WriteFile(rel, []byte("escaped")); err == nil {
			t.Errorf("WriteFile(%q) succeeded", rel)
		}
	}
	for _, rel := range []string{"out/secret.txt", "../" + filepath.Base(outside) + "/secret.txt"} {
		if data, err := ws.ReadFile(rel, 100); err == nil {
			t.Errorf("ReadFile(%q) = %q, want an error", rel, data)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Error("a file was created outside the workspace")
	}
	if data, _ := os.ReadFile(filepath.Join(outside, "secret.txt")); string(data) != "secret" {
		t.Errorf("file outside the workspace changed to %q", data)
	}
}

func TestReadFileRefusesFoldersLargeFilesAndNonRegularFiles(t *testing.T) {
	ws, _ := newWorkspace(t)
	if err := ws.WriteFile("notes/page.html", []byte("0123456789")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(ws.Dir(), "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}

	if data, err := ws.ReadFile("notes/page.html", 10); err != nil || string(data) != "0123456789" {
		t.Errorf("ReadFile() = %q, %v", data, err)
	}
	for rel, wantInError := range map[string]string{
		"notes":           "is a folder",
		"notes/page.html": "over the 9 byte limit",
		"pipe":            "not a regular file",
		"missing.txt":     "does not exist",
	} {
		_, err := ws.ReadFile(rel, 9)
		if err == nil || !strings.Contains(err.Error(), wantInError) {
			t.Errorf("ReadFile(%q) error = %v, want %q", rel, err, wantInError)
		}
		if err != nil && strings.Contains(err.Error(), ws.Dir()) {
			t.Errorf("ReadFile(%q) error %q names the host path", rel, err)
		}
	}
}
