package mcp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilePackHTMLReturnsSiblingLink(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	params := map[string]any{"path": "reports/example chart.html"}
	wantFileError(t, cap, "pack_html", params, "unknown file action")
	cap = cap.WithHTMLPacking(filepath.Dir(dir))
	var writes bool
	for _, action := range cap.Actions() {
		if action.Name == "pack_html" {
			writes = action.Writes
		}
	}
	if !writes {
		t.Fatal("configured capability does not advertise pack_html as a write action")
	}

	mustCallFile(t, cap, "write", map[string]any{
		"path": params["path"], "content": `<script type=module>import msg from "./label.txt"; document.title = msg;</script>`,
	})
	mustCallFile(t, cap, "write", map[string]any{"path": "reports/label.txt", "content": "Offline chart"})
	out := mustCallFile(t, cap, "pack_html", params)
	if !strings.Contains(out, `"link":"[reports/example chart.packed.html](http://127.0.0.1:8086/reports/example%20chart.packed.html)"`) {
		t.Fatalf("pack_html = %s", out)
	}
	artifact, err := os.ReadFile(filepath.Join(dir, "reports", "example chart.packed.html"))
	if err != nil || !strings.Contains(string(artifact), "Offline chart") || strings.Contains(string(artifact), "label.txt") {
		t.Fatalf("artifact = %q, %v", artifact, err)
	}
}
