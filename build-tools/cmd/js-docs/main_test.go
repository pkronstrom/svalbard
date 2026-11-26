package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTarball(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: "package/" + name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestJSDocsJoinsRepoMarkdownAndTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/repos/o/chart/git/trees/v4.6.2":
			json.NewEncoder(w).Encode(map[string]any{"tree": []map[string]string{
				{"path": "docs/axes/linear.md", "type": "blob"},
				{"path": "docs/samples/bar.md", "type": "blob"},
				{"path": "docs/index.md", "type": "blob"},
				{"path": "CHANGELOG.md", "type": "blob"},
			}})
		case "/raw/o/chart/v4.6.2/docs/index.md":
			w.Write([]byte("---\ntitle: x\n---\n# Chart.js\n\nIntro.\n\n## Install\n\n```sh\n# not a heading\n```\n"))
		case "/raw/o/chart/v4.6.2/docs/axes/linear.md":
			w.Write([]byte("# Linear Axis\n\n<script setup>import x from 'y'</script>\nText.\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	os.MkdirAll(cache, 0o755)
	writeTarball(t, filepath.Join(cache, "chart.js-4.6.2.tgz"), map[string]string{"README.md": "# chart.js\n\nSimple charts.\n", "package.json": "{}"})
	writeTarball(t, filepath.Join(cache, "dompurify-3.5.0.tgz"), map[string]string{"README.md": "# DOMPurify\n", "dist/purify.es.d.mts": "export declare function sanitize(s: string): string;"})
	spec := filepath.Join(dir, "js-core.lock.json")
	os.WriteFile(spec, []byte(`{"id":"js-core","docs":{
		"chart.js":{"repo":"o/chart","ref":"v{version}","include":["docs/**/*.md"],"exclude":["docs/samples/**"],"license":"MIT"},
		"dompurify":{"types":["dist/*.d.mts"],"license":"MPL-2.0 OR Apache-2.0"}}}`), 0o644)
	recipe := filepath.Join(dir, "js-core.yaml")
	os.WriteFile(recipe, []byte("id: js-core\nlibraries:\n  - package: chart.js\n    version: 4.6.2\n    license: MIT\n    description: Charts\n  - package: dompurify\n    version: 3.5.0\n"), 0o644)
	out := filepath.Join(dir, "out")

	g := &generator{cache: cache, client: server.Client(), github: server.URL + "/api", raw: server.URL + "/raw"}
	if err := g.run(context.Background(), spec, recipe, out); err != nil {
		t.Fatal(err)
	}
	chart, _ := os.ReadFile(filepath.Join(out, "chart.js.reference.md"))
	text := string(chart)
	for _, want := range []string{"# chart.js 4.6.2 reference", "## chart.js\n\nSimple charts.", "## Linear Axis\n", "## Chart.js\n\nIntro.", "## Chart.js: Install", "# not a heading"} {
		if !strings.Contains(text, want) {
			t.Errorf("chart.js reference lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"title: x", "<script", "samples", "CHANGELOG"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("chart.js reference holds %q", unwanted)
		}
	}
	purify, _ := os.ReadFile(filepath.Join(out, "dompurify.reference.md"))
	if !strings.Contains(string(purify), "### dist/purify.es.d.mts\n\n```ts\nexport declare function sanitize") {
		t.Errorf("dompurify reference = %s", purify)
	}
	updated, _ := os.ReadFile(recipe)
	if !strings.Contains(string(updated), "of github.com/o/chart at v4.6.2; MIT") || !strings.Contains(string(updated), "description: Charts") {
		t.Errorf("recipe = %s", updated)
	}
}

func TestJSDocsRefusesAnotherVersion(t *testing.T) {
	g := &generator{}
	dir := t.TempDir()
	writeTarball(t, filepath.Join(dir, "a-1.0.0.tgz"), map[string]string{"README.md": "# a"})
	_, _, err := g.reference(context.Background(), recipeLibrary{pkg: "a", version: "1.0.0", tarball: filepath.Join(dir, "a-1.0.0.tgz")}, libraryDocSpec{Repo: "o/a", Ref: "main"})
	if err == nil || !strings.Contains(err.Error(), "pinned version") {
		t.Fatalf("err = %v, want a ref other than the pinned version refused", err)
	}
}

func TestMatchGlob(t *testing.T) {
	for _, tc := range []struct {
		pattern, name string
		want          bool
	}{
		{"docs/**/*.md", "docs/a.md", true}, {"docs/**/*.md", "docs/x/y/a.md", true}, {"docs/**/*.md", "src/a.md", false},
		{"packages/turf-*/README.md", "packages/turf-area/README.md", true}, {"es/api/**/*.d.ts", "es/api/PDFDocument.d.ts", true},
	} {
		if got := matchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v", tc.pattern, tc.name, got)
		}
	}
}

func TestMarkNetworkUse(t *testing.T) {
	in := "Install it:\n\n```sh\nnpm install chart.js\n```\n\nOr `<script src=\"https://cdn.jsdelivr.net/npm/chart.js\"></script>`.\n\n```js\nnew Chart(ctx, {});\n```\n"
	out := markNetworkUse(in)
	for _, want := range []string{"```not-for-this-stick\nnpm install chart.js", "Not for this drive: Or `<script src", "```js\nnew Chart"} {
		if !strings.Contains(out, want) {
			t.Errorf("marked text lacks %q:\n%s", want, out)
		}
	}
}
