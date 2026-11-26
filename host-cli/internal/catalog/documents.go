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

// ExpectedDocuments lists the files a collection should hold, in definition
// order, with folder entries expanded to the documents below them. It reads
// the local disk but never the network. A local source that cannot be read is
// an error for the whole collection, so a missing folder is never mistaken for
// an emptied one.
func ExpectedDocuments(item Item) ([]Document, error) {
	var docs []Document
	for _, src := range item.Documents {
		if src.URL != "" {
			name, err := documentURLName(src)
			if err != nil {
				return nil, err
			}
			docs = append(docs, Document{RelPath: name, Title: src.Title, URL: src.URL, SHA256: strings.ToLower(src.SHA256)})
			continue
		}
		info, err := os.Stat(src.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", src.Path, errUnwrapPath(err))
		}
		if !info.IsDir() {
			name := src.Filename
			if name == "" {
				name = filepath.Base(src.Path)
			}
			docs = append(docs, Document{RelPath: name, Title: src.Title, Source: src.Path, SHA256: strings.ToLower(src.SHA256), Size: info.Size()})
			continue
		}
		if src.Title != "" || src.Filename != "" || src.SHA256 != "" {
			return nil, fmt.Errorf("%s is a folder: title, filename and sha256 apply to a single file", src.Path)
		}
		found, err := folderDocuments(src.Path)
		if err != nil {
			return nil, err
		}
		docs = append(docs, found...)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents found")
	}
	// Sticks are usually exFAT, where names differing only in case collide.
	seen := make(map[string]string, len(docs))
	folders := make(map[string]string)
	for _, doc := range docs {
		key := strings.ToLower(doc.RelPath)
		if other, dup := seen[key]; dup {
			return nil, fmt.Errorf("%s and %s would both be saved as %s; set filename: on one of them", other, doc.Origin(), doc.RelPath)
		}
		seen[key] = doc.Origin()
		for dir := path.Dir(key); dir != "."; dir = path.Dir(dir) {
			folders[dir] = doc.Origin()
		}
	}
	for _, doc := range docs {
		if other, clash := folders[strings.ToLower(doc.RelPath)]; clash {
			return nil, fmt.Errorf("%s would be saved as %s, which is also the folder of %s; set filename:", doc.Origin(), doc.RelPath, other)
		}
	}
	return docs, nil
}

// folderDocuments returns the documents below dir, sorted, with paths that
// start at the folder's own name so its subfolders are kept. Hidden entries
// (including macOS "._" sidecars) are skipped.
func folderDocuments(dir string) ([]Document, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, errUnwrapPath(err))
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
		return nil, fmt.Errorf("cannot read %s: %w", dir, errUnwrapPath(err))
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
	if name == "." || name == "/" || filepath.Ext(name) == "" {
		return "", fmt.Errorf("cannot tell a file name from %s; set filename:", src.URL)
	}
	return name, nil
}

// errUnwrapPath drops the path an *fs.PathError repeats, since callers name it.
func errUnwrapPath(err error) error {
	if pathErr, ok := err.(*fs.PathError); ok {
		return pathErr.Err
	}
	return err
}
