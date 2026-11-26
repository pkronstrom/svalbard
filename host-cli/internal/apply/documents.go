package apply

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// assembleDocuments brings <type dir>/<id>/ in line with a collection's
// document list. Documents already realized and unchanged are kept without a
// request; missing or changed ones are downloaded or copied through staging,
// and realized files the list no longer names are deleted.
//
// The returned entries are the collection's complete realized set, also when
// an error is returned with them: one failing document fails the item but the
// documents that did arrive stay recorded, so the next apply fetches only the
// rest.
func assembleDocuments(ctx context.Context, root, id string, recipe catalog.Item, realized []manifest.RealizedEntry, onProgress downloader.ProgressFunc) ([]manifest.RealizedEntry, error) {
	typeDir, ok := toolkit.TypeDirs[recipe.Type]
	if !ok {
		return realized, fmt.Errorf("type %q has no directory for documents", recipe.Type)
	}
	docs, err := catalog.ExpectedDocuments(recipe)
	if err != nil {
		// Nothing is known about the wanted set, so nothing is touched.
		return realized, err
	}
	dir := filepath.Join(typeDir, id)
	have := make(map[string]manifest.RealizedEntry, len(realized))
	for _, entry := range realized {
		if rel, ok := collectionRelPath(dir, entry.RelativePath); ok {
			have[rel] = entry
		}
	}

	var entries []manifest.RealizedEntry
	var pending []catalog.Document
	expected := make(map[string]bool, len(docs))
	for _, doc := range docs {
		expected[doc.RelPath] = true
		if entry, ok := have[doc.RelPath]; ok && documentCurrent(root, entry, doc) {
			entries = append(entries, entry)
			continue
		}
		pending = append(pending, doc)
	}

	var errs []error
	for rel, entry := range have {
		if expected[rel] {
			continue
		}
		if err := removeDocument(root, dir, entry.RelativePath); err != nil {
			errs = append(errs, fmt.Errorf("removing %s: %w", entry.RelativePath, err))
			entries = append(entries, entry)
		}
	}

	staging := filepath.Join(root, ".staging", "documents", id)
	for _, doc := range pending {
		var entry manifest.RealizedEntry
		err := ctx.Err()
		if err == nil {
			entry, err = fetchDocument(ctx, root, id, recipe, dir, staging, doc, onProgress)
		}
		if err == nil {
			entries = append(entries, entry)
			continue
		}
		if ctx.Err() == nil {
			errs = append(errs, fmt.Errorf("%s: %w", doc.Origin(), err))
		}
		// A changed document that could not be refreshed keeps its previous
		// file and record; the plan still sees the difference and retries.
		if old, ok := have[doc.RelPath]; ok && entryOnDisk(root, old) {
			entries = append(entries, old)
		}
	}
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return entries, errors.Join(errs...)
	}
	if err := os.RemoveAll(staging); err != nil {
		return entries, err
	}
	return entries, nil
}

// fetchDocument stages one document, checks it and moves it into place.
func fetchDocument(ctx context.Context, root, id string, recipe catalog.Item, dir, staging string, doc catalog.Document, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	rel := filepath.FromSlash(doc.RelPath)
	if !filepath.IsLocal(rel) {
		return manifest.RealizedEntry{}, fmt.Errorf("path %q leaves the collection directory", doc.RelPath)
	}
	staged := filepath.Join(staging, rel)
	strategy := "download"
	var checksum string
	if doc.URL != "" {
		result, err := fetchFile(ctx, id, doc.URL, staged, path.Base(doc.RelPath), doc.SHA256, onProgress)
		if err != nil {
			return manifest.RealizedEntry{}, err
		}
		checksum = result.SHA256
	} else {
		strategy = "local"
		sum, err := copyDocument(ctx, doc.Source, staged, onProgress)
		if err != nil {
			return manifest.RealizedEntry{}, err
		}
		if doc.SHA256 != "" && sum != doc.SHA256 {
			os.Remove(staged)
			return manifest.RealizedEntry{}, fmt.Errorf("sha256 mismatch: got %s, want %s", sum, doc.SHA256)
		}
		checksum = sum
	}
	if err := checkDocumentContent(staged); err != nil {
		os.Remove(staged)
		os.Remove(staged + ".download.json")
		return manifest.RealizedEntry{}, err
	}

	final := filepath.Join(root, dir, rel)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return manifest.RealizedEntry{}, err
	}
	if err := os.Rename(staged, final); err != nil {
		return manifest.RealizedEntry{}, err
	}
	os.Remove(staged + ".download.json")
	info, err := os.Stat(final)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("stat %s: %w", final, err)
	}
	return manifest.RealizedEntry{
		ID:             id,
		Type:           recipe.Type,
		Filename:       filepath.Base(rel),
		RelativePath:   filepath.Join(dir, rel),
		SizeBytes:      info.Size(),
		ChecksumSHA256: checksum,
		SourceStrategy: strategy,
	}, nil
}

