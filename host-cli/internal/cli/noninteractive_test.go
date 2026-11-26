package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// run executes the CLI with args and returns stdout, stderr and the error.
func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestInitWithPresetCreatesTheVaultTheWizardWould(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	out, _, err := run(t, "init", root, "--preset", "default-2", "--add", "zimgit-knots", "--remove", "cd3wd", "--platform", "all")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "svalbard apply --vault "+root) {
		t.Errorf("init should name the next command:\n%s", out)
	}
	m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Desired.Presets; len(got) != 1 || got[0] != "default-2" {
		t.Errorf("presets = %v", got)
	}
	if len(m.Desired.Options.HostPlatforms) != 4 || m.Desired.Options.Region != "default" {
		t.Errorf("options = %+v", m.Desired.Options)
	}
	items := m.Desired.Items
	// kiwix-serve is the dependency every ZIM needs; nothing names it.
	if !slices.Contains(items, "zimgit-knots") || !slices.Contains(items, "kiwix-serve") || slices.Contains(items, "cd3wd") {
		t.Errorf("items = %v", items)
	}
	if !slices.IsSorted(items) {
		t.Errorf("items are not sorted: %v", items)
	}

	if _, _, err := run(t, "init", root, "--preset", "default-2"); err == nil {
		t.Error("init over an existing vault should fail")
	}
}

func TestInitWithoutWizardRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	for name, args := range map[string][]string{
		"unknown preset":   {"init", filepath.Join(dir, "a"), "--preset", "no-such-preset"},
		"unknown item":     {"init", filepath.Join(dir, "b"), "--preset", "default-2", "--add", "no-such-item"},
		"unknown platform": {"init", filepath.Join(dir, "c"), "--preset", "default-2", "--platform", "amiga"},
		"missing parent":   {"init", filepath.Join(dir, "missing", "d"), "--preset", "default-2"},
		"no path":          {"init", "--preset", "default-2"},
		"flag sans preset": {"init", filepath.Join(dir, "e"), "--apply"},
	} {
		if _, _, err := run(t, args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := manifest.Load(filepath.Join(dir, "b", "manifest.yaml")); err == nil {
		t.Error("a rejected init left a manifest behind")
	}
}

func TestAddValidatesIdsAndPullsDependencies(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, nil, "", "", nil); err != nil {
		t.Fatal(err)
	}
	mPath := filepath.Join(root, "manifest.yaml")

	if _, _, err := run(t, "add", "zimgit-knots", "no-such-item", "--vault", root); err == nil || !strings.Contains(err.Error(), "no-such-item") {
		t.Fatalf("add of an unknown id: err = %v", err)
	}
	if m, _ := manifest.Load(mPath); len(m.Desired.Items) != 0 {
		t.Fatalf("a rejected add changed the vault: %v", m.Desired.Items)
	}

	out, _, err := run(t, "add", "zimgit-knots", "--vault", root)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := manifest.Load(mPath)
	if !slices.Contains(m.Desired.Items, "zimgit-knots") || !slices.Contains(m.Desired.Items, "kiwix-serve") {
		t.Errorf("items = %v", m.Desired.Items)
	}
	if !strings.Contains(out, "Includes required: ") || !strings.Contains(out, "svalbard apply") {
		t.Errorf("add output:\n%s", out)
	}

	out, _, err = run(t, "add", "zimgit-knots", "--vault", root)
	if err != nil || !strings.Contains(out, "nothing added") {
		t.Errorf("repeated add: err = %v, out = %q", err, out)
	}
}

func TestRemoveReportsWhatChanged(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, []string{"kiwix-serve", "zimgit-knots"}, "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "remove", "cd3wd", "--vault", root); err == nil {
		t.Error("removing an unselected item should fail")
	}
	out, errOut, err := run(t, "remove", "kiwix-serve", "--vault", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Removed 1 item(s)") || !strings.Contains(errOut, "still need kiwix-serve") {
		t.Errorf("out = %q, stderr = %q", out, errOut)
	}
}

