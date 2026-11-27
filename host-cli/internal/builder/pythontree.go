package builder

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// treeLinkCopyLimit is the largest file a symlink in an installed tree is
// replaced by a copy of. The interpreter's python and python3 point at a 20 MB
// binary, so they become scripts instead.
const treeLinkCopyLimit = 1 << 20

// installTree installs a Python tree (an interpreter or a package's --target
// dir) into dest on the vault so that it works from any vault filesystem:
//  1. the tools container runs commands with {dir} replaced by a directory of
//     its own, never the vault, and archives that directory to the build dir
//  2. rewriteTreeArchive turns the archive into plain files and directories
//  3. the container unpacks it into dest.partial, which then replaces dest
//
// The container writes the vault, not this computer, and creates each file
// with its mode instead of changing the mode afterwards: on exFAT, macOS keeps
// an extended attribute as a ._ file, one more cluster per file, and both a
// mode change through Docker Desktop and a file this computer writes (macOS's
// provenance attribute) set one.
func installTree(ctx context.Context, root, workdir string, env []string, commands [][]string, skip func(string) bool, dest string) error {
	raw, err := createTempPath(workdir, "tree-*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(raw)
	clean, err := createTempPath(workdir, "tree-*-clean.tar")
	if err != nil {
		return err
	}
	defer os.Remove(clean)

	owner := containerOwner()
	spec, err := json.Marshal(map[string]any{"env": env, "commands": commands, "owner": owner})
	if err != nil {
		return err
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "python3", []string{"-c", containerInstallScript, string(spec), raw}); err != nil {
		return err
	}
	dropped, err := rewriteTreeArchive(raw, clean, skip)
	if err != nil {
		return err
	}
	for _, link := range dropped {
		slog.Info("python tree: link left out", "tree", dest, "link", link)
	}
	staging := dest + ".partial"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	unpack := []string{"-c", containerUnpackScript, clean, staging}
	if owner != nil {
		unpack = append(unpack, fmt.Sprintf("%d:%d", owner[0], owner[1]))
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "python3", unpack); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(staging, dest)
}

// containerOwner is the user and group the containers give what they write,
// nil to leave it to Docker. Docker Desktop maps a container's files to this
// computer's user; Docker on Linux would leave them owned by root, in the
// user's own cache dir and on the vault.
var containerOwner = func() []int {
	if runtime.GOOS != "linux" {
		return nil
	}
	return []int{os.Getuid(), os.Getgid()}
}

// containerInstallScript runs a JSON list of commands with {dir} replaced by a
// new directory inside the container and archives that directory. A failed
// command exits with its status and no traceback, so its own error ends the
// output the build reports. With an owner, everything in the archive's
// directory (the build dir with uv's cache) is handed to it.
const containerInstallScript = `import json, os, subprocess, sys, tarfile, tempfile
spec, archive = json.loads(sys.argv[1]), sys.argv[2]
d = tempfile.mkdtemp()
env = dict(os.environ, **dict(e.split("=", 1) for e in spec["env"] or []))
for command in spec["commands"]:
    code = subprocess.run([a.replace("{dir}", d) for a in command], env=env).returncode
    if code:
        sys.exit(code)
with tarfile.open(archive, "w") as tar:
    tar.add(d, arcname=".")
if spec.get("owner"):
    uid, gid = spec["owner"]
    top = os.path.dirname(archive)
    for base, dirs, files in os.walk(top):
        for name in [base] + [os.path.join(base, n) for n in dirs + files]:
            try:
                os.lchown(name, uid, gid)
            except OSError:
                pass
`

// containerUnpackScript unpacks a rewritten archive (directories and regular
// files only) into a new directory, creating each file with its mode, and
// hands what it creates to an optional uid:gid.
const containerUnpackScript = `import os, shutil, sys, tarfile
archive, dest = sys.argv[1], sys.argv[2]
owner = [int(n) for n in sys.argv[3].split(":")] if len(sys.argv) > 3 else None
def own(path):
    if owner:
        try:
            os.lchown(path, *owner)
        except OSError:
            pass
os.makedirs(dest)
own(dest)
with tarfile.open(archive) as tar:
    for member in tar:
        path = os.path.join(dest, member.name)
        if member.isdir():
            os.makedirs(path, exist_ok=True)
        elif member.isfile():
            os.makedirs(os.path.dirname(path), exist_ok=True)
            try:
                fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, member.mode & 0o777)
            except FileExistsError:
                sys.exit(member.name + " already exists: this volume holds two names of the tree as one (case or Unicode normalization)")
            with os.fdopen(fd, "wb") as out, tar.extractfile(member) as data:
                shutil.copyfileobj(data, out, 1 << 20)
        else:
            sys.exit("unexpected archive entry " + member.name)
        own(path)
`

// installInterpreter installs the python-build-standalone build request into
// dest (runtime/python/<platform>/python).
func installInterpreter(ctx context.Context, root, workdir, request, dest string) error {
	commands := [][]string{{"uv", "python", "install", "--no-bin", "--install-dir", "{dir}", request}}
	return installTree(ctx, root, workdir, nil, commands, skipInterpreterEntry, dest)
}

func createTempPath(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	return f.Name(), f.Close()
}

// rewriteTreeArchive copies the archive of an installed tree into one that
// any vault filesystem holds the same way. exFAT has no symlinks (macOS stores
// them as small files that Linux reads as text), and exFAT and default APFS
// fold case:
//   - entries skip reports are left out
//   - a link to a file becomes a copy of it, unless the file is large: then a
//     link in bin/ becomes a script that execs the file (python, python3) and
//     any other is left out (libpython3.12.so, which only a linker reads)
//   - a link to a directory, out of the tree or to a path the tree lacks
//     (such as one through a linked directory) is left out
//   - names that one of those volumes would still hold as one fail: names
//     that differ only in case (full Unicode folding) or normalization
//
// It returns each link it left out, with the reason.
func rewriteTreeArchive(src, dst string, skip func(string) bool) ([]string, error) {
	type entry struct {
		header *tar.Header
		target string // links: the archive path the link resolves from, "" if it leaves the tree
	}
	var order []string
	entries := map[string]entry{}
	err := walkTar(src, func(name string, header *tar.Header, _ io.Reader) error {
		if name == "." || (skip != nil && skip(name)) {
			return nil
		}
		e := entry{header: header}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		case tar.TypeSymlink:
			if !path.IsAbs(header.Linkname) {
				e.target = path.Join(path.Dir(name), header.Linkname)
			}
		case tar.TypeLink:
			e.target = cleanArchiveName(header.Linkname)
		default:
			return nil
		}
		if e.target != "" && !filepath.IsLocal(e.target) {
			e.target = ""
		}
		order = append(order, name)
		entries[name] = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Each link resolves to a regular file or is left out.
	resolved := map[string]string{}
	copied := map[string][]byte{}
	var dropped []string
	for _, name := range order {
		e := entries[name]
		if e.header.Typeflag != tar.TypeSymlink && e.header.Typeflag != tar.TypeLink {
			continue
		}
		target := e.target
		for hops := 0; hops < 16; hops++ {
			next, ok := entries[target]
			if !ok || next.header.Typeflag == tar.TypeReg || next.header.Typeflag == tar.TypeDir {
				break
			}
			target = next.target
		}
		t, ok := entries[target]
		switch {
		case e.target == "":
			dropped = append(dropped, name+" (leads out of the tree)")
			continue
		case !ok:
			dropped = append(dropped, name+" (its target is not a file in the tree)")
			continue
		case t.header.Typeflag != tar.TypeReg:
			dropped = append(dropped, name+" (links a directory)")
			continue
		}
		size := t.header.Size
		if size > treeLinkCopyLimit && path.Base(path.Dir(name)) != "bin" {
			dropped = append(dropped, fmt.Sprintf("%s (links a %d-byte file outside bin/)", name, size))
			continue
		}
		resolved[name] = target
		if size <= treeLinkCopyLimit {
			copied[target] = nil
		}
	}

	folded := map[string]string{}
	fold := cases.Fold()
	for _, name := range order {
		e := entries[name]
		if (e.header.Typeflag == tar.TypeSymlink || e.header.Typeflag == tar.TypeLink) && resolved[name] == "" {
			continue
		}
		key := norm.NFD.String(fold.String(name))
		if other, ok := folded[key]; ok {
			return nil, fmt.Errorf("installed Python tree has names that differ only in case or Unicode normalization, which exFAT and macOS volumes hold as one: %s, %s", other, name)
		}
		folded[key] = name
	}

	out, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(out)
	header := func(name string, typ byte, mode, size int64, from *tar.Header) *tar.Header {
		if typ == tar.TypeDir {
			name += "/"
		}
		return &tar.Header{Name: name, Typeflag: typ, Mode: mode, Size: size, ModTime: from.ModTime.Truncate(1e9), Format: tar.FormatGNU}
	}
	err = walkTar(src, func(name string, h *tar.Header, r io.Reader) error {
		if _, ok := entries[name]; !ok || (h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir) {
			return nil
		}
		if err := tw.WriteHeader(header(name, h.Typeflag, h.Mode&0o7777, h.Size, h)); err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir {
			return nil
		}
		if _, wanted := copied[name]; wanted {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			copied[name] = data
			_, err = tw.Write(data)
			return err
		}
		_, err := io.Copy(tw, r)
		return err
	})
	if err == nil {
		links := make([]string, 0, len(resolved))
		for name := range resolved {
			links = append(links, name)
		}
		sort.Strings(links)
		for _, name := range links {
			target := entries[resolved[name]].header
			data, ok := copied[resolved[name]]
			mode := target.Mode & 0o7777
			if !ok {
				rel, relErr := filepath.Rel(path.Dir(name), resolved[name])
				if relErr != nil {
					err = relErr
					break
				}
				// Called through a link of its own, the script finds its
				// directory as the interpreter's scripts do; realpath is
				// missing on macOS before 13.
				data = []byte(fmt.Sprintf("#!/bin/sh\nself=$(realpath -- \"$0\" 2>/dev/null) || self=$0\nexec \"$(dirname -- \"$self\")/%s\" \"$@\"\n", filepath.ToSlash(rel)))
				mode = 0o755
			}
			if err = tw.WriteHeader(header(name, tar.TypeReg, mode, int64(len(data)), entries[name].header)); err != nil {
				break
			}
			if _, err = tw.Write(data); err != nil {
				break
			}
		}
	}
	if closeErr := tw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return dropped, err
}

// skipInterpreterEntry reports whether an archive path of a uv python install
// dir is left out of the vault:
//   - uv's own files (.lock, .temp); its minor-version alias link leaves the
//     tree and is left out anyway
//   - share/terminfo: its names differ only in case, and the interpreter's
//     ncurses never reads it (it searches $TERMINFO, ~/.terminfo,
//     $TERMINFO_DIRS and /etc/terminfo:/lib/terminfo:/usr/share/terminfo)
func skipInterpreterEntry(name string) bool {
	return strings.HasPrefix(name, ".") || strings.Contains("/"+name+"/", "/share/terminfo/")
}

func cleanArchiveName(name string) string {
	return path.Clean(strings.TrimPrefix(name, "./"))
}

// walkTar calls fn for each entry of a tar file with its cleaned name.
func walkTar(file string, fn func(name string, header *tar.Header, r io.Reader) error) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := cleanArchiveName(header.Name)
		if name != "." && !filepath.IsLocal(name) {
			return fmt.Errorf("archive entry %q leaves its directory", header.Name)
		}
		if err := fn(name, header, tr); err != nil {
			return err
		}
	}
}
