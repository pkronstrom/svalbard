package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "package/" + name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

type fakePackage struct {
	name, version string
	latest        bool // tagged latest
	noLicense     bool // the tarball holds no LICENSE
	deps          map[string]string
	files         map[string]string
	badIntegrity  bool
}

// fakeRegistry serves the packages' metadata and tarballs, and any extra
// paths given (for direct tarballs outside the registry).
func fakeRegistry(t *testing.T, packages []fakePackage, extra map[string][]byte) *httptest.Server {
	t.Helper()
	docs := map[string]map[string]any{}
	tarballs := map[string][]byte{}
	tags := map[string]map[string]string{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := tarballs[r.URL.Path]; ok {
			w.Write(body)
			return
		}
		if body, ok := extra[r.URL.Path]; ok {
			w.Write(body)
			return
		}
		name := strings.Replace(strings.TrimPrefix(r.URL.Path, "/"), "%2F", "/", 1)
		doc, ok := docs[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"name": name, "versions": doc, "dist-tags": tags[name]})
	}))
	t.Cleanup(server.Close)
	for _, p := range packages {
		files := map[string]string{"package.json": `{"name":"` + p.name + `","version":"` + p.version + `","license":"MIT"}`}
		if !p.noLicense {
			files["LICENSE"] = "MIT License\nCopyright " + p.name
		}
		for k, v := range p.files {
			files[k] = v
		}
		body := tgz(t, files)
		sum := sha512.Sum512(body)
		integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
		if p.badIntegrity {
			other := sha512.Sum512([]byte("something else"))
			integrity = "sha512-" + base64.StdEncoding.EncodeToString(other[:])
		}
		if p.latest {
			tags[p.name] = map[string]string{"latest": p.version}
		}
		tarPath := "/" + p.name + "/-/" + filepath.Base(p.name) + "-" + p.version + ".tgz"
		tarballs[tarPath] = body
		if docs[p.name] == nil {
			docs[p.name] = map[string]any{}
		}
		docs[p.name][p.version] = map[string]any{
			"version": p.version, "dependencies": p.deps,
			"dist": map[string]string{"tarball": server.URL + tarPath, "integrity": integrity},
		}
	}
	return server
}

func runLock(t *testing.T, server *httptest.Server, spec lockSpec, out string) error {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, spec.ID+".lock.json")
	data, _ := json.Marshal(spec)
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	l := &locker{registry: server.URL, cache: filepath.Join(dir, "cache"), client: server.Client()}
	return l.run(context.Background(), specPath, out)
}

type recipeOut struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	License   any    `yaml:"license"`
	Libraries []struct {
		Package, Version, License, Description, Entry, Quickstart string
	} `yaml:"libraries"`
	Bundle []struct {
		URL, SHA256, Strip, Dest, Main string
		Include                        []string
		Extracted                      struct{ Files int }
	} `yaml:"bundle"`
	Archive struct{ Files, Bytes int } `yaml:"archive"`
}

