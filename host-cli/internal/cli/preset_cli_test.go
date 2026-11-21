package cli

import (
	"bytes"
	"io"
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
	config, err := buildWizardConfig("", io.Discard)
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

func TestBrokenUserPresetIsSkippedWithWarning(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "svalbard", "presets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good-32.yaml"), []byte("extends: [default-32]\nsources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(broken, []byte("extends: [default-32]\nsources: [no-such-recipe]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantWarning := "warning: skipped user preset " + broken + `: unknown source id "no-such-recipe"`

	// Commands still load the catalog; the broken file is named once on stderr.
	var out, errOut bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"preset", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preset list with a broken user preset: %v", err)
	}
	if !strings.Contains(out.String(), "good-32  (user: ") || strings.Contains(out.String(), "broken") {
		t.Errorf("preset list should show good-32 and not broken:\n%s", out.String())
	}
	if strings.TrimSpace(errOut.String()) != wantWarning {
		t.Errorf("stderr = %q, want %q", errOut.String(), wantWarning)
	}

	// Naming the skipped preset explains why it is unavailable.
	vault := t.TempDir()
	if err := commands.InitVaultWithOptions(vault, nil, "", "", nil); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"preset", "copy", "broken", "mine"},
		{"add", "broken", "--vault", vault},
	} {
		cmd = NewRootCommand()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "was skipped") || !strings.Contains(err.Error(), "no-such-recipe") {
			t.Errorf("%v: err = %v, want the skip reason", args, err)
		}
	}

	// The wizard offers only the valid preset.
	errOut.Reset()
	config, err := buildWizardConfig("", &errOut)
	if err != nil {
		t.Fatal(err)
	}
	var custom []string
	for _, p := range config.Presets {
		if p.Custom {
			custom = append(custom, p.Name)
		}
	}
	if len(custom) != 1 || custom[0] != "good-32" {
		t.Errorf("wizard custom presets = %v, want [good-32]", custom)
	}
	if !strings.Contains(errOut.String(), wantWarning) {
		t.Errorf("wizard warning = %q, want %q", errOut.String(), wantWarning)
	}
}
