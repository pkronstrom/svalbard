package frontpage

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestAtlasReadsOnlyDiscoveredSourceViews(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/js/core/docs/demo.md", "# Quickstart")
	writeFile(t, root, "skills/field/map/SKILL.md", "# Map skill")
	f, err := os.Create(filepath.Join(root, "lib/js/core.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, body := range map[string]string{"svalbard-js.json": `{"main":{"node_modules/demo":"index.js"}}`, "node_modules/demo/index.js": "export const answer = 42;", "node_modules/demo/package.json": `{"version":"1.0.0"}`} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	client, origin, _ := startDashboard(t, root, config.RuntimeConfig{})
	var inv inventory
	readResponse(t, requestBody(t, client, origin+"/api/inventory", 200), &inv)
	if len(inv.Groups) != 2 || len(inv.Groups[0].Items) != 1 || len(inv.Groups[0].Items[0].Parts) != 3 {
		t.Fatalf("unexpected resources: %+v", inv.Groups)
	}
	if got := requestBody(t, client, origin+"/api/source?id=lib/js/core.zip/demo&view=Source", 200); !strings.Contains(got, "answer = 42") {
		t.Fatal(got)
	}
	requestBody(t, client, origin+"/api/source?id=lib/js/core.zip/demo&view=../private", 404)
	if err := os.Remove(filepath.Join(root, "lib/js/core/docs/demo.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "skills/field/map/SKILL.md"), filepath.Join(root, "lib/js/core/docs/demo.md")); err != nil {
		t.Skip(err)
	}
	requestBody(t, client, origin+"/api/source?id=lib/js/core.zip/demo&view=Overview", 404)
}
