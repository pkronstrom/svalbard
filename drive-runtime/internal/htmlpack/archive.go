package htmlpack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

// The drive's JS libraries: one archive per js-lib recipe, lib/js/<id>.zip,
// holding a node_modules tree and svalbard-js.json, read in place. Each
// library's quickstart, lib/js/<id>/docs/<name>.md, says which recipe owns
// the package and how portable it measured. Keep the layout in sync with
// host-cli/internal/apply (writeLibraryArchive) and toolkit (libdocs.go).

const (
	libraryDir       = "lib/js"
	libraryIndexFile = "svalbard-js.json"
)

// archive is one recipe's library archive.
type archive struct {
	id     string
	reader io.Closer
	limits *workspaceLimits
	files  map[string]*zip.File
	dirs   map[string]bool
	// mains maps a package folder to the file a bare import of it resolves
	// to (a self-contained build), from svalbard-js.json.
	mains map[string]string
	// pkgJSON caches parsed package.json files by package folder; mu guards
	// it, since esbuild resolves from several goroutines.
	pkgJSON map[string]*packageJSON
	mu      sync.Mutex
}

func openArchive(id string, files fs.FS, name string, limits *workspaceLimits) (*archive, error) {
	if limits != nil {
		info, err := fs.Stat(files, name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular library archive", name)
		}
	}
	f, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	a, err := readArchive(id, f, limits)
	if err != nil {
		f.Close()
	}
	return a, err
}

func readArchive(id string, f fs.File, limits *workspaceLimits) (*archive, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular library archive", info.Name())
	}
	reader, ok := f.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf("library archive %s does not support reading at an offset", id)
	}
	if limits != nil {
		reader = workspaceZIPReader{source: reader, limits: limits}
	}
	zr, err := zip.NewReader(reader, info.Size())
	if err != nil {
		return nil, err
	}
	if limits != nil && len(zr.File) > maxZIPEntries {
		return nil, fmt.Errorf("%s.zip has more than %d entries", id, maxZIPEntries)
	}
	a := &archive{id: id, reader: f, limits: limits, files: map[string]*zip.File{}, dirs: map[string]bool{"": true}, mains: map[string]string{}, pkgJSON: map[string]*packageJSON{}}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimPrefix(path.Clean("/"+f.Name), "/")
		a.files[name] = f
		for dir := path.Dir(name); dir != "." && !a.dirs[dir]; dir = path.Dir(dir) {
			a.dirs[dir] = true
		}
	}
	if data, err := a.read(libraryIndexFile); err == nil {
		var index struct {
			Main map[string]string `json:"main"`
		}
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("%s: %w", libraryIndexFile, err)
		}
		for dir, main := range index.Main {
			a.mains[path.Clean(dir)] = path.Clean(main)
		}
	}
	return a, nil
}

func (a *archive) isFile(p string) bool { _, ok := a.files[p]; return ok }
func (a *archive) isDir(p string) bool  { return a.dirs[p] }