// copyDocument copies a local file to dest and returns its SHA-256.
func copyDocument(ctx context.Context, src, dest string, onProgress downloader.ProgressFunc) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	buf := make([]byte, 1024*1024)
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			return "", err
		}
		n, readErr := in.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return "", err
			}
			copied += int64(n)
			if onProgress != nil {
				onProgress(copied, info.Size())
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			return "", readErr
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// checkDocumentContent rejects a file saved as .pdf that is not a PDF, which
// is what a login or error page behind the URL looks like.
func checkDocumentContent(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	// The header may follow a little leading junk, but not more than 1 KiB.
	head := make([]byte, 1024)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return err
	}
	head = head[:n]
	if bytes.Contains(head, []byte("%PDF-")) {
		return nil
	}
	if trimmed := bytes.TrimSpace(head); len(trimmed) > 0 && trimmed[0] == '<' {
		return errors.New("not a PDF: the source returned a web page (a login or error page?)")
	}
	return errors.New("not a PDF: the file has no %PDF- header")
}

// DocumentDiff counts how a collection's definition differs from what the
// vault has realized for it.
type DocumentDiff struct {
	Added   int // listed but not realized
	Changed int // realized, but the pin or the local source changed
	Removed int // realized but no longer listed
}

// Any reports whether an apply has something to do for the collection.
func (d DocumentDiff) Any() bool { return d.Added+d.Changed+d.Removed > 0 }

// DiffDocuments compares a collection's expected documents with its realized
// entries. It reads neither the vault nor the network, so a plan can show it.
func DiffDocuments(recipe catalog.Item, docs []catalog.Document, realized []manifest.RealizedEntry) DocumentDiff {
	dir := filepath.Join(toolkit.TypeDirs[recipe.Type], recipe.ID)
	have := make(map[string]manifest.RealizedEntry, len(realized))
	for _, entry := range realized {
		if rel, ok := collectionRelPath(dir, entry.RelativePath); ok {
			have[rel] = entry
		}
	}
	var diff DocumentDiff
	for _, doc := range docs {
		entry, ok := have[doc.RelPath]
		switch {
		case !ok:
			diff.Added++
		case !documentMatches(entry, doc):
			diff.Changed++
		}
		delete(have, doc.RelPath)
	}
	diff.Removed = len(have)
	return diff
}

// IsCollectionEntry reports whether a realized entry is one document of a
// collection: a file recorded inside <type dir>/<id>/. It lets a vault be
// read on a computer that does not have the collection's definition.
func IsCollectionEntry(entry manifest.RealizedEntry) bool {
	typeDir, ok := toolkit.TypeDirs[entry.Type]
	if !ok {
		return false
	}
	_, inside := collectionRelPath(filepath.Join(typeDir, entry.ID), entry.RelativePath)
	return inside
}

// documentMatches reports whether a realized document is still what the
// collection asks for, judged from the manifest alone: a sha256 pin matches
// the recorded checksum and a local source has not changed size.
func documentMatches(entry manifest.RealizedEntry, doc catalog.Document) bool {
	if doc.SHA256 != "" && doc.SHA256 != entry.ChecksumSHA256 {
		return false
	}
	return doc.Source == "" || doc.Size == entry.SizeBytes
}

