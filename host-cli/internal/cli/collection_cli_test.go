package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// TestUserCollectionCLIFlow adds a collection defined in the user recipe dir
// and reads its plan, the way a user does before the first apply.
func TestUserCollectionCLIFlow(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	recipes := filepath.Join(cfg, "svalbard", "recipes")
	if err := os.MkdirAll(filepath.Join(recipes, "manuals", "workshop"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"manuals/trailer.pdf", "manuals/workshop/brakes.pdf"} {
		if err := os.WriteFile(filepath.Join(recipes, filepath.FromSlash(rel)), []byte("%PDF-1.7\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Relative paths start at the recipe file's folder.
	collection := "description: Owner manuals\ndocuments:\n  - path: manuals/trailer.pdf\n  - path: manuals/workshop\n"
	if err := os.WriteFile(filepath.Join(recipes, "car-manuals.yaml"), []byte(collection), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipes, "broken.yaml"), []byte("description: nothing listed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	vault := t.TempDir()
	if err := manifest.Save(filepath.Join(vault, "manifest.yaml"), manifest.New("test")); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, string, error) {
		t.Helper()
		var out, errOut bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(append(args, "--vault", vault))
		err := cmd.Execute()
		return out.String(), errOut.String(), err
	}

	out, warnings, err := run("add", "car-manuals")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "Added 1 item(s)") {
		t.Errorf("add output = %q", out)
	}
	if want := "warning: skipped user recipe " + filepath.Join(recipes, "broken.yaml") + ": missing documents"; !strings.Contains(warnings, want) {
		t.Errorf("warnings = %q, want %q", warnings, want)
	}
	if _, _, err := run("add", "broken"); err == nil || !strings.Contains(err.Error(), "missing documents") {
		t.Errorf("adding a skipped recipe: err = %v, want the reason", err)
	}

	out, _, err = run("plan")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if want := "download: 1\n  + car-manuals\nnew car-manuals: 2 documents\n\nRun \"svalbard apply\" to make these changes.\n"; out != want {
		t.Errorf("plan = %q, want %q", out, want)
	}

	// The wizard's customise list offers the collection like any recipe.
	config, err := buildWizardConfig("", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	offered := false
	for _, group := range config.PackGroups {
		for _, pack := range group.Packs {
			for _, source := range pack.Sources {
				offered = offered || (source.ID == "car-manuals" && source.Description == "Owner manuals")
			}
		}
	}
	if !offered {
		t.Error("wizard config does not offer the user collection")
	}
}

// TestCollectionPlanOnAnotherComputer plans a vault whose collection is not
// defined in this computer's config folder: the copy on the vault is used,
// and the command says so.
func TestCollectionPlanOnAnotherComputer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	vault := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"car-manuals"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "car-manuals", Type: "pdf", Filename: "volvo.pdf", RelativePath: filepath.Join("books", "car-manuals", "volvo.pdf"), SizeBytes: 9},
		{ID: "car-manuals", Type: "pdf", Filename: "trailer.pdf", RelativePath: filepath.Join("books", "car-manuals", "trailer.pdf"), SizeBytes: 9},
	}
	if err := manifest.Save(filepath.Join(vault, "manifest.yaml"), m); err != nil {
		t.Fatal(err)
	}
	copyDir := filepath.Join(vault, ".svalbard", "recipes")
	if err := os.MkdirAll(copyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	definition := "documents:\n  - url: https://example.test/volvo.pdf\n  - url: https://example.test/hmmwv.pdf\n  - path: ~/Documents/manuals/trailer.pdf\n"
	if err := os.WriteFile(filepath.Join(copyDir, "car-manuals.yaml"), []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"plan", "--vault", vault})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if want := "note: car-manuals is not defined on this computer; using the vault's copy of its recipe\n"; errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
	want := "download: 1\n  + car-manuals\n" +
		"update car-manuals: +1 document\n" +
		"car-manuals: kept trailer.pdf: local source, not refreshed from the vault's copy\n" +
		"\nRun \"svalbard apply\" to make these changes.\n"
	if out.String() != want {
		t.Errorf("plan = %q, want %q", out.String(), want)
	}
}
