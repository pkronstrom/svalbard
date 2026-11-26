package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWithNamesItsOwnFolderInErrors(t *testing.T) {
	outside := t.TempDir()
	wikiDir := filepath.Join(t.TempDir(), "wiki")
	ws, err := OpenWith(wikiDir, Options{Place: "wiki", Example: "Projects/Garden plan.md"})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	symlink(t, outside, filepath.Join(wikiDir, "out"))

	for name, wants := range map[string][]string{
		"/etc/passwd":    {"is absolute", "inside the wiki", "Projects/Garden plan.md"},
		"../secret.md":   {`contains ".."`, "inside the wiki", "Projects/Garden plan.md"},
		`a\b.md`:         {`uses "\"`, "Projects/Garden plan.md"},
		"out/secret.txt": {"leaves the wiki through a link", "inside the wiki", "Projects/Garden plan.md"},
	} {
		_, err := ws.Resolve(name)
		if err == nil {
			t.Errorf("Resolve(%q) succeeded", name)
			continue
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Resolve(%q) error = %q, want it to mention %q", name, err, want)
			}
		}
		if strings.Contains(err.Error(), "workspace") || strings.Contains(err.Error(), "page.html") {
			t.Errorf("Resolve(%q) error = %q, which speaks of the workspace", name, err)
		}
	}
	if err := ws.WriteFile(".", []byte("x")); err == nil || !strings.Contains(err.Error(), "the wiki folder itself") {
		t.Errorf("WriteFile(.) error = %v, want it to name the wiki folder", err)
	}

	// The dot-name rule is the same in every folder: SilverBullet hides
	// such names too, and the atomic writer's temporary files rely on it.
	if _, err := ws.Resolve(".hidden.md"); err == nil || !strings.Contains(err.Error(), "starting with a dot") {
		t.Errorf("Resolve(.hidden.md) error = %v, want the dot-name refusal", err)
	}
}

