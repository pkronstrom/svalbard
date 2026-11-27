package htmlpack

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

func workspaceFixture(t *testing.T) (string, *workspace.Workspace) {
	t.Helper()
	drive := fixtureDrive(t)
	ws, err := workspace.Open(filepath.Join(drive, workspace.DirName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	return drive, ws
}

func writeWorkspace(t *testing.T, ws *workspace.Workspace, name, contents string) {
	t.Helper()
	if err := ws.WriteFile(name, []byte(contents)); err != nil {
		t.Fatal(err)
	}
}

func TestPackWorkspaceInlinesDependenciesAndIgnoresImplicitConfig(t *testing.T) {
	drive, ws := workspaceFixture(t)
	for file, contents := range map[string]string{
		"doc/report.html":     `<link rel="stylesheet" href="theme/outer.css"><script type="module" src="js/app.js"></script><img src="../dot.svg">`,
		"doc/theme/outer.css": `@import "./inner.css";`,
		"doc/theme/inner.css": `body { background: url(../../dot.svg) }`,
		"doc/js/app.js":       `import draw from "chartish"; import rows from "./rows.csv"; import "./jsx"; document.title = draw(rows)`,
		"doc/js/jsx.tsx":      `window.preview = <div/>;`,
		"doc/js/rows.csv":     "MARKER,2",
		"dot.svg":             `<svg xmlns="http://www.w3.org/2000/svg"/>`,
	} {
		writeWorkspace(t, ws, file, contents)
	}
	if err := os.WriteFile(filepath.Join(drive, "tsconfig.json"), []byte(`{"compilerOptions":{"jsxFactory":"hostFactory"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := PackWorkspace(context.Background(), drive, ws, "doc/report.html")
	if err != nil {
		t.Fatal(err)
	}
	if result.Out != "doc/report.packed.html" || result.Bytes <= 0 || result.Portability != "single-file" {
		t.Fatalf("result = %+v", result)
	}
	artifact, err := ws.ReadFile(result.Out, workspace.MaxWriteBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(artifact), "hostFactory") {
		t.Error("host tsconfig affected the bundled JSX")
	}
	for _, expected := range []string{"data:image/svg+xml", "MARKER,2", "chartish"} {
		if !strings.Contains(string(artifact), expected) {
			t.Errorf("artifact does not contain %q", expected)
		}
	}
}

func TestPackWorkspaceRefusesUnconfinedDependencies(t *testing.T) {
	drive, ws := workspaceFixture(t)
	secret := filepath.ToSlash(filepath.Join(drive, "outside.js"))
	if err := os.WriteFile(secret, []byte(`export default "OUTSIDE";`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ws.Dir(), "link.js")); err != nil {
		t.Fatal(err)
	}
	// A failed rebuild leaves the prior artifact in place.
	writeWorkspace(t, ws, "report.packed.html", "previous")
	for _, tt := range []struct{ name, page, dependency, want string }{
		{"absolute script", fmt.Sprintf(`<script src=%q></script>`, secret), "", "relative workspace path"},
		{"absolute module", fmt.Sprintf(`<script type="module" src=%q></script>`, secret), "", "relative workspace path"},
		{"file URL", fmt.Sprintf(`<img src="file://%s">`, secret), "", "relative workspace path"},
		{"transitive module", `<script type=module src="./app.js"></script>`, `import x from "../outside.js"; document.title = x`, "outside the workspace"},
		{"symlink module", `<script type=module src="./app.js"></script>`, `import x from "./link.js"; document.title = x`, "leaves the workspace"},
		{"transitive CSS", `<link rel="stylesheet" href="./level.css">`, `body { background: url(../outside.js) }`, "outside the workspace"},
		{"implicit glob import", `<script type=module src="./app.js"></script>`, "import(`../outside${location.hash}.js`)", "literal import"},
		{"implicit glob require", `<script type=module src="./app.js"></script>`, "require('./' + location.hash + '.js')", "literal import"},
		{"script data URL with whitespace", `<script src="d&#10;at&#9;a:text/javascript,alert(1)"></script>`, "", "blocked by the workspace preview"},
		{"served library", `<script type="module">import "tiles"</script>`, "", "needs the drive's server"},
		{"script data URL", `<script src="data:text/javascript,alert(1)"></script>`, "", "blocked by the workspace preview"},
		{"module data import", `<script type="module">import "data:text/javascript,alert(1)"</script>`, "", "blocked by the workspace preview"},
		{"generated script data URL", `<script src="app.js"></script>`, `document.title = "<!-- <script>";`, "blocked by the workspace preview"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writeWorkspace(t, ws, "report.html", tt.page)
			writeWorkspace(t, ws, "app.js", tt.dependency)
			writeWorkspace(t, ws, "level.css", tt.dependency)
			_, err := PackWorkspace(context.Background(), drive, ws, "report.html")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			artifact, err := ws.ReadFile("report.packed.html", workspace.MaxWriteBytes)
			if err != nil || string(artifact) != "previous" {
				t.Fatalf("previous output = %q, %v", artifact, err)
			}
		})
	}
}

func TestPackWorkspaceLimitsOutputAndArchivedMembers(t *testing.T) {
	drive, ws := workspaceFixture(t)
	writeWorkspace(t, ws, "report.html", `<img src="image.png">`)
	writeWorkspace(t, ws, "image.png", strings.Repeat("a", workspace.MaxWriteBytes))
	if _, err := PackWorkspace(context.Background(), drive, ws, "report.html"); err == nil || !strings.Contains(err.Error(), "packed page exceeds") {
		t.Fatalf("oversized artifact error = %v", err)
	}
	if _, err := ws.Stat("report.packed.html"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oversized output was written, stat = %v", err)
	}

	archive, err := os.Create(filepath.Join(drive, "lib", "js", "oversized.zip"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	member, err := zw.Create("node_modules/oversized/index.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write([]byte(strings.Repeat("a", (8<<20)+1))); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	writeWorkspace(t, ws, "report.html", `<script type="module">import "oversized"</script>`)
	if _, err := PackWorkspace(context.Background(), drive, ws, "report.html"); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("oversized archive member error = %v", err)
	}
}

func TestPackWorkspaceHonorsCancellation(t *testing.T) {
	drive, ws := workspaceFixture(t)
	writeWorkspace(t, ws, "report.html", `<h1>test</h1>`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PackWorkspace(ctx, drive, ws, "report.html"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if _, err := ws.Stat("report.packed.html"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled output was written, stat = %v", err)
	}
}

func TestPackWorkspaceCannotLoadLibraryArchiveOutsideItsStore(t *testing.T) {
	drive, ws := workspaceFixture(t)
	store := filepath.Join(drive, "lib", "js")
	moved := filepath.Join(drive, "outside.zip")
	if err := os.Rename(filepath.Join(store, "js-a.zip"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../outside.zip", filepath.Join(store, "js-a.zip")); err != nil {
		t.Fatal(err)
	}
	writeWorkspace(t, ws, "page.html", `<script type=module>import "chartish"</script>`)
	if _, err := PackWorkspace(context.Background(), drive, ws, "page.html"); err == nil || !strings.Contains(err.Error(), "js-a.zip cannot be read") {
		t.Fatalf("archive link error = %v", err)
	}
}

func TestPackWorkspaceRefusesToOverwriteItsSourceThroughALink(t *testing.T) {
	drive, ws := workspaceFixture(t)
	writeWorkspace(t, ws, "report.packed.html", `<h1>Original source</h1>`)
	if err := os.Symlink("report.packed.html", filepath.Join(ws.Dir(), "report.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := PackWorkspace(context.Background(), drive, ws, "report.html"); err == nil || !strings.Contains(err.Error(), "replace the source") {
		t.Fatalf("source alias error = %v", err)
	}
}
