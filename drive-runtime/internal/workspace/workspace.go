// Package workspace confines a model's file tools to one folder on the drive.
// It checks every path the model supplies and serves the folder read-only for
// links. The browser front end's file tools use it on <drive>/workspace, and
// the wiki tools on the page folder and on its trash.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	// DirName is the name of the drive's workspace folder, at its root.
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
	opts Options
}

// Options is what the errors of one opened folder say about it. They are
// sentences for a model, which acts on the folder's name and the example.
type Options struct {
	// Place names the folder: "workspace".
	Place string
	// Example is a path the folder would accept: "notes/page.html".
	Example string
}

// Open creates the folder dir if it is missing and opens it as the drive's
// workspace, which is filepath.Join(driveRoot, DirName).
func Open(dir string) (*Workspace, error) {
	return OpenWith(dir, Options{})
}

// OpenWith is Open for another confined folder, whose errors name it as opts
// says. An empty field keeps the workspace's wording.
func OpenWith(dir string, opts Options) (*Workspace, error) {
	if opts.Place == "" {
		opts.Place = "workspace"
	}
	if opts.Example == "" {
		opts.Example = "notes/page.html"
	}
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
	return &Workspace{dir: dir, root: root, opts: opts}, nil
}

func (w *Workspace) Close() error { return w.root.Close() }

// Dir is the folder's path on the host.
func (w *Workspace) Dir() string { return w.dir }

// Resolve checks a path a model supplied and returns it cleaned, slash
// separated and relative to the workspace ("." for the workspace itself).
// The errors are written for the model: each says what to send instead.
func (w *Workspace) Resolve(name string) (string, error) {
	example := fmt.Sprintf("use a path inside the %s, for example: %s", w.opts.Place, w.opts.Example)
	switch {
	case strings.ContainsRune(name, 0):
		return "", fmt.Errorf("path contains a NUL byte; %s", example)
	case strings.Contains(name, `\`):
		return "", fmt.Errorf(`path %q uses "\"; separate folders with "/", for example: %s`, name, w.opts.Example)
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
	outside := fmt.Errorf("path %q leaves the %s through a link; use a plain path inside the %s, for example: %s", rel, w.opts.Place, w.opts.Place, w.opts.Example)
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
		return nil, &refusal{fmt.Sprintf("%s is a folder, not a file", rel), ErrIsFolder}
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
		return fmt.Errorf("path is the %s folder itself; give a file name, for example: %s", w.opts.Place, w.opts.Example)
	}
	if len(data) > MaxWriteBytes {
		return fmt.Errorf("content is %d bytes, over the %d byte limit; write a shorter file or split it into several", len(data), MaxWriteBytes)
	}
	if info, err := w.root.Lstat(rel); err == nil {
		switch {
		case info.IsDir():
			return fmt.Errorf("%s is a folder; give a file name inside it, for example: %s", rel, path.Join(rel, path.Base(w.opts.Example)))
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s is not a regular file and cannot be replaced; choose another name", rel)
		}
	}
	dir := path.Dir(rel)
	if err := w.makeParents(rel); err != nil {
		return err
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

// Rename gives the file from the free name to, creating to's parent folders.
// It refuses to replace anything; a name that differs only in letter case is
// the same file on most sticks and may be renamed to.
func (w *Workspace) Rename(from, to string) error {
	info, err := w.movable(from)
	if err != nil {
		return err
	}
	if existing, err := w.root.Lstat(to); err == nil && !os.SameFile(info, existing) {
		return &refusal{to + " already exists", fs.ErrExist}
	}
	if err := w.makeParents(to); err != nil {
		return err
	}
	if err := w.root.Rename(from, to); err != nil {
		return describe(from, err)
	}
	return nil
}

// MoveTo moves the file rel into the folder dest as the free name destRel,
// creating its parent folders there. Both paths must have passed their
// folder's Resolve. On one volume the move is a rename, so the file is never
// in both folders or in neither.
func (w *Workspace) MoveTo(rel string, dest *Workspace, destRel string) error {
	if _, err := w.movable(rel); err != nil {
		return err
	}
	if _, err := dest.root.Lstat(destRel); err == nil {
		return &refusal{destRel + " already exists", fs.ErrExist}
	}
	if err := dest.makeParents(destRel); err != nil {
		return err
	}
	// os.Root has no rename between two roots.
	err := os.Rename(filepath.Join(w.dir, filepath.FromSlash(rel)), filepath.Join(dest.dir, filepath.FromSlash(destRel)))
	if errors.Is(err, syscall.EXDEV) {
		err = w.copyTo(rel, dest, destRel)
	}
	if err != nil {
		return describe(rel, err)
	}
	return nil
}

// movable describes the regular file rel, which Rename and MoveTo may move.
func (w *Workspace) movable(rel string) (fs.FileInfo, error) {
	info, err := w.root.Lstat(rel)
	switch {
	case err != nil:
		return nil, describe(rel, err)
	case info.IsDir():
		return nil, &refusal{fmt.Sprintf("%s is a folder, not a file", rel), ErrIsFolder}
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%s is not a regular file and cannot be moved", rel)
	}
	return info, nil
}

func (w *Workspace) makeParents(rel string) error {
	dir := path.Dir(rel)
	if err := w.root.MkdirAll(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			err = syscall.ENOTDIR
		}
		return describe(dir, err)
	}
	return nil
}

// copyTo is MoveTo across volumes: the copy is complete before the original
// is removed.
func (w *Workspace) copyTo(rel string, dest *Workspace, destRel string) error {
	source, err := w.root.Open(rel)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := dest.root.OpenFile(destRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(target, source)
	if err == nil {
		err = target.Sync()
	}
	if closeErr := target.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = dest.root.Remove(destRel)
		return err
	}
	return w.root.Remove(rel)
}

// ErrIsFolder is the kind of a refusal to read a folder as a file.
var ErrIsFolder = errors.New("is a folder")

// refusal is an error whose text is a sentence for the model. Its kind
// (fs.ErrNotExist, fs.ErrExist, syscall.ENOTDIR or ErrIsFolder) is what
// errors.Is matches, so the caller can add which of its own tools to use next.
type refusal struct {
	text string
	kind error
}

func (r *refusal) Error() string { return r.text }
func (r *refusal) Unwrap() error { return r.kind }

// describe turns a file system error into a sentence without the host path.
func describe(rel string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &refusal{rel + " does not exist", fs.ErrNotExist}
	case errors.Is(err, syscall.ENOTDIR):
		return &refusal{fmt.Sprintf("part of %s is a file, not a folder", rel), syscall.ENOTDIR}
	default:
		return fmt.Errorf("%s: %v", rel, pathErr(err))
	}
}

// pathErr drops the operation and host paths an *fs.PathError or, from a
// rename, an *os.LinkError carries.
func pathErr(err error) error {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return pathError.Err
	}
	var linkError *os.LinkError
	if errors.As(err, &linkError) {
		return linkError.Err
	}
	return err
}