func readRecipe(t *testing.T, path string) recipeOut {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r recipeOut
	if err := yaml.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestJSLockNestedVersions(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.0.0", deps: map[string]string{"c": "^1.0.0"}, files: map[string]string{"index.js": `import c from "c"; export default c;`, "README.md": "# a", "LICENSE": "MIT", "test/a.test.js": "x"}},
		{name: "b", version: "1.0.0", deps: map[string]string{"c": "2.x"}, files: map[string]string{"index.js": `import c from "c"; export default c;`}},
		{name: "c", version: "1.4.0", files: map[string]string{"index.js": `export default "c1";`}},
		{name: "c", version: "2.1.0", files: map[string]string{"index.js": `export default "c2";`}},
		{name: "c", version: "1.2.0", files: map[string]string{"index.js": `export default "old";`}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x", "b": "1.x"}}, out); err != nil {
		t.Fatal(err)
	}
	r := readRecipe(t, out)
	dests := map[string]string{}
	for _, s := range r.Bundle {
		dests[s.Dest] = s.URL
		if s.SHA256 == "" || s.Strip != "package" {
			t.Errorf("%s: sha256 %q strip %q", s.Dest, s.SHA256, s.Strip)
		}
	}
	if !strings.HasSuffix(dests["node_modules/c"], "c-1.4.0.tgz") || !strings.HasSuffix(dests["node_modules/b/node_modules/c"], "c-2.1.0.tgz") || len(dests) != 4 {
		t.Fatalf("dests = %v, want c@1.4.0 at the top and c@2.1.0 below b", dests)
	}
	for _, s := range r.Bundle {
		if s.Dest == "node_modules/a" && strings.Join(s.Include, ",") != "LICENSE,README.md,index.js,package.json" {
			t.Errorf("a includes %v, want its closure, licence, README and package.json", s.Include)
		}
	}
	if len(r.Libraries) != 2 || r.Libraries[0].Version != "1.0.0" || r.Libraries[0].Quickstart != "a.md" || r.Libraries[0].License != "MIT" {
		t.Errorf("libraries = %+v", r.Libraries)
	}
	if r.Archive.Files != 14 || r.Archive.Bytes == 0 {
		t.Errorf("archive = %+v, want 13 files and the index", r.Archive)
	}
}

func TestJSLockIntegrityMismatchWritesNothing(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.0.0", deps: map[string]string{"c": "1"}, files: map[string]string{"index.js": `import "c";`}},
		{name: "c", version: "1.0.0", files: map[string]string{"index.js": ``}, badIntegrity: true},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	os.WriteFile(out, []byte("id: js-test\ntype: js-lib\n"), 0o644)
	err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x"}}, out)
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("err = %v, want the integrity mismatch", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "id: js-test\ntype: js-lib\n" {
		t.Errorf("the recipe was changed: %q", data)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Error("a temporary recipe was left")
	}
}

func TestJSLockDropsNodeOnly(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.0.0", deps: map[string]string{"cli-tool": "1", "helper": "1"}, files: map[string]string{"index.js": `import h from "helper"; export default h;`}},
		{name: "cli-tool", version: "1.0.0", files: map[string]string{"bin/cli.js": "#!/usr/bin/env node"}},
		{name: "helper", version: "1.0.0", files: map[string]string{"index.js": `export default 1;`, "node.js": "require('fs')"}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x"}}, out); err != nil {
		t.Fatal(err)
	}
	r := readRecipe(t, out)
	var dests []string
	for _, s := range r.Bundle {
		dests = append(dests, s.Dest)
		if s.Dest == "node_modules/helper" && strings.Join(s.Include, ",") != "LICENSE,index.js,package.json" {
			t.Errorf("helper includes %v, want only what a page reads", s.Include)
		}
	}
	if strings.Join(dests, ",") != "node_modules/a,node_modules/helper" {
		t.Errorf("bundle = %v, want cli-tool dropped", dests)
	}
}

func TestJSLockPreservesHandFields(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.1.0", files: map[string]string{"index.js": `export default 1;`}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	hand := `id: js-test
type: js-lib
description: Hand-written description
license:
  id: MIT
  attribution: A authors
libraries:
  - package: a
    version: 1.0.0
    license: MIT
    description: Draws things
    entry: a
    portability: single-file
    quickstart: a.md
bundle: []
`
	os.WriteFile(out, []byte(hand), 0o644)
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x"}}, out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	for _, want := range []string{"description: Hand-written description", "attribution: A authors", "description: Draws things", "portability: single-file", "version: 1.1.0"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("recipe lost %q:\n%s", want, data)
		}
	}
	if r := readRecipe(t, out); len(r.Bundle) != 1 {
		t.Errorf("bundle = %+v", r.Bundle)
	}
}

func TestJSLockDirectTarball(t *testing.T) {
	sheet := tgz(t, map[string]string{"package.json": `{"name":"xlsx","version":"0.20.3","dependencies":{"helper":"1"}}`, "xlsx.mjs": `import h from "helper"; export default h;`, "LICENSE": "Apache-2.0"})
	server := fakeRegistry(t, []fakePackage{
		{name: "helper", version: "1.0.0", files: map[string]string{"index.js": `export default 1;`}},
	}, map[string][]byte{"/cdn/xlsx-0.20.3.tgz": sheet})
	sum := sha256.Sum256(sheet)
	pin := hex.EncodeToString(sum[:])
	spec := lockSpec{ID: "js-test", Entries: map[string][]string{"xlsx": {"xlsx/xlsx.mjs"}}, Tarballs: map[string]tarballSpec{
		"xlsx": {URL: server.URL + "/cdn/xlsx-0.20.3.tgz", Version: "0.20.3", SHA256: pin},
	}}
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, spec, out); err != nil {
		t.Fatal(err)
	}
	r := readRecipe(t, out)
	if len(r.Bundle) != 2 || r.Bundle[1].Dest != "node_modules/xlsx" || r.Bundle[1].SHA256 != pin || !strings.HasSuffix(r.Bundle[1].URL, "/cdn/xlsx-0.20.3.tgz") {
		t.Fatalf("bundle = %+v, want helper and the direct tarball with its sha256", r.Bundle)
	}
	spec.Tarballs["xlsx"] = tarballSpec{URL: server.URL + "/cdn/xlsx-0.20.3.tgz", Version: "0.20.3", SHA256: strings.Repeat("0", 64)}
	if err := runLock(t, server, spec, filepath.Join(t.TempDir(), "x.yaml")); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err = %v, want the wrong sha256 refused", err)
	}
}

func TestJSLockUMDBringsNoDependencies(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "d3", version: "7.9.0", deps: map[string]string{"d3-array": "3"}, files: map[string]string{"dist/d3.min.js": "umd", "src/index.js": `export * from "d3-array";`}},
		{name: "stl", version: "1.0.0", deps: map[string]string{"d3": "7"}, files: map[string]string{"index.js": `const d3 = require("d3"); module.exports = d3;`}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"d3": "7.x", "stl": "1.x"}, UMD: map[string]string{"d3": "dist/d3.min.js"}}, out); err != nil {
		t.Fatal(err)
	}
	r := readRecipe(t, out)
	if len(r.Bundle) != 2 || r.Bundle[0].Main != "dist/d3.min.js" || strings.Join(r.Bundle[0].Include, ",") != "LICENSE,dist/d3.min.js,package.json" {
		t.Fatalf("bundle = %+v, want d3 as its one build with main set and no d3-array", r.Bundle)
	}
}

func TestSemverRanges(t *testing.T) {
	for _, tc := range []struct {
		rng, version string
		want         bool
	}{
		{"4.x", "4.5.1", true}, {"4.x", "5.0.0", false}, {"^1.2.3", "1.9.0", true}, {"^1.2.3", "2.0.0", false},
		{"^0.2.3", "0.2.9", true}, {"^0.2.3", "0.3.0", false}, {"~1.11.0", "1.11.3", true}, {"~1.11.0", "1.12.0", false},
		{">=1.2.0 <2.0.0", "1.5.0", true}, {">=1.2.0 <2.0.0", "2.0.0", false}, {"1 || 3", "3.1.0", true}, {"1 || 3", "2.0.0", false},
		{"*", "9.9.9", true}, {"1.11.x", "1.11.1", true}, {"5.x", "5.1.0-beta.1", false}, {"1.2.3", "1.2.3", true},
		{"1.0.0 - 2.0.0", "2.0.0", true}, {"1.0.0 - 2", "2.5.0", true},
	} {
		r, err := parseRange(tc.rng)
		if err != nil {
			t.Fatalf("%s: %v", tc.rng, err)
		}
		v, _ := parseVersion(tc.version)
		if got := r.allows(v); got != tc.want {
			t.Errorf("%s allows %s = %v, want %v", tc.rng, tc.version, got, tc.want)
		}
	}
}

func TestJSLockPrefersTheLatestTag(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "tone", version: "15.1.22", latest: true, files: map[string]string{"index.js": `export default 1;`}},
		{name: "tone", version: "15.5.73", files: map[string]string{"index.js": `export default 2;`}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"tone": "15.x"}}, out); err != nil {
		t.Fatal(err)
	}
	if r := readRecipe(t, out); r.Libraries[0].Version != "15.1.22" {
		t.Errorf("version = %s, want the latest tag's 15.1.22", r.Libraries[0].Version)
	}
}

// A kept package without a licence file needs its licence text pinned from
// the spec; the text is installed beside it as a plain file.
func TestJSLockLicenceFiles(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.0.0", noLicense: true, files: map[string]string{"dist/a.min.js": "x", "dist/a.min.js.LICENSE.txt": "bundled: decimal.js MIT"}},
	}, map[string][]byte{"/raw/a/v1.0.0/LICENSE": []byte("MIT License\nCopyright a")})
	spec := lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x"}, UMD: map[string]string{"a": "dist/a.min.js"}}
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, spec, out); err == nil || !strings.Contains(err.Error(), "license_files") {
		t.Fatalf("err = %v, want the missing licence named", err)
	}
	spec.LicenseFiles = map[string]string{"a": server.URL + "/raw/a/v{version}/LICENSE"}
	if err := runLock(t, server, spec, out); err != nil {
		t.Fatal(err)
	}
	r := readRecipe(t, out)
	if len(r.Bundle) != 2 || !strings.Contains(strings.Join(r.Bundle[0].Include, ","), "dist/a.min.js.LICENSE.txt") {
		t.Fatalf("bundle = %+v, want the build's .LICENSE.txt kept", r.Bundle)
	}
	if lic := r.Bundle[1]; lic.Dest != "node_modules/a" || !strings.HasSuffix(lic.URL, "/raw/a/v1.0.0/LICENSE") || lic.SHA256 == "" || lic.Extracted.Files != 1 {
		t.Errorf("licence source = %+v", lic)
	}
	if r.Archive.Files != 5 {
		t.Errorf("archive = %+v, want package.json, the build, its .LICENSE.txt, LICENSE and the index", r.Archive)
	}
}

func TestJSLockDropsLibrariesThatLeftTheSpec(t *testing.T) {
	server := fakeRegistry(t, []fakePackage{
		{name: "a", version: "1.0.0", files: map[string]string{"index.js": `export default 1;`}},
		{name: "b", version: "1.0.0", files: map[string]string{"index.js": `export default 2;`}},
	}, nil)
	out := filepath.Join(t.TempDir(), "js-test.yaml")
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x", "b": "1.x"}}, out); err != nil {
		t.Fatal(err)
	}
	if err := runLock(t, server, lockSpec{ID: "js-test", Packages: map[string]string{"a": "1.x"}}, out); err != nil {
		t.Fatal(err)
	}
	if r := readRecipe(t, out); len(r.Libraries) != 1 || r.Libraries[0].Package != "a" {
		t.Errorf("libraries = %+v, want b gone with its source", r.Libraries)
	}
}

func TestJSLockDirectTarballNeedsAPin(t *testing.T) {
	server := fakeRegistry(t, nil, map[string][]byte{"/cdn/x-1.0.0.tgz": tgz(t, map[string]string{"package.json": "{}"})})
	err := runLock(t, server, lockSpec{ID: "js-test", Tarballs: map[string]tarballSpec{"x": {URL: server.URL + "/cdn/x-1.0.0.tgz", Version: "1.0.0"}}}, filepath.Join(t.TempDir(), "x.yaml"))
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err = %v, want an unpinned direct tarball refused", err)
	}
}
