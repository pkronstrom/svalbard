package htmlpack

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureDrive builds a drive from testdata/libs/<recipe>/: its
// node_modules and svalbard-js.json become lib/js/<recipe>.zip, as apply
// writes them, and its docs/ the recipe's quickstarts.
func fixtureDrive(t *testing.T) string {
	t.Helper()
	drive := t.TempDir()
	recipes, err := os.ReadDir("testdata/libs")
	if err != nil {
		t.Fatal(err)
	}
	for _, recipe := range recipes {
		src := filepath.Join("testdata", "libs", recipe.Name())
		zipPath := filepath.Join(drive, "lib", "js", recipe.Name()+".zip")
		os.MkdirAll(filepath.Dir(zipPath), 0o755)
		out, err := os.Create(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(out)
		filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			rel = filepath.ToSlash(rel)
			data, _ := os.ReadFile(p)
			if strings.HasPrefix(rel, "docs/") {
				target := filepath.Join(drive, "lib", "js", recipe.Name(), filepath.FromSlash(rel))
				os.MkdirAll(filepath.Dir(target), 0o755)
				return os.WriteFile(target, data, 0o644)
			}
			w, err := zw.Create(rel)
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		})
		zw.Close()
		out.Close()
	}
	return drive
}

// page writes a page and the files beside it into a work folder.
func page(t *testing.T, body string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644)
	}
	p := filepath.Join(dir, "page.html")
	os.WriteFile(p, []byte(body), 0o644)
	return p
}

func pack(t *testing.T, drive, pagePath string, dir bool) (Result, string, error) {
	t.Helper()
	res, err := Pack(context.Background(), Options{DriveRoot: drive, Page: pagePath, Dir: dir})
	if err != nil {
		return res, "", err
	}
	target := res.Out
	if dir {
		target = filepath.Join(res.Out, "index.html")
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return res, string(data), nil
}

func problemsOf(t *testing.T, err error) []Problem {
	t.Helper()
	var pe *ProblemsError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want problems", err)
	}
	return pe.Problems
}

func hasProblem(problems []Problem, kind, text string) bool {
	for _, p := range problems {
		if p.Kind == kind && strings.Contains(p.Detail, text) {
			return true
		}
	}
	return false
}

func TestPackInlinesModuleAndCSS(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<!doctype html><html><head><title>t</title><link rel="stylesheet" href="./own.css"></head><body>
<div id="out" class="x"></div>
<script type="module">
import draw from "chartish";
import { extra } from "chartish/extra";
import "style-lib";
draw(document.getElementById("out"));
console.log(extra);
</script></body></html>`, map[string]string{"own.css": "body { color: #123456; }"})
	res, out, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Portability != "single-file" || res.Out != strings.TrimSuffix(p, ".html")+".packed.html" || res.Bytes != int64(len(out)) {
		t.Errorf("result = %+v", res)
	}
	for _, want := range []string{"chartish:", "C1-MARKER", "EXTRA-MARKER", "data:image/png;base64,", "#123456", `<script type="module">`} {
		if !strings.Contains(out, want) {
			t.Errorf("packed page lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "src=") || strings.Contains(out, "own.css") {
		t.Errorf("packed page still references files:\n%s", out)
	}
	if problems := Check([]byte(out)); len(problems) != 0 {
		t.Errorf("Check(packed) = %v", problems)
	}
}

func TestPackInlinesImportedCSV(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><body><pre id="o"></pre><script type="module">
import text from "./data.csv";
import places from "./places.geojson";
document.getElementById("o").textContent = text + places.features.length;
</script></body></html>`, map[string]string{
		"data.csv":       "name,count\nwater,12\n",
		"places.geojson": `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"name":"HELSINKI-MARKER"},"geometry":{"type":"Point","coordinates":[24.9,60.2]}}]}`,
	})
	_, out, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "name,count\nwater,12") || !strings.Contains(out, "HELSINKI-MARKER") {
		t.Errorf("the imported files are not in the page:\n%s", out)
	}
}

func TestPackRejectsRemoteScript(t *testing.T) {
	drive := fixtureDrive(t)
	for name, body := range map[string]string{
		"classic":   `<html><head><script src="https://cdn.jsdelivr.net/npm/chart.js"></script></head><body></body></html>`,
		"import":    `<html><body><script type="module">import x from "https://esm.sh/chart.js"; x();</script></body></html>`,
		"style":     `<html><head><link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter"></head></html>`,
		"font face": `<html><head><style>@font-face { font-family: X; src: url(https://example.com/x.woff2); }</style></head></html>`,
	} {
		t.Run(name, func(t *testing.T) {
			p := page(t, body, nil)
			_, _, err := pack(t, drive, p, false)
			if problems := problemsOf(t, err); !hasProblem(problems, "remote-resource", "") {
				t.Fatalf("problems = %v, want a remote resource", problems)
			}
			if _, err := os.Stat(strings.TrimSuffix(p, ".html") + ".packed.html"); !os.IsNotExist(err) {
				t.Error("a page with problems was written")
			}
		})
	}
}

