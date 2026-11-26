package apply

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// assembleBundle downloads each pinned source of a recipe's bundle, checks its
// SHA-256 and writes it into <type dir>/<id>/: archives are unpacked, other
// files are copied as they are. The tree is assembled in staging and moved into
// place only when every source succeeded; the downloads are deleted afterwards.
func assembleBundle(ctx context.Context, root, id string, recipe catalog.Item, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	typeDir, ok := toolkit.TypeDirs[recipe.Type]
	if !ok {
		return manifest.RealizedEntry{}, fmt.Errorf("%s: type %q has no directory for a bundle", id, recipe.Type)
	}
	staging := filepath.Join(root, ".staging", "bundle", id)
	tree := filepath.Join(staging, "tree")
	if err := os.RemoveAll(tree); err != nil {
		return manifest.RealizedEntry{}, err
	}
	for _, source := range recipe.Bundle {
		if source.URL == "" || source.SHA256 == "" {
			return manifest.RealizedEntry{}, fmt.Errorf("%s: every bundle source needs a url and a sha256", id)
		}
		dest, err := localPath(source.Dest)
		if err != nil {
			return manifest.RealizedEntry{}, fmt.Errorf("%s: bundle dest: %w", id, err)
		}
		filename := filenameFromURL(source.URL)
		downloaded := filepath.Join(staging, "downloads", filename)
		if !isArchive(filename) && (source.Strip != "" || len(source.Files) > 0 || len(source.Include) > 0) {
			return manifest.RealizedEntry{}, fmt.Errorf("%s: strip, files and include apply only to archives, not %s", id, filename)
		}
		if _, err := fetchFile(ctx, id, source.URL, downloaded, filename, source.SHA256, onProgress); err != nil {
			return manifest.RealizedEntry{}, err
		}
		if !isArchive(filename) {
			if err := os.MkdirAll(filepath.Join(tree, dest), 0o755); err != nil {
				return manifest.RealizedEntry{}, err
			}
			if err := os.Rename(downloaded, filepath.Join(tree, dest, filename)); err != nil {
				return manifest.RealizedEntry{}, err
			}
			continue
		}
		if err := extractArchive(downloaded, filepath.Join(tree, dest), source.Strip, source.Files, source.Include); err != nil {
			return manifest.RealizedEntry{}, fmt.Errorf("%s: unpacking %s: %w", id, filename, err)
		}
	}

	final := filepath.Join(root, typeDir, id)
	if err := os.RemoveAll(final); err != nil {
		return manifest.RealizedEntry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return manifest.RealizedEntry{}, err
	}
	if err := os.Rename(tree, final); err != nil {
		return manifest.RealizedEntry{}, err
	}
	if err := os.RemoveAll(staging); err != nil {
		return manifest.RealizedEntry{}, err
	}
	size, err := treeSize(final)
	if err != nil {
		return manifest.RealizedEntry{}, err
	}
	return manifest.RealizedEntry{
		ID:             id,
		Type:           recipe.Type,
		Filename:       id,
		RelativePath:   filepath.Join(typeDir, id),
		SizeBytes:      size,
		SourceStrategy: recipe.Strategy,
	}, nil
}

// extractArchive writes the regular files below strip into destDir:
// optionally only those listed in files, or only those matching an include
// pattern. Symlinks and other special entries are skipped, every listed file
// must be present and every pattern must match a file.
func extractArchive(archivePath, destDir, strip string, files, include []string) error {
	if len(files) > 0 && len(include) > 0 {
		return fmt.Errorf("files and include cannot be used together: list the exact names as include patterns")
	}
	strip = strings.Trim(path.Clean("/"+strip), "/")
	matched := make([]bool, len(include))
	wanted := make(map[string]bool, len(files))
	for _, name := range files {
		clean, err := localPath(name)
		if err != nil {
			return fmt.Errorf("files: %w", err)
		}
		wanted[filepath.ToSlash(clean)] = false
	}
	written := 0
	write := func(name string, mode fs.FileMode, r io.Reader) error {
		// Cleaning against "/" drops any leading "..", so rel stays inside destDir.
		rel := strings.TrimPrefix(path.Clean("/"+name), "/")
		if strip != "" {
			var ok bool
			if rel, ok = strings.CutPrefix(rel, strip+"/"); !ok {
				return nil
			}
		}
		if rel == "" {
			return nil
		}
		if len(wanted) > 0 {
			if _, ok := wanted[rel]; !ok {
				return nil
			}
			wanted[rel] = true
		}
		if len(include) > 0 {
			keep := false
			for i, pattern := range include {
				if matchInclude(pattern, rel) {
					matched[i], keep = true, true
				}
			}
			if !keep {
				return nil
			}
		}
		target := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		perm := fs.FileMode(0o644)
		if mode&0o111 != 0 {
			perm = 0o755
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, r); err != nil {
			out.Close()
			return err
		}
		written++
		return out.Close()
	}

	var err error
	if strings.HasSuffix(archivePath, ".zip") {
		err = walkZip(archivePath, write)
	} else {
		err = walkTarGz(archivePath, write)
	}
	if err != nil {
		return err
	}
	for name, found := range wanted {
		if !found {
			return fmt.Errorf("archive has no %s", path.Join(strip, name))
		}
	}
	for i, pattern := range include {
		if !matched[i] {
			return fmt.Errorf("include pattern %q matches no file below %q", pattern, strip)
		}
	}
	if written == 0 {
		return fmt.Errorf("no files below %q", strip)
	}
	return nil
}

// matchInclude reports whether rel, a slash path below the strip directory,
// matches an include pattern: path.Match segment by segment, where a final
// "**" segment matches everything below the segments before it.
func matchInclude(pattern, rel string) bool {
	patternSegs := strings.Split(strings.Trim(pattern, "/"), "/")
	relSegs := strings.Split(rel, "/")
	if last := len(patternSegs) - 1; patternSegs[last] == "**" {
		if len(relSegs) <= last {
			return false
		}
		patternSegs, relSegs = patternSegs[:last], relSegs[:last]
	}
	if len(patternSegs) != len(relSegs) {
		return false
	}
	for i, seg := range patternSegs {
		if ok, err := path.Match(seg, relSegs[i]); err != nil || !ok {
			return false
		}
	}
	return true
}

func isArchive(filename string) bool {
	for _, suffix := range []string{".zip", ".tgz", ".tar.gz"} {
		if strings.HasSuffix(filename, suffix) {
			return true
		}
	}
	return false
}

type archiveFileFunc func(name string, mode fs.FileMode, r io.Reader) error

func walkTarGz(archivePath string, fn archiveFileFunc) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := fn(hdr.Name, hdr.FileInfo().Mode(), tr); err != nil {
			return err
		}
	}
}

func walkZip(archivePath string, fn archiveFileFunc) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !file.Mode().IsRegular() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = fn(file.Name, file.Mode(), rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// localPath cleans a slash-separated recipe path and rejects anything that
// would escape the directory it is joined to. Empty means the directory itself.
func localPath(p string) (string, error) {
	if p == "" {
		return ".", nil
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if !filepath.IsLocal(clean) {
		return "", fmt.Errorf("path %q leaves the item directory", p)
	}
	return clean, nil
}

func treeSize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}
