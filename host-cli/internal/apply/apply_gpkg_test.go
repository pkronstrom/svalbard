package apply

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// geoPackageZip is shaped like Natural Earth's download: the GeoPackage in a
// folder beside a few small files.
func geoPackageZip(t *testing.T, gpkg string) string {
	t.Helper()
	return zipArchive(t,
		archiveEntry{name: "packages/ne.gpkg", body: gpkg},
		archiveEntry{name: "VERSION", body: "5.1.1"},
		archiveEntry{name: "README.md", body: "# Natural Earth"},
	)
}

func gpkgRecipe(zip string) catalog.Item {
	return catalog.Item{ID: "ne", Type: "gpkg", URL: "http://example.test/ne.gpkg.zip", Filename: "ne.gpkg.zip", SHA256: sha(zip)}
}

func TestApplyGpkgZipLandsUnzippedInData(t *testing.T) {
	gpkg := "SQLite format 3\x00 geopackage"
	zip := geoPackageZip(t, gpkg)
	useMockHTTPResponses(t, map[string]string{"/ne.gpkg.zip": zip})
	root := t.TempDir()

	m, err := runBundle(t, root, gpkgRecipe(zip))
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "data", "ne.gpkg")); got != gpkg {
		t.Fatalf("data/ne.gpkg = %q, want the GeoPackage", got)
	}
	for _, stray := range []string{"ne.gpkg.zip", "ne.gpkg.zip.download.json", filepath.Join("data", "ne.gpkg.zip"), filepath.Join(".staging", "gpkg", "ne")} {
		if _, err := os.Stat(filepath.Join(root, stray)); !os.IsNotExist(err) {
			t.Errorf("%s is left on the vault: %v", stray, err)
		}
	}
	if len(m.Realized.Entries) != 1 {
		t.Fatalf("entries = %+v", m.Realized.Entries)
	}
	entry := m.Realized.Entries[0]
	want := manifest.RealizedEntry{ID: "ne", Type: "gpkg", Filename: "ne.gpkg", RelativePath: filepath.Join("data", "ne.gpkg"), SizeBytes: int64(len(gpkg)), ChecksumSHA256: sha(gpkg)}
	if entry.ID != want.ID || entry.Type != want.Type || entry.Filename != want.Filename || entry.RelativePath != want.RelativePath ||
		entry.SizeBytes != want.SizeBytes || entry.ChecksumSHA256 != want.ChecksumSHA256 {
		t.Fatalf("entry = %+v, want %+v", entry, want)
	}
	if recipe := gpkgRecipe(zip); entry.AcquisitionHash != recipe.AcquisitionHash() {
		t.Errorf("acquisition hash = %q, want the recipe's (url and zip sha256)", entry.AcquisitionHash)
	}
	if got := read(t, filepath.Join(root, ".svalbard", "checksums.sha256")); !strings.Contains(got, sha(gpkg)+"  data/ne.gpkg") {
		t.Errorf("checksums.sha256 = %q, want the GeoPackage's checksum", got)
	}
}

func TestApplyGpkgZipNeedsExactlyOneGeoPackage(t *testing.T) {
	for name, zip := range map[string]string{
		"none": zipArchive(t, archiveEntry{name: "README.md", body: "x"}),
		"two":  zipArchive(t, archiveEntry{name: "a.gpkg", body: "a"}, archiveEntry{name: "b/b.gpkg", body: "b"}),
	} {
		t.Run(name, func(t *testing.T) {
			useMockHTTPResponses(t, map[string]string{"/ne.gpkg.zip": zip})
			root := t.TempDir()
			_, err := runBundle(t, root, gpkgRecipe(zip))
			if err == nil || !strings.Contains(err.Error(), "GeoPackage") {
				t.Fatalf("err = %v, want one naming the missing or extra GeoPackage", err)
			}
			if entries, _ := os.ReadDir(filepath.Join(root, "data")); len(entries) != 0 {
				t.Errorf("data/ holds %v after a failed install", entries)
			}
		})
	}
}

// RF4: a vault installed before GeoPackages had a directory holds the zip at
// its root. Updating it unpacks that zip into data/ without downloading it
// again and removes it.
func TestApplyMovesStrayRootGpkgZip(t *testing.T) {
	gpkg := "SQLite format 3\x00 geopackage"
	zip := geoPackageZip(t, gpkg)
	useMockHTTPResponses(t, map[string]string{}) // nothing to download
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ne.gpkg.zip"), []byte(zip), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ne.gpkg.zip.download.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := gpkgRecipe(zip)
	old.SHA256 = "" // the recipe had no pin then
	m := manifest.New("test")
	m.Desired.Items = []string{"ne"}
	m.Realized.Entries = []manifest.RealizedEntry{{
		ID: "ne", Type: "gpkg", Filename: "ne.gpkg.zip", RelativePath: "ne.gpkg.zip",
		SizeBytes: int64(len(zip)), ChecksumSHA256: sha(zip), AcquisitionHash: old.AcquisitionHash(),
	}}

	if err := Run(context.Background(), root, &m, planner.Plan{ToUpdate: []string{"ne"}}, bundleCatalog(t, gpkgRecipe(zip))); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "data", "ne.gpkg")); got != gpkg {
		t.Fatalf("data/ne.gpkg = %q, want the GeoPackage from the stray zip", got)
	}
	for _, stray := range []string{"ne.gpkg.zip", "ne.gpkg.zip.download.json", "ne.gpkg.zip" + asideSuffix} {
		if _, err := os.Stat(filepath.Join(root, stray)); !os.IsNotExist(err) {
			t.Errorf("%s is left at the vault's root: %v", stray, err)
		}
	}
	if len(m.Realized.Entries) != 1 || m.Realized.Entries[0].RelativePath != filepath.Join("data", "ne.gpkg") || m.Realized.Entries[0].ChecksumSHA256 != sha(gpkg) {
		t.Fatalf("entries = %+v, want data/ne.gpkg with its checksum", m.Realized.Entries)
	}
}
