package frontpage

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"mime"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

type sourcePart struct {
	Label   string `json:"label"`
	Path    string `json:"path"`
	Archive string `json:"-"`
}

// Open only regular, installed source files. Refuse links in every path
// component, including when a library has changed since inventory discovery.
func openSourceFile(root *os.Root, name string) (*os.File, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return nil, fs.ErrInvalid
	}
	parent := root
	defer func() {
		if parent != root {
			parent.Close()
		}
	}()
	components := strings.Split(name, "/")
	for i, part := range components {
		if strings.HasPrefix(part, ".") {
			return nil, fs.ErrInvalid
		}
		info, err := parent.Lstat(part)
		if err != nil {
			return nil, err
		}
		if i == len(components)-1 && info.Mode().IsRegular() {
			return parent.Open(part)
		}
		if !info.IsDir() {
			return nil, fs.ErrInvalid
		}
		next, err := parent.OpenRoot(part)
		if err != nil {
			return nil, err
		}
		if parent != root {
			parent.Close()
		}
		parent = next
	}
	return nil, fs.ErrInvalid
}

func libraryArchive(root *os.Root, filename string) (*os.File, *zip.Reader, error) {
	f, err := openSourceFile(root, filename)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err == nil {
		var zr *zip.Reader
		zr, err = zip.NewReader(f, info.Size())
		if err == nil {
			return f, zr, nil
		}
	}
	f.Close()
	return nil, nil, err
}

func sourceGroups(folder string) []group {
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil
	}
	defer root.Close()
	libraries := group{Label: "Libraries"}
	archives, _ := fs.Glob(root.FS(), "lib/js/*.zip")
	for _, name := range archives {
		f, archive, err := libraryArchive(root, name)
		if err != nil {
			continue
		}
		libraries.Items = append(libraries.Items, libraryEntries(archive, name, root)...)
		f.Close()
	}
	skills := group{Label: "Skills"}
	names, _ := fs.Glob(root.FS(), "skills/*/*/SKILL.md")
	for _, name := range names {
		f, err := openSourceFile(root, name)
		if err != nil {
			continue
		}
		f.Close()
		e := entry{ID: name, Label: path.Base(path.Dir(name)), Kind: "source", Icon: "skill", Note: path.Dir(name), Parts: []sourcePart{{Label: "Procedure", Path: name}}}
		base := path.Dir(name)
		for _, sub := range []string{"references", "scripts"} {
			siblings, _ := fs.Glob(root.FS(), base+"/"+sub+"/*")
			for _, file := range siblings {
				switch strings.ToLower(path.Ext(file)) {
				case ".md", ".txt", ".py", ".sh", ".js", ".json", ".yaml", ".yml":
					if f, err := openSourceFile(root, file); err == nil {
						f.Close()
						e.Parts = append(e.Parts, sourcePart{Label: strings.TrimPrefix(file, base+"/"), Path: file})
					}
				}
			}
		}
		skills.Items = append(skills.Items, e)
	}
	var groups []group
	for _, g := range []group{libraries, skills} {
		if len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

func libraryEntries(archive *zip.Reader, filename string, root *os.Root) []entry {
	idx, err := archive.Open("svalbard-js.json")
	if err != nil {
		return nil
	}
	defer idx.Close()
	var index struct{ Main map[string]string }
	if json.NewDecoder(io.LimitReader(idx, 1<<20)).Decode(&index) != nil {
		return nil
	}
	var items []entry
	for _, pkg := range slices.Sorted(maps.Keys(index.Main)) {
		module, ok := strings.CutPrefix(pkg, "node_modules/")
		main := index.Main[pkg]
		if !ok || !fs.ValidPath(module) || !fs.ValidPath(main) {
			continue
		}
		source := path.Join(pkg, main)
		if _, err := fs.Stat(archive, source); err != nil {
			continue
		}
		e := entry{ID: filename + "/" + module, Label: module, Kind: "source", Icon: "library", Note: path.Base(filename), Download: filename}
		doc := strings.TrimSuffix(filename, ".zip") + "/docs/" + strings.ReplaceAll(module, "/", "__")
		for _, view := range []sourcePart{{Label: "Overview", Path: doc + ".md"}, {Label: "API", Path: doc + ".reference.md"}} {
			f, err := openSourceFile(root, view.Path)
			if err == nil {
				f.Close()
				e.Parts = append(e.Parts, view)
			}
		}
		e.Parts = append(e.Parts, sourcePart{Label: "Source", Path: source, Archive: filename})
		manifest := path.Join(pkg, "package.json")
		if f, err := archive.Open(manifest); err == nil {
			var meta struct {
				Version     string
				License     string
				Description string
			}
			if json.NewDecoder(io.LimitReader(f, 256<<10)).Decode(&meta) == nil {
				e.Description = meta.Description
				e.Subheader = strings.Trim(strings.Join([]string{meta.Version, meta.License}, " · "), " ·")
			}
			f.Close()
		}
		for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
			file := path.Join(pkg, name)
			if _, err := fs.Stat(archive, file); err == nil {
				e.Parts = append(e.Parts, sourcePart{Label: "License", Path: file, Archive: filename})
				break
			}
		}
		if _, err := fs.Stat(archive, manifest); err == nil {
			e.Parts = append(e.Parts, sourcePart{Label: "Manifest", Path: manifest, Archive: filename})
		}
		// Include real package examples when the installed archive ships them.
		for _, file := range archive.File {
			rel, ok := strings.CutPrefix(file.Name, pkg+"/examples/")
			if !ok || !fs.ValidPath(rel) || len(e.Parts) >= 64 {
				continue
			}
			switch strings.ToLower(path.Ext(rel)) {
			case ".js", ".html", ".css", ".md", ".json":
				e.Parts = append(e.Parts, sourcePart{Label: "Examples/" + rel, Path: file.Name, Archive: filename})
			}
		}
		items = append(items, e)
	}
	return items
}