func (a *archive) read(p string) ([]byte, error) {
	f, ok := a.files[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	if a.limits == nil {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return a.limits.read(func(maxBytes int64) ([]byte, error) {
		if f.UncompressedSize64 > uint64(maxBytes) {
			return nil, fmt.Errorf("%s: %s exceeds the %d byte limit for an archived dependency", a.id, p, maxBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(packReader{ctx: a.limits.ctx, reader: rc}, maxBytes+1))
		if err == nil && int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("%s: %s exceeds the %d byte limit for an archived dependency", a.id, p, maxBytes)
		}
		return data, err
	})
}

// topLevel lists the packages installed at the top of the tree.
func (a *archive) topLevel() []string {
	var pkgs []string
	for dir := range a.dirs {
		rest, ok := strings.CutPrefix(dir, "node_modules/")
		if !ok || strings.Contains(rest, "/node_modules/") {
			continue
		}
		parts := strings.Split(rest, "/")
		switch {
		case len(parts) == 1 && !strings.HasPrefix(parts[0], "@"):
			pkgs = append(pkgs, parts[0])
		case len(parts) == 2 && strings.HasPrefix(parts[0], "@"):
			pkgs = append(pkgs, rest)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// library is a package a recipe's quickstart documents.
type library struct {
	pkg         string
	archive     *archive
	portability string
}

// libraries is what the drive has installed.
type libraries struct {
	archives []*archive
	// byPackage lists, per top-level package, the archives holding it.
	byPackage map[string][]*archive
	// documented are the packages with a quickstart, by package.
	documented map[string]library
	// twice are packages two recipes document; a page cannot import them.
	twice map[string][]string
	// broken are the archives that could not be read, by recipe; a page
	// fails only when it needs one.
	broken   map[string]error
	warnings []string
}

// loadLibraries opens every archive under lib/js/ of the drive. A drive
// without any has none; that is not an error until a page imports one.
func loadLibraries(driveRoot string, limits *workspaceLimits) (*libraries, error) {
	libs := &libraries{byPackage: map[string][]*archive{}, documented: map[string]library{}, twice: map[string][]string{}, broken: map[string]error{}}
	var files fs.FS = os.DirFS(filepath.Join(driveRoot, libraryDir))
	if limits != nil {
		root, err := os.OpenRoot(driveRoot)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		store, err := root.OpenRoot(libraryDir)
		if errors.Is(err, fs.ErrNotExist) {
			return libs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("open the installed library store: %w", err)
		}
		defer store.Close()
		files = store.FS()
	}
	entries, err := libraryEntries(files, limits)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return libs, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if limits != nil {
			if err := limits.failure(); err != nil {
				libs.close()
				return nil, err
			}
		}
		id, ok := strings.CutSuffix(e.Name(), ".zip")
		if !ok || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		a, err := openArchive(id, files, e.Name(), limits)
		if err != nil {
			libs.broken[id] = err
			libs.warnings = append(libs.warnings, fmt.Sprintf("lib/js/%s cannot be read (%v); its libraries are left out", e.Name(), err))
			continue
		}
		libs.archives = append(libs.archives, a)
		for _, pkg := range a.topLevel() {
			libs.byPackage[pkg] = append(libs.byPackage[pkg], a)
			quickstart := path.Join(id, "docs", strings.ReplaceAll(pkg, "/", "__")+".md")
			if _, err := fs.Stat(files, quickstart); err == nil {
				if other, ok := libs.documented[pkg]; ok {
					libs.twice[pkg] = append(libs.twice[pkg], other.archive.id, id)
					continue
				}
				libs.documented[pkg] = library{pkg: pkg, archive: a, portability: frontmatterValue(files, quickstart, "portability", limits)}
			}
		}
	}
	if limits != nil {
		if err := limits.failure(); err != nil {
			libs.close()
			return nil, err
		}
	}
	return libs, nil
}

func libraryEntries(files fs.FS, limits *workspaceLimits) ([]fs.DirEntry, error) {
	if limits == nil {
		return fs.ReadDir(files, ".")
	}
	f, err := files.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dir, ok := f.(fs.ReadDirFile)
	if !ok {
		return nil, fmt.Errorf("%s is not a library folder", libraryDir)
	}
	entries, err := dir.ReadDir(maxLibraryDirEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > maxLibraryDirEntries {
		return nil, fmt.Errorf("the %s store has more than %d entries", libraryDir, maxLibraryDirEntries)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func (l *libraries) close() {
	for _, a := range l.archives {
		a.reader.Close()
	}
}

// owner returns the archive a page's import of pkg resolves in: the one
// recipe that has it as a library, or the one archive that holds it.
func (l *libraries) owner(pkg string) (*archive, error) {
	if ids, ok := l.twice[pkg]; ok {
		return nil, fmt.Errorf("%q is a library of two recipes on this drive (%s); remove one of them", pkg, strings.Join(ids, " and "))
	}
	if lib, ok := l.documented[pkg]; ok {
		return lib.archive, nil
	}
	holders := l.byPackage[pkg]
	switch len(holders) {
	case 1:
		return holders[0], nil
	case 0:
		var names []string
		for name := range l.documented {
			names = append(names, name)
		}
		sort.Strings(names)
		var broken []string
		for id := range l.broken {
			broken = append(broken, "lib/js/"+id+".zip")
		}
		sort.Strings(broken)
		if len(broken) > 0 {
			return nil, fmt.Errorf("%q is not in a readable library archive; %s cannot be read", pkg, strings.Join(broken, ", "))
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("%q is not on this drive, which has no JS libraries installed", pkg)
		}
		return nil, fmt.Errorf("%q is not a library on this drive. The drive has: %s", pkg, strings.Join(names, ", "))
	default:
		ids := make([]string, len(holders))
		for i, a := range holders {
			ids[i] = a.id
		}
		return nil, fmt.Errorf("%q is installed by %s; import a library instead", pkg, strings.Join(ids, " and "))
	}
}

// frontmatterValue reads one top-level key of a markdown file's frontmatter.
func frontmatterValue(files fs.FS, file, key string, limits *workspaceLimits) string {
	if limits != nil {
		info, err := fs.Stat(files, file)
		if err != nil || !info.Mode().IsRegular() {
			return ""
		}
		data, err := limits.read(func(maxBytes int64) ([]byte, error) {
			f, err := files.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return io.ReadAll(io.LimitReader(packReader{ctx: limits.ctx, reader: f}, min(64<<10, maxBytes)))
		})
		if err != nil {
			return ""
		}
		return scanFrontmatter(bytes.NewReader(data), key)
	}
	f, err := files.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	return scanFrontmatter(io.LimitReader(f, 64<<10), key)
}

func scanFrontmatter(r io.Reader, key string) string {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return ""
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == key {
			v = strings.TrimSpace(v)
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			return v
		}
	}
	return ""
}

// packageJSON is the part of a package.json that resolution reads.
type packageJSON struct {
	Name    string
	Version string
	License string
	Main    string
	Module  string
	// Browser is the browser field: a replacement entry (string) or a map
	// of replacements, where false means an empty module.
	BrowserEntry string
	BrowserMap   map[string]any
	Exports      *orderedValue
}

func (a *archive) packageJSON(dir string) (*packageJSON, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.pkgJSON[dir]; ok {
		return p, nil
	}
	data, err := a.read(path.Join(dir, "package.json"))
	if err != nil {
		if a.limits != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		a.pkgJSON[dir] = nil
		return nil, nil // a package without package.json resolves by file
	}
	var raw struct {
		Name    string          `json:"name"`
		Version string          `json:"version"`
		License any             `json:"license"`
		Main    string          `json:"main"`
		Module  string          `json:"module"`
		Browser json.RawMessage `json:"browser"`
		Exports json.RawMessage `json:"exports"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s/package.json: %w", dir, err)
	}
	p := &packageJSON{Name: raw.Name, Version: raw.Version, Main: raw.Main, Module: raw.Module}
	switch v := raw.License.(type) {
	case string:
		p.License = v
	case map[string]any:
		p.License, _ = v["type"].(string)
	}
	if len(raw.Browser) > 0 {
		var entry string
		if json.Unmarshal(raw.Browser, &entry) == nil {
			p.BrowserEntry = entry
		} else {
			_ = json.Unmarshal(raw.Browser, &p.BrowserMap)
		}
	}
	if len(raw.Exports) > 0 && !bytes.Equal(bytes.TrimSpace(raw.Exports), []byte("null")) {
		v, err := parseOrdered(raw.Exports)
		if err != nil {
			return nil, fmt.Errorf("%s/package.json exports: %w", dir, err)
		}
		p.Exports = v
	}
	a.pkgJSON[dir] = p
	return p, nil
}

// orderedValue is a JSON value whose objects keep their key order, which
// package.json exports conditions depend on.
type orderedValue struct {
	str    *string
	null   bool
	keys   []string
	values map[string]*orderedValue
	array  []*orderedValue
}

func parseOrdered(data []byte) (*orderedValue, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	return decodeOrdered(dec)
}

func decodeOrdered(dec *json.Decoder) (*orderedValue, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			v := &orderedValue{values: map[string]*orderedValue{}}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := keyTok.(string)
				child, err := decodeOrdered(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := v.values[key]; !dup {
					v.keys = append(v.keys, key)
				}
				v.values[key] = child
			}
			_, err := dec.Token()
			return v, err
		case '[':
			v := &orderedValue{array: []*orderedValue{}}
			for dec.More() {
				child, err := decodeOrdered(dec)
				if err != nil {
					return nil, err
				}
				v.array = append(v.array, child)
			}
			_, err := dec.Token()
			return v, err
		}
	case string:
		return &orderedValue{str: &t}, nil
	case nil:
		return &orderedValue{null: true}, nil
	}
	// Numbers and booleans mean nothing in exports.
	return &orderedValue{null: true}, nil
}
