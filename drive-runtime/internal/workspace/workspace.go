// Package workspace is the one folder on the drive that the browser front
// end's file tools may read and write: <drive>/workspace. It checks every
// path the model supplies and serves the folder read-only for links.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	// DirName is the folder's name at the root of the drive.
	DirName = "workspace"

	// MaxWriteBytes caps one written file. A page a local model writes is a
	// few kilobytes; the cap stops a runaway generation filling the stick.
	MaxWriteBytes = 2 << 20
)

// Workspace is the opened folder. All access goes through an os.Root, so a
// path that slips past Resolve still cannot leave the folder.
type Workspace struct {
	dir  string // symlinks resolved, so resolved paths can be compared to it
	root *os.Root
}

// Open creates <driveRoot>/workspace if it is missing and opens it.
func Open(driveRoot string) (*Workspace, error) {
	dir := filepath.Join(driveRoot, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Workspace{dir: dir, root: root}, nil
}

func (w *Workspace) Close() error { return w.root.Close() }

// Dir is the folder's path on the host.
func (w *Workspace) Dir() string { return w.dir }

// Resolve checks a path a model supplied and returns it cleaned, slash
// separated and relative to the workspace ("." for the workspace itself).
// The errors are written for the model: each says what to send instead.
func (w *Workspace) Resolve(name string) (string, error) {
	const example = `use a path inside the workspace, for example: notes/page.html`
	switch {
	case strings.ContainsRune(name, 0):
		return "", fmt.Errorf("path contains a NUL byte; %s", example)
	case strings.Contains(name, `\`):
		return "", fmt.Errorf(`path %q uses "\"; separate folders with "/", for example: notes/page.html`, name)
	case strings.HasPrefix(name, "/") || strings.HasPrefix(name, "~") || filepath.IsAbs(name):
		return "", fmt.Errorf("path %q is absolute; %s", name, example)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return "", fmt.Errorf(`path %q contains ".."; %s`, name, example)
		}
	}
	rel := path.Clean(name)
	if rel == "." {
		return rel, nil
	}
	for _, segment := range strings.Split(rel, "/") {
		// Hidden names are the atomic writer's temporary files and the host's
		// own litter (.DS_Store, ._name); they are not listed or served either.
		if strings.HasPrefix(segment, ".") {
			return "", fmt.Errorf("path %q has a name starting with a dot; choose a name without the leading dot", name)
		}
	}
	if err := w.checkLinks(rel); err != nil {
		return "", err
	}
	return rel, nil
}

// checkLinks resolves the deepest part of rel that exists and rejects it when
// a symlink takes it out of the workspace. Missing parts are created later
// below that point, so they cannot be links.
func (w *Workspace) checkLinks(rel string) error {
	outside := fmt.Errorf("path %q leaves the workspace through a link; use a plain path inside the workspace, for example: notes/page.html", rel)
	for current := filepath.Join(w.dir, filepath.FromSlash(rel)); ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		switch {
		case err == nil:
			if resolved != w.dir && !strings.HasPrefix(resolved, w.dir+string(filepath.Separator)) {
				return outside
			}
			return nil
		case errors.Is(err, fs.ErrNotExist):
			// A link whose target is missing would be created through.
			if _, statErr := os.Lstat(current); statErr == nil {
				return outside
			}
		case errors.Is(err, syscall.ENOTDIR):
			// A file where a folder was expected; the operation reports it.
		default:
			return fmt.Errorf("cannot check path %q: %v", rel, pathErr(err))
		}
		if current == w.dir {
			return nil
		}
	}
}

// Stat describes rel, following links that stay inside the workspace.
func (w *Workspace) Stat(rel string) (fs.FileInfo, error) {
	info, err := w.root.Stat(rel)
	if err != nil {
		return nil, describe(rel, err)
	}
	return info, nil
}

// ReadDir lists the folder rel.
func (w *Workspace) ReadDir(rel string) ([]fs.DirEntry, error) {
	dir, err := w.root.Open(rel)
	if err != nil {
		return nil, describe(rel, err)
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, describe(rel, err)
	}
	return entries, nil
}

// ReadFile returns the content of the regular file rel, refusing one larger
// than maxBytes.
func (w *Workspace) ReadFile(rel string, maxBytes int64) ([]byte, error) {
	info, err := w.Stat(rel)
	if err != nil {
		return nil, err
	}
	switch {
	case info.IsDir():
		return nil, fmt.Errorf("%s is a folder, not a file; list it with file_list", rel)
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%s is not a regular file and cannot be read", rel)
	case info.Size() > maxBytes:
		return nil, fmt.Errorf("%s is %d bytes, over the %d byte limit for reading", rel, info.Size(), maxBytes)
	}
	data, err := w.root.ReadFile(rel)
	if err != nil {
		return nil, describe(rel, err)
	}
	return data, nil
}

// WriteFile creates or replaces the file rel, creating its parent folders.
// The content goes to a temporary file that is renamed into place, so a stick
// pulled mid-write leaves the old file or the new one, never half of one.
func (w *Workspace) WriteFile(rel string, data []byte) error {
	if rel == "." {
		return errors.New("path is the workspace folder itself; give a file name, for example: notes/page.html")
	}
	if len(data) > MaxWriteBytes {
		return fmt.Errorf("content is %d bytes, over the %d byte limit; write a shorter file or split it into several", len(data), MaxWriteBytes)
	}
	if info, err := w.root.Lstat(rel); err == nil {
		switch {
		case info.IsDir():
			return fmt.Errorf("%s is a folder; give a file name inside it, for example: %s/page.html", rel, rel)
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s is not a regular file and cannot be replaced; choose another name", rel)
		}
	}
	dir := path.Dir(rel)
	if err := w.root.MkdirAll(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			err = syscall.ENOTDIR
		}
		return describe(dir, err)
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	temp := path.Join(dir, "."+path.Base(rel)+".tmp-"+hex.EncodeToString(suffix))
	file, err := w.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return describe(rel, err)
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = w.root.Rename(temp, rel)
	}
	if err != nil {
		_ = w.root.Remove(temp)
		return describe(rel, err)
	}
	return nil
}

// describe turns a file system error into a sentence without the host path.
func describe(rel string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s does not exist; call file_list to see what is in the workspace", rel)
	case errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("part of %s is a file, not a folder; call file_list to see what is in the workspace", rel)
	default:
		return fmt.Errorf("%s: %v", rel, pathErr(err))
	}
}

// pathErr drops the operation and host path an *fs.PathError carries.
func pathErr(err error) error {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return pathError.Err
	}
	return err
}
