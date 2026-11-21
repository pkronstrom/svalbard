package commands

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withConfiguredIndexSeams(t *testing.T, keyword func(context.Context, string, bool, io.Writer, func(IndexProgress)) error, semantic func(context.Context, string, bool, io.Writer, func(SemanticProgress)) error) {
	t.Helper()
	oldKeyword, oldSemantic := runConfiguredKeyword, runConfiguredSemantic
	runConfiguredKeyword, runConfiguredSemantic = keyword, semantic
	t.Cleanup(func() { runConfiguredKeyword, runConfiguredSemantic = oldKeyword, oldSemantic })
}

func TestRunConfiguredIndexNoneDoesNotCreateDatabase(t *testing.T) {
	root := t.TempDir()
	if err := RunConfiguredIndex(context.Background(), root, "none", io.Discard, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); !os.IsNotExist(err) {
		t.Fatalf("none created search database: %v", err)
	}
}

func TestRunConfiguredIndexEmptyStrategyReconcilesEmptySet(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RunConfiguredIndex(context.Background(), root, "", io.Discard, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); err != nil {
		t.Fatalf("empty automatic strategy did not reconcile: %v", err)
	}
}

func TestRunConfiguredIndexRejectsUnknownStrategy(t *testing.T) {
	err := RunConfiguredIndex(context.Background(), t.TempDir(), "surprise", io.Discard, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown index strategy") {
		t.Fatalf("unknown strategy error = %v", err)
	}
}

func TestRunConfiguredIndexFullRunsKeywordThenSemanticOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	var calls []string
	withConfiguredIndexSeams(t,
		func(context.Context, string, bool, io.Writer, func(IndexProgress)) error {
			calls = append(calls, "keyword")
			return nil
		},
		func(context.Context, string, bool, io.Writer, func(SemanticProgress)) error {
			calls = append(calls, "semantic")
			return nil
		},
	)
	if err := RunConfiguredIndex(context.Background(), root, "full", io.Discard, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "keyword,semantic" {
		t.Fatalf("configured order = %q", got)
	}
}

func TestRunConfiguredIndexSemanticFailurePreservesKeywordResult(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	keywordDone := false
	withConfiguredIndexSeams(t,
		func(context.Context, string, bool, io.Writer, func(IndexProgress)) error {
			keywordDone = true
			return nil
		},
		func(context.Context, string, bool, io.Writer, func(SemanticProgress)) error {
			return errors.New("model missing")
		},
	)
	err := RunConfiguredIndex(context.Background(), root, "semantic", io.Discard, nil, nil)
	if !keywordDone || err == nil || !strings.Contains(err.Error(), "semantic indexing after keyword indexing") {
		t.Fatalf("keywordDone=%v, err=%v", keywordDone, err)
	}
}

func TestConfiguredIndexFreshToolOnlyVaultNeedsNoZIMDirectory(t *testing.T) {
	root := t.TempDir()
	data := []byte("version: 2\nrealized:\n  entries:\n    - id: platformio-core\n      type: python-package\n")
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RunConfiguredIndex(context.Background(), root, "", io.Discard, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); !os.IsNotExist(err) {
		t.Fatal("created unneeded search database")
	}
}

func TestConfiguredIndexMissingZIMDirectoryPreservesExistingIndex(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "data"), 0755)
	os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte("version: 2\nrealized:\n  entries: []\n"), 0600)
	original := []byte("last successful search database")
	os.WriteFile(filepath.Join(root, "data", "search.db"), original, 0600)
	if err := RunConfiguredIndex(context.Background(), root, "", io.Discard, nil, nil); err == nil {
		t.Fatal("missing archive directory accepted over existing index")
	}
	got, _ := os.ReadFile(filepath.Join(root, "data", "search.db"))
	if string(got) != string(original) {
		t.Fatal("existing index altered")
	}
}