// RF3: a page that fetches a file beside it cannot work from file://.
func TestPackRejectsRelativeFetch(t *testing.T) {
	drive := fixtureDrive(t)
	for name, call := range map[string]string{
		"fetch": `fetch("./data.csv").then(r => r.text()).then(console.log);`,
		"d3":    `const d3 = { csv: () => 0 }; d3.csv("data.csv").then(console.log);`,
	} {
		t.Run(name, func(t *testing.T) {
			p := page(t, `<html><body><script type="module">`+call+`</script></body></html>`, map[string]string{"data.csv": "a\n1\n"})
			_, _, err := pack(t, drive, p, false)
			problems := problemsOf(t, err)
			if !hasProblem(problems, "relative-fetch", "relative fetch cannot work from file://; import the file instead") {
				t.Fatalf("problems = %v", problems)
			}
		})
	}
}

// RF2: a library measured "served" cannot go into a single file; --dir
// writes a folder for the drive's server.
func TestPackServedLibraryNeedsDir(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><body><script type="module">import { Protocol } from "tiles"; console.log(new Protocol().tile());</script></body></html>`, nil)
	_, _, err := pack(t, drive, p, false)
	if problems := problemsOf(t, err); !hasProblem(problems, "served-library", "tiles") || !hasProblem(problems, "served-library", "--dir") {
		t.Fatalf("problems = %v, want tiles named with --dir", problems)
	}
	res, out, err := pack(t, drive, p, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Portability != "directory" || !strings.Contains(out, `src="./script-1.js"`) {
		t.Fatalf("dir result = %+v, index.html:\n%s", res, out)
	}
	script, err := os.ReadFile(filepath.Join(res.Out, "script-1.js"))
	if err != nil || !strings.Contains(string(script), "TILES") {
		t.Fatalf("script-1.js = %q, %v", script, err)
	}
}

// RF1: a page's import resolves in the recipe that has the library, and the
// library's own imports in that recipe's tree: never another recipe's
// version of the same package.
func TestPackResolvesOwnRecipeTree(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><body><script type="module">
import draw from "chartish";
import { other } from "other";
draw(document.body); console.log(other);
</script></body></html>`, nil)
	_, out, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "C1-MARKER") || !strings.Contains(out, "C2-NESTED-MARKER") {
		t.Errorf("each library should bring its own c:\n%s", out)
	}
	if strings.Contains(out, "C2-TOP-MARKER") {
		t.Errorf("js-b's top-level c was mixed in:\n%s", out)
	}
	// c itself is in both archives and a library of neither.
	q := page(t, `<html><body><script type="module">import { c } from "c"; console.log(c);</script></body></html>`, nil)
	_, _, err = pack(t, drive, q, false)
	if problems := problemsOf(t, err); !hasProblem(problems, "build", "js-a and js-b") {
		t.Errorf("problems = %v, want the two recipes named", problems)
	}
}

func TestPackWarnsOverTenMB(t *testing.T) {
	drive := fixtureDrive(t)
	big := strings.Repeat("0123456789abcdef", 11<<16) // 11 MB
	p := page(t, `<html><body><script type="module">import t from "./big.txt"; console.log(t.length);</script></body></html>`, map[string]string{"big.txt": big})
	res, _, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "MB") {
		t.Errorf("warnings = %v, want one about the size", res.Warnings)
	}
}

func TestPackNoticeComment(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><body><script type="module">import draw from "chartish"; draw(document.body);</script></body></html>`, nil)
	_, out, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- licenses: c 1.0.0 (ISC); chartish 1.0.0 (MIT) -->") {
		t.Errorf("licence comment missing:\n%s", out)
	}
}

func TestPackClassicScriptAndMainByPackage(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><head><script src="umdlib"></script></head><body><script type="module">
import lib from "umdlib";
document.body.textContent = lib.hello() + umdlib.hello();
</script></body></html>`, nil)
	_, out, err := pack(t, drive, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "UMD-MARKER") != 2 || strings.Contains(out, `src="umdlib"`) {
		t.Errorf("the classic script and the import should both hold the build:\n%s", out)
	}
	q := page(t, `<html><head><script src="nosuchlib"></script></head></html>`, nil)
	if _, _, err := pack(t, drive, q, false); !hasProblem(problemsOf(t, err), "missing-asset", "nosuchlib") {
		t.Error("an unknown package in a classic script was not reported")
	}
}

func TestPackUnknownLibraryNamesTheDrivesOwn(t *testing.T) {
	drive := fixtureDrive(t)
	p := page(t, `<html><body><script type="module">import Chart from "chart.js/auto";</script></body></html>`, nil)
	_, _, err := pack(t, drive, p, false)
	if problems := problemsOf(t, err); !hasProblem(problems, "build", "chartish") {
		t.Errorf("problems = %v, want the drive's libraries listed", problems)
	}
}

func TestCheckFindsProblems(t *testing.T) {
	problems := Check([]byte(`<html><head>
<link rel="stylesheet" href="//cdn.example/x.css">
<link rel="icon" href="favicon.png">
<style>.a { background: url("https://x/y.png") }</style>
</head><body>
<img src="photo.jpg"><a href="https://example.com">a link is fine</a>
<img src="data:image/png;base64,AAAA">
<script>fetch("rows.json").then(r => r.json()); fetch("https://api.example/x");
const svg = "http://www.w3.org/2000/svg";</script>
</body></html>`))
	want := map[string]int{"remote-resource": 3, "missing-asset": 2, "relative-fetch": 1}
	got := map[string]int{}
	for _, p := range problems {
		got[p.Kind]++
	}
	for kind, n := range want {
		if got[kind] != n {
			t.Errorf("%s: %d problems, want %d (%v)", kind, got[kind], n, problems)
		}
	}
}