func TestAWithdrawnItemCanBeRemovedAndTakenBack(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, []string{"old-and-gone"}, "", "", nil); err != nil {
		t.Fatal(err)
	}
	mPath := filepath.Join(root, "manifest.yaml")
	m, _ := manifest.Load(mPath)
	m.Realized.Entries = []manifest.RealizedEntry{{ID: "old-and-gone", Type: "zim", RelativePath: "zim/gone.zim"}}
	if err := manifest.Save(mPath, m); err != nil {
		t.Fatal(err)
	}

	out, _, err := run(t, "update", "--vault", root)
	if err != nil || !strings.Contains(out, "No updates available") || !strings.Contains(out, "kept on this vault: old-and-gone") {
		t.Errorf("update preview: err = %v\n%s", err, out)
	}
	if _, _, err := run(t, "update", "old-and-gone", "--vault", root); err == nil {
		t.Error("updating a withdrawn item should fail")
	}
	_, errOut, err := run(t, "remove", "old-and-gone", "--vault", root)
	if err != nil || !strings.Contains(errOut, "cannot be downloaded again") {
		t.Errorf("remove: err = %v, stderr = %q", err, errOut)
	}
	if _, _, err := run(t, "add", "old-and-gone", "--vault", root); err != nil {
		t.Errorf("taking the removal back: %v", err)
	}
	if m, _ := manifest.Load(mPath); !slices.Contains(m.Desired.Items, "old-and-gone") {
		t.Errorf("items = %v", m.Desired.Items)
	}
}

func TestPlanStatusAndCatalogSpeakJSON(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, []string{"zimgit-knots"}, "", "", nil); err != nil {
		t.Fatal(err)
	}

	out, _, err := run(t, "plan", "--json", "--vault", root)
	if err != nil {
		t.Fatal(err)
	}
	var plan commands.PlanDetail
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("plan --json: %v\n%s", err, out)
	}
	if len(plan.Download) != 1 || plan.Download[0].ID != "zimgit-knots" || plan.DownloadGB <= 0 || plan.Remove == nil {
		t.Errorf("plan = %+v", plan)
	}
	if text, _, _ := run(t, "plan", "--vault", root); !strings.Contains(text, "+ zimgit-knots") || !strings.Contains(text, "svalbard apply") {
		t.Errorf("plan text:\n%s", text)
	}

	out, _, err = run(t, "status", "--json", "--vault", root)
	if err != nil {
		t.Fatal(err)
	}
	var status commands.StatusReport
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("status --json: %v\n%s", err, out)
	}
	if status.Pending != 1 || len(status.Items) != 1 || status.Items[0].Status != "pending" {
		t.Errorf("status = %+v", status)
	}

	out, _, err = run(t, "catalog", "--json", "zimgit", "knots")
	if err != nil {
		t.Fatal(err)
	}
	var entries []commands.CatalogEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("catalog --json: %v\n%s", err, out)
	}
	if len(entries) != 1 || entries[0].ID != "zimgit-knots" || entries[0].Packs == nil {
		t.Errorf("catalog entries = %+v", entries)
	}
	if _, _, err := run(t, "catalog", "no-such-thing-anywhere"); err == nil {
		t.Error("a catalog query with no match should fail")
	}

	out, _, err = run(t, "preset", "show", "default-2")
	if err != nil || !strings.Contains(out, "kiwix-serve") {
		t.Errorf("preset show: err = %v\n%s", err, out)
	}
	out, _, err = run(t, "preset", "list", "--json")
	var presets []commands.PresetSummary
	if err != nil || json.Unmarshal([]byte(out), &presets) != nil || len(presets) == 0 {
		t.Errorf("preset list --json: err = %v", err)
	}
}

func TestAnItemForOtherPlatformsIsSkippedNotPending(t *testing.T) {
	// toybox has Linux payloads only.
	mac := filepath.Join(t.TempDir(), "mac")
	if _, _, err := run(t, "init", mac, "--add", "toybox", "--platform", "macos-arm64"); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "plan", "--vault", mac, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var plan commands.PlanDetail
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Download) != 0 {
		t.Fatalf("plan downloads %v on a macOS-only vault", plan.Download)
	}
	out, _, err = run(t, "status", "--vault", mac, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var status commands.StatusReport
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(status.OtherPlatform, "toybox") || status.Pending != 0 {
		t.Fatalf("status = other_platform %v, pending %d; want toybox skipped and nothing pending", status.OtherPlatform, status.Pending)
	}
	text, _, _ := run(t, "status", "--vault", mac)
	if !strings.Contains(text, "not available for macos-arm64: toybox") {
		t.Errorf("status does not explain the skipped item:\n%s", text)
	}

	linux := filepath.Join(t.TempDir(), "linux")
	if _, _, err := run(t, "init", linux, "--add", "toybox", "--platform", "linux-x86_64"); err != nil {
		t.Fatal(err)
	}
	out, _, _ = run(t, "plan", "--vault", linux, "--json")
	if !strings.Contains(out, `"id": "toybox"`) {
		t.Errorf("plan on a Linux vault does not download toybox:\n%s", out)
	}
}
