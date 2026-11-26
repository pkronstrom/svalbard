package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const documentsFile = "documents.json"

// DocumentMetadata is one entry of .svalbard/documents.json: what the
// collection definition says about a document on the drive. The index reads
// the title from it, since the definition itself stays on the computer.
type DocumentMetadata struct {
	Title      string `json:"title,omitempty"`
	Collection string `json:"collection"`
	URL        string `json:"url,omitempty"`
}

// WriteDocumentsMetadata writes .svalbard/documents.json for the realized
// documents of collections, keyed by slash-separated vault-relative path.
// A document whose collection definition is not available on this computer
// keeps the metadata an earlier apply recorded. Without any documents, a
// stale file is removed.
func WriteDocumentsMetadata(root string, entries []manifest.RealizedEntry, recipes map[string]catalog.Item) error {
	path := filepath.Join(root, svalbardDir, documentsFile)
	previous, _ := ReadDocumentsMetadata(root)
	defined := make(map[string]map[string]catalog.Document)
	out := make(map[string]DocumentMetadata)
	for _, entry := range entries {
		typeDir, ok := TypeDirs[entry.Type]
		if !ok {
			continue
		}
		rel, err := filepath.Rel(filepath.Join(typeDir, entry.ID), entry.RelativePath)
		if err != nil || rel == "." || !filepath.IsLocal(rel) {
			continue
		}
		key := filepath.ToSlash(entry.RelativePath)
		docs, loaded := defined[entry.ID]
		if !loaded {
			if recipe, ok := recipes[entry.ID]; ok && recipe.IsCollection() {
				if expected, err := catalog.ExpectedDocuments(recipe); err == nil {
					docs = make(map[string]catalog.Document, len(expected))
					for _, doc := range expected {
						docs[doc.RelPath] = doc
					}
				}
			}
			defined[entry.ID] = docs
		}
		if doc, ok := docs[filepath.ToSlash(rel)]; ok {
			out[key] = DocumentMetadata{Title: doc.Title, Collection: entry.ID, URL: doc.URL}
		} else if meta, ok := previous[key]; ok {
			out[key] = meta
		} else {
			out[key] = DocumentMetadata{Collection: entry.ID}
		}
	}
	if len(out) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".documents-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// ReadDocumentsMetadata reads .svalbard/documents.json. A vault without one
// has no collection documents and returns an empty map.
func ReadDocumentsMetadata(root string) (map[string]DocumentMetadata, error) {
	data, err := os.ReadFile(filepath.Join(root, svalbardDir, documentsFile))
	if os.IsNotExist(err) {
		return map[string]DocumentMetadata{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]DocumentMetadata)
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
