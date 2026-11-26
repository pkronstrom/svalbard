package htmlpack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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
	id    string
	path  string
	zr    *zip.ReadCloser
	files map[string]*zip.File
	dirs  map[string]bool
	// mains maps a package folder to the file a bare import of it resolves
	// to (a self-contained build), from svalbard-js.json.
	mains map[string]string
	// pkgJSON caches parsed package.json files by package folder.
	pkgJSON map[string]*packageJSON
}

func openArchive(id, file string) (*archive, error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	a := &archive{id: id, path: file, zr: zr, files: map[string]*zip.File{}, dirs: map[string]bool{"": true}, mains: map[string]string{}, pkgJSON: map[string]*packageJSON{}}
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
			zr.Close()
			return nil, fmt.Errorf("%s: %s: %w", file, libraryIndexFile, err)
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
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
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
}

// loadLibraries opens every archive under lib/js/ of the drive. A drive
// without any has none; that is not an error until a page imports one.
func loadLibraries(driveRoot string) (*libraries, error) {
	libs := &libraries{byPackage: map[string][]*archive{}, documented: map[string]library{}}
	dir := filepath.Join(driveRoot, filepath.FromSlash(libraryDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return libs, nil
		}
		return nil, err
	}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".zip")
		if !ok || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		a, err := openArchive(id, filepath.Join(dir, e.Name()))
		if err != nil {
			libs.close()
			return nil, fmt.Errorf("library archive %s: %w", e.Name(), err)
		}
		libs.archives = append(libs.archives, a)
		for _, pkg := range a.topLevel() {
			libs.byPackage[pkg] = append(libs.byPackage[pkg], a)
			quickstart := filepath.Join(dir, id, "docs", strings.ReplaceAll(pkg, "/", "__")+".md")
			if _, err := os.Stat(quickstart); err == nil {
				libs.documented[pkg] = library{pkg: pkg, archive: a, portability: frontmatterValue(quickstart, "portability")}
			}
		}
	}
	return libs, nil
}

func (l *libraries) close() {
	for _, a := range l.archives {
		a.zr.Close()
	}
}

// owner returns the archive a page's import of pkg resolves in: the one
// recipe that has it as a library, or the one archive that holds it.
func (l *libraries) owner(pkg string) (*archive, error) {
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
func frontmatterValue(file, key string) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, 64<<10))
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
	if p, ok := a.pkgJSON[dir]; ok {
		return p, nil
	}
	data, err := a.read(path.Join(dir, "package.json"))
	if err != nil {
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
