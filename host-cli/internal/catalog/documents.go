package catalog

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// DocumentSource is one entry of a collection recipe's documents list: a file
// behind a URL, a local file, or a local folder whose documents are all taken.
type DocumentSource struct {
	Title string `yaml:"title,omitempty"`
	URL   string `yaml:"url,omitempty"`
	// Path is a local file or folder. LoadUserRecipes makes it absolute.
	Path string `yaml:"path,omitempty"`
	// Filename overrides the name the document is saved under.
	Filename string `yaml:"filename,omitempty"`
	SHA256   string `yaml:"sha256,omitempty"`
}

// Document is one file a collection puts into the vault.
type Document struct {
	// RelPath is the slash-separated path below <type dir>/<collection id>/.
	RelPath string
	Title   string
	URL     string // set when the file is downloaded
	Source  string // absolute local path, set when the file is copied
	SHA256  string // optional pin
	Size    int64  // size of Source; 0 for a download
}

// Origin names where the document comes from, for messages.
func (d Document) Origin() string {
	if d.URL != "" {
		return d.URL
	}
	return d.Source
}

// folderDocumentExtensions are the files a folder entry takes. Anything else
// in the folder stays on the computer.
var folderDocumentExtensions = map[string]bool{".pdf": true}

// IsCollection reports whether the recipe is a document collection.
func (i Item) IsCollection() bool { return len(i.Documents) > 0 }

// DocumentSet is what a collection asks for, as far as this computer can
// tell.
type DocumentSet struct {
	// Documents are the files the collection should hold, in definition
	// order, with folder entries expanded to the documents below them.
	Documents []Document
	// Unreadable are the local sources that cannot be read here. Nothing is
	// known about what they hold now, so whatever the vault has from them is
	// kept as it is.
	Unreadable []UnreadableSource
}

// UnreadableSource is a path: entry whose file or folder cannot be read on
// this computer: it is missing, an unplugged disk, an empty folder, or the
// definition is a vault's copy written elsewhere.
type UnreadableSource struct {
	Path string // the local path, as resolved (as written, for a vault copy)
	// Name is the first path segment its documents have on the vault: the
	// file's name, or the folder's with its documents below it.
	Name   string
	Reason string
	// Foreign is set when the definition is a vault's copy. Its local
	// sources belong to the computer that wrote it and are never read here.
	Foreign bool
}

// Holds returns the unreadable source that a document path below the
// collection directory came from, if any.
func (s DocumentSet) Holds(rel string) (UnreadableSource, bool) {
	for _, source := range s.Unreadable {
		if rel == source.Name || strings.HasPrefix(rel, source.Name+"/") {
			return source, true
		}
	}
	return UnreadableSource{}, false
}

// ExpectedDocuments works out the files a collection should hold. It reads
// the local disk but never the network. A local source that cannot be read,
// or a folder with no documents in it, is returned as unreadable instead of
// shrinking the set: a missing or not-yet-synced folder must never be
// mistaken for one whose documents were deleted. An error means the
// definition itself cannot be used.
func ExpectedDocuments(item Item) (DocumentSet, error) {
	var set DocumentSet
	unreadable := func(src DocumentSource, name, reason string) {
		set.Unreadable = append(set.Unreadable, UnreadableSource{Path: src.Path, Name: name, Reason: reason, Foreign: item.VaultCopy})
	}
	for _, src := range item.Documents {
		if src.URL != "" {
			name, err := documentURLName(src)
			if err != nil {
				return DocumentSet{}, err
			}
			set.Documents = append(set.Documents, Document{RelPath: name, Title: src.Title, URL: src.URL, SHA256: strings.ToLower(src.SHA256)})
			continue
		}
		name := src.Filename
		if name == "" {
			name = filepath.Base(filepath.Clean(src.Path))
		}
		if item.VaultCopy {
			unreadable(src, name, "defined on another computer")
			continue
		}
		info, err := os.Stat(src.Path)
		if err != nil {
			unreadable(src, name, errUnwrapPath(err).Error())
			continue
		}
		if !info.IsDir() {
			set.Documents = append(set.Documents, Document{RelPath: name, Title: src.Title, Source: src.Path, SHA256: strings.ToLower(src.SHA256), Size: info.Size()})
			continue
		}
		if src.Title != "" || src.Filename != "" || src.SHA256 != "" {
			return DocumentSet{}, fmt.Errorf("%s is a folder: title, filename and sha256 apply to a single file", src.Path)
		}
		found, err := folderDocuments(src.Path)
		switch {
		case err != nil:
			unreadable(src, name, err.Error())
		case len(found) == 0:
			unreadable(src, name, "no documents found in it")
		}
		set.Documents = append(set.Documents, found...)
	}
	// Sticks are usually exFAT, where names differing only in case collide.
	seen := make(map[string]string, len(set.Documents))
	folders := make(map[string]string)
	for _, doc := range set.Documents {
		key := strings.ToLower(doc.RelPath)
		if other, dup := seen[key]; dup {
			return DocumentSet{}, fmt.Errorf("%s and %s would both be saved as %s; set filename: on one of them", other, doc.Origin(), doc.RelPath)
		}
		seen[key] = doc.Origin()
		for dir := path.Dir(key); dir != "."; dir = path.Dir(dir) {
			folders[dir] = doc.Origin()
		}
	}
	for _, doc := range set.Documents {
		if other, clash := folders[strings.ToLower(doc.RelPath)]; clash {
			return DocumentSet{}, fmt.Errorf("%s would be saved as %s, which is also the folder of %s; set filename:", doc.Origin(), doc.RelPath, other)
		}
	}
	return set, nil
}

// folderDocuments returns the documents below dir, sorted, with paths that
// start at the folder's own name so its subfolders are kept. Hidden entries
// (including macOS "._" sidecars) are skipped.
func folderDocuments(dir string) ([]Document, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, errUnwrapPath(err)
	}
	top := filepath.Base(filepath.Clean(dir))
	var docs []Document
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() || !folderDocumentExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		docs = append(docs, Document{RelPath: path.Join(top, filepath.ToSlash(rel)), Source: p, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, errUnwrapPath(err)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
	return docs, nil
}

// documentURLName is the name a URL document is saved under: filename: when
// set, otherwise the last segment of the URL path.
func documentURLName(src DocumentSource) (string, error) {
	if src.Filename != "" {
		return src.Filename, nil
	}
	u, err := url.Parse(src.URL)
	if err != nil {
		return "", fmt.Errorf("invalid url %q: %v", src.URL, err)
	}
	name := path.Base(strings.TrimRight(u.Path, "/"))
	if strings.Trim(name, "./") == "" || strings.TrimPrefix(filepath.Ext(name), ".") == "" {
		return "", fmt.Errorf("cannot tell a file name from %s; set filename:", src.URL)
	}
	if !portableName(name) {
		return "", fmt.Errorf("the file name %q from %s cannot be stored on every drive; set filename:", name, src.URL)
	}
	return name, nil
}

// portableName reports whether a file name can be written to an exFAT or FAT
// stick as well as to a Unix disk.
func portableName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	return !strings.ContainsFunc(name, func(r rune) bool {
		return r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r)
	})
}

// errUnwrapPath drops the path an *fs.PathError repeats, since callers name it.
func errUnwrapPath(err error) error {
	if pathErr, ok := err.(*fs.PathError); ok {
		return pathErr.Err
	}
	return err
}