func TestOpenKeepsTheWorkspaceWording(t *testing.T) {
	ws, _ := newWorkspace(t)
	_, err := ws.Resolve("/etc/passwd")
	if want := `path "/etc/passwd" is absolute; use a path inside the workspace, for example: notes/page.html`; err == nil || err.Error() != want {
		t.Errorf("Resolve error = %v\nwant          %s", err, want)
	}
	if err := os.Mkdir(filepath.Join(ws.Dir(), "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = ws.WriteFile("notes", []byte("x"))
	if want := "notes is a folder; give a file name inside it, for example: notes/page.html"; err == nil || err.Error() != want {
		t.Errorf("WriteFile error = %v\nwant            %s", err, want)
	}
}

func TestRenameMovesAFileAndNeverReplacesOne(t *testing.T) {
	ws, _ := newWorkspace(t)
	write := func(rel, content string) {
		t.Helper()
		if err := ws.WriteFile(rel, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "first")
	write("b.md", "second")

	// Parents of the new name are created.
	if err := ws.Rename("a.md", "archive/2026/a.md"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(ws.Dir(), "archive", "2026", "a.md")); err != nil || string(data) != "first" {
		t.Errorf("moved file = %q (%v), want first", data, err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir(), "a.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the old name is still there: %v", err)
	}

	err := ws.Rename("b.md", "archive/2026/a.md")
	if !errors.Is(err, fs.ErrExist) || err.Error() != "archive/2026/a.md already exists" {
		t.Errorf("Rename onto a file error = %v, want an already-exists refusal", err)
	}
	if data, _ := os.ReadFile(filepath.Join(ws.Dir(), "archive", "2026", "a.md")); string(data) != "first" {
		t.Errorf("the existing file was replaced: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(ws.Dir(), "b.md")); string(data) != "second" {
		t.Errorf("the refused file changed: %q", data)
	}

	if err := ws.Rename("missing.md", "c.md"); !errors.Is(err, fs.ErrNotExist) || strings.Contains(err.Error(), ws.Dir()) {
		t.Errorf("Rename of a missing file error = %v, want not-exist without the host path", err)
	}
	if err := ws.Rename("archive", "elsewhere"); !errors.Is(err, ErrIsFolder) {
		t.Errorf("Rename of a folder error = %v, want the folder refusal", err)
	}
	// A file where a parent folder would go.
	if err := ws.Rename("b.md", "archive/2026/a.md/b.md"); err == nil || strings.Contains(err.Error(), ws.Dir()) {
		t.Errorf("Rename below a file error = %v, want a refusal without the host path", err)
	}
}

func TestRenameChangesLetterCaseOnly(t *testing.T) {
	ws, _ := newWorkspace(t)
	if err := ws.WriteFile("garden.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	// On a case-insensitive volume the new name already "exists": it is the
	// same file. On a case-sensitive one it is simply free.
	if err := ws.Rename("garden.md", "Garden.md"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	entries, err := os.ReadDir(ws.Dir())
	if err != nil || len(entries) != 1 || entries[0].Name() != "Garden.md" {
		t.Errorf("folder = %v (%v), want only Garden.md", entries, err)
	}
}

func TestMoveToPutsAFileInAnotherFolder(t *testing.T) {
	ws, outside := newWorkspace(t)
	trash, err := OpenWith(filepath.Join(t.TempDir(), ".svalbard", "state", "wiki-trash"), Options{Place: "wiki trash"})
	if err != nil {
		t.Fatal(err)
	}
	defer trash.Close()
	if err := ws.WriteFile("Projects/Garden.md", []byte("beans")); err != nil {
		t.Fatal(err)
	}

	if err := ws.MoveTo("Projects/Garden.md", trash, "Projects/Garden.1.md"); err != nil {
		t.Fatalf("MoveTo() error = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(trash.Dir(), "Projects", "Garden.1.md")); err != nil || string(data) != "beans" {
		t.Errorf("trashed file = %q (%v), want beans", data, err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir(), "Projects", "Garden.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the file is still in the first folder: %v", err)
	}

	// The name in the other folder must be free.
	if err := ws.WriteFile("Projects/Garden.md", []byte("peas")); err != nil {
		t.Fatal(err)
	}
	if err := ws.MoveTo("Projects/Garden.md", trash, "Projects/Garden.1.md"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("MoveTo onto a file error = %v, want an already-exists refusal", err)
	}
	if data, _ := os.ReadFile(filepath.Join(trash.Dir(), "Projects", "Garden.1.md")); string(data) != "beans" {
		t.Errorf("the trashed file was replaced: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(ws.Dir(), "Projects", "Garden.md")); string(data) != "peas" {
		t.Errorf("the refused file changed: %q", data)
	}

	if err := ws.MoveTo("missing.md", trash, "missing.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("MoveTo of a missing file error = %v, want not-exist", err)
	}
	if err := ws.MoveTo("Projects", trash, "Projects.md"); !errors.Is(err, ErrIsFolder) {
		t.Errorf("MoveTo of a folder error = %v, want the folder refusal", err)
	}
	// A link is not a regular file: what it points at stays where it is.
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(ws.Dir(), "link.md"))
	if err := ws.MoveTo("link.md", trash, "link.md"); err == nil {
		t.Error("MoveTo moved a symlink")
	}
	if data, _ := os.ReadFile(filepath.Join(outside, "secret.txt")); string(data) != "secret" {
		t.Errorf("the file outside changed: %q", data)
	}
}

func TestCopyToMovesAcrossVolumes(t *testing.T) {
	// Two temporary folders are one volume, so the fallback for two volumes
	// is called directly.
	ws, _ := newWorkspace(t)
	other, err := Open(filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := ws.WriteFile("a.md", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := ws.copyTo("a.md", other, "a.md"); err != nil {
		t.Fatalf("copyTo() error = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(other.Dir(), "a.md")); err != nil || string(data) != "first" {
		t.Errorf("copied file = %q (%v), want first", data, err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir(), "a.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the original is still there: %v", err)
	}
}
