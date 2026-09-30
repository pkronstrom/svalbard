package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestUserPresetCLIFlow(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	// preset copy with a bare name writes an extends stub to the user preset dir.
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"preset", "copy", "default-32", "my-32"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preset copy: %v", err)
	}
	stub := filepath.Join(cfg, "svalbard", "presets", "my-32.yaml")
	if _, err := os.Stat(stub); err != nil {
		t.Fatalf("stub not written to user preset dir: %v", err)
	}

	// preset list shows it, marked as a user preset.
	out.Reset()
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"preset", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preset list: %v", err)
	}
	if !strings.Contains(out.String(), "my-32  (user: "+stub+")") {
		t.Fatalf("preset list missing user preset:\n%s", out.String())
	}

	// The wizard offers it.
	config, err := buildWizardConfig("")
	if err != nil {
		t.Fatal(err)
	}
	offered := false
	for _, p := range config.Presets {
		offered = offered || (p.Name == "my-32" && p.Custom && p.Region == "default")
	}
	if !offered {
		t.Error("wizard config does not offer my-32 as a custom preset")
	}

	// add/remove accept pack names.
	vault := t.TempDir()
	cat, err := catalog.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := commands.InitVaultWithOptions(vault, nil, "", "", nil); err != nil {
		t.Fatal(err)
	}
	pack, err := cat.ResolvePreset("tools-base")
	if err != nil || len(pack.Items) == 0 {
		t.Fatalf("tools-base pack: %v (%d items)", err, len(pack.Items))
	}
	for _, verb := range []string{"add", "remove"} {
		cmd = NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{verb, "tools-base", "local:keep", "--vault", vault})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s tools-base: %v", verb, err)
		}
		m, err := manifest.Load(filepath.Join(vault, "manifest.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if verb == "add" {
			want = len(pack.Items) + 1
		}
		if len(m.Desired.Items) != want {
			t.Fatalf("after %s: %d desired items, want %d: %v", verb, len(m.Desired.Items), want, m.Desired.Items)
		}
	}
}
