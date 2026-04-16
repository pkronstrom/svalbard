package verify_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/verify"
)

func TestRunReportsPassedFailedAndMissingChecksums(t *testing.T) {
	driveRoot := t.TempDir()
	matchingPath := filepath.Join(driveRoot, "zim", "wikipedia.zim")
	failingPath := filepath.Join(driveRoot, "models", "gemma.gguf")
	if err := os.MkdirAll(filepath.Dir(matchingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(failingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(matchingPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile(matching) error = %v", err)
	}
	if err := os.WriteFile(failingPath, []byte("bad"), 0o644); err != nil {
		t.Fatalf("WriteFile(failing) error = %v", err)
	}

	checksumFile := filepath.Join(driveRoot, ".svalbard", "checksums.sha256")
	if err := os.MkdirAll(filepath.Dir(checksumFile), 0o755); err != nil {
		t.Fatalf("MkdirAll(.svalbard) error = %v", err)
	}
	content := strings.Join([]string{
		fmt.Sprintf("%x  zim/wikipedia.zim", sha256.Sum256([]byte("ok"))),
		fmt.Sprintf("%x  models/gemma.gguf", sha256.Sum256([]byte("expected-other"))),
		fmt.Sprintf("%x  data/missing.sqlite", sha256.Sum256([]byte("missing"))),
	}, "\n") + "\n"
	if err := os.WriteFile(checksumFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(checksums) error = %v", err)
	}

	var out bytes.Buffer
	if err := verify.Run(&out, driveRoot); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Verifying drive integrity",
		"OK       zim/wikipedia.zim",
		"FAIL     models/gemma.gguf",
		"MISSING  data/missing.sqlite",
		"Passed: 1  Failed: 1  Missing: 1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
