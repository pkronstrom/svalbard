package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestPlanShowsTheRebuildOfOlderPythonTrees(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nstrategy: build\nbuild: {family: python-venv}\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"svalbard-python"}
	m.Desired.Options.HostPlatforms = []string{"linux-arm64"}
	m.Realized.Entries = []manifest.RealizedEntry{{ID: "svalbard-python", Type: "python-venv", RelativePath: "runtime/python"}}
	plan := func() string {
		var text bytes.Buffer
		d := DescribeVaultPlan(root, cat, m)
		if err := WritePlanDetail(&text, d); err != nil {
			t.Fatal(err)
		}
		WritePlanNext(&text, d)
		return text.String()
	}

	if got, want := plan(), "Nothing to download or remove.\n"; got != want {
		t.Errorf("plan of a current vault =\n%s\nwant\n%s", got, want)
	}
	// An interpreter an older CLI installed, without a layout stamp.
	if err := os.MkdirAll(filepath.Join(root, "runtime", "python", "linux-arm64", "python", "cpython-3.12.16-linux-aarch64-gnu"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "Nothing to download or remove.\nrebuild: Python trees (older layout)\n\nRun \"svalbard apply\" to make these changes.\n"
	if got := plan(); got != want {
		t.Errorf("plan =\n%s\nwant\n%s", got, want)
	}
}
