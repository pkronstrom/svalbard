package apply

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/pkronstrom/svalbard/host-cli/internal/resolver"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// isGeoPackageZip reports whether a recipe downloads a GeoPackage packed in a
// zip, which apply installs unpacked so the drive's query tools can open it.
func isGeoPackageZip(recipe catalog.Item) bool {
	if recipe.Type != "gpkg" {
		return false
	}
	name := recipe.Filename
	if name == "" {
		name = filenameFromURL(recipe.URL)
	}
	return strings.HasSuffix(strings.ToLower(name), ".zip")
}

// installGeoPackageZip downloads a recipe's zip (checked against its sha256),
// writes the one .gpkg inside it to <type dir>/<name>.gpkg and deletes the
// zip. The entry describes the GeoPackage: its checksum and size are the
// unpacked file's, while the acquisition hash stays the recipe's URL and zip
// pin. A zip of the same name and pin at the vault's root, where apply put
// GeoPackages before they had a directory, is used instead of downloading
// and is removed.
func installGeoPackageZip(ctx context.Context, root, id string, recipe catalog.Item, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	dlURL, err := resolver.Resolve(recipe.URL, recipe.URLPattern)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("resolving URL for %s: %w", id, err)
	}
	zipName := recipe.Filename
	if zipName == "" {
		zipName = filenameFromURL(dlURL)
	}
	typeDir := toolkit.TypeDirs[recipe.Type]
	staging := filepath.Join(root, ".staging", "gpkg", id)
	defer os.RemoveAll(staging)

	source := strayGeoPackageZip(root, zipName, recipe.SHA256)
	if source == "" {
		source = filepath.Join(staging, zipName)
		if _, err := fetchFile(ctx, id, dlURL, source, zipName, recipe.SHA256, onProgress); err != nil {
			return manifest.RealizedEntry{}, err
		}
	} else {
		slog.Info("installing a GeoPackage from the zip at the vault's root", "id", id, "zip", source)
	}

	name, size, checksum, err := unzipGeoPackage(source, filepath.Join(root, typeDir))
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("%s: %s: %w", id, zipName, err)
	}
	// The zip at the root and the downloader's record of it; an update's
	// set-aside copy is deleted by the update itself once this succeeds.
	for _, stray := range []string{zipName, zipName + ".download.json"} {
		if err := os.Remove(filepath.Join(root, stray)); err != nil && !os.IsNotExist(err) {
			slog.Warn("removing the GeoPackage zip at the vault's root", "id", id, "path", stray, "error", err)
		}
	}
	return manifest.RealizedEntry{
		ID:             id,
		Type:           recipe.Type,
		Filename:       name,
		RelativePath:   filepath.Join(typeDir, name),
		SizeBytes:      size,
		ChecksumSHA256: checksum,
		SourceStrategy: recipe.Strategy,
	}, nil
}

// strayGeoPackageZip returns a zip named zipName at the vault's root, or the
// copy an update set aside, when it matches the pin; "" when there is none.
func strayGeoPackageZip(root, zipName, pin string) string {
	if pin == "" {
		return ""
	}
	for _, candidate := range []string{filepath.Join(root, zipName+asideSuffix), filepath.Join(root, zipName)} {
		if info, err := os.Stat(candidate); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if sum, err := downloader.ComputeSHA256(candidate); err == nil && strings.EqualFold(sum, pin) {
			return candidate
		}
	}
	return ""
}

// unzipGeoPackage writes the zip's single .gpkg member into destDir under its
// own base name, through a temporary file, and returns the name, size and
// SHA-256 of what it wrote.
func unzipGeoPackage(zipPath, destDir string) (string, int64, string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", 0, "", err
	}
	defer reader.Close()
	var members []*zip.File
	for _, file := range reader.File {
		if file.Mode().IsRegular() && strings.HasSuffix(strings.ToLower(file.Name), ".gpkg") {
			members = append(members, file)
		}
	}
	switch len(members) {
	case 0:
		return "", 0, "", fmt.Errorf("the zip holds no GeoPackage (.gpkg)")
	case 1:
	default:
		names := make([]string, len(members))
		for i, m := range members {
			names[i] = m.Name
		}
		return "", 0, "", fmt.Errorf("the zip holds %d GeoPackages (%s); a recipe installs one", len(members), strings.Join(names, ", "))
	}
	member := members[0]
	name := path.Base(path.Clean("/" + member.Name))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", 0, "", err
	}
	target := filepath.Join(destDir, name)
	partial := target + ".part"
	in, err := member.Open()
	if err != nil {
		return "", 0, "", err
	}
	defer in.Close()
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", 0, "", err
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hash), in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(partial, target)
	}
	if err != nil {
		os.Remove(partial)
		return "", 0, "", err
	}
	return name, size, hex.EncodeToString(hash.Sum(nil)), nil
}
