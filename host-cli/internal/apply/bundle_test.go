package apply

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"gopkg.in/yaml.v3"
)

type archiveEntry struct {
	name, body string
	symlink    bool
}

func tarGz(t *testing.T, entries ...archiveEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if e.symlink {
			hdr = &tar.Header{Name: e.name, Linkname: e.body, Typeflag: tar.TypeSymlink}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !e.symlink {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func zipArchive(t *testing.T, entries ...archiveEntry) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func sha(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// bundleCatalog round-trips the recipe through YAML, so the test also covers
// the bundle field names a recipe file uses.
func bundleCatalog(t *testing.T, recipe catalog.Item) *catalog.Catalog {
	t.Helper()
	data, err := yaml.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.LoadFromFS(fstest.MapFS{recipe.ID + ".yaml": {Data: data}}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func runBundle(t *testing.T, root string, recipe catalog.Item) (manifest.Manifest, error) {
	t.Helper()
	m := manifest.New("test")
	m.Desired.Items = []string{recipe.ID}
	err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{recipe.ID}}, bundleCatalog(t, recipe))
	return m, err
}

func TestApplyAssemblesPinnedBundle(t *testing.T) {
	npm := tarGz(t,
		archiveEntry{name: "package/package.json", body: "{}"},
		archiveEntry{name: "package/dist/lib.mjs", body: "export {}"},
		archiveEntry{name: "package/dist/engine.wasm", body: "wasm"},
		archiveEntry{name: "package/dist/engine.wasm.map", body: "map"},
		archiveEntry{name: "package/dist/link.mjs", body: "lib.mjs", symlink: true},
	)
	site := zipArchive(t, archiveEntry{name: "assets/app.css", body: "css"}, archiveEntry{name: "readme.txt", body: "hi"})
	plain := "extension"
	useMockHTTPResponses(t, map[string]string{"/lib-1.0.0.tgz": npm, "/site.zip": site, "/v1/ext.wasm": plain})

	root := t.TempDir()
	recipe := catalog.Item{ID: "web-app", Type: "app", Bundle: []catalog.BundleSource{
		{URL: "http://example.test/lib-1.0.0.tgz", SHA256: sha(npm), Strip: "package/dist", Files: []string{"lib.mjs", "engine.wasm"}, Dest: "lib"},
		{URL: "http://example.test/site.zip", SHA256: sha(site)},
		{URL: "http://example.test/v1/ext.wasm", SHA256: sha(plain), Dest: "extensions/v1"},
	}}
	m, err := runBundle(t, root, recipe)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"lib/lib.mjs":            "export {}",
		"lib/engine.wasm":        "wasm",
		"assets/app.css":         "css",
		"readme.txt":             "hi",
		"extensions/v1/ext.wasm": plain,
	}
	appDir := filepath.Join(root, "apps", "web-app")
	var got []string
	var size int64
	filepath.WalkDir(appDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(appDir, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return err
	})
	if len(got) != len(want) {
		t.Fatalf("app files = %v, want %d files", got, len(want))
	}
	for rel, body := range want {
		data, err := os.ReadFile(filepath.Join(appDir, filepath.FromSlash(rel)))
		if err != nil || string(data) != body {
			t.Fatalf("%s = %q, %v; want %q", rel, data, err, body)
		}
		size += int64(len(body))
	}
	if len(m.Realized.Entries) != 1 {
		t.Fatalf("entries = %+v", m.Realized.Entries)
	}
	entry := m.Realized.Entries[0]
	if entry.RelativePath != filepath.Join("apps", "web-app") || entry.Type != "app" || entry.SizeBytes != size {
		t.Fatalf("entry = %+v, want apps/web-app of %d bytes", entry, size)
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", "bundle", "web-app")); !os.IsNotExist(err) {
		t.Fatalf("downloads left in staging: %v", err)
	}
}

func TestApplyBundleFailuresLeaveNoApp(t *testing.T) {
	npm := tarGz(t, archiveEntry{name: "package/dist/lib.mjs", body: "export {}"})
	useMockHTTPResponses(t, map[string]string{"/lib.tgz": npm, "/ext.wasm": "x"})
	for name, tc := range map[string]struct {
		source catalog.BundleSource
		want   string
	}{
		"checksum mismatch":   {catalog.BundleSource{URL: "http://example.test/lib.tgz", SHA256: sha("other")}, "sha256"},
		"unpinned":            {catalog.BundleSource{URL: "http://example.test/lib.tgz"}, "needs a url and a sha256"},
		"listed file absent":  {catalog.BundleSource{URL: "http://example.test/lib.tgz", SHA256: sha(npm), Strip: "package/dist", Files: []string{"missing.wasm"}}, "archive has no package/dist/missing.wasm"},
		"strip misses":        {catalog.BundleSource{URL: "http://example.test/lib.tgz", SHA256: sha(npm), Strip: "dist"}, `no files below "dist"`},
		"dest escapes":        {catalog.BundleSource{URL: "http://example.test/lib.tgz", SHA256: sha(npm), Dest: "../outside"}, "leaves the item directory"},
		"strip on plain file": {catalog.BundleSource{URL: "http://example.test/ext.wasm", SHA256: sha("x"), Strip: "package"}, "only to archives"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			_, err := runBundle(t, root, catalog.Item{ID: "web-app", Type: "app", Bundle: []catalog.BundleSource{tc.source}})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(root, "apps", "web-app")); !os.IsNotExist(err) {
				t.Fatalf("failed bundle left apps/web-app: %v", err)
			}
		})
	}
}

func TestExtractArchiveKeepsEntriesInsideDestination(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tgz")
	body := tarGz(t,
		archiveEntry{name: "package/../../escape.txt", body: "x"},
		archiveEntry{name: "/abs.txt", body: "y"},
		archiveEntry{name: "package/link", body: "/etc/passwd", symlink: true},
	)
	if err := os.WriteFile(archive, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out", "app")
	if err := extractArchive(archive, dest, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"escape.txt", "abs.txt"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Fatalf("%s not contained in destination: %v", rel, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(dest, "package", "link")); !os.IsNotExist(err) {
		t.Fatalf("symlink was extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("entry escaped the destination: %v", err)
	}
}

// npmPackage is a package tarball with more than a trimmed install keeps.
func npmPackage(t *testing.T, dir string) string {
	t.Helper()
	archive := filepath.Join(dir, "pkg.tgz")
	body := tarGz(t,
		archiveEntry{name: "package/package.json", body: "{}"},
		archiveEntry{name: "package/README.md", body: "readme"},
		archiveEntry{name: "package/LICENSE.txt", body: "MIT"},
		archiveEntry{name: "package/index.d.ts", body: "types"},
		archiveEntry{name: "package/src/index.ts", body: "src"},
		archiveEntry{name: "package/dist/lib.min.js", body: "min"},
		archiveEntry{name: "package/dist/lib.min.js.map", body: "map"},
		archiveEntry{name: "package/dist/lib.js", body: "full"},
		archiveEntry{name: "package/build/a.js", body: "a"},
		archiveEntry{name: "package/build/deep/b.js", body: "b"},
	)
	if err := os.WriteFile(archive, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return archive
}

func extracted(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return err
	})
	return files
}

func TestExtractIncludeGlobs(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	include := []string{"package.json", "dist/*.min.js", "*.d.ts", "LICENSE*", "README.md"}
	if err := extractArchive(npmPackage(t, dir), dest, "package", nil, include); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(extracted(t, dest), ","), "LICENSE.txt,README.md,dist/lib.min.js,index.d.ts,package.json"; got != want {
		t.Errorf("extracted %s, want %s", got, want)
	}
}

func TestExtractIncludeDoubleStar(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := extractArchive(npmPackage(t, dir), dest, "package", nil, []string{"build/**"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(extracted(t, dest), ","), "build/a.js,build/deep/b.js"; got != want {
		t.Errorf("extracted %s, want %s", got, want)
	}
}

func TestExtractIncludeNoMatchFails(t *testing.T) {
	dir := t.TempDir()
	err := extractArchive(npmPackage(t, dir), filepath.Join(dir, "out"), "package", nil, []string{"package.json", "dist/*.cjs"})
	if err == nil || !strings.Contains(err.Error(), "dist/*.cjs") {
		t.Fatalf("err = %v, want one naming the pattern that matched nothing", err)
	}
	err = extractArchive(npmPackage(t, dir), filepath.Join(dir, "out2"), "package", []string{"package.json"}, []string{"*.d.ts"})
	if err == nil || !strings.Contains(err.Error(), "files and include") {
		t.Fatalf("err = %v, want files and include refused together", err)
	}
}

func zipNames(t *testing.T, path string) map[string]string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	files := map[string]string{}
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		buf.ReadFrom(rc)
		rc.Close()
		files[f.Name] = buf.String()
	}
	return files
}

func TestApplyJSLibWritesOneArchive(t *testing.T) {
	d3 := tarGz(t,
		archiveEntry{name: "package/package.json", body: `{"name":"d3"}`},
		archiveEntry{name: "package/LICENSE", body: "ISC"},
		archiveEntry{name: "package/dist/d3.min.js", body: "umd"},
		archiveEntry{name: "package/dist/d3.js", body: "full"},
		archiveEntry{name: "package/src/index.js", body: "src"},
	)
	color := tarGz(t,
		archiveEntry{name: "package/package.json", body: `{"name":"color"}`},
		archiveEntry{name: "package/dist/color.esm.js", body: "esm"},
	)
	useMockHTTPResponses(t, map[string]string{"/d3.tgz": d3, "/color.tgz": color})
	recipe := catalog.Item{ID: "js-test", Type: "js-lib", Bundle: []catalog.BundleSource{
		{URL: "http://example.test/d3.tgz", SHA256: sha(d3), Strip: "package", Dest: "node_modules/d3", Include: []string{"package.json", "LICENSE", "dist/d3.min.js"}, Main: "dist/d3.min.js"},
		{URL: "http://example.test/color.tgz", SHA256: sha(color), Strip: "package", Dest: "node_modules/d3/node_modules/color", Include: []string{"package.json", "dist/*.js"}},
	}}
	root := t.TempDir()
	m, err := runBundle(t, root, recipe)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "lib", "js", "js-test.zip")
	got := zipNames(t, archive)
	want := map[string]string{
		"node_modules/d3/package.json":                         `{"name":"d3"}`,
		"node_modules/d3/LICENSE":                              "ISC",
		"node_modules/d3/dist/d3.min.js":                       "umd",
		"node_modules/d3/node_modules/color/package.json":      `{"name":"color"}`,
		"node_modules/d3/node_modules/color/dist/color.esm.js": "esm",
	}
	for name, body := range want {
		if got[name] != body {
			t.Errorf("%s = %q, want %q", name, got[name], body)
		}
	}
	if index := got["svalbard-js.json"]; !strings.Contains(index, `"node_modules/d3":"dist/d3.min.js"`) {
		t.Errorf("svalbard-js.json = %q, want d3's main", index)
	}
	if len(got) != len(want)+1 {
		t.Errorf("archive holds %d files, want %d: %v", len(got), len(want)+1, got)
	}
	if _, err := os.Stat(filepath.Join(root, "lib", "js", "js-test")); !os.IsNotExist(err) {
		t.Errorf("a tree was left beside the archive: %v", err)
	}
	entry := m.Realized.Entries[0]
	data, _ := os.ReadFile(archive)
	if entry.RelativePath != filepath.Join("lib", "js", "js-test.zip") || entry.Filename != "js-test.zip" || entry.ChecksumSHA256 != sha(string(data)) || entry.SizeBytes != int64(len(data)) {
		t.Errorf("entry = %+v, want the archive with its checksum and size", entry)
	}

	// The same sources make the same archive, byte for byte.
	again := t.TempDir()
	if _, err := runBundle(t, again, recipe); err != nil {
		t.Fatal(err)
	}
	if other, _ := os.ReadFile(filepath.Join(again, "lib", "js", "js-test.zip")); !bytes.Equal(other, data) {
		t.Error("a second install wrote a different archive")
	}
}