// documentCurrent adds the vault's own state to documentMatches: the file is
// on disk at its recorded size.
func documentCurrent(root string, entry manifest.RealizedEntry, doc catalog.Document) bool {
	return entryOnDisk(root, entry) && documentMatches(entry, doc)
}

func entryOnDisk(root string, entry manifest.RealizedEntry) bool {
	info, err := os.Stat(filepath.Join(root, entry.RelativePath))
	return err == nil && info.Mode().IsRegular() && info.Size() == entry.SizeBytes
}

// collectionRelPath returns the slash-separated path of relativePath below a
// collection directory, or false when it is not inside it.
func collectionRelPath(dir, relativePath string) (string, bool) {
	rel, err := filepath.Rel(dir, relativePath)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// removeDocument deletes one realized document and the folders it leaves
// empty, up to but not including the collection directory.
func removeDocument(root, dir, relativePath string) error {
	if _, ok := collectionRelPath(dir, relativePath); !ok {
		return fmt.Errorf("outside %s", dir)
	}
	target := filepath.Join(root, relativePath)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	// macOS leaves a "._name" sidecar next to files on exFAT and FAT sticks.
	os.Remove(filepath.Join(filepath.Dir(target), "._"+filepath.Base(target)))
	stop := filepath.Join(root, dir)
	for parent := filepath.Dir(target); parent != stop && strings.HasPrefix(parent, stop); parent = filepath.Dir(parent) {
		if !removeIfEmpty(parent) {
			break
		}
	}
	return nil
}

// removeIfEmpty removes dir when it holds nothing but macOS metadata files.
func removeIfEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || (entry.Name() != ".DS_Store" && !strings.HasPrefix(entry.Name(), "._")) {
			return false
		}
	}
	return os.RemoveAll(dir) == nil
}

// collectionEntries returns the realized entries recorded for id.
func collectionEntries(entries []manifest.RealizedEntry, id string) []manifest.RealizedEntry {
	var own []manifest.RealizedEntry
	for _, entry := range entries {
		if entry.ID == id {
			own = append(own, entry)
		}
	}
	return own
}

// replaceEntriesForID replaces every entry of id with additions, also when
// additions is empty.
func replaceEntriesForID(existing []manifest.RealizedEntry, id string, additions []manifest.RealizedEntry) []manifest.RealizedEntry {
	entries := make([]manifest.RealizedEntry, 0, len(existing)+len(additions))
	for _, entry := range existing {
		if entry.ID != id {
			entries = append(entries, entry)
		}
	}
	return append(entries, additions...)
}

// recordDocumentTitles writes the document metadata of one collection as soon
// as its entries are saved. The complete file is written when an apply
// succeeds; this covers the apply that fails on one document, after which
// `svalbard index` is run by hand and should still know the titles. It is
// best effort: the index falls back to the file's own title.
func recordDocumentTitles(root string, entries []manifest.RealizedEntry, cat *catalog.Catalog, id string) {
	recipe, ok := cat.RecipeByID(id)
	if !ok || !recipe.IsCollection() {
		return
	}
	if err := toolkit.WriteDocumentsMetadata(root, entries, map[string]catalog.Item{id: recipe}); err != nil {
		slog.Warn("writing document metadata", "id", id, "error", err)
	}
}

// removeCollectionDirs deletes the directory of every removed item whose
// files were recorded one by one inside <type dir>/<id>/, as a document
// collection's are, together with its staging directory. Removing the entries
// alone would leave the emptied folders behind.
func removeCollectionDirs(root string, removed []manifest.RealizedEntry) {
	for _, entry := range removed {
		if !IsCollectionEntry(entry) {
			continue
		}
		os.RemoveAll(filepath.Join(root, toolkit.TypeDirs[entry.Type], entry.ID))
		os.RemoveAll(filepath.Join(root, ".staging", "documents", entry.ID))
	}
}