func (d *dashboard) serveSource(w http.ResponseWriter, r *http.Request) {
	id, part := r.URL.Query().Get("id"), r.URL.Query().Get("view")
	for _, g := range d.inv.Groups {
		for _, e := range g.Items {
			if e.ID != id {
				continue
			}
			for _, p := range e.Parts {
				if p.Label != part {
					continue
				}
				body, truncated, err := readSource(d.root, p)
				if err != nil {
					http.Error(w, "This source is unavailable.", http.StatusNotFound)
					return
				}
				writeJSON(w, struct {
					Text      string `json:"text"`
					Truncated bool   `json:"truncated"`
				}{body, truncated})
				return
			}
		}
	}
	http.NotFound(w, r)
}

func readSource(folder string, p sourcePart) (string, bool, error) {
	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", false, err
	}
	defer root.Close()
	var src io.ReadCloser
	if p.Archive != "" {
		f, zr, err := libraryArchive(root, p.Archive)
		if err != nil {
			return "", false, err
		}
		defer f.Close()
		src, err = zr.Open(p.Path)
		if err != nil {
			return "", false, err
		}
	} else {
		src, err = openSourceFile(root, p.Path)
		if err != nil {
			return "", false, err
		}
	}
	defer src.Close()
	const limit = 256 << 10
	data, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return "", false, err
	}
	truncated := len(data) > limit
	if truncated {
		data = data[:limit]
		for i := 0; i < utf8.UTFMax && len(data) > 0 && !utf8.Valid(data); i++ {
			data = data[:len(data)-1]
		}
	}
	if !utf8.Valid(data) {
		return "", false, errors.New("not text")
	}
	return string(data), truncated, nil
}

func (d *dashboard) download(w http.ResponseWriter, r *http.Request) {
	for _, g := range d.inv.Groups {
		for _, entry := range g.Items {
			if entry.ID != r.URL.Query().Get("id") || entry.Download == "" {
				continue
			}
			root, err := os.OpenRoot(d.root)
			if err != nil {
				break
			}
			defer root.Close()
			f, err := openSourceFile(root, entry.Download)
			if err != nil {
				break
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil {
				break
			}
			name := path.Base(entry.Download)
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
			w.Header().Set("Content-Type", "application/octet-stream")
			http.ServeContent(w, r, name, info.ModTime(), f)
			return
		}
	}
	http.NotFound(w, r)
}
