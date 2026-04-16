package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestFullCLIContractEndToEnd(t *testing.T) {
	// This test exercises the entire v2 CLI contract in order:
	// init -> add -> remove -> plan -> status -> preset list -> preset copy -> import -> index

	root := t.TempDir()

	// 1. INIT -- creates vault with preset
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"init", root, "--preset", "default-32"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	// Verify manifest.yaml exists
	manifestPath := filepath.Join(root, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest.yaml not created: %v", err)
	}

	// 2. ADD -- add an item to desired state
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"add", "extra-item", "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	// Verify item was added
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest after add: %v", err)
	}
	found := false
	for _, item := range m.Desired.Items {
		if item == "extra-item" {
			found = true
		}
	}
	if !found {
		t.Fatal("add did not add extra-item to desired state")
	}

	// 3. REMOVE -- remove the item we just added
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"remove", "extra-item", "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest after remove: %v", err)
	}
	for _, item := range m.Desired.Items {
		if item == "extra-item" {
			t.Fatal("remove did not remove extra-item")
		}
	}

	// 4. PLAN -- show what needs to happen
	var planBuf bytes.Buffer
	cmd = NewRootCommand()
	cmd.SetOut(&planBuf)
	cmd.SetArgs([]string{"plan", "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	planOutput := planBuf.String()
	if !strings.Contains(planOutput, "download") {
		t.Fatalf("plan output should mention downloads: %q", planOutput)
	}

	// 5. STATUS -- show current vault state
	var statusBuf bytes.Buffer
	cmd = NewRootCommand()
	cmd.SetOut(&statusBuf)
	cmd.SetArgs([]string{"status", "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(statusBuf.String(), "Desired") {
		t.Fatalf("status should show desired count: %q", statusBuf.String())
	}

	// 6. PRESET LIST
	var presetBuf bytes.Buffer
	cmd = NewRootCommand()
	cmd.SetOut(&presetBuf)
	cmd.SetArgs([]string{"preset", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preset list failed: %v", err)
	}
	if !strings.Contains(presetBuf.String(), "default-32") {
		t.Fatalf("preset list should contain default-32: %q", presetBuf.String())
	}

	// 7. PRESET COPY
	copyTarget := filepath.Join(t.TempDir(), "custom.yaml")
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"preset", "copy", "default-32", copyTarget})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preset copy failed: %v", err)
	}
	if _, err := os.Stat(copyTarget); err != nil {
		t.Fatalf("preset copy did not create target file: %v", err)
	}

	// 8. IMPORT
	importFile := filepath.Join(t.TempDir(), "manual.pdf")
	if err := os.WriteFile(importFile, []byte("pdf content"), 0644); err != nil {
		t.Fatalf("creating import file: %v", err)
	}
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"import", importFile, "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// 9. INDEX (after creating a fake zim dir)
	zimDir := filepath.Join(root, "zim")
	if err := os.MkdirAll(zimDir, 0755); err != nil {
		t.Fatalf("creating zim dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zimDir, "test.zim"), []byte("fake"), 0644); err != nil {
		t.Fatalf("creating test zim file: %v", err)
	}
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"index", "--vault", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); err != nil {
		t.Fatalf("index did not create search.db: %v", err)
	}

	t.Log("Full CLI contract verified: init, add, remove, plan, status, preset list, preset copy, import, index -- all pass")
}
